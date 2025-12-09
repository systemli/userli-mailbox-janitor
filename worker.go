package main

import (
	"context"
	"fmt"
	"os/exec"
	"time"

	"go.uber.org/zap"
)

// Worker processes mailbox purging tasks periodically
type Worker struct {
	db             *Database
	tickInterval   time.Duration
	retentionHours int
	doveadmPath    string
	useSudo        bool
}

// NewWorker creates a new worker instance
func NewWorker(db *Database, tickInterval time.Duration, retentionHours int, doveadmPath string, useSudo bool) *Worker {
	return &Worker{
		db:             db,
		tickInterval:   tickInterval,
		retentionHours: retentionHours,
		doveadmPath:    doveadmPath,
		useSudo:        useSudo,
	}
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

// purgeMailbox executes the doveadm purge command for a mailbox
func (w *Worker) purgeMailbox(email string) error {
	var cmd *exec.Cmd

	if w.useSudo {
		cmd = exec.Command("sudo", w.doveadmPath, "purge", "-u", email)
	} else {
		cmd = exec.Command(w.doveadmPath, "purge", "-u", email)
	}

	logger.Debug("Executing command",
		zap.String("command", cmd.String()),
		zap.String("email", email))

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("doveadm purge failed: %w, output: %s", err, string(output))
	}

	logger.Debug("Command executed successfully",
		zap.String("output", string(output)),
		zap.String("email", email))

	return nil
}
