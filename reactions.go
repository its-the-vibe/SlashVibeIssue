package main

import (
	"context"
	"encoding/json"

	"github.com/redis/go-redis/v9"
	"github.com/slack-go/slack"
)

func subscribeToReactions(ctx context.Context, rdb *redis.Client, slackClient *slack.Client, config Config) {
	pubsub := rdb.Subscribe(ctx, config.RedisReactionChannel)
	defer pubsub.Close()

	Info("Subscribed to Redis channel: %s", config.RedisReactionChannel)

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
		Error("Error unmarshaling reaction event: %v", err)
		return
	}

	// Only handle reaction_added events
	if reaction.Event.Type != "reaction_added" {
		return
	}

	// Ignore reactions from bots
	for _, auth := range reaction.Authorizations {
		if auth.IsBot && auth.UserID == reaction.Event.User {
			Debug("Ignoring reaction from bot user: %s", reaction.Event.User)
			return
		}
	}

	// Only handle sparkles or ticket emoji
	if reaction.Event.Reaction != "sparkles" && reaction.Event.Reaction != "ticket" {
		return
	}

	// Only handle message reactions
	if reaction.Event.Item.Type != "message" {
		return
	}

	Info("Received %s reaction from user %s on message %s", reaction.Event.Reaction, reaction.Event.User, reaction.Event.Item.Ts)

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
		Error("Error fetching message from Slack: %v", err)
		return
	}

	if len(history.Messages) == 0 {
		Warn("No message found for timestamp: %s", reaction.Event.Item.Ts)
		return
	}

	message := history.Messages[0]

	// Check if message has metadata
	if message.Metadata.EventType == "" {
		Debug("Message has no metadata, ignoring reaction")
		return
	}

	// Parse metadata
	var metadata MessageMetadata
	metadata.EventType = message.Metadata.EventType

	// Convert EventPayload to map
	if payloadBytes, err := json.Marshal(message.Metadata.EventPayload); err == nil {
		if err := json.Unmarshal(payloadBytes, &metadata.EventPayload); err != nil {
			Error("Error unmarshaling event payload: %v", err)
			return
		}
	} else {
		Error("Error marshaling event payload: %v", err)
		return
	}

	// Check if it's an issue_created event
	if metadata.EventType != issueCreatedEventType {
		Debug("Event type is not issue_created: %s", metadata.EventType)
		return
	}

	// Extract issue data from metadata
	issueURL, _ := metadata.EventPayload["issue_url"].(string)
	repository, _ := metadata.EventPayload["repository"].(string)
	assignedToCopilot, _ := metadata.EventPayload["assignedToCopilot"].(bool)

	if issueURL == "" {
		Warn("Missing issue_url in metadata")
		return
	}

	// Handle different reactions
	switch reaction.Event.Reaction {
	case "sparkles":
		if assignedToCopilot {
			Debug("Issue already assigned to Copilot, ignoring reaction: %s", issueURL)
			return
		}

		Info("Assigning issue to Copilot: %s", issueURL)

		// Assign issue to Copilot
		err = assignIssueToCopilot(ctx, rdb, issueURL, repository, config)
		if err != nil {
			Error("Error assigning issue to Copilot: %v", err)
			return
		}

		Info("Successfully assigned issue to Copilot: %s", issueURL)
	case "ticket":
		// Handle issue sanitisation
		// Skip if repository metadata is missing or issue is already assigned to Copilot
		// (Copilot-assigned issues will be handled by Copilot itself)
		if repository == "" || assignedToCopilot {
			Debug("Skipping sanitisation: repository=%s, assignedToCopilot=%v", repository, assignedToCopilot)
			return
		}

		Info("Triggering issue sanitisation for: %s", issueURL)

		// Add :brain: reaction to indicate sanitisation is starting
		reactionErr := sendReactionToSlackLiner(ctx, rdb, issueSanitisingReactionEmoji, reaction.Event.Item.Channel, reaction.Event.Item.Ts, config)
		if reactionErr != nil {
			Error("Error sending brain reaction: %v", reactionErr)
		} else {
			Debug("Sent %s reaction for sanitisation start", issueSanitisingReactionEmoji)
		}

		// Trigger issue sanitisation (no deferred copilot assignment for manual sanitisation)
		err = sanitiseIssue(ctx, rdb, issueURL, repository, false, config)
		if err != nil {
			Error("Error sanitising issue: %v", err)
			return
		}

		Info("Successfully triggered issue sanitisation: %s", issueURL)
	}
}
