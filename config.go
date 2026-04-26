package main

import (
	"log"
	"os"
	"strconv"
	"time"

	"gopkg.in/yaml.v3"
)

type Config struct {
	RedisAddr                  string
	RedisPassword              string
	RedisChannel               string
	RedisViewSubmissionChannel string
	RedisReactionChannel       string
	RedisMessageActionChannel  string
	RedisSlackLinerList        string
	RedisPoppitList            string
	RedisPoppitBuilderList     string
	RedisPoppitOutputChannel   string
	RedisGitHubWebhookChannel  string
	RedisSlackReactionsList    string
	RedisTimeBombChannel       string
	SlackBotToken              string
	SlackLinerURL              string
	GitHubOrg                  string
	WorkingDir                 string
	ConfirmationChannelID      string
	ConfirmationTTL            int
	ConfirmationSearchLimit    int
	ProjectID                  string
	ProjectOrg                 string
	AgentWorkingDir            string
	LogLevel                   string
}

// fileConfig mirrors the fields in config.sample.yaml.
// Only non-secret settings are read from the file; secrets remain in env vars.
type fileConfig struct {
	RedisAddr                  string `yaml:"redis_addr"`
	RedisChannel               string `yaml:"redis_channel"`
	RedisViewSubmissionChannel string `yaml:"redis_view_submission_channel"`
	RedisReactionChannel       string `yaml:"redis_reaction_channel"`
	RedisMessageActionChannel  string `yaml:"redis_message_action_channel"`
	RedisSlackLinerList        string `yaml:"redis_slackliner_list"`
	RedisPoppitList            string `yaml:"redis_poppit_list"`
	RedisPoppitBuilderList     string `yaml:"redis_poppit_builder_list"`
	RedisPoppitOutputChannel   string `yaml:"redis_poppit_output_channel"`
	RedisGitHubWebhookChannel  string `yaml:"redis_github_webhook_channel"`
	RedisSlackReactionsList    string `yaml:"redis_slack_reactions_list"`
	RedisTimeBombChannel       string `yaml:"redis_timebomb_channel"`
	SlackLinerURL              string `yaml:"slackliner_url"`
	GitHubOrg                  string `yaml:"github_org"`
	WorkingDir                 string `yaml:"working_dir"`
	ConfirmationChannelID      string `yaml:"confirmation_channel_id"`
	ConfirmationTTL            string `yaml:"confirmation_ttl"`
	ConfirmationSearchLimit    string `yaml:"confirmation_search_limit"`
	ProjectID                  string `yaml:"project_id"`
	ProjectOrg                 string `yaml:"project_org"`
	AgentWorkingDir            string `yaml:"agent_working_dir"`
	LogLevel                   string `yaml:"log_level"`
}

// loadFileConfig reads config.yaml if it exists and returns the parsed values.
// Missing or unreadable files are silently ignored.
func loadFileConfig(path string) fileConfig {
	data, err := os.ReadFile(path)
	if err != nil {
		return fileConfig{}
	}
	var fc fileConfig
	if err := yaml.Unmarshal(data, &fc); err != nil {
		log.Printf("Warning: failed to parse config file %s: %v", path, err)
		return fileConfig{}
	}
	return fc
}

func loadConfig() Config {
	fc := loadFileConfig("config.yaml")

	return Config{
		// Secrets are env-var only — no file fallback.
		RedisPassword: getEnv("REDIS_PASSWORD", ""),
		SlackBotToken: getEnv("SLACK_BOT_TOKEN", ""),

		// Non-secret settings: env var > config file > hard-coded default.
		RedisAddr:                  getEnvWithFile("REDIS_ADDR", fc.RedisAddr, "host.docker.internal:6379"),
		RedisChannel:               getEnvWithFile("REDIS_CHANNEL", fc.RedisChannel, "slack-commands"),
		RedisViewSubmissionChannel: getEnvWithFile("REDIS_VIEW_SUBMISSION_CHANNEL", fc.RedisViewSubmissionChannel, "slack-relay-view-submission"),
		RedisReactionChannel:       getEnvWithFile("REDIS_REACTION_CHANNEL", fc.RedisReactionChannel, "slack-relay-reaction-added"),
		RedisMessageActionChannel:  getEnvWithFile("REDIS_MESSAGE_ACTION_CHANNEL", fc.RedisMessageActionChannel, "slack-relay-message-action"),
		RedisSlackLinerList:        getEnvWithFile("REDIS_SLACKLINER_LIST", fc.RedisSlackLinerList, "slack_messages"),
		RedisPoppitList:            getEnvWithFile("REDIS_POPPIT_LIST", fc.RedisPoppitList, "poppit:commands"),
		RedisPoppitBuilderList:     getEnvWithFile("REDIS_POPPIT_BUILDER_LIST", fc.RedisPoppitBuilderList, "poppit:build-commands"),
		RedisPoppitOutputChannel:   getEnvWithFile("REDIS_POPPIT_OUTPUT_CHANNEL", fc.RedisPoppitOutputChannel, "poppit:command-output"),
		RedisGitHubWebhookChannel:  getEnvWithFile("REDIS_GITHUB_WEBHOOK_CHANNEL", fc.RedisGitHubWebhookChannel, "github-webhook-issues"),
		RedisSlackReactionsList:    getEnvWithFile("REDIS_SLACK_REACTIONS_LIST", fc.RedisSlackReactionsList, "slack_reactions"),
		RedisTimeBombChannel:       getEnvWithFile("REDIS_TIMEBOMB_CHANNEL", fc.RedisTimeBombChannel, "timebomb-messages"),
		SlackLinerURL:              getEnvWithFile("SLACKLINER_URL", fc.SlackLinerURL, ""),
		GitHubOrg:                  getEnvWithFile("GITHUB_ORG", fc.GitHubOrg, ""),
		WorkingDir:                 getEnvWithFile("WORKING_DIR", fc.WorkingDir, "/tmp"),
		ConfirmationChannelID:      getEnvWithFile("CONFIRMATION_CHANNEL_ID", fc.ConfirmationChannelID, ""),
		ConfirmationTTL:            getEnvAsIntSecondsWithFile("CONFIRMATION_TTL", fc.ConfirmationTTL, "48h"),
		ConfirmationSearchLimit:    getEnvAsIntWithFile("CONFIRMATION_SEARCH_LIMIT", fc.ConfirmationSearchLimit, "100"),
		ProjectID:                  getEnvWithFile("PROJECT_ID", fc.ProjectID, "1"),
		ProjectOrg:                 getEnvWithFile("PROJECT_ORG", fc.ProjectOrg, "its-the-vibe"),
		AgentWorkingDir:            getEnvWithFile("AGENT_WORKING_DIR", fc.AgentWorkingDir, "/tmp/agent"),
		LogLevel:                   getEnvWithFile("LOG_LEVEL", fc.LogLevel, "INFO"),
	}
}

func getEnvAsIntSeconds(key, defaultValue string) int {
	val := os.Getenv(key)
	if val == "" {
		val = defaultValue
	}
	return parseIntSeconds(val, key)
}

func getEnvAsInt(key, defaultValue string) int {
	val := os.Getenv(key)
	if val == "" {
		val = defaultValue
	}
	if i, err := strconv.Atoi(val); err == nil {
		return i
	}
	// If parsing fails, try to parse the default value
	if i, err := strconv.Atoi(defaultValue); err == nil {
		log.Printf("Unable to parse %s=%q as int; using default %d", key, val, i)
		return i
	}
	log.Printf("Unable to parse %s=%q as int; defaulting to 0", key, val)
	return 0
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// getEnvWithFile returns the env var value if set, the config-file value if
// non-empty, or the hard-coded defaultValue otherwise.
func getEnvWithFile(key, fileValue, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	if fileValue != "" {
		return fileValue
	}
	return defaultValue
}

// getEnvAsIntSecondsWithFile is getEnvAsIntSeconds extended with a config-file
// fallback between the env var and the hard-coded default.
func getEnvAsIntSecondsWithFile(key, fileValue, defaultValue string) int {
	if val := os.Getenv(key); val != "" {
		return parseIntSeconds(val, key)
	}
	if fileValue != "" {
		return parseIntSeconds(fileValue, key)
	}
	return parseIntSeconds(defaultValue, key)
}

// getEnvAsIntWithFile is getEnvAsInt extended with a config-file fallback.
func getEnvAsIntWithFile(key, fileValue, defaultValue string) int {
	if val := os.Getenv(key); val != "" {
		if i, err := strconv.Atoi(val); err == nil {
			return i
		}
	}
	if fileValue != "" {
		if i, err := strconv.Atoi(fileValue); err == nil {
			return i
		}
		log.Printf("Unable to parse config-file value for %s=%q as int; using default", key, fileValue)
	}
	if i, err := strconv.Atoi(defaultValue); err == nil {
		return i
	}
	log.Printf("Unable to parse %s default=%q as int; defaulting to 0", key, defaultValue)
	return 0
}

// parseIntSeconds parses val as a plain integer (seconds) or a Go duration
// string (e.g. "48h").  key is used only for log messages.
func parseIntSeconds(val, key string) int {
	if i, err := strconv.Atoi(val); err == nil {
		return i
	}
	if d, err := time.ParseDuration(val); err == nil {
		return int(d.Seconds())
	}
	log.Printf("Unable to parse %s=%q as int seconds or duration; defaulting to 0", key, val)
	return 0
}
