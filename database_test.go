package main

import (
	"testing"

	"github.com/stretchr/testify/suite"
	"go.uber.org/zap"
)

type DatabaseTestSuite struct {
	suite.Suite
	db     *Database
	logger *zap.Logger
}

func (s *DatabaseTestSuite) SetupTest() {
	s.logger = zap.NewNop()

	// Use in-memory database for tests
	var err error
	s.db, err = NewDatabase(":memory:", s.logger)
	s.Require().NoError(err)
}

func (s *DatabaseTestSuite) TearDownTest() {
	s.db.Close()
}

func (s *DatabaseTestSuite) TestAddMailbox() {
	err := s.db.AddMailbox("test@example.com")
	s.NoError(err)

	// Verify mailbox was added
	mailboxes, err := s.db.GetDueMailboxes(0)
	s.NoError(err)
	s.Len(mailboxes, 1)
	s.Equal("test@example.com", mailboxes[0].Email)
}

func (s *DatabaseTestSuite) TestAddMailbox_Duplicate() {
	err := s.db.AddMailbox("test@example.com")
	s.NoError(err)

	// Try to add same mailbox again
	err = s.db.AddMailbox("test@example.com")
	s.Error(err) // Should fail due to PRIMARY KEY constraint
}

func (s *DatabaseTestSuite) TestGetDueMailboxes_Empty() {
	mailboxes, err := s.db.GetDueMailboxes(24)
	s.NoError(err)
	s.Empty(mailboxes)
}

func (s *DatabaseTestSuite) TestGetDueMailboxes_NotDue() {
	err := s.db.AddMailbox("test@example.com")
	s.NoError(err)

	// Mailbox should not be due with 24 hour retention
	mailboxes, err := s.db.GetDueMailboxes(24)
	s.NoError(err)
	s.Empty(mailboxes)
}

func (s *DatabaseTestSuite) TestGetDueMailboxes_Due() {
	err := s.db.AddMailbox("test@example.com")
	s.NoError(err)

	// Mailbox should be due with 0 hour retention
	mailboxes, err := s.db.GetDueMailboxes(0)
	s.NoError(err)
	s.Len(mailboxes, 1)
	s.Equal("test@example.com", mailboxes[0].Email)
}

func (s *DatabaseTestSuite) TestRemoveMailbox() {
	err := s.db.AddMailbox("test@example.com")
	s.NoError(err)

	err = s.db.RemoveMailbox("test@example.com")
	s.NoError(err)

	// Verify mailbox was removed
	mailboxes, err := s.db.GetDueMailboxes(0)
	s.NoError(err)
	s.Empty(mailboxes)
}

func (s *DatabaseTestSuite) TestRemoveMailbox_NotExists() {
	err := s.db.RemoveMailbox("nonexistent@example.com")
	s.NoError(err) // Should not error, just no-op
}

func TestDatabaseTestSuite(t *testing.T) {
	suite.Run(t, new(DatabaseTestSuite))
}
