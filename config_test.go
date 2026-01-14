package main

import (
	"os"
	"testing"

	"github.com/stretchr/testify/suite"
)

type ConfigTestSuite struct {
	suite.Suite
}

func (s *ConfigTestSuite) SetupTest() {
	// Clear environment variables
	os.Unsetenv("LOG_LEVEL")
	os.Unsetenv("LISTEN_ADDR")
	os.Unsetenv("WEBHOOK_SECRET")
	os.Unsetenv("DATABASE_PATH")
	os.Unsetenv("RETENTION_HOURS")
	os.Unsetenv("TICK_INTERVAL")
	os.Unsetenv("PURGE_COMMAND")
}

func (s *ConfigTestSuite) TestBuildConfig_Defaults() {
	os.Setenv("WEBHOOK_SECRET", "test-secret")

	cfg := BuildConfig()

	s.Equal("info", cfg.LogLevel)
	s.Equal(":8080", cfg.ListenAddr)
	s.Equal("test-secret", cfg.WebhookSecret)
	s.Equal("./mailboxes.csv", cfg.DatabasePath)
	s.Equal(24, cfg.RetentionHours)
	s.Equal("echo 'No PURGE_COMMAND configured; skipping purge for {domain}/{local_part}'", cfg.PurgeCommand)
}

func (s *ConfigTestSuite) TestBuildConfig_CustomValues() {
	os.Setenv("LOG_LEVEL", "debug")
	os.Setenv("LISTEN_ADDR", ":9090")
	os.Setenv("WEBHOOK_SECRET", "custom-secret")
	os.Setenv("DATABASE_PATH", "/tmp/test.csv")
	os.Setenv("RETENTION_HOURS", "48")
	os.Setenv("TICK_INTERVAL", "10m")
	os.Setenv("PURGE_COMMAND", "doveadm purge -u {email}")

	cfg := BuildConfig()

	s.Equal("debug", cfg.LogLevel)
	s.Equal(":9090", cfg.ListenAddr)
	s.Equal("custom-secret", cfg.WebhookSecret)
	s.Equal("/tmp/test.csv", cfg.DatabasePath)
	s.Equal(48, cfg.RetentionHours)
	s.Equal("doveadm purge -u {email}", cfg.PurgeCommand)
}

func TestConfigTestSuite(t *testing.T) {
	suite.Run(t, new(ConfigTestSuite))
}
