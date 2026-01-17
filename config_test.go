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
	os.Unsetenv("PURGER_DATABASE_PATH")
	os.Unsetenv("PURGER_RETENTION_HOURS")
	os.Unsetenv("PURGER_TICK_INTERVAL")
	os.Unsetenv("PURGER_COMMAND")
	os.Unsetenv("TOUCHER_SIEVE_LOCATION")
	os.Unsetenv("TOUCHER_TICK_INTERVAL")
	os.Unsetenv("USERLI_URL")
	os.Unsetenv("USERLI_TOKEN")
	os.Unsetenv("TOUCHER_USE_SUDO")
}

func (s *ConfigTestSuite) TestBuildConfig_Defaults() {
	os.Setenv("WEBHOOK_SECRET", "test-secret")
	os.Setenv("TOUCHER_SIEVE_LOCATION", "/var/mail/{domain}/{local_part}/.dovecot.sieve")
	os.Setenv("USERLI_URL", "http://example.com")
	os.Setenv("USERLI_TOKEN", "test-token")

	cfg := BuildConfig()

	s.Equal("info", cfg.LogLevel)
	s.Equal(":8080", cfg.ListenAddr)
	s.Equal("test-secret", cfg.WebhookSecret)
	s.Equal("./mailboxes.csv", cfg.PurgerDatabasePath)
	s.Equal(24, cfg.PurgerRetentionHours)
	s.Equal("echo 'No PURGER_COMMAND configured; skipping purge for {domain}/{local_part}'", cfg.PurgerCommand)
	s.Equal("/var/mail/{domain}/{local_part}/.dovecot.sieve", cfg.ToucherSieveLocation)
	s.Equal("1h0m0s", cfg.PurgerTickInterval.String())
	s.Equal("24h0m0s", cfg.ToucherTickInterval.String())
	s.Equal(false, cfg.ToucherUseSudo)
}

func (s *ConfigTestSuite) TestBuildConfig_CustomValues() {
	os.Setenv("LOG_LEVEL", "debug")
	os.Setenv("LISTEN_ADDR", ":9090")
	os.Setenv("WEBHOOK_SECRET", "custom-secret")
	os.Setenv("PURGER_DATABASE_PATH", "/tmp/test.csv")
	os.Setenv("PURGER_RETENTION_HOURS", "48")
	os.Setenv("PURGER_TICK_INTERVAL", "2h")
	os.Setenv("PURGER_COMMAND", "doveadm purge -u {email}")
	os.Setenv("TOUCHER_SIEVE_LOCATION", "/var/mail/{domain}/{local_part}/.dovecot.sieve")
	os.Setenv("USERLI_URL", "http://example.com")
	os.Setenv("USERLI_TOKEN", "test-token")
	os.Setenv("TOUCHER_TICK_INTERVAL", "24h")
	os.Setenv("TOUCHER_USE_SUDO", "true")

	cfg := BuildConfig()

	s.Equal("debug", cfg.LogLevel)
	s.Equal(":9090", cfg.ListenAddr)
	s.Equal("custom-secret", cfg.WebhookSecret)
	s.Equal("/tmp/test.csv", cfg.PurgerDatabasePath)
	s.Equal(48, cfg.PurgerRetentionHours)
	s.Equal("2h0m0s", cfg.PurgerTickInterval.String())
	s.Equal("doveadm purge -u {email}", cfg.PurgerCommand)
	s.Equal("/var/mail/{domain}/{local_part}/.dovecot.sieve", cfg.ToucherSieveLocation)
	s.Equal("http://example.com", cfg.UserliURL)
	s.Equal("test-token", cfg.UserliToken)
	s.Equal("24h0m0s", cfg.ToucherTickInterval.String())
	s.Equal(true, cfg.ToucherUseSudo)
}

func TestConfigTestSuite(t *testing.T) {
	suite.Run(t, new(ConfigTestSuite))
}
