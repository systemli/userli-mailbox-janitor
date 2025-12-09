package main

import (
	"encoding/csv"
	"errors"
	"fmt"
	"os"
	"sync"
	"time"

	"go.uber.org/zap"
)

// Database handles all database operations for mailbox management
type Database struct {
	filePath string
	mu       sync.RWMutex
}

// Mailbox represents a mailbox entry in the database
type Mailbox struct {
	Email     string
	CreatedAt time.Time
}

const timeFormat = time.RFC3339

// NewDatabase creates a new database instance and ensures the CSV file exists
func NewDatabase(filePath string) (*Database, error) {
	database := &Database{
		filePath: filePath,
	}

	// Create file with header if it doesn't exist
	if _, err := os.Stat(filePath); errors.Is(err, os.ErrNotExist) {
		if err := database.initFile(); err != nil {
			return nil, fmt.Errorf("failed to initialize CSV file: %w", err)
		}
	}

	logger.Info("Database initialized", zap.String("path", filePath))
	return database, nil
}

// initFile creates the CSV file with header
func (d *Database) initFile() error {
	file, err := os.Create(d.filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	return writer.Write([]string{"email", "created_at"})
}

// readAll reads all mailboxes from the CSV file
func (d *Database) readAll() ([]Mailbox, error) {
	file, err := os.Open(d.filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	reader := csv.NewReader(file)
	records, err := reader.ReadAll()
	if err != nil {
		return nil, err
	}

	var mailboxes []Mailbox
	for i, record := range records {
		// Skip header
		if i == 0 {
			continue
		}
		if len(record) < 2 {
			continue
		}

		createdAt, err := time.Parse(timeFormat, record[1])
		if err != nil {
			logger.Warn("Failed to parse timestamp", zap.String("email", record[0]), zap.Error(err))
			continue
		}

		mailboxes = append(mailboxes, Mailbox{
			Email:     record[0],
			CreatedAt: createdAt,
		})
	}

	return mailboxes, nil
}

// writeAll writes all mailboxes to the CSV file
func (d *Database) writeAll(mailboxes []Mailbox) error {
	file, err := os.Create(d.filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	// Write header
	if err := writer.Write([]string{"email", "created_at"}); err != nil {
		return err
	}

	// Write records
	for _, m := range mailboxes {
		if err := writer.Write([]string{m.Email, m.CreatedAt.Format(timeFormat)}); err != nil {
			return err
		}
	}

	return nil
}

// AddMailbox adds a new mailbox to the purge queue
func (d *Database) AddMailbox(email string) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	mailboxes, err := d.readAll()
	if err != nil {
		return fmt.Errorf("failed to read mailboxes: %w", err)
	}

	// Check for duplicate
	for _, m := range mailboxes {
		if m.Email == email {
			return fmt.Errorf("mailbox already exists: %s", email)
		}
	}

	mailboxes = append(mailboxes, Mailbox{
		Email:     email,
		CreatedAt: time.Now(),
	})

	if err := d.writeAll(mailboxes); err != nil {
		return fmt.Errorf("failed to write mailboxes: %w", err)
	}

	logger.Info("Mailbox added to database", zap.String("email", email))
	return nil
}

// GetDueMailboxes returns mailboxes that are ready to be purged
func (d *Database) GetDueMailboxes(retentionHours int) ([]Mailbox, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	mailboxes, err := d.readAll()
	if err != nil {
		return nil, fmt.Errorf("failed to read mailboxes: %w", err)
	}

	cutoffTime := time.Now().Add(-time.Duration(retentionHours) * time.Hour)

	var dueMailboxes []Mailbox
	for _, m := range mailboxes {
		if m.CreatedAt.Before(cutoffTime) || m.CreatedAt.Equal(cutoffTime) {
			dueMailboxes = append(dueMailboxes, m)
		}
	}

	return dueMailboxes, nil
}

// RemoveMailbox removes a mailbox from the purge queue
func (d *Database) RemoveMailbox(email string) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	mailboxes, err := d.readAll()
	if err != nil {
		return fmt.Errorf("failed to read mailboxes: %w", err)
	}

	var newMailboxes []Mailbox
	for _, m := range mailboxes {
		if m.Email != email {
			newMailboxes = append(newMailboxes, m)
		}
	}

	if err := d.writeAll(newMailboxes); err != nil {
		return fmt.Errorf("failed to write mailboxes: %w", err)
	}

	logger.Info("Mailbox removed from database", zap.String("email", email))
	return nil
}

// Close is a no-op for CSV-based database (for interface compatibility)
func (d *Database) Close() error {
	return nil
}
