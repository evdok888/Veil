package config

import (
	"fmt"
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

// Config holds all configuration for the bot
type Config struct {
	BotToken           string
	SuperGroupID       int64
	OwnerUserID        int64
	WebhookURL         string
	ServerAddr         string
	DBPath             string
	RateLimitPerMinute int
	BlockedMessage     string
	DebugLog           bool
}

// Load reads configuration from environment variables
func Load() (*Config, error) {
	// Load .env file if it exists
	_ = godotenv.Load()

	cfg := &Config{}

	// Required fields
	cfg.BotToken = os.Getenv("BOT_TOKEN")
	if cfg.BotToken == "" {
		return nil, fmt.Errorf("BOT_TOKEN is required")
	}

	superGroupIDStr := os.Getenv("SUPERGROUP_ID")
	if superGroupIDStr == "" {
		return nil, fmt.Errorf("SUPERGROUP_ID is required")
	}
	var err error
	cfg.SuperGroupID, err = strconv.ParseInt(superGroupIDStr, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid SUPERGROUP_ID: %w", err)
	}

	ownerUserIDStr := os.Getenv("OWNER_USER_ID")
	if ownerUserIDStr == "" {
		return nil, fmt.Errorf("OWNER_USER_ID is required")
	}
	cfg.OwnerUserID, err = strconv.ParseInt(ownerUserIDStr, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid OWNER_USER_ID: %w", err)
	}

	// Optional fields with defaults
	cfg.WebhookURL = os.Getenv("WEBHOOK_URL")
	cfg.ServerAddr = getEnvOrDefault("SERVER_ADDR", ":8443")
	cfg.DBPath = getEnvOrDefault("DB_PATH", "./bot.db")

	rateLimitStr := getEnvOrDefault("RATE_LIMIT_PER_MINUTE", "60")
	cfg.RateLimitPerMinute, err = strconv.Atoi(rateLimitStr)
	if err != nil {
		return nil, fmt.Errorf("invalid RATE_LIMIT_PER_MINUTE: %w", err)
	}

	cfg.BlockedMessage = getEnvOrDefault("BLOCKED_MESSAGE", "Диалог завершён. Вы не можете отправлять сообщения.")

	debugStr := getEnvOrDefault("DEBUG_LOG", "false")
	cfg.DebugLog = debugStr == "true"

	return cfg, nil
}

func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
