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

type purgerTestSuite struct {
	suite.Suite
	db       *Database
	purger   *Purger
	tempFile string
}

func (s *purgerTestSuite) SetupTest() {
	logger = zap.NewNop()

	// Use unique temporary file for each test
	tempDir := os.TempDir()
	s.tempFile = filepath.Join(tempDir, "test_purger_mailboxes.csv")
	os.Remove(s.tempFile) // Ensure clean state

	var err error
	s.db, err = NewDatabase(s.tempFile)
	s.Require().NoError(err)

	// Use 'echo' command with placeholders for testing
	s.purger = NewPurger(s.db, 100*time.Millisecond, 0, "echo {email} {domain} {local_part}")
}

func (s *purgerTestSuite) TearDownTest() {
	s.db.Close()
	os.Remove(s.tempFile)
}

func (s *purgerTestSuite) TestProcessDueMailboxes_Empty() {
	s.purger.processDueMailboxes()
	// Should not panic with empty database
}

func (s *purgerTestSuite) TestProcessDueMailboxes_Success() {
	// Add a mailbox
	err := s.db.AddMailbox("test@example.com")
	s.NoError(err)

	// Process mailboxes
	s.purger.processDueMailboxes()

	// Verify mailbox was removed after processing
	mailboxes, err := s.db.GetDueMailboxes(0)
	s.NoError(err)
	s.Empty(mailboxes)
}

func (s *purgerTestSuite) TestProcessDueMailboxes_CommandFails() {
	// Use invalid command that will fail
	s.purger.purgeCommand = "/nonexistent/command {email}"

	// Add a mailbox
	err := s.db.AddMailbox("test@example.com")
	s.NoError(err)

	// Process mailboxes
	s.purger.processDueMailboxes()

	// Mailbox should still be in database because command failed
	mailboxes, err := s.db.GetDueMailboxes(0)
	s.NoError(err)
	s.Len(mailboxes, 1)
}

func (s *purgerTestSuite) TestpurgerStart_Stop() {
	ctx, cancel := context.WithCancel(context.Background())

	// Start purger in goroutine
	done := make(chan struct{})
	go func() {
		s.purger.Start(ctx)
		close(done)
	}()

	// Wait a bit then stop
	time.Sleep(200 * time.Millisecond)
	cancel()

	// Wait for purger to stop
	select {
	case <-done:
		// Purger stopped successfully
	case <-time.After(1 * time.Second):
		s.Fail("Purger did not stop in time")
	}
}

func TestPurgerTestSuite(t *testing.T) {
	suite.Run(t, new(purgerTestSuite))
}

func (s *purgerTestSuite) TestBuildPurgeCommand() {
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
		s.Run(tt.name, func() {
			got, err := s.purger.buildPurgeCommand(tt.cmdTemplate, tt.email)
			if (err != nil) != tt.wantErr {
				s.Errorf(err, "buildPurgeCommand(%q, %q) error = %v, wantErr %v", tt.cmdTemplate, tt.email, err, tt.wantErr)
				return
			}
			if got != tt.want {
				s.Equal(tt.want, got, "buildPurgeCommand(%q, %q) = %v, want %v", tt.cmdTemplate, tt.email, got, tt.want)
			}
		})
	}
}
