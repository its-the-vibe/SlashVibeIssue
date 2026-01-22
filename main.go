package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/slack-go/slack"
)

func main() {
	config := loadConfig()

	// Initialize logger with configured level
	SetLogLevel(config.LogLevel)

	if config.SlackBotToken == "" {
		Fatal("SLACK_BOT_TOKEN is required")
	}
	if config.GitHubOrg == "" {
		Fatal("GITHUB_ORG is required")
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
		Fatal("Failed to connect to Redis: %v", err)
	}
	Info("Connected to Redis")

	// Setup Slack client
	slackClient := slack.New(config.SlackBotToken)

	// Start subscribers
	go subscribeToSlashCommands(ctx, rdb, slackClient, config)
	go subscribeToViewSubmissions(ctx, rdb, slackClient, config)
	go subscribeToPoppitOutput(ctx, rdb, slackClient, config)
	go subscribeToReactions(ctx, rdb, slackClient, config)
	go subscribeToMessageActions(ctx, rdb, slackClient, config)
	go subscribeToGitHubWebhooks(ctx, rdb, slackClient, config)

	log.Println("SlashVibeIssue service started")

	// Wait for interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	<-sigChan

	Info("Shutting down...")
	cancel()
	time.Sleep(1 * time.Second)
}
