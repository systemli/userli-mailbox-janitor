package main

import (
	"os"
	"strconv"
	"time"

	"go.uber.org/zap"
)

// Config holds all application configuration
type Config struct {
	LogLevel             string
	ListenAddr           string
	WebhookSecret        string
	PurgerDatabasePath   string
	PurgerRetentionHours int
	PurgerTickInterval   time.Duration
	PurgerCommand        string
	ToucherSieveLocation string
	ToucherTickInterval  time.Duration
	ToucherUseSudo       bool
	UserliURL            string
	UserliToken          string
}

// BuildConfig creates a configuration from environment variables
func BuildConfig() *Config {
	cfg := &Config{
		LogLevel:             getEnvOrDefault("LOG_LEVEL", "info"),
		ListenAddr:           getEnvOrDefault("LISTEN_ADDR", ":8080"),
		PurgerDatabasePath:   getEnvOrDefault("PURGER_DATABASE_PATH", "./mailboxes.csv"),
		PurgerCommand:        getEnvOrDefault("PURGER_COMMAND", "echo 'No PURGER_COMMAND configured; skipping purge for {domain}/{local_part}'"),
		WebhookSecret:        getEnvOrFatal("WEBHOOK_SECRET"),
		PurgerRetentionHours: getEnvAsIntOrDefault("PURGER_RETENTION_HOURS", 24),
		ToucherSieveLocation: getEnvOrFatal("TOUCHER_SIEVE_LOCATION"),
		ToucherUseSudo:       getEnvAsBoolOrDefault("TOUCHER_USE_SUDO", false),
		UserliURL:            getEnvOrFatal("USERLI_URL"),
		UserliToken:          getEnvOrFatal("USERLI_TOKEN"),
	}

	// Parse purger tick interval
	purgerTickIntervalStr := getEnvOrDefault("PURGER_TICK_INTERVAL", "1h")
	purgerTickInterval, err := time.ParseDuration(purgerTickIntervalStr)
	if err != nil {
		logger.Fatal("Invalid PURGER_TICK_INTERVAL format", zap.String("value", purgerTickIntervalStr))
	}
	cfg.PurgerTickInterval = purgerTickInterval

	// Parse toucher tick interval
	toucherTickIntervalStr := getEnvOrDefault("TOUCHER_TICK_INTERVAL", "24h")
	toucherTickInterval, err := time.ParseDuration(toucherTickIntervalStr)
	if err != nil {
		logger.Fatal("Invalid TOUCHER_TICK_INTERVAL format", zap.String("value", toucherTickIntervalStr))
	}
	cfg.ToucherTickInterval = toucherTickInterval

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

// getEnvAsBoolOrDefault returns an environment variable as bool or a default value
func getEnvAsBoolOrDefault(key string, defaultValue bool) bool {
	valStr := os.Getenv(key)
	if valStr == "" {
		return defaultValue
	}

	val, err := strconv.ParseBool(valStr)
	if err != nil {
		logger.Fatal("Invalid boolean value for "+key, zap.String("value", valStr))
	}

	return val
}
