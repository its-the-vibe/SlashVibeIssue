package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/slack-go/slack"
)

type Config struct {
	RedisAddr                  string
	RedisPassword              string
	RedisChannel               string
	RedisViewSubmissionChannel string
	RedisSlackLinerList        string
	RedisPoppitList            string
	SlackBotToken              string
	GitHubOrg                  string
	WorkingDir                 string
	ConfirmationChannel        string
	ConfirmationTTL            int
}

type SlackCommand struct {
	Command     string `json:"command"`
	Text        string `json:"text"`
	ResponseURL string `json:"response_url"`
	TriggerID   string `json:"trigger_id"`
	UserID      string `json:"user_id"`
	UserName    string `json:"user_name"`
	ChannelID   string `json:"channel_id"`
}

type ViewSubmission struct {
	Type string `json:"type"`
	View struct {
		CallbackID string `json:"callback_id"`
		State      struct {
			Values map[string]map[string]interface{} `json:"values"`
		} `json:"state"`
	} `json:"view"`
	User struct {
		ID       string `json:"id"`
		Username string `json:"username"`
	} `json:"user"`
}

type SlackLinerMessage struct {
	Channel string `json:"channel"`
	Text    string `json:"text"`
	TTL     int    `json:"ttl"`
}

type PoppitCommand struct {
	Repo     string                 `json:"repo"`
	Branch   string                 `json:"branch"`
	Type     string                 `json:"type"`
	Dir      string                 `json:"dir"`
	Commands []string               `json:"commands"`
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

func loadConfig() Config {
	return Config{
		RedisAddr:                  getEnv("REDIS_ADDR", "host.docker.internal:6379"),
		RedisPassword:              getEnv("REDIS_PASSWORD", ""),
		RedisChannel:               getEnv("REDIS_CHANNEL", "slack-commands"),
		RedisViewSubmissionChannel: getEnv("REDIS_VIEW_SUBMISSION_CHANNEL", "slack-relay-view-submission"),
		RedisSlackLinerList:        getEnv("REDIS_SLACKLINER_LIST", "slack_messages"),
		RedisPoppitList:            getEnv("REDIS_POPPIT_LIST", "poppit:commands"),
		SlackBotToken:              getEnv("SLACK_BOT_TOKEN", ""),
		GitHubOrg:                  getEnv("GITHUB_ORG", ""),
		WorkingDir:                 getEnv("WORKING_DIR", "/tmp"),
		ConfirmationChannel:        getEnv("CONFIRMATION_CHANNEL", "#gh-issues"),
		ConfirmationTTL:            getEnvAsIntSeconds("CONFIRMATION_TTL", "48h"),
	}
}

func getEnvAsIntSeconds(key, defaultValue string) int {
	val := os.Getenv(key)
	if val == "" {
		val = defaultValue
	}
	if i, err := strconv.Atoi(val); err == nil {
		return i
	}
	if d, err := time.ParseDuration(val); err == nil {
		return int(d.Seconds())
	}
	log.Printf("Unable to parse %s=%q as int seconds or duration; defaulting to 0", key, val)
	return 0
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func main() {
	config := loadConfig()

	if config.SlackBotToken == "" {
		log.Fatal("SLACK_BOT_TOKEN is required")
	}
	if config.GitHubOrg == "" {
		log.Fatal("GITHUB_ORG is required")
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Setup Redis client
	rdb := redis.NewClient(&redis.Options{
		Addr:     config.RedisAddr,
		Password: config.RedisPassword,
		DB:       0,
	})
	defer rdb.Close()

	// Test Redis connection
	if err := rdb.Ping(ctx).Err(); err != nil {
		log.Fatalf("Failed to connect to Redis: %v", err)
	}
	log.Println("Connected to Redis")

	// Setup Slack client
	slackClient := slack.New(config.SlackBotToken)

	// Start subscribers
	go subscribeToSlashCommands(ctx, rdb, slackClient, config)
	go subscribeToViewSubmissions(ctx, rdb, slackClient, config)

	log.Println("SlashVibeIssue service started")

	// Wait for interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	<-sigChan

	log.Println("Shutting down...")
	cancel()
	time.Sleep(1 * time.Second)
}

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

func createIssueModal(initialTitle, initialDescription string, preselectCopilot bool) slack.ModalViewRequest {
	titleInput := &slack.PlainTextInputBlockElement{
		Type:     slack.METPlainTextInput,
		ActionID: "issue_title",
		Placeholder: &slack.TextBlockObject{
			Type: slack.PlainTextType,
			Text: "Brief summary of the issue",
		},
	}

	// Pre-populate title if provided
	if initialTitle != "" {
		titleInput.InitialValue = initialTitle
	}

	descriptionInput := &slack.PlainTextInputBlockElement{
		Type:      slack.METPlainTextInput,
		ActionID:  "issue_description",
		Multiline: true,
		Placeholder: &slack.TextBlockObject{
			Type: slack.PlainTextType,
			Text: "Provide more details, reproduction steps, etc.",
		},
	}

	// Pre-populate description if provided
	if initialDescription != "" {
		descriptionInput.InitialValue = initialDescription
	}

	// Create checkbox option
	copilotOption := &slack.OptionBlockObject{
		Text: &slack.TextBlockObject{
			Type: slack.PlainTextType,
			Text: "Assign to Copilot",
		},
		Value: "true",
	}

	// Create checkbox element with optional pre-selection
	checkboxElement := slack.NewCheckboxGroupsBlockElement(
		"assign_copilot",
		copilotOption,
	)

	// Pre-select the checkbox if requested
	if preselectCopilot {
		checkboxElement.InitialOptions = []*slack.OptionBlockObject{copilotOption}
	}

	return slack.ModalViewRequest{
		Type:       slack.VTModal,
		CallbackID: "create_github_issue_modal",
		Title: slack.NewTextBlockObject(slack.PlainTextType,
			"New GitHub Issue 🐙",
			true,
			false),
		Submit: &slack.TextBlockObject{
			Type: slack.PlainTextType,
			Text: "Create Issue",
		},
		Close: &slack.TextBlockObject{
			Type: slack.PlainTextType,
			Text: "Cancel",
		},
		Blocks: slack.Blocks{
			BlockSet: []slack.Block{
				&slack.SectionBlock{
					Type: slack.MBTSection,
					Text: &slack.TextBlockObject{
						Type: slack.MarkdownType,
						Text: "Fill out the details below to open a new issue in your repository.",
					},
				},
				&slack.InputBlock{
					Type:    slack.MBTInput,
					BlockID: "repo_selection_block",
					Label: &slack.TextBlockObject{
						Type: slack.PlainTextType,
						Text: "Select Repository",
					},
					Element: &slack.SelectBlockElement{
						Type:     slack.OptTypeExternal,
						ActionID: "SlashVibeIssue",
						Placeholder: &slack.TextBlockObject{
							Type: slack.PlainTextType,
							Text: "Search for a repo...",
						},
					},
				},
				&slack.InputBlock{
					Type:    slack.MBTInput,
					BlockID: "title_block",
					Label: &slack.TextBlockObject{
						Type: slack.PlainTextType,
						Text: "Issue Title",
					},
					Element: titleInput,
				},
				&slack.InputBlock{
					Type:    slack.MBTInput,
					BlockID: "description_block",
					Label: &slack.TextBlockObject{
						Type: slack.PlainTextType,
						Text: "Description",
					},
					Element: descriptionInput,
				},
				&slack.ActionBlock{
					Type:    slack.MBTAction,
					BlockID: "assignment_block",
					Elements: &slack.BlockElements{
						ElementSet: []slack.BlockElement{
							checkboxElement,
						},
					},
				},
			},
		},
	}
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

	if repo == "" || title == "" {
		log.Println("Missing required fields: repo or title")
		return
	}

	// Create GitHub issue via Poppit
	err := createGitHubIssue(ctx, rdb, repo, title, description, assignToCopilot, config)
	if err != nil {
		log.Printf("Error creating GitHub issue: %v", err)
		return
	}

	log.Printf("GitHub issue creation command sent to Poppit for repo: %s/%s", config.GitHubOrg, repo)

	// Send confirmation message via SlackLiner
	sendConfirmation(ctx, rdb, repo, title, submission.User.Username, config)
}

func createGitHubIssue(ctx context.Context, rdb *redis.Client, repo, title, description string, assignToCopilot bool, config Config) error {
	// Build the full repository name
	repoFullName := fmt.Sprintf("%s/%s", config.GitHubOrg, repo)

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

	// Create Poppit command message
	poppitCmd := PoppitCommand{
		Repo:     repoFullName,
		Branch:   "refs/heads/main",
		Type:     "slash-vibe-issue",
		Dir:      config.WorkingDir,
		Commands: []string{ghCmd},
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

func sendConfirmation(ctx context.Context, rdb *redis.Client, repo, title, username string, config Config) {
	message := fmt.Sprintf("⏳ *GitHub Issue Creation Initiated by @%s*\n\n*Repository:* %s/%s\n*Title:* %s",
		username, config.GitHubOrg, repo, title)

	slackLinerMsg := SlackLinerMessage{
		Channel: config.ConfirmationChannel,
		Text:    message,
		TTL:     config.ConfirmationTTL,
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

	log.Printf("Confirmation message sent to SlackLiner for issue in repo: %s/%s", config.GitHubOrg, repo)
}
