package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/h2non/gock"
	"github.com/stretchr/testify/suite"
	"go.uber.org/zap"
)

type ToucherTestSuite struct {
	suite.Suite
	toucher *Toucher
	client  *UserliClient
	tempDir string
}

func (s *ToucherTestSuite) SetupTest() {
	defer gock.Off()
	logger = zap.NewNop()

	// Create temporary directory for sieve files
	var err error
	s.tempDir, err = os.MkdirTemp("", "toucher_test_")
	s.Require().NoError(err)

	// Create real UserliClient with test URL and credentials
	s.client = NewUserliClient("https://api.example.com", "test-token")
	s.toucher = NewToucher(
		s.client,
		100*time.Millisecond,
		filepath.Join(s.tempDir, "{domain}", "{local_part}", ".dovecot.sieve"),
		false,
	)
}

func (s *ToucherTestSuite) TearDownTest() {
	gock.Off()
	os.RemoveAll(s.tempDir)
}

func (s *ToucherTestSuite) TestNewToucher() {
	client := NewUserliClient("https://example.com", "token")
	interval := 5 * time.Minute
	location := "/tmp/toucher_test/{domain}/{local_part}/.dovecot.sieve"
	useSudo := false

	toucher := NewToucher(client, interval, location, useSudo)

	s.Equal(client, toucher.client)
	s.Equal(interval, toucher.tickInterval)
	s.Equal(location, toucher.sieveLocation)
}

func (s *ToucherTestSuite) TestProcess_NoInactiveUsers() {
	gock.New("https://api.example.com").
		Get("/api/retention/users").
		MatchHeader("Authorization", "Bearer test-token").
		MatchHeader("Content-Type", "application/json").
		MatchHeader("User-Agent", "Userli-Mailbox-Janitor").
		Reply(200).
		JSON([]string{})

	s.toucher.processInactiveUsers()

	s.True(gock.IsDone())
}

func (s *ToucherTestSuite) TestProcess_FetchError() {
	gock.New("https://api.example.com").
		Get("/api/retention/users").
		Reply(500)

	s.toucher.processInactiveUsers()

	s.True(gock.IsDone())
}

func (s *ToucherTestSuite) TestProcess_InvalidEmail() {
	gock.New("https://api.example.com").
		Get("/api/retention/users").
		MatchHeader("Authorization", "Bearer test-token").
		Reply(200).
		JSON([]string{"invalid-email"})

	s.toucher.processInactiveUsers()

	s.True(gock.IsDone())
}

func (s *ToucherTestSuite) TestProcess_SieveFileNotExists() {
	gock.New("https://api.example.com").
		Get("/api/retention/users").
		Reply(200).
		JSON([]string{"user@example.com"})

	s.toucher.processInactiveUsers()

	s.True(gock.IsDone())
}

func (s *ToucherTestSuite) TestProcess_SieveFileWithoutRedirect() {
	email := "user@example.com"

	gock.New("https://api.example.com").
		Get("/api/retention/users").
		Reply(200).
		JSON([]string{email})

	// Sieve file without redirect
	s.createSieveFile(email, "# Simple sieve script\nif exists \"X-Spam-Flag\" {\n  fileinto \"Spam\";\n}")

	s.toucher.processInactiveUsers()

	s.True(gock.IsDone())
}

func (s *ToucherTestSuite) TestProcess_SieveFileWithRedirect() {
	email := "user@example.com"

	gock.New("https://api.example.com").
		Get("/api/retention/users").
		Reply(200).
		JSON([]string{email})

	gock.New("https://api.example.com").
		Put("/api/retention/user@example.com/touch").
		MatchHeader("Authorization", "Bearer test-token").
		MatchHeader("Content-Type", "application/json").
		MatchHeader("User-Agent", "Userli-Mailbox-Janitor").
		Reply(200)

	// Create sieve file with "if true" redirect rule
	s.createSieveFile(email, "# Sieve script with redirect\nif true {\n  redirect \"admin@example.com\";\n}")

	s.toucher.processInactiveUsers()

	s.True(gock.IsDone())
}

func (s *ToucherTestSuite) TestProcess_TouchUserFails() {
	email := "user@example.com"

	gock.New("https://api.example.com").
		Get("/api/retention/users").
		Reply(200).
		JSON([]string{email})

	gock.New("https://api.example.com").
		Put("/api/retention/user@example.com/touch").
		Reply(500)

	// Create sieve file with redirect
	s.createSieveFile(email, "if true {\n  redirect \"admin@example.com\";\n}")

	s.toucher.processInactiveUsers()

	s.True(gock.IsDone())
}

func (s *ToucherTestSuite) TestProcess_MultipleUsers() {
	emails := []string{"user1@example.com", "user2@example.com", "user3@example.com"}

	gock.New("https://api.example.com").
		Get("/api/retention/users").
		Reply(200).
		JSON(emails)

	// Only user1 and user3 should be touched (they have redirect rules)
	gock.New("https://api.example.com").
		Put("/api/retention/user1@example.com/touch").
		Reply(200)

	gock.New("https://api.example.com").
		Put("/api/retention/user3@example.com/touch").
		Reply(200)

	// Create sieve files - only user1 and user3 have redirect
	s.createSieveFile(emails[0], "if true {\n  redirect \"admin@example.com\";\n}")
	s.createSieveFile(emails[1], "# No redirect rule")
	s.createSieveFile(emails[2], "if true {\n  redirect \"backup@example.com\";\n}")

	s.toucher.processInactiveUsers()

	s.True(gock.IsDone())
}

func (s *ToucherTestSuite) TestProcess_SieveFileWithMixedConditions() {
	email := "user@example.com"

	gock.New("https://api.example.com").
		Get("/api/retention/users").
		Reply(200).
		JSON([]string{email})

	gock.New("https://api.example.com").
		Put("/api/retention/user@example.com/touch").
		Reply(200)

	// Create sieve file with mixed conditions including "if true" - should trigger touch
	sieveContent := `# Complex sieve script
if header :contains "subject" "test" {
  fileinto "Test";
}
if allof (header :contains "from" "spam@test.com") {
  discard;
}
if true {
  redirect "admin@example.com";
}`
	s.createSieveFile(email, sieveContent)

	s.toucher.processInactiveUsers()

	s.True(gock.IsDone())
}

func (s *ToucherTestSuite) TestStart_Stop() {
	ctx, cancel := context.WithCancel(context.Background())

	// Use a longer tick interval to avoid multiple executions during the test
	toucher := NewToucher(
		s.client,
		5*time.Second, // Long interval so it won't tick during our short test
		filepath.Join(s.tempDir, "{domain}", "{local_part}", ".dovecot.sieve"),
		false,
	)

	// Set up mock for the immediate execution on start
	gock.New("https://api.example.com").
		Get("/api/retention/users").
		Reply(200).
		JSON([]string{})

	// Start toucher in goroutine
	done := make(chan struct{})
	go func() {
		toucher.Start(ctx)
		close(done)
	}()

	// Wait a bit then stop (much shorter than tick interval)
	time.Sleep(100 * time.Millisecond)
	cancel()

	// Wait for toucher to stop
	select {
	case <-done:
		// Toucher stopped successfully
	case <-time.After(1 * time.Second):
		s.Fail("Toucher did not stop in time")
	}

	s.True(gock.IsDone())
}

func (s *ToucherTestSuite) TestBuildSieveFilePath() {
	tests := []struct {
		name     string
		email    string
		wantPath string
		wantErr  bool
	}{
		{
			name:     "valid email",
			email:    "user@example.com",
			wantPath: filepath.Join(s.tempDir, "example.com", "user", ".dovecot.sieve"),
			wantErr:  false,
		},
		{
			name:     "email with subdomain",
			email:    "user@mail.example.com",
			wantPath: filepath.Join(s.tempDir, "mail.example.com", "user", ".dovecot.sieve"),
			wantErr:  false,
		},
		{
			name:     "email with plus addressing",
			email:    "user+tag@example.com",
			wantPath: filepath.Join(s.tempDir, "example.com", "user+tag", ".dovecot.sieve"),
			wantErr:  false,
		},
		{
			name:    "invalid email",
			email:   "invalid-email",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			path, err := s.toucher.buildSieveFilePath(tt.email)
			if tt.wantErr {
				s.Error(err)
				return
			}
			s.NoError(err)
			s.Equal(tt.wantPath, path)
		})
	}
}

// Helper methods

func (s *ToucherTestSuite) createSieveFile(email, content string) {
	sieveDir := s.getSieveDir(email)
	err := os.MkdirAll(sieveDir, 0755)
	s.Require().NoError(err)

	sieveFile := filepath.Join(sieveDir, ".dovecot.sieve")
	err = os.WriteFile(sieveFile, []byte(content), 0644)
	s.Require().NoError(err)
}

func (s *ToucherTestSuite) getSieveDir(email string) string {
	localPart, domain, err := parseEmail(email)
	s.Require().NoError(err)
	return filepath.Join(s.tempDir, domain, localPart)
}

func TestToucherTestSuite(t *testing.T) {
	suite.Run(t, new(ToucherTestSuite))
}

// Integration tests with real HTTP client using gock

func TestToucherIntegration(t *testing.T) {
	defer gock.Off()

	logger = zap.NewNop()

	// Create temporary directory for sieve files
	tempDir, err := os.MkdirTemp("", "toucher_integration_test_")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	// Create real UserliClient
	client := NewUserliClient("https://api.example.com", "test-token")

	// Create toucher
	toucher := NewToucher(
		client,
		100*time.Millisecond,
		filepath.Join(tempDir, "{domain}", "{local_part}", ".dovecot.sieve"),
		false,
	)

	// Mock API responses
	gock.New("https://api.example.com").
		Get("/api/retention/users").
		Reply(200).
		JSON([]string{"user@example.com", "inactive@example.com"})

	gock.New("https://api.example.com").
		Put("/api/retention/user@example.com/touch").
		Reply(200)

	// Create sieve files
	createTestSieveFile(t, tempDir, "user@example.com", "if true {\n  redirect \"admin@example.com\";\n}")
	createTestSieveFile(t, tempDir, "inactive@example.com", "# No redirect")

	// Process users
	toucher.processInactiveUsers()

	// Verify all expected HTTP calls were made
	if !gock.IsDone() {
		t.Errorf("Not all expected HTTP calls were made: %+v", gock.Pending())
	}
}

func createTestSieveFile(t *testing.T, tempDir, email, content string) {
	localPart, domain, err := parseEmail(email)
	if err != nil {
		t.Fatal(err)
	}

	sieveDir := filepath.Join(tempDir, domain, localPart)
	err = os.MkdirAll(sieveDir, 0755)
	if err != nil {
		t.Fatal(err)
	}

	sieveFile := filepath.Join(sieveDir, ".dovecot.sieve")
	err = os.WriteFile(sieveFile, []byte(content), 0644)
	if err != nil {
		t.Fatal(err)
	}
}

// Tests for individual toucher methods

func (s *ToucherTestSuite) TestProcessInactiveUser_ValidEmailWithRedirect() {
	email := "user@example.com"

	gock.New("https://api.example.com").
		Put("/api/retention/user@example.com/touch").
		Reply(200)

	s.createSieveFile(email, "if true {\n  redirect \"admin@example.com\";\n}")

	s.toucher.processInactiveUser(email)

	s.True(gock.IsDone())
}

func (s *ToucherTestSuite) TestProcessInactiveUser_ValidEmailWithoutRedirect() {
	email := "user@example.com"

	s.createSieveFile(email, "# No redirect rule\nkeep;")

	s.toucher.processInactiveUser(email)

	// No HTTP calls should be made since there's no redirect
	s.True(gock.IsDone())
}

func (s *ToucherTestSuite) TestProcessInactiveUser_InvalidEmail() {
	email := "invalid-email"

	s.toucher.processInactiveUser(email)

	// No HTTP calls should be made for invalid email
	s.True(gock.IsDone())
}

func (s *ToucherTestSuite) TestProcessInactiveUser_SieveFileNotExists() {
	email := "nonexistent@example.com"

	s.toucher.processInactiveUser(email)

	// No HTTP calls should be made if sieve file doesn't exist
	s.True(gock.IsDone())
}

func (s *ToucherTestSuite) TestProcessInactiveUser_SieveFileReadError() {
	email := "user@example.com"

	// Create a directory instead of a file to cause read error
	sieveDir := s.getSieveDir(email)
	err := os.MkdirAll(sieveDir, 0755)
	s.Require().NoError(err)

	s.toucher.processInactiveUser(email)

	// No HTTP calls should be made if sieve file can't be read
	s.True(gock.IsDone())
}

func (s *ToucherTestSuite) TestProcessInactiveUser_TouchUserFails() {
	email := "user@example.com"

	gock.New("https://api.example.com").
		Put("/api/retention/user@example.com/touch").
		Reply(500)

	s.createSieveFile(email, "if true {\n  redirect \"admin@example.com\";\n}")

	s.toucher.processInactiveUser(email)

	s.True(gock.IsDone())
}
