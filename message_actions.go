package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/redis/go-redis/v9"
	"github.com/slack-go/slack"
)

func subscribeToMessageActions(ctx context.Context, rdb *redis.Client, slackClient *slack.Client, config Config) {
	pubsub := rdb.Subscribe(ctx, config.RedisMessageActionChannel)
	defer pubsub.Close()

	Info("Subscribed to Redis channel: %s", config.RedisMessageActionChannel)

	ch := pubsub.Channel()
	for {
		select {
		case <-ctx.Done():
			return
		case msg := <-ch:
			if msg == nil {
				continue
			}
			handleMessageAction(ctx, rdb, slackClient, msg.Payload, config)
		}
	}
}

func handleMessageAction(ctx context.Context, rdb *redis.Client, slackClient *slack.Client, payload string, config Config) {
	var action MessageActionEvent
	if err := json.Unmarshal([]byte(payload), &action); err != nil {
		Error("Error unmarshaling message action: %v", err)
		return
	}

	// Only handle message_action type with callback_id "create_github_issue"
	if action.Type != "message_action" {
		return
	}

	if action.CallbackID != "create_github_issue" {
		return
	}

	Debug("Received create_github_issue message action from user %s", action.User.Username)

	// Get the message text
	messageText := action.Message.Text
	if messageText == "" {
		Warn("Message has no text, ignoring action")
		return
	}

	Debug("Opening modal with loading state for message text (length: %d)", len(messageText))

	// Open modal immediately with loading state to avoid trigger_id expiration
	// loadingModal := createIssueModal("⏳ Generating title...", messageText, false)
	// NOTE: leaving blank otherwise Slack does not seem to update
	loadingModal := createIssueModal("", messageText, false)
	viewResponse, err := slackClient.OpenView(action.TriggerID, loadingModal)
	if err != nil {
		Error("Error opening modal: %v", err)
		return
	}

	Debug("Modal opened successfully with view_id: %s", viewResponse.ID)

	// Send command to Poppit to generate title with view_id for later update
	err = generateIssueTitleViaCopilot(ctx, rdb, messageText, action.User.Username, viewResponse.ID, viewResponse.Hash, config)
	if err != nil {
		Error("Error generating issue title: %v", err)
		return
	}

	Debug("Title generation command sent to Poppit for user: %s", action.User.Username)
}

func generateIssueTitleViaCopilot(ctx context.Context, rdb *redis.Client, messageBody, username, viewID string, hash string, config Config) error {
	// Escape single quotes in the message for shell command
	escapedMessage := strings.ReplaceAll(messageBody, `'`, `'\''`)

	// Build the issue-summariser command with the message as argument
	copilotCmd := fmt.Sprintf("issue-summariser '%s'", escapedMessage)

	// Create Poppit command message with metadata including view_id
	poppitCmd := PoppitCommand{
		Repo:     fmt.Sprintf("%s/SlashVibeIssue", config.GitHubOrg),
		Branch:   "refs/heads/main",
		Type:     "slash-vibe-issue-ticket-title",
		Dir:      config.AgentWorkingDir,
		Commands: []string{copilotCmd},
		Metadata: map[string]interface{}{
			"username": username,
			"view_id":  viewID,
			"hash":     hash,
		},
	}

	payload, err := json.Marshal(poppitCmd)
	if err != nil {
		return fmt.Errorf("failed to marshal Poppit command: %v", err)
	}

	// Push command to Poppit list
	err = rdb.RPush(ctx, config.RedisPoppitList, payload).Err()
	if err != nil {
		return fmt.Errorf("failed to push command to Poppit: %v", err)
	}

	return nil
}

func handleTitleGenerationOutput(ctx context.Context, slackClient *slack.Client, output PoppitOutput, config Config) {
	Debug("Received Poppit output for title generation")

	// Extract metadata
	metadata := output.Metadata
	if metadata == nil {
		Warn("No metadata in Poppit output")
		return
	}

	username, _ := metadata["username"].(string)
	viewID, _ := metadata["view_id"].(string)
	hash, _ := metadata["hash"].(string)

	if username == "" {
		Warn("Missing username in metadata")
		return
	}

	if viewID == "" {
		Warn("Missing view_id in metadata")
		return
	}

	if hash == "" {
		Warn("Missing hash in metadata")
		return
	}

	// Parse the JSON output
	var titleOutput TitleGenerationOutput
	if err := json.Unmarshal([]byte(output.Output), &titleOutput); err != nil {
		Error("Error unmarshaling title generation output: %v", err)
		return
	}

	if titleOutput.Title == "" {
		Warn("Generated title is empty")
		return
	}

	Info("Generated title for user %s: %s", username, titleOutput.Title)

	// Update modal with generated title and description
	updatedModal := createIssueModal(titleOutput.Title, titleOutput.Prompt, false)

	// NOTE: not using hash
	viewResp, err := slackClient.UpdateView(updatedModal, "", "", viewID)
	if err != nil {
		Error("Error updating modal: %v", err)
		return
	}
	// Optionally log the response for debugging
	if viewResp != nil {
		Debug("Slack API UpdateView response: %+v", viewResp)
	}

	Debug("Modal updated successfully for user %s", username)
}

func handleIssueSanitisationOutput(ctx context.Context, rdb *redis.Client, slackClient *slack.Client, output PoppitOutput, config Config) {
	Debug("Received Poppit output for issue sanitisation")

	// Extract metadata
	metadata := output.Metadata
	if metadata == nil {
		Warn("No metadata in Poppit output for issue sanitisation")
		return
	}

	issueURL, ok := metadata["issueURL"].(string)
	if !ok {
		Warn("issueURL in metadata is not a string for issue sanitisation")
		return
	}
	if issueURL == "" {
		Warn("issueURL in metadata is empty for issue sanitisation")
		return
	}

	Info("Issue sanitisation completed for: %s", issueURL)

	// Check if we need to assign to Copilot after sanitisation
	deferCopilotAssignment, _ := metadata["deferCopilotAssignment"].(bool)
	if deferCopilotAssignment {
		repository, _ := metadata["repository"].(string)
		if repository == "" {
			Warn("Repository metadata missing for deferred Copilot assignment")
		} else {
			Info("Assigning issue to Copilot after sanitisation: %s", issueURL)
			err := assignIssueToCopilot(ctx, rdb, issueURL, repository, config)
			if err != nil {
				Error("Error assigning issue to Copilot after sanitisation: %v", err)
			} else {
				Info("Successfully assigned issue to Copilot after sanitisation: %s", issueURL)
			}
		}
	}

	// Find the confirmation message with matching issue URL
	channelID, messageTs, err := findMessageByIssueURL(ctx, slackClient, issueURL, config)
	if err != nil {
		Error("Error finding message by issue URL: %v", err)
		return
	}

	if channelID == "" || messageTs == "" {
		Debug("No message found for issue URL: %s", issueURL)
		return
	}

	Debug("Found message for issue %s at channel=%s, ts=%s", issueURL, channelID, messageTs)

	// Remove :brain: reaction
	err = removeReactionFromSlackLiner(ctx, rdb, issueSanitisingReactionEmoji, channelID, messageTs, config)
	if err != nil {
		Error("Error removing brain reaction: %v", err)
	} else {
		Debug("Removed %s reaction for sanitised issue", issueSanitisingReactionEmoji)
	}

	// Send :ticket: reaction to SlackLiner
	err = sendReactionToSlackLiner(ctx, rdb, issueSanitisedReactionEmoji, channelID, messageTs, config)
	if err != nil {
		Error("Error sending reaction: %v", err)
		return
	}

	Info("Sent %s reaction for sanitised issue: %s", issueSanitisedReactionEmoji, issueURL)
}
