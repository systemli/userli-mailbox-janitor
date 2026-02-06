package main

import "time"

const (
	// EventTypeUserDeleted is the event type for user deletion
	EventTypeUserDeleted = "user.deleted"
	// EventTypeUserReset is the event type for user reset (immediate mailbox purge)
	EventTypeUserReset = "user.reset"
)

// UserEvent represents a user event from userli
// It contains the event type, timestamp, and user data
type UserEvent struct {
	Type      string    `json:"type"`
	Timestamp time.Time `json:"timestamp"`
	Data      struct {
		Email string `json:"email"`
	} `json:"data"`
}
