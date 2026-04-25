package main

import (
	"context"
	"encoding/json"

	"github.com/redis/go-redis/v9"
	"github.com/slack-go/slack"
)

func subscribeToGitHubWebhooks(ctx context.Context, rdb *redis.Client, slackClient *slack.Client, config Config) {
	pubsub := rdb.Subscribe(ctx, config.RedisGitHubWebhookChannel)
	defer pubsub.Close()

	Info("Subscribed to Redis channel: %s", config.RedisGitHubWebhookChannel)

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
		Error("Error unmarshaling GitHub webhook event: %v", err)
		return
	}

	// Handle issue closed events
	if event.Action == "closed" {
		handleIssueClosed(ctx, rdb, slackClient, event, config)
		return
	}

	// Handle issue assigned events
	if event.Action == "assigned" {
		handleIssueAssigned(ctx, rdb, slackClient, event, config)
		return
	}
}

func handleIssueClosed(ctx context.Context, rdb *redis.Client, slackClient *slack.Client, event GitHubWebhookEvent, config Config) {
	Info("Received issue closed event for issue #%d: %s", event.Issue.Number, event.Issue.Title)

	// Use the html_url from the event payload
	issueURL := event.Issue.HTMLURL
	if issueURL == "" {
		Warn("Missing html_url in issue event")
		return
	}

	Debug("Issue URL: %s", issueURL)

	// Search for the message with matching metadata
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

	// Send reaction to SlackLiner
	err = sendReactionToSlackLiner(ctx, rdb, issueClosedReactionEmoji, channelID, messageTs, config)
	if err != nil {
		Error("Error sending reaction: %v", err)
		return
	}

	Debug("Sent %s reaction for message ts=%s", issueClosedReactionEmoji, messageTs)

	// Set TTL to 24 hours
	err = sendTTLToTimeBomb(ctx, rdb, channelID, messageTs, issueClosedTTLSeconds, config)
	if err != nil {
		Error("Error setting TTL: %v", err)
		return
	}

	Debug("Set TTL to 24 hours for message ts=%s", messageTs)
}

func handleIssueAssigned(ctx context.Context, rdb *redis.Client, slackClient *slack.Client, event GitHubWebhookEvent, config Config) {
	Info("Received issue assigned event for issue #%d: %s", event.Issue.Number, event.Issue.Title)

	// Check if assignee data is present
	if event.Assignee == nil {
		Warn("No assignee data in event")
		return
	}

	// Check if assignee is Copilot
	if event.Assignee.Login != copilotAssigneeName {
		Debug("Assignee is not Copilot: %s", event.Assignee.Login)
		return
	}

	Debug("Issue assigned to Copilot")

	// Use the html_url from the event payload
	issueURL := event.Issue.HTMLURL
	if issueURL == "" {
		Warn("Missing html_url in issue event")
		return
	}

	Debug("Issue URL: %s", issueURL)

	// Search for the message with matching metadata
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

	// Send sparkles reaction to SlackLiner
	err = sendReactionToSlackLiner(ctx, rdb, issueAssignedReactionEmoji, channelID, messageTs, config)
	if err != nil {
		Error("Error sending reaction: %v", err)
		return
	}

	Debug("Sent sparkles reaction for message ts=%s", messageTs)
}
