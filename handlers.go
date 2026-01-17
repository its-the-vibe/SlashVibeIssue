package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"

	"github.com/redis/go-redis/v9"
	"github.com/slack-go/slack"
)

const (
	issueClosedReactionEmoji = "cat2"
	issueClosedTTLSeconds    = 86400 // 24 hours
	issueCreatedEventType    = "issue_created"
)

func subscribeToSlashCommands(ctx context.Context, rdb *redis.Client, slackClient *slack.Client, config Config) {
	pubsub := rdb.Subscribe(ctx, config.RedisChannel)
	defer pubsub.Close()

	log.Printf("Subscribed to Redis channel: %s", config.RedisChannel)

	ch := pubsub.Channel()
	for {
		select {
		case <-ctx.Done():
			return
		case msg := <-ch:
			if msg == nil {
				continue
			}
			handleSlashCommand(ctx, slackClient, msg.Payload, config)
		}
	}
}

func handleSlashCommand(ctx context.Context, slackClient *slack.Client, payload string, config Config) {
	var cmd SlackCommand
	if err := json.Unmarshal([]byte(payload), &cmd); err != nil {
		log.Printf("Error unmarshaling slash command: %v", err)
		return
	}

	// Only handle /issue command
	if cmd.Command != "/issue" {
		return
	}

	log.Printf("Received /issue command from user %s", cmd.UserName)

	// Check if the text is the sparkles emoji for setup-ai command
	text := strings.TrimSpace(cmd.Text)
	var initialTitle, initialDescription string
	var preselectCopilot bool

	if text == ":sparkles:" {
		initialTitle = "✨ Set up Copilot instructions"
		initialDescription = "Configure instructions for this repository as documented in [Best practices for Copilot coding agent in your repository](https://gh.io/copilot-coding-agent-tips).\n\n<Onboard this repo>"
		preselectCopilot = true
	} else {
		initialTitle = text
		initialDescription = ""
		preselectCopilot = false
	}

	// Open modal with pre-populated values
	modal := createIssueModal(initialTitle, initialDescription, preselectCopilot)
	_, err := slackClient.OpenView(cmd.TriggerID, modal)
	if err != nil {
		log.Printf("Error opening modal: %v", err)
		return
	}

	log.Println("Modal opened successfully")
}

func subscribeToViewSubmissions(ctx context.Context, rdb *redis.Client, slackClient *slack.Client, config Config) {
	pubsub := rdb.Subscribe(ctx, config.RedisViewSubmissionChannel)
	defer pubsub.Close()

	log.Printf("Subscribed to Redis channel: %s", config.RedisViewSubmissionChannel)

	ch := pubsub.Channel()
	for {
		select {
		case <-ctx.Done():
			return
		case msg := <-ch:
			if msg == nil {
				continue
			}
			handleViewSubmission(ctx, rdb, slackClient, msg.Payload, config)
		}
	}
}

func handleViewSubmission(ctx context.Context, rdb *redis.Client, slackClient *slack.Client, payload string, config Config) {
	var submission ViewSubmission
	if err := json.Unmarshal([]byte(payload), &submission); err != nil {
		log.Printf("Error unmarshaling view submission: %v", err)
		return
	}

	// Only handle our specific callback_id
	if submission.View.CallbackID != "create_github_issue_modal" {
		return
	}

	log.Printf("Received view submission from user %s", submission.User.Username)

	// Extract values from the submission
	values := submission.View.State.Values

	// Get repository
	var repo string
	if repoBlock, ok := values["repo_selection_block"]; ok {
		if repoData, ok := repoBlock["SlashVibeIssue"]; ok {
			if repoMap, ok := repoData.(map[string]interface{}); ok {
				if selectedOption, ok := repoMap["selected_option"].(map[string]interface{}); ok {
					if value, ok := selectedOption["value"].(string); ok {
						repo = value
					}
				}
			}
		}
	}

	// Get title
	var title string
	if titleBlock, ok := values["title_block"]; ok {
		if titleData, ok := titleBlock["issue_title"]; ok {
			if titleMap, ok := titleData.(map[string]interface{}); ok {
				if value, ok := titleMap["value"].(string); ok {
					title = value
				}
			}
		}
	}

	// Get description
	var description string
	if descBlock, ok := values["description_block"]; ok {
		if descData, ok := descBlock["issue_description"]; ok {
			if descMap, ok := descData.(map[string]interface{}); ok {
				if value, ok := descMap["value"].(string); ok {
					description = value
				}
			}
		}
	}

	// Check if copilot assignment is selected
	assignToCopilot := false
	if assignBlock, ok := values["assignment_block"]; ok {
		if assignData, ok := assignBlock["assign_copilot"]; ok {
			if assignMap, ok := assignData.(map[string]interface{}); ok {
				if selectedOptions, ok := assignMap["selected_options"].([]interface{}); ok {
					assignToCopilot = len(selectedOptions) > 0
				}
			}
		}
	}

	// Check if add to project is selected
	addToProject := false
	if assignBlock, ok := values["assignment_block"]; ok {
		if projectData, ok := assignBlock["add_to_project"]; ok {
			if projectMap, ok := projectData.(map[string]interface{}); ok {
				if selectedOptions, ok := projectMap["selected_options"].([]interface{}); ok {
					addToProject = len(selectedOptions) > 0
				}
			}
		}
	}

	if repo == "" || title == "" {
		log.Println("Missing required fields: repo or title")
		return
	}

	// Create GitHub issue via Poppit
	err := createGitHubIssue(ctx, rdb, repo, title, description, assignToCopilot, addToProject, submission.User.Username, config)
	if err != nil {
		log.Printf("Error creating GitHub issue: %v", err)
		return
	}

	log.Printf("GitHub issue creation command sent to Poppit for repo: %s/%s", config.GitHubOrg, repo)
}

func subscribeToPoppitOutput(ctx context.Context, rdb *redis.Client, slackClient *slack.Client, config Config) {
	pubsub := rdb.Subscribe(ctx, config.RedisPoppitOutputChannel)
	defer pubsub.Close()

	log.Printf("Subscribed to Redis channel: %s", config.RedisPoppitOutputChannel)

	ch := pubsub.Channel()
	for {
		select {
		case <-ctx.Done():
			return
		case msg := <-ch:
			if msg == nil {
				continue
			}
			handlePoppitOutput(ctx, rdb, slackClient, msg.Payload, config)
		}
	}
}

func handlePoppitOutput(ctx context.Context, rdb *redis.Client, slackClient *slack.Client, payload string, config Config) {
	var output PoppitOutput
	if err := json.Unmarshal([]byte(payload), &output); err != nil {
		log.Printf("Error unmarshaling Poppit output: %v", err)
		return
	}

	// Handle title generation output
	if output.Type == "slash-vibe-issue-ticket-title" {
		handleTitleGenerationOutput(ctx, slackClient, output, config)
		return
	}

	// Only handle slash-vibe-issue type
	if output.Type != "slash-vibe-issue" {
		return
	}

	log.Printf("Received Poppit output for slash-vibe-issue")

	// Extract metadata
	metadata := output.Metadata
	if metadata == nil {
		log.Printf("No metadata in Poppit output")
		return
	}

	repo, _ := metadata["repo"].(string)
	title, _ := metadata["title"].(string)
	username, _ := metadata["username"].(string)
	assignedToCopilot, _ := metadata["assignedToCopilot"].(bool)

	if repo == "" || title == "" || username == "" {
		log.Printf("Missing required metadata: repo=%s, title=%s, username=%s", repo, title, username)
		return
	}

	// Only process output from "gh issue create" commands
	if !strings.HasPrefix(output.Command, "gh issue create") {
		log.Printf("Ignoring non-issue-create command: %s", output.Command)
		return
	}

	// Parse issue URL from output
	issueURL := extractIssueURL(output.Output)
	if issueURL == "" {
		log.Printf("Failed to extract issue URL from output: %s", output.Output)
		return
	}

	log.Printf("Extracted issue URL: %s", issueURL)

	// Check if we should add to project
	addToProject, _ := metadata["addToProject"].(bool)
	if addToProject {
		log.Printf("Adding issue to project")
		err := addIssueToProject(ctx, rdb, issueURL, config)
		if err != nil {
			log.Printf("Error adding issue to project: %v", err)
		}
	}

	// Send confirmation message with issue URL
	sendConfirmation(ctx, rdb, repo, title, username, issueURL, assignedToCopilot, config)
}

func subscribeToReactions(ctx context.Context, rdb *redis.Client, slackClient *slack.Client, config Config) {
	pubsub := rdb.Subscribe(ctx, config.RedisReactionChannel)
	defer pubsub.Close()

	log.Printf("Subscribed to Redis channel: %s", config.RedisReactionChannel)

	ch := pubsub.Channel()
	for {
		select {
		case <-ctx.Done():
			return
		case msg := <-ch:
			if msg == nil {
				continue
			}
			handleReactionAdded(ctx, rdb, slackClient, msg.Payload, config)
		}
	}
}

func handleReactionAdded(ctx context.Context, rdb *redis.Client, slackClient *slack.Client, payload string, config Config) {
	var reaction ReactionAddedEvent
	if err := json.Unmarshal([]byte(payload), &reaction); err != nil {
		log.Printf("Error unmarshaling reaction event: %v", err)
		return
	}

	// Only handle reaction_added events
	if reaction.Event.Type != "reaction_added" {
		return
	}

	// Ignore reactions from bots
	for _, auth := range reaction.Authorizations {
		if auth.IsBot && auth.UserID == reaction.Event.User {
			log.Printf("Ignoring reaction from bot user: %s", reaction.Event.User)
			return
		}
	}

	// Only handle sparkles emoji
	if reaction.Event.Reaction != "sparkles" {
		return
	}

	// Only handle message reactions
	if reaction.Event.Item.Type != "message" {
		return
	}

	log.Printf("Received sparkles reaction from user %s on message %s", reaction.Event.User, reaction.Event.Item.Ts)

	// Fetch the message from Slack to get metadata
	historyParams := &slack.GetConversationHistoryParameters{
		ChannelID:          reaction.Event.Item.Channel,
		Latest:             reaction.Event.Item.Ts,
		Limit:              1,
		Inclusive:          true,
		IncludeAllMetadata: true,
	}

	history, err := slackClient.GetConversationHistory(historyParams)
	if err != nil {
		log.Printf("Error fetching message from Slack: %v", err)
		return
	}

	if len(history.Messages) == 0 {
		log.Printf("No message found for timestamp: %s", reaction.Event.Item.Ts)
		return
	}

	message := history.Messages[0]

	// Check if message has metadata
	if message.Metadata.EventType == "" {
		log.Printf("Message has no metadata, ignoring reaction")
		return
	}

	// Parse metadata
	var metadata MessageMetadata
	metadata.EventType = message.Metadata.EventType

	// Convert EventPayload to map
	if payloadBytes, err := json.Marshal(message.Metadata.EventPayload); err == nil {
		if err := json.Unmarshal(payloadBytes, &metadata.EventPayload); err != nil {
			log.Printf("Error unmarshaling event payload: %v", err)
			return
		}
	} else {
		log.Printf("Error marshaling event payload: %v", err)
		return
	}

	// Check if it's an issue_created event
	if metadata.EventType != issueCreatedEventType {
		log.Printf("Event type is not issue_created: %s", metadata.EventType)
		return
	}

	// Extract issue data from metadata
	issueURL, _ := metadata.EventPayload["issue_url"].(string)
	repository, _ := metadata.EventPayload["repository"].(string)
	assignedToCopilot, _ := metadata.EventPayload["assignedToCopilot"].(bool)

	if issueURL == "" {
		log.Printf("Missing issue_url in metadata")
		return
	}

	if assignedToCopilot {
		log.Printf("Issue already assigned to Copilot, ignoring reaction: %s", issueURL)
		return
	}

	log.Printf("Assigning issue to Copilot: %s", issueURL)

	// Assign issue to Copilot
	err = assignIssueToCopilot(ctx, rdb, issueURL, repository, config)
	if err != nil {
		log.Printf("Error assigning issue to Copilot: %v", err)
		return
	}

	log.Printf("Successfully assigned issue to Copilot: %s", issueURL)
}

func subscribeToGitHubWebhooks(ctx context.Context, rdb *redis.Client, slackClient *slack.Client, config Config) {
	pubsub := rdb.Subscribe(ctx, config.RedisGitHubWebhookChannel)
	defer pubsub.Close()

	log.Printf("Subscribed to Redis channel: %s", config.RedisGitHubWebhookChannel)

	ch := pubsub.Channel()
	for {
		select {
		case <-ctx.Done():
			return
		case msg := <-ch:
			if msg == nil {
				continue
			}
			handleGitHubIssueEvent(ctx, rdb, slackClient, msg.Payload, config)
		}
	}
}

func handleGitHubIssueEvent(ctx context.Context, rdb *redis.Client, slackClient *slack.Client, payload string, config Config) {
	var event GitHubWebhookEvent
	if err := json.Unmarshal([]byte(payload), &event); err != nil {
		log.Printf("Error unmarshaling GitHub webhook event: %v", err)
		return
	}

	// Only handle issue closed events
	if event.Action != "closed" {
		return
	}

	log.Printf("Received issue closed event for issue #%d: %s", event.Issue.Number, event.Issue.Title)

	// Transform API URL to web URL
	// From: https://api.github.com/repos/its-the-vibe/SlashVibeIssue/issues/13
	// To: https://github.com/its-the-vibe/SlashVibeIssue/issues/13
	issueURL := transformAPIURLToWebURL(event.Issue.URL)
	if issueURL == "" {
		log.Printf("Failed to transform API URL to web URL: %s", event.Issue.URL)
		return
	}

	log.Printf("Transformed issue URL: %s", issueURL)

	// Search for the message with matching metadata
	channelID, messageTs, err := findMessageByIssueURL(ctx, slackClient, issueURL, config)
	if err != nil {
		log.Printf("Error finding message by issue URL: %v", err)
		return
	}

	if channelID == "" || messageTs == "" {
		log.Printf("No message found for issue URL: %s", issueURL)
		return
	}

	log.Printf("Found message for issue %s at channel=%s, ts=%s", issueURL, channelID, messageTs)

	// Send reaction to SlackLiner
	err = sendReactionToSlackLiner(ctx, rdb, issueClosedReactionEmoji, channelID, messageTs, config)
	if err != nil {
		log.Printf("Error sending reaction: %v", err)
		return
	}

	log.Printf("Sent %s reaction for message ts=%s", issueClosedReactionEmoji, messageTs)

	// Set TTL to 24 hours
	err = sendTTLToTimeBomb(ctx, rdb, channelID, messageTs, issueClosedTTLSeconds, config)
	if err != nil {
		log.Printf("Error setting TTL: %v", err)
		return
	}

	log.Printf("Set TTL to 24 hours for message ts=%s", messageTs)
}

func transformAPIURLToWebURL(apiURL string) string {
	// Transform from: https://api.github.com/repos/its-the-vibe/SlashVibeIssue/issues/13
	// To: https://github.com/its-the-vibe/SlashVibeIssue/issues/13
	if !strings.HasPrefix(apiURL, "https://api.github.com/repos/") {
		return ""
	}

	// Remove the "https://api.github.com/repos/" prefix
	path := strings.TrimPrefix(apiURL, "https://api.github.com/repos/")

	// Build the web URL
	return "https://github.com/" + path
}

func findMessageByIssueURL(ctx context.Context, slackClient *slack.Client, issueURL string, config Config) (string, string, error) {
	// Use the channel ID directly from config
	if config.ConfirmationChannelID == "" {
		return "", "", fmt.Errorf("confirmation channel ID not configured")
	}

	// Search through recent messages in the confirmation channel
	// Only search the most recent messages up to the configured limit
	historyParams := &slack.GetConversationHistoryParameters{
		ChannelID:          config.ConfirmationChannelID,
		Limit:              config.ConfirmationSearchLimit,
		IncludeAllMetadata: true,
	}

	history, err := slackClient.GetConversationHistory(historyParams)
	if err != nil {
		return "", "", fmt.Errorf("failed to get conversation history: %v", err)
	}

	// Search through messages for matching metadata
	for _, message := range history.Messages {
		if message.Metadata.EventType == issueCreatedEventType {
			// Check for matching issue URL directly from EventPayload
			if msgIssueURL, ok := message.Metadata.EventPayload["issue_url"].(string); ok {
				if msgIssueURL == issueURL {
					return config.ConfirmationChannelID, message.Timestamp, nil
				}
			}
		}
	}

	// Message not found in the recent messages
	return "", "", nil
}

func sendReactionToSlackLiner(ctx context.Context, rdb *redis.Client, reaction, channel, ts string, config Config) error {
	slackReaction := SlackReaction{
		Reaction: reaction,
		Channel:  channel,
		Ts:       ts,
	}

	payload, err := json.Marshal(slackReaction)
	if err != nil {
		return fmt.Errorf("failed to marshal slack reaction: %v", err)
	}

	err = rdb.RPush(ctx, config.RedisSlackReactionsList, payload).Err()
	if err != nil {
		return fmt.Errorf("failed to push reaction to Redis list: %v", err)
	}

	return nil
}

func sendTTLToTimeBomb(ctx context.Context, rdb *redis.Client, channel, ts string, ttl int, config Config) error {
	timeBombMsg := TimeBombMessage{
		Channel: channel,
		Ts:      ts,
		TTL:     ttl,
	}

	payload, err := json.Marshal(timeBombMsg)
	if err != nil {
		return fmt.Errorf("failed to marshal timebomb message: %v", err)
	}

	err = rdb.Publish(ctx, config.RedisTimeBombChannel, payload).Err()
	if err != nil {
		return fmt.Errorf("failed to publish to timebomb channel: %v", err)
	}

	return nil
}

func subscribeToMessageActions(ctx context.Context, rdb *redis.Client, slackClient *slack.Client, config Config) {
	pubsub := rdb.Subscribe(ctx, config.RedisMessageActionChannel)
	defer pubsub.Close()

	log.Printf("Subscribed to Redis channel: %s", config.RedisMessageActionChannel)

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
		log.Printf("Error unmarshaling message action: %v", err)
		return
	}

	// Only handle message_action type with callback_id "create_github_issue"
	if action.Type != "message_action" {
		return
	}

	if action.CallbackID != "create_github_issue" {
		return
	}

	log.Printf("Received create_github_issue message action from user %s", action.User.Username)

	// Get the message text
	messageText := action.Message.Text
	if messageText == "" {
		log.Printf("Message has no text, ignoring action")
		return
	}

	log.Printf("Opening modal with loading state for message text (length: %d)", len(messageText))

	// Open modal immediately with loading state to avoid trigger_id expiration
	loadingModal := createIssueModal("⏳ Generating title...", messageText, false)
	viewResponse, err := slackClient.OpenView(action.TriggerID, loadingModal)
	if err != nil {
		log.Printf("Error opening modal: %v", err)
		return
	}

	log.Printf("Modal opened successfully with view_id: %s", viewResponse.ID)

	// Send command to Poppit to generate title with view_id for later update
	err = generateIssueTitleViaCopilot(ctx, rdb, messageText, action.User.Username, viewResponse.ID, config)
	if err != nil {
		log.Printf("Error generating issue title: %v", err)
		return
	}

	log.Printf("Title generation command sent to Poppit for user: %s", action.User.Username)
}

func generateIssueTitleViaCopilot(ctx context.Context, rdb *redis.Client, messageBody, username, viewID string, config Config) error {
	// Escape single quotes in the message body for shell command
	escapedMessage := strings.ReplaceAll(messageBody, `'`, `'\''`)

	// Build the copilot command
	copilotCmd := fmt.Sprintf("copilot --model gpt-4.1 --agent issue-summariser --prompt '%s'", escapedMessage)

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
	log.Printf("Received Poppit output for title generation")

	// Extract metadata
	metadata := output.Metadata
	if metadata == nil {
		log.Printf("No metadata in Poppit output")
		return
	}

	username, _ := metadata["username"].(string)
	viewID, _ := metadata["view_id"].(string)

	if username == "" {
		log.Printf("Missing username in metadata")
		return
	}

	if viewID == "" {
		log.Printf("Missing view_id in metadata")
		return
	}

	// Parse the JSON output
	var titleOutput TitleGenerationOutput
	if err := json.Unmarshal([]byte(output.Output), &titleOutput); err != nil {
		log.Printf("Error unmarshaling title generation output: %v", err)
		return
	}

	if titleOutput.Title == "" {
		log.Printf("Generated title is empty")
		return
	}

	log.Printf("Generated title for user %s: %s", username, titleOutput.Title)

	// Update modal with generated title and description
	updatedModal := createIssueModal(titleOutput.Title, titleOutput.Prompt, false)
	_, err := slackClient.UpdateView(updatedModal, "", "", viewID)
	if err != nil {
		log.Printf("Error updating modal: %v", err)
		return
	}

	log.Printf("Modal updated successfully for user %s", username)
}
