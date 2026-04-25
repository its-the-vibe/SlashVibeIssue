package main

import (
	"context"
	"encoding/json"

	"github.com/redis/go-redis/v9"
	"github.com/slack-go/slack"
)

func subscribeToViewSubmissions(ctx context.Context, rdb *redis.Client, slackClient *slack.Client, config Config) {
	pubsub := rdb.Subscribe(ctx, config.RedisViewSubmissionChannel)
	defer pubsub.Close()

	Info("Subscribed to Redis channel: %s", config.RedisViewSubmissionChannel)

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
		Error("Error unmarshaling view submission: %v", err)
		return
	}

	// Only handle our specific callback_id
	if submission.View.CallbackID != "create_github_issue_modal" {
		return
	}

	Debug("Received view submission from user %s", submission.User.Username)

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

	// If title is empty from state (happens when initial_value was set via UpdateView
	// and the user did not modify the field), fall back to initial_value from blocks
	if title == "" {
		for _, block := range submission.View.Blocks {
			if block.BlockID == "title_block" {
				title = block.Element.InitialValue
				break
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

	// Check if sanitise issue is selected
	sanitiseIssue := false
	if assignBlock, ok := values["assignment_block"]; ok {
		if sanitiseData, ok := assignBlock["sanitise_issue"]; ok {
			if sanitiseMap, ok := sanitiseData.(map[string]interface{}); ok {
				if selectedOptions, ok := sanitiseMap["selected_options"].([]interface{}); ok {
					sanitiseIssue = len(selectedOptions) > 0
				}
			}
		}
	}

	if repo == "" || title == "" {
		Warn("Missing required fields: repo or title")
		return
	}

	// Create GitHub issue via Poppit
	err := createGitHubIssue(ctx, rdb, repo, title, description, assignToCopilot, addToProject, sanitiseIssue, submission.User.Username, config)
	if err != nil {
		Error("Error creating GitHub issue: %v", err)
		return
	}

	// Log the full repo name (supports both "org/repo" and "repo" formats)
	repoFullName := parseRepoFullName(repo, config.GitHubOrg)
	Info("GitHub issue creation command sent to Poppit for repo: %s", repoFullName)
}
