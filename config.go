package main

import (
	"log"
	"os"
	"strconv"
	"time"
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
	RedisPoppitOutputChannel   string
	RedisGitHubWebhookChannel  string
	RedisSlackReactionsList    string
	RedisTimeBombChannel       string
	SlackBotToken              string
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

func loadConfig() Config {
	return Config{
		RedisAddr:                  getEnv("REDIS_ADDR", "host.docker.internal:6379"),
		RedisPassword:              getEnv("REDIS_PASSWORD", ""),
		RedisChannel:               getEnv("REDIS_CHANNEL", "slack-commands"),
		RedisViewSubmissionChannel: getEnv("REDIS_VIEW_SUBMISSION_CHANNEL", "slack-relay-view-submission"),
		RedisReactionChannel:       getEnv("REDIS_REACTION_CHANNEL", "slack-relay-reaction-added"),
		RedisMessageActionChannel:  getEnv("REDIS_MESSAGE_ACTION_CHANNEL", "slack-relay-message-action"),
		RedisSlackLinerList:        getEnv("REDIS_SLACKLINER_LIST", "slack_messages"),
		RedisPoppitList:            getEnv("REDIS_POPPIT_LIST", "poppit:commands"),
		RedisPoppitOutputChannel:   getEnv("REDIS_POPPIT_OUTPUT_CHANNEL", "poppit:command-output"),
		RedisGitHubWebhookChannel:  getEnv("REDIS_GITHUB_WEBHOOK_CHANNEL", "github-webhook-issues"),
		RedisSlackReactionsList:    getEnv("REDIS_SLACK_REACTIONS_LIST", "slack_reactions"),
		RedisTimeBombChannel:       getEnv("REDIS_TIMEBOMB_CHANNEL", "timebomb-messages"),
		SlackBotToken:              getEnv("SLACK_BOT_TOKEN", ""),
		GitHubOrg:                  getEnv("GITHUB_ORG", ""),
		WorkingDir:                 getEnv("WORKING_DIR", "/tmp"),
		ConfirmationChannelID:      getEnv("CONFIRMATION_CHANNEL_ID", ""),
		ConfirmationTTL:            getEnvAsIntSeconds("CONFIRMATION_TTL", "48h"),
		ConfirmationSearchLimit:    getEnvAsInt("CONFIRMATION_SEARCH_LIMIT", "100"),
		ProjectID:                  getEnv("PROJECT_ID", "1"),
		ProjectOrg:                 getEnv("PROJECT_ORG", "its-the-vibe"),
		AgentWorkingDir:            getEnv("AGENT_WORKING_DIR", "/tmp/agent"),
		LogLevel:                   getEnv("LOG_LEVEL", "INFO"),
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
