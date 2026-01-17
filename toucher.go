package main

import (
	"context"
	"os/exec"
	"strings"
	"time"

	"go.uber.org/zap"
)

// Toucher processes touching tasks periodically
type Toucher struct {
	client        *UserliClient
	tickInterval  time.Duration
	sieveLocation string
	useSudo       bool
}

// NewToucher creates a new toucher instance
func NewToucher(client *UserliClient, tickInterval time.Duration, sieveLocation string) *Toucher {
	return &Toucher{
		client:        client,
		tickInterval:  tickInterval,
		sieveLocation: sieveLocation,
	}
}

// Start starts the toucher background processInactiveUsers
func (t *Toucher) Start(ctx context.Context) {
	logger.Info("Starting toucher", zap.Duration("tickInterval", t.tickInterval))

	ticker := time.NewTicker(t.tickInterval)
	defer ticker.Stop()

	// Run immediately on start
	t.processInactiveUsers()

	for {
		select {
		case <-ticker.C:
			t.processInactiveUsers()
		case <-ctx.Done():
			logger.Info("Toucher stopped")
			return
		}
	}
}

// processInactiveUsers processes inactive users
func (t *Toucher) processInactiveUsers() {
	logger.Info("Starting processing")

	// Fetch inactive users from the API
	inactiveUsers, err := t.client.FetchInactiveUsers()
	if err != nil {
		logger.Error("Failed to fetch inactive users", zap.Error(err))
		return
	}

	if len(inactiveUsers) == 0 {
		logger.Debug("No inactive users found")
		return
	}

	logger.Info("Processing inactive users", zap.Int("count", len(inactiveUsers)))

	for _, email := range inactiveUsers {
		t.processInactiveUser(email)
	}

	logger.Info("Processing completed")
}

// processInactiveUser processes a single inactive user
func (t *Toucher) processInactiveUser(email string) {
	logger.Debug("Processing inactive user", zap.String("email", email))

	// Validate the email first
	if err := validateEmail(email); err != nil {
		logger.Error("Invalid email address", zap.String("email", email), zap.Error(err))
		return
	}

	// Build sieve file path
	sieveFilePath, err := t.buildSieveFilePath(email)
	if err != nil {
		logger.Error("Failed to build sieve file path", zap.String("email", email), zap.Error(err))
		return
	}

	var sieveContents []byte

	if t.useSudo {
		cmd := exec.Command("/usr/bin/sudo", "/usr/bin/cat", sieveFilePath)
		sieveContents, err = cmd.Output()
	} else {
		cmd := exec.Command("/usr/bin/cat", sieveFilePath)
		sieveContents, err = cmd.Output()
	}

	if err != nil {
		logger.Debug("Cannot read sieve file. File might not exist", zap.String("email", email), zap.String("path", sieveFilePath), zap.Error(err))
		return
	}

	// Parse sieve script for redirect rules
	hasRedirect := hasGlobalRedirect(sieveContents)
	if !hasRedirect {
		logger.Debug("No redirect rule found in sieve script", zap.String("email", email))
		return
	}

	logger.Info("Found redirect rule, touching user", zap.String("email", email))

	// Touch the user with current timestamp
	currentTime := time.Now().Unix() * 1000 // Convert to milliseconds
	if err := t.client.TouchUser(email, currentTime); err != nil {
		logger.Error("Failed to touch user", zap.String("email", email), zap.Error(err))
		return
	}

	logger.Info("Successfully touched user", zap.String("email", email))
}

// buildSieveFilePath builds the full path to a user's sieve file
func (t *Toucher) buildSieveFilePath(email string) (string, error) {
	localPart, domain, err := parseEmail(email)
	if err != nil {
		return "", err
	}

	sieveFilePath := t.sieveLocation
	sieveFilePath = strings.ReplaceAll(sieveFilePath, "{domain}", domain)
	sieveFilePath = strings.ReplaceAll(sieveFilePath, "{local_part}", localPart)

	return sieveFilePath, nil
}
