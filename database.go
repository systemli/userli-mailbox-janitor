package main

import (
	"database/sql"
	"fmt"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"go.uber.org/zap"
)

// Database handles all database operations for mailbox management
type Database struct {
	db     *sql.DB
	logger *zap.Logger
}

// Mailbox represents a mailbox entry in the database
type Mailbox struct {
	Email     string
	CreatedAt time.Time
}

// NewDatabase creates a new database connection and initializes the schema
func NewDatabase(dbPath string, logger *zap.Logger) (*Database, error) {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	database := &Database{
		db:     db,
		logger: logger,
	}

	if err := database.initSchema(); err != nil {
		return nil, fmt.Errorf("failed to initialize schema: %w", err)
	}

	return database, nil
}

// initSchema creates the mailboxes table if it doesn't exist
func (d *Database) initSchema() error {
	query := `
		CREATE TABLE IF NOT EXISTS mailboxes (
			email TEXT PRIMARY KEY,
			created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
		);
		CREATE INDEX IF NOT EXISTS idx_created_at ON mailboxes(created_at);
	`

	_, err := d.db.Exec(query)
	if err != nil {
		return fmt.Errorf("failed to create schema: %w", err)
	}

	d.logger.Info("Database schema initialized")
	return nil
}

// AddMailbox adds a new mailbox to the purge queue
func (d *Database) AddMailbox(email string) error {
	query := `INSERT INTO mailboxes (email, created_at) VALUES (?, ?)`

	_, err := d.db.Exec(query, email, time.Now())
	if err != nil {
		return fmt.Errorf("failed to add mailbox: %w", err)
	}

	d.logger.Info("Mailbox added to database", zap.String("email", email))
	return nil
}

// GetDueMailboxes returns mailboxes that are ready to be purged
func (d *Database) GetDueMailboxes(retentionHours int) ([]Mailbox, error) {
	query := `SELECT email, created_at FROM mailboxes WHERE created_at <= ?`

	cutoffTime := time.Now().Add(-time.Duration(retentionHours) * time.Hour)
	rows, err := d.db.Query(query, cutoffTime)
	if err != nil {
		return nil, fmt.Errorf("failed to query mailboxes: %w", err)
	}
	defer rows.Close()

	var mailboxes []Mailbox
	for rows.Next() {
		var m Mailbox
		if err := rows.Scan(&m.Email, &m.CreatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan mailbox: %w", err)
		}
		mailboxes = append(mailboxes, m)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating rows: %w", err)
	}

	return mailboxes, nil
}

// RemoveMailbox removes a mailbox from the purge queue
func (d *Database) RemoveMailbox(email string) error {
	query := `DELETE FROM mailboxes WHERE email = ?`

	_, err := d.db.Exec(query, email)
	if err != nil {
		return fmt.Errorf("failed to remove mailbox: %w", err)
	}

	d.logger.Info("Mailbox removed from database", zap.String("email", email))
	return nil
}

// Close closes the database connection
func (d *Database) Close() error {
	return d.db.Close()
}
