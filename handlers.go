package main

import (
	"context"
	"encoding/json"
	"log"
	"strings"

	"github.com/redis/go-redis/v9"
	"github.com/slack-go/slack"
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

func subscribeToPoppitOutput(ctx context.Context, rdb *redis.Client, config Config) {
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
			handlePoppitOutput(ctx, rdb, msg.Payload, config)
		}
	}
}

func handlePoppitOutput(ctx context.Context, rdb *redis.Client, payload string, config Config) {
	var output PoppitOutput
	if err := json.Unmarshal([]byte(payload), &output); err != nil {
		log.Printf("Error unmarshaling Poppit output: %v", err)
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
		json.Unmarshal(payloadBytes, &metadata.EventPayload)
	}

	// Check if it's an issue_created event
	if metadata.EventType != "issue_created" {
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
