package main

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/redis/go-redis/v9"
	"github.com/slack-go/slack"
)

func subscribeToPoppitOutput(ctx context.Context, rdb *redis.Client, slackClient *slack.Client, config Config) {
	pubsub := rdb.Subscribe(ctx, config.RedisPoppitOutputChannel)
	defer pubsub.Close()

	Info("Subscribed to Redis channel: %s", config.RedisPoppitOutputChannel)

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
		Error("Error unmarshaling Poppit output: %v", err)
		return
	}

	// Handle title generation output
	if output.Type == "slash-vibe-issue-ticket-title" {
		handleTitleGenerationOutput(ctx, slackClient, output, config)
		return
	}

	// Handle issue sanitisation output
	if output.Type == "slash-vibe-issue-sanitise" {
		handleIssueSanitisationOutput(ctx, rdb, slackClient, output, config)
		return
	}

	// Only handle slash-vibe-issue type
	if output.Type != "slash-vibe-issue" {
		return
	}

	Debug("Received Poppit output for slash-vibe-issue")

	// Extract metadata
	metadata := output.Metadata
	if metadata == nil {
		Warn("No metadata in Poppit output")
		return
	}

	repo, _ := metadata["repo"].(string)
	title, _ := metadata["title"].(string)
	username, _ := metadata["username"].(string)
	assignedToCopilot, _ := metadata["assignedToCopilot"].(bool)
	shouldSanitiseIssue, _ := metadata["sanitiseIssue"].(bool)
	deferCopilotAssignment, _ := metadata["deferCopilotAssignment"].(bool)

	if repo == "" || title == "" || username == "" {
		Warn("Missing required metadata: repo=%s, title=%s, username=%s", repo, title, username)
		return
	}

	// Only process output from "gh issue create" commands
	if !strings.HasPrefix(output.Command, "gh issue create") {
		Debug("Ignoring non-issue-create command: %s", output.Command)
		return
	}

	// Parse issue URL from output
	issueURL := extractIssueURL(output.Output)
	if issueURL == "" {
		Error("Failed to extract issue URL from output: %s", output.Output)
		return
	}

	Info("Extracted issue URL: %s", issueURL)

	// Check if we should add to project
	addToProject, _ := metadata["addToProject"].(bool)
	if addToProject {
		Debug("Adding issue to project")
		err := addIssueToProject(ctx, rdb, issueURL, config)
		if err != nil {
			Error("Error adding issue to project: %v", err)
		}
	}

	// Check if we should sanitise the issue
	// Only sanitise if not already assigned to Copilot (assignedToCopilot is false when deferring)
	if shouldSanitiseIssue && !assignedToCopilot {
		Debug("Triggering automatic issue sanitisation")

		// Send the confirmation message first via HTTP so we get the channel and ts
		// back synchronously, then immediately add the :brain: reaction to it.
		if config.SlackLinerURL != "" {
			channelID, messageTs, httpErr := sendConfirmationHTTP(ctx, repo, title, username, issueURL, assignedToCopilot, config)
			if httpErr != nil {
				Error("Error sending confirmation via HTTP: %v", httpErr)
			} else if channelID != "" && messageTs != "" {
				// Add :brain: reaction to indicate sanitisation is starting
				reactionErr := sendReactionToSlackLiner(ctx, rdb, issueSanitisingReactionEmoji, channelID, messageTs, config)
				if reactionErr != nil {
					Error("Error sending brain reaction: %v", reactionErr)
				} else {
					Debug("Sent %s reaction for sanitisation start", issueSanitisingReactionEmoji)
				}
			}

			// Trigger issue sanitisation
			err := sanitiseIssue(ctx, rdb, issueURL, repo, deferCopilotAssignment, config)
			if err != nil {
				Error("Error triggering issue sanitisation: %v", err)
			} else {
				Info("Automatic issue sanitisation triggered for: %s", issueURL)
			}

			// Confirmation already sent via HTTP; return early.
			return
		}

		// SlackLiner URL not configured: fall back to searching for the message
		// after the confirmation has been published via Redis.  This means the
		// brain reaction may fail if the message has not yet been delivered.
		channelID, messageTs, findErr := findMessageByIssueURL(ctx, slackClient, issueURL, config)
		if findErr != nil {
			Error("Error finding message for brain reaction: %v", findErr)
		} else if channelID != "" && messageTs != "" {
			// Add :brain: reaction to indicate sanitisation is starting
			reactionErr := sendReactionToSlackLiner(ctx, rdb, issueSanitisingReactionEmoji, channelID, messageTs, config)
			if reactionErr != nil {
				Error("Error sending brain reaction: %v", reactionErr)
			} else {
				Debug("Sent %s reaction for sanitisation start", issueSanitisingReactionEmoji)
			}
		}

		// Trigger issue sanitisation
		err := sanitiseIssue(ctx, rdb, issueURL, repo, deferCopilotAssignment, config)
		if err != nil {
			Error("Error triggering issue sanitisation: %v", err)
		} else {
			Info("Automatic issue sanitisation triggered for: %s", issueURL)
		}
	}

	// Send confirmation message with issue URL
	sendConfirmation(ctx, rdb, repo, title, username, issueURL, assignedToCopilot, config)
}
