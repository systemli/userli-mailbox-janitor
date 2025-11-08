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
	os.Unsetenv("DOVEADM_PATH")
	os.Unsetenv("USE_SUDO")
}

func (s *ConfigTestSuite) TestBuildConfig_Defaults() {
	os.Setenv("WEBHOOK_SECRET", "test-secret")

	cfg := BuildConfig()

	s.Equal("info", cfg.LogLevel)
	s.Equal(":8080", cfg.ListenAddr)
	s.Equal("test-secret", cfg.WebhookSecret)
	s.Equal("./janitor.db", cfg.DatabasePath)
	s.Equal(24, cfg.RetentionHours)
	s.Equal("/usr/bin/doveadm", cfg.DoveadmPath)
	s.True(cfg.UseSudo)
}

func (s *ConfigTestSuite) TestBuildConfig_CustomValues() {
	os.Setenv("LOG_LEVEL", "debug")
	os.Setenv("LISTEN_ADDR", ":9090")
	os.Setenv("WEBHOOK_SECRET", "custom-secret")
	os.Setenv("DATABASE_PATH", "/tmp/test.db")
	os.Setenv("RETENTION_HOURS", "48")
	os.Setenv("TICK_INTERVAL", "10m")
	os.Setenv("DOVEADM_PATH", "/usr/local/bin/doveadm")
	os.Setenv("USE_SUDO", "false")

	cfg := BuildConfig()

	s.Equal("debug", cfg.LogLevel)
	s.Equal(":9090", cfg.ListenAddr)
	s.Equal("custom-secret", cfg.WebhookSecret)
	s.Equal("/tmp/test.db", cfg.DatabasePath)
	s.Equal(48, cfg.RetentionHours)
	s.Equal("/usr/local/bin/doveadm", cfg.DoveadmPath)
	s.False(cfg.UseSudo)
}

func TestConfigTestSuite(t *testing.T) {
	suite.Run(t, new(ConfigTestSuite))
}
