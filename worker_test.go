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

	// Use mock doveadm command for testing (just use 'echo' which exists on all systems)
	s.worker = NewWorker(s.db, 100*time.Millisecond, 0, "/bin/echo", false)
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
	s.worker.doveadmPath = "/nonexistent/command"

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
