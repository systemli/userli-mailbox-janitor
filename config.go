package main

import (
	"os"
	"strconv"
	"time"

	"go.uber.org/zap"
)

// Config holds all application configuration
type Config struct {
	LogLevel       string
	ListenAddr     string
	WebhookSecret  string
	DatabasePath   string
	RetentionHours int
	TickInterval   time.Duration
	PurgeCommand   string
}

// BuildConfig creates a configuration from environment variables
func BuildConfig() *Config {
	cfg := &Config{
		LogLevel:       getEnvOrDefault("LOG_LEVEL", "info"),
		ListenAddr:     getEnvOrDefault("LISTEN_ADDR", ":8080"),
		DatabasePath:   getEnvOrDefault("DATABASE_PATH", "./mailboxes.csv"),
		PurgeCommand:   getEnvOrDefault("PURGE_COMMAND", "echo 'No PURGE_COMMAND configured; skipping purge for {domain}/{local_part}'"),
		WebhookSecret:  getEnvOrFatal("WEBHOOK_SECRET"),
		RetentionHours: getEnvAsIntOrDefault("RETENTION_HOURS", 24),
	}

	// Parse tick interval
	tickIntervalStr := getEnvOrDefault("TICK_INTERVAL", "5m")
	tickInterval, err := time.ParseDuration(tickIntervalStr)
	if err != nil {
		logger.Fatal("Invalid TICK_INTERVAL format", zap.String("value", tickIntervalStr))
	}
	cfg.TickInterval = tickInterval

	return cfg
}

// getEnvOrDefault returns an environment variable value or a default value
func getEnvOrDefault(key, defaultValue string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultValue
}

// getEnvOrFatal returns an environment variable value or exits if not set
func getEnvOrFatal(key string) string {
	val := os.Getenv(key)
	if val == "" {
		logger.Fatal(key + " environment variable is required")
	}
	return val
}

// getEnvAsIntOrDefault returns an environment variable as int or a default value
func getEnvAsIntOrDefault(key string, defaultValue int) int {
	valStr := os.Getenv(key)
	if valStr == "" {
		return defaultValue
	}

	val, err := strconv.Atoi(valStr)
	if err != nil {
		logger.Fatal("Invalid integer value for "+key, zap.String("value", valStr))
	}

	return val
}
