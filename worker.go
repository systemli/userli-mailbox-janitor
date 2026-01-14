package main

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"regexp"
	"strings"
	"time"

	"go.uber.org/zap"
)

// ErrInvalidEmail is returned when an email address contains invalid characters
var ErrInvalidEmail = errors.New("invalid email address")

// emailRegex is an allowlist pattern for valid email addresses.
// Only lowercase alphanumeric characters, dots, underscore, and hyphen are allowed.
// The local part can contain dots, underscores, or hyphens between alphanumeric characters.
// The domain is composed of labels that cannot start or end with a hyphen and are separated by
// single dots (no consecutive dots).
var emailRegex = regexp.MustCompile(`^[a-z0-9_-]+(?:[._-][a-z0-9]+)*@(?:[a-z0-9]+(?:-[a-z0-9]+)*\.)+[a-z]{2,}$`)

// Worker processes mailbox purging tasks periodically
type Worker struct {
	db             *Database
	tickInterval   time.Duration
	retentionHours int
	purgeCommand   string
}

// NewWorker creates a new worker instance
func NewWorker(db *Database, tickInterval time.Duration, retentionHours int, purgeCommand string) *Worker {
	return &Worker{
		db:             db,
		tickInterval:   tickInterval,
		retentionHours: retentionHours,
		purgeCommand:   purgeCommand,
	}
}

// parseEmail splits an email address into local_part and domain
func parseEmail(email string) (localPart, domain string, err error) {
	parts := strings.SplitN(email, "@", 2)
	if len(parts) != 2 {
		return "", "", fmt.Errorf("%w: invalid format", ErrInvalidEmail)
	}
	return parts[0], parts[1], nil
}

// buildPurgeCommand replaces placeholders in the command template with actual values
func buildPurgeCommand(cmdTemplate, email string) (string, error) {
	localPart, domain, err := parseEmail(email)
	if err != nil {
		return "", err
	}

	cmd := cmdTemplate
	cmd = strings.ReplaceAll(cmd, "{email}", email)
	cmd = strings.ReplaceAll(cmd, "{domain}", domain)
	cmd = strings.ReplaceAll(cmd, "{local_part}", localPart)

	return cmd, nil
}

// validateEmail checks if an email address matches the allowlist pattern.
// Only lowercase alphanumeric characters, dots, plus, underscore, and hyphen are allowed.
// This prevents shell injection and path traversal attacks by design.
func validateEmail(email string) error {
	if !emailRegex.MatchString(email) {
		return fmt.Errorf("%w: does not match allowed pattern", ErrInvalidEmail)
	}

	// Additional check for path traversal (double dots)
	if strings.Contains(email, "..") {
		return fmt.Errorf("%w: contains path traversal sequence", ErrInvalidEmail)
	}

	return nil
}

// Start starts the worker background process
func (w *Worker) Start(ctx context.Context) {
	logger.Info("Starting worker",
		zap.Duration("tickInterval", w.tickInterval),
		zap.Int("retentionHours", w.retentionHours))

	ticker := time.NewTicker(w.tickInterval)
	defer ticker.Stop()

	// Run immediately on start
	w.processDueMailboxes()

	for {
		select {
		case <-ticker.C:
			w.processDueMailboxes()
		case <-ctx.Done():
			logger.Info("Worker stopped")
			return
		}
	}
}

// processDueMailboxes processes all mailboxes that are due for purging
func (w *Worker) processDueMailboxes() {
	mailboxes, err := w.db.GetDueMailboxes(w.retentionHours)
	if err != nil {
		logger.Error("Failed to get due mailboxes", zap.Error(err))
		return
	}

	if len(mailboxes) == 0 {
		logger.Debug("No mailboxes due for purging")
		return
	}

	logger.Info("Processing due mailboxes", zap.Int("count", len(mailboxes)))

	for _, mailbox := range mailboxes {
		w.processSingleMailbox(mailbox)
	}
}

// processSingleMailbox purges a single mailbox
func (w *Worker) processSingleMailbox(mailbox Mailbox) {
	logger.Info("Purging mailbox",
		zap.String("email", mailbox.Email),
		zap.Time("created_at", mailbox.CreatedAt))

	if err := w.purgeMailbox(mailbox.Email); err != nil {
		logger.Error("Failed to purge mailbox",
			zap.String("email", mailbox.Email),
			zap.Error(err))
		return
	}

	if err := w.db.RemoveMailbox(mailbox.Email); err != nil {
		logger.Error("Failed to remove mailbox from database",
			zap.String("email", mailbox.Email),
			zap.Error(err))
		return
	}

	logger.Info("Mailbox purged successfully", zap.String("email", mailbox.Email))
}

// purgeMailbox executes the configured purge command for a mailbox
func (w *Worker) purgeMailbox(email string) error {
	// Build the command with placeholders replaced
	cmdString, err := buildPurgeCommand(w.purgeCommand, email)
	if err != nil {
		return fmt.Errorf("failed to build purge command: %w", err)
	}

	// Execute the command via shell to support complex commands
	cmd := exec.Command("sh", "-c", cmdString)

	logger.Debug("Executing command",
		zap.String("command", cmdString),
		zap.String("email", email))

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("purge command failed: %w, output: %s", err, string(output))
	}

	logger.Debug("Command executed successfully",
		zap.String("output", string(output)),
		zap.String("email", email))

	return nil
}
