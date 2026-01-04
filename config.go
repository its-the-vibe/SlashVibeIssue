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
	RedisSlackLinerList        string
	RedisPoppitList            string
	RedisPoppitOutputChannel   string
	SlackBotToken              string
	GitHubOrg                  string
	WorkingDir                 string
	ConfirmationChannel        string
	ConfirmationTTL            int
	ProjectID                  string
	ProjectOrg                 string
}

func loadConfig() Config {
	return Config{
		RedisAddr:                  getEnv("REDIS_ADDR", "host.docker.internal:6379"),
		RedisPassword:              getEnv("REDIS_PASSWORD", ""),
		RedisChannel:               getEnv("REDIS_CHANNEL", "slack-commands"),
		RedisViewSubmissionChannel: getEnv("REDIS_VIEW_SUBMISSION_CHANNEL", "slack-relay-view-submission"),
		RedisSlackLinerList:        getEnv("REDIS_SLACKLINER_LIST", "slack_messages"),
		RedisPoppitList:            getEnv("REDIS_POPPIT_LIST", "poppit:commands"),
		RedisPoppitOutputChannel:   getEnv("REDIS_POPPIT_OUTPUT_CHANNEL", "poppit:command-output"),
		SlackBotToken:              getEnv("SLACK_BOT_TOKEN", ""),
		GitHubOrg:                  getEnv("GITHUB_ORG", ""),
		WorkingDir:                 getEnv("WORKING_DIR", "/tmp"),
		ConfirmationChannel:        getEnv("CONFIRMATION_CHANNEL", "#gh-issues"),
		ConfirmationTTL:            getEnvAsIntSeconds("CONFIRMATION_TTL", "48h"),
		ProjectID:                  getEnv("PROJECT_ID", "1"),
		ProjectOrg:                 getEnv("PROJECT_ORG", "its-the-vibe"),
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
