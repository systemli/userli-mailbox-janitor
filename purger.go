package main

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"go.uber.org/zap"
)

// Purger processes mailbox purging tasks periodically
type Purger struct {
	db             *Database
	tickInterval   time.Duration
	retentionHours int
	purgeCommand   string
}

// NewPurger creates a new purger instance
func NewPurger(db *Database, tickInterval time.Duration, retentionHours int, purgeCommand string) *Purger {
	return &Purger{
		db:             db,
		tickInterval:   tickInterval,
		retentionHours: retentionHours,
		purgeCommand:   purgeCommand,
	}
}

// Start starts the purger background processInactiveUsers
func (p *Purger) Start(ctx context.Context) {
	logger.Info("Starting purger",
		zap.Duration("tickInterval", p.tickInterval),
		zap.Int("retentionHours", p.retentionHours))

	ticker := time.NewTicker(p.tickInterval)
	defer ticker.Stop()

	// Run immediately on start
	p.processDueMailboxes()

	for {
		select {
		case <-ticker.C:
			p.processDueMailboxes()
		case <-ctx.Done():
			logger.Info("Purger stopped")
			return
		}
	}
}

// buildPurgeCommand replaces placeholders in the command template with actual values
func (p *Purger) buildPurgeCommand(cmdTemplate, email string) (string, error) {
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

// processDueMailboxes processes all mailboxes that are due for purging
func (p *Purger) processDueMailboxes() {
	mailboxes, err := p.db.GetDueMailboxes(p.retentionHours)
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
		p.processSingleMailbox(mailbox)
	}
}

// processSingleMailbox purges a single mailbox
func (p *Purger) processSingleMailbox(mailbox Mailbox) {
	logger.Info("Purging mailbox",
		zap.String("email", mailbox.Email),
		zap.Time("created_at", mailbox.CreatedAt))

	if err := p.purgeMailbox(mailbox.Email); err != nil {
		logger.Error("Failed to purge mailbox",
			zap.String("email", mailbox.Email),
			zap.Error(err))
		return
	}

	if err := p.db.RemoveMailbox(mailbox.Email); err != nil {
		logger.Error("Failed to remove mailbox from database",
			zap.String("email", mailbox.Email),
			zap.Error(err))
		return
	}

	logger.Info("Mailbox purged successfully", zap.String("email", mailbox.Email))
}

// purgeMailbox executes the configured purge command for a mailbox
func (p *Purger) purgeMailbox(email string) error {
	// Build the command with placeholders replaced
	cmdString, err := p.buildPurgeCommand(p.purgeCommand, email)
	if err != nil {
		return fmt.Errorf("failed to build purge command: %p", err)
	}

	// Execute the command via shell to support complex commands
	cmd := exec.Command("/bin/sh", "-c", cmdString)

	logger.Debug("Executing command",
		zap.String("command", cmdString),
		zap.String("email", email))

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("purge command failed: %p, output: %s", err, string(output))
	}

	logger.Debug("Command executed successfully",
		zap.String("output", string(output)),
		zap.String("email", email))

	return nil
}
