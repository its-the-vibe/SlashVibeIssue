package main

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/redis/go-redis/v9"
	"github.com/slack-go/slack"
)

const (
	issueClosedReactionEmoji     = "cat2"
	issueAssignedReactionEmoji   = "sparkles"
	issueSanitisedReactionEmoji  = "ticket"
	issueSanitisingReactionEmoji = "brain"
	issueClosedTTLSeconds        = 86400 // 24 hours
	issueCreatedEventType        = "issue_created"
	copilotAssigneeName          = "Copilot"
)

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
	return sendOrRemoveReaction(ctx, rdb, reaction, channel, ts, false, config)
}

func removeReactionFromSlackLiner(ctx context.Context, rdb *redis.Client, reaction, channel, ts string, config Config) error {
	return sendOrRemoveReaction(ctx, rdb, reaction, channel, ts, true, config)
}

func sendOrRemoveReaction(ctx context.Context, rdb *redis.Client, reaction, channel, ts string, remove bool, config Config) error {
	slackReaction := SlackReaction{
		Reaction: reaction,
		Channel:  channel,
		Ts:       ts,
		Remove:   remove,
	}

	payload, err := json.Marshal(slackReaction)
	if err != nil {
		if remove {
			return fmt.Errorf("failed to marshal slack reaction removal: %v", err)
		}
		return fmt.Errorf("failed to marshal slack reaction: %v", err)
	}

	err = rdb.RPush(ctx, config.RedisSlackReactionsList, payload).Err()
	if err != nil {
		if remove {
			return fmt.Errorf("failed to push reaction removal to Redis list: %v", err)
		}
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
