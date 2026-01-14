package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
	"go.uber.org/zap"
)

type WorkerTestSuite struct {
	suite.Suite
	db       *Database
	worker   *Worker
	tempFile string
}

func (s *WorkerTestSuite) SetupTest() {
	logger = zap.NewNop()

	// Use unique temporary file for each test
	tempDir := os.TempDir()
	s.tempFile = filepath.Join(tempDir, "test_worker_mailboxes.csv")
	os.Remove(s.tempFile) // Ensure clean state

	var err error
	s.db, err = NewDatabase(s.tempFile)
	s.Require().NoError(err)

	// Use 'echo' command with placeholders for testing
	s.worker = NewWorker(s.db, 100*time.Millisecond, 0, "echo {email} {domain} {local_part}")
}

func (s *WorkerTestSuite) TearDownTest() {
	s.db.Close()
	os.Remove(s.tempFile)
}

func (s *WorkerTestSuite) TestProcessDueMailboxes_Empty() {
	s.worker.processDueMailboxes()
	// Should not panic with empty database
}

func (s *WorkerTestSuite) TestProcessDueMailboxes_Success() {
	// Add a mailbox
	err := s.db.AddMailbox("test@example.com")
	s.NoError(err)

	// Process mailboxes
	s.worker.processDueMailboxes()

	// Verify mailbox was removed after processing
	mailboxes, err := s.db.GetDueMailboxes(0)
	s.NoError(err)
	s.Empty(mailboxes)
}

func (s *WorkerTestSuite) TestProcessDueMailboxes_CommandFails() {
	// Use invalid command that will fail
	s.worker.purgeCommand = "/nonexistent/command {email}"

	// Add a mailbox
	err := s.db.AddMailbox("test@example.com")
	s.NoError(err)

	// Process mailboxes
	s.worker.processDueMailboxes()

	// Mailbox should still be in database because command failed
	mailboxes, err := s.db.GetDueMailboxes(0)
	s.NoError(err)
	s.Len(mailboxes, 1)
}

func (s *WorkerTestSuite) TestWorkerStart_Stop() {
	ctx, cancel := context.WithCancel(context.Background())

	// Start worker in goroutine
	done := make(chan struct{})
	go func() {
		s.worker.Start(ctx)
		close(done)
	}()

	// Wait a bit then stop
	time.Sleep(200 * time.Millisecond)
	cancel()

	// Wait for worker to stop
	select {
	case <-done:
		// Worker stopped successfully
	case <-time.After(1 * time.Second):
		s.Fail("Worker did not stop in time")
	}
}

func TestWorkerTestSuite(t *testing.T) {
	suite.Run(t, new(WorkerTestSuite))
}

func TestParseEmail(t *testing.T) {
	tests := []struct {
		name          string
		email         string
		wantLocalPart string
		wantDomain    string
		wantErr       bool
	}{
		{"valid email", "user@example.com", "user", "example.com", false},
		{"valid email with subdomain", "user@mail.example.com", "user", "mail.example.com", false},
		{"valid email with plus", "user+tag@example.com", "user+tag", "example.com", false},
		{"valid email with dots", "user.name@example.com", "user.name", "example.com", false},
		{"no at sign", "userexample.com", "", "", true},
		{"empty string", "", "", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			localPart, domain, err := parseEmail(tt.email)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseEmail(%q) error = %v, wantErr %v", tt.email, err, tt.wantErr)
				return
			}
			if localPart != tt.wantLocalPart {
				t.Errorf("parseEmail(%q) localPart = %v, want %v", tt.email, localPart, tt.wantLocalPart)
			}
			if domain != tt.wantDomain {
				t.Errorf("parseEmail(%q) domain = %v, want %v", tt.email, domain, tt.wantDomain)
			}
		})
	}
}

func TestBuildPurgeCommand(t *testing.T) {
	tests := []struct {
		name        string
		cmdTemplate string
		email       string
		want        string
		wantErr     bool
	}{
		{
			name:        "all placeholders",
			cmdTemplate: "rm -rf /var/vmail/{domain}/{local_part}",
			email:       "user@example.com",
			want:        "rm -rf /var/vmail/example.com/user",
			wantErr:     false,
		},
		{
			name:        "email placeholder only",
			cmdTemplate: "doveadm purge -u {email}",
			email:       "user@example.com",
			want:        "doveadm purge -u user@example.com",
			wantErr:     false,
		},
		{
			name:        "domain placeholder only",
			cmdTemplate: "echo {domain}",
			email:       "user@mail.example.org",
			want:        "echo mail.example.org",
			wantErr:     false,
		},
		{
			name:        "local_part placeholder only",
			cmdTemplate: "echo {local_part}",
			email:       "john.doe+test@example.com",
			want:        "echo john.doe+test",
			wantErr:     false,
		},
		{
			name:        "no placeholders",
			cmdTemplate: "echo hello",
			email:       "user@example.com",
			want:        "echo hello",
			wantErr:     false,
		},
		{
			name:        "multiple same placeholders",
			cmdTemplate: "{email} and {email}",
			email:       "user@example.com",
			want:        "user@example.com and user@example.com",
			wantErr:     false,
		},
		{
			name:        "complex command",
			cmdTemplate: "sudo rm -rf /var/vmail/{domain}/{local_part} && echo 'Deleted {email}'",
			email:       "admin@test.org",
			want:        "sudo rm -rf /var/vmail/test.org/admin && echo 'Deleted admin@test.org'",
			wantErr:     false,
		},
		{
			name:        "invalid email - no at sign",
			cmdTemplate: "echo {email}",
			email:       "invalid-email",
			want:        "",
			wantErr:     true,
		},
		{
			name:        "invalid email - empty",
			cmdTemplate: "echo {email}",
			email:       "",
			want:        "",
			wantErr:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := buildPurgeCommand(tt.cmdTemplate, tt.email)
			if (err != nil) != tt.wantErr {
				t.Errorf("buildPurgeCommand(%q, %q) error = %v, wantErr %v", tt.cmdTemplate, tt.email, err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("buildPurgeCommand(%q, %q) = %v, want %v", tt.cmdTemplate, tt.email, got, tt.want)
			}
		})
	}
}

func TestValidateEmail(t *testing.T) {
	tests := []struct {
		name    string
		email   string
		wantErr bool
	}{
		// Valid emails (allowlist)
		{"valid email", "user@example.com", false},
		{"valid email with subdomain", "user@mail.example.com", false},
		{"valid email with dots", "user.name@example.com", false},
		{"valid email with hyphen", "user-name@example.com", false},
		{"valid email with underscore", "user_name@example.com", false},
		{"valid email with numbers", "user123@example.com", false},
		{"valid email with start underscore", "_user123@example.com", false},
		{"valid email with start dash", "-user123@example.com", false},

		// Invalid emails (not matching allowlist)
		{"empty email", "", true},
		{"uppercase letters", "User@example.com", true},
		{"email with plus", "user+tag@example.com", true},
		{"email with dot start", ".user@example.com", true},
		{"space in email", "user @example.com", true},
		{"wildcard star", "*@example.com", true},
		{"wildcard question", "user?@example.com", true},
		{"wildcard in domain", "user@*.com", true},
		{"no at sign", "userexample.com", true},
		{"multiple at signs", "user@exam@ple.com", true},
		{"semicolon injection", "user@example.com;rm", true},
		{"pipe injection", "user@example.com|cat", true},
		{"ampersand injection", "user@example.com&whoami", true},
		{"backtick injection", "user@example.com`id`", true},
		{"dollar injection", "user@example.com$HOME", true},
		{"newline injection", "user@example.com\ninjected", true},
		{"quote injection", "user\"@example.com", true},
		{"single quote injection", "user'@example.com", true},
		{"double dot path traversal", "..@example.com", true},
		{"path traversal in local", "user..name@example.com", true},
		{"missing TLD", "user@example", true},
		{"single char TLD", "user@example.c", true},
		{"special chars", "user!@example.com", true},
		{"parentheses", "user()@example.com", true},
		{"brackets", "user[]@example.com", true},
		{"backslash", "user\\@example.com", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateEmail(tt.email)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateEmail(%q) error = %v, wantErr %v", tt.email, err, tt.wantErr)
			}
		})
	}
}
