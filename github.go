package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"

	"github.com/redis/go-redis/v9"
)

// parseRepoFullName parses the repository parameter and returns the full "org/repo" format.
// If the repo parameter contains '/', it's already in "org/repo" format and is returned as-is.
// Otherwise, it combines the configured org with the repo name.
func parseRepoFullName(repo string, configOrg string) string {
	if strings.Contains(repo, "/") {
		return repo
	}
	return fmt.Sprintf("%s/%s", configOrg, repo)
}

func createGitHubIssue(ctx context.Context, rdb *redis.Client, repo, title, description string, assignToCopilot, addToProject bool, username string, config Config) error {
	// Parse org and repo from the repo parameter
	repoFullName := parseRepoFullName(repo, config.GitHubOrg)

	// Build the gh command with proper escaping
	// Escape single quotes in title and description
	escapedTitle := strings.ReplaceAll(title, `'`, `'\''`)
	ghCmd := fmt.Sprintf("gh issue create --repo %s --title '%s'", repoFullName, escapedTitle)

	if description != "" {
		escapedDesc := strings.ReplaceAll(description, `'`, `'\''`)
		ghCmd = fmt.Sprintf("%s --body '%s'", ghCmd, escapedDesc)
	}

	if assignToCopilot {
		ghCmd = fmt.Sprintf("%s --assignee @copilot", ghCmd)
	}

	// Create Poppit command message with metadata
	poppitCmd := PoppitCommand{
		Repo:     repoFullName,
		Branch:   "refs/heads/main",
		Type:     "slash-vibe-issue",
		Dir:      config.WorkingDir,
		Commands: []string{ghCmd},
		Metadata: map[string]interface{}{
			"repo":              repoFullName,
			"title":             title,
			"username":          username,
			"addToProject":      addToProject,
			"assignedToCopilot": assignToCopilot,
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

func addIssueToProject(ctx context.Context, rdb *redis.Client, issueURL string, config Config) error {
	// Validate issue URL format
	if !strings.HasPrefix(issueURL, "https://github.com/") || !strings.Contains(issueURL, "/issues/") {
		return fmt.Errorf("invalid issue URL format: %s", issueURL)
	}

	// Build the gh command to add issue to project
	ghCmd := fmt.Sprintf("gh project item-add %s --owner %s --url %s",
		config.ProjectID, config.ProjectOrg, issueURL)

	// Extract repo from the issue URL for consistency
	// URL format: https://github.com/org/repo/issues/number
	repo := config.GitHubOrg + "/SlashVibeIssue" // fallback
	parts := strings.Split(issueURL, "/")
	if len(parts) >= 5 {
		repo = parts[3] + "/" + parts[4]
	}

	// Create Poppit command message
	poppitCmd := PoppitCommand{
		Repo:     repo,
		Branch:   "refs/heads/main",
		Type:     "slash-vibe-issue-project",
		Dir:      config.WorkingDir,
		Commands: []string{ghCmd},
		Metadata: map[string]interface{}{
			"issueURL": issueURL,
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

	log.Printf("Project assignment command sent to Poppit for issue: %s", issueURL)
	return nil
}

func extractIssueURL(output string) string {
	// Example output:
	// Creating issue in its-the-vibe/SlashVibeIssue
	//
	// https://github.com/its-the-vibe/SlashVibeIssue/issues/13

	lines := strings.Split(output, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "https://github.com/") && strings.Contains(line, "/issues/") {
			return line
		}
	}
	return ""
}

func extractIssueNumber(issueURL string) int {
	// URL format: https://github.com/org/repo/issues/123
	var issueNumber int
	parts := strings.Split(issueURL, "/")
	if len(parts) >= 7 {
		fmt.Sscanf(parts[6], "%d", &issueNumber)
	}
	return issueNumber
}

func sendConfirmation(ctx context.Context, rdb *redis.Client, repo, title, username, issueURL string, assignedToCopilot bool, config Config) {
	// Parse the repository to get full org/repo format
	repoFullName := parseRepoFullName(repo, config.GitHubOrg)

	message := fmt.Sprintf("✅ *GitHub Issue Created by @%s*\n\n*Repository:* %s\n*Title:* %s\n*URL:* %s",
		username, repoFullName, title, issueURL)

	// Extract issue number from URL
	issueNumber := extractIssueNumber(issueURL)

	// Build metadata
	metadata := map[string]interface{}{
		"event_type": issueCreatedEventType,
		"event_payload": map[string]interface{}{
			"username":          username,
			"title":             title,
			"issue_number":      issueNumber,
			"issue_url":         issueURL,
			"repository":        repoFullName,
			"assignedToCopilot": assignedToCopilot,
		},
	}

	slackLinerMsg := SlackLinerMessage{
		Channel:  config.ConfirmationChannelID,
		Text:     message,
		TTL:      config.ConfirmationTTL,
		Metadata: metadata,
	}

	payload, err := json.Marshal(slackLinerMsg)
	if err != nil {
		log.Printf("Error marshaling SlackLiner message: %v", err)
		return
	}

	err = rdb.RPush(ctx, config.RedisSlackLinerList, payload).Err()
	if err != nil {
		log.Printf("Error pushing to SlackLiner list: %v", err)
		return
	}

	log.Printf("Confirmation message sent to SlackLiner for issue: %s", issueURL)
}

func assignIssueToCopilot(ctx context.Context, rdb *redis.Client, issueURL, repo string, config Config) error {
	// Build the gh command to assign issue to copilot
	ghCmd := fmt.Sprintf("gh issue edit --add-assignee=\"@copilot\" %s", issueURL)

	// Create Poppit command message
	poppitCmd := PoppitCommand{
		Repo:     repo,
		Branch:   "refs/heads/main",
		Type:     "slash-vibe-issue-assign-copilot",
		Dir:      config.WorkingDir,
		Commands: []string{ghCmd},
		Metadata: map[string]interface{}{
			"issueURL": issueURL,
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

	log.Printf("Copilot assignment command sent to Poppit for issue: %s", issueURL)
	return nil
}
