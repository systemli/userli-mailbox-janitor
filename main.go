package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var logger *zap.Logger

func init() {
	// Initialize logger with default config
	logLevel := "info"
	if os.Getenv("LOG_LEVEL") != "" {
		logLevel = os.Getenv("LOG_LEVEL")
	}

	atomic := zap.NewAtomicLevel()
	level, err := zapcore.ParseLevel(logLevel)
	if err != nil {
		log.Fatal(err)
	}
	atomic.SetLevel(level)

	logger = zap.New(zapcore.NewCore(
		zapcore.NewJSONEncoder(zap.NewProductionEncoderConfig()),
		zapcore.Lock(os.Stdout),
		atomic,
	))
}

func main() {
	defer func() {
		_ = logger.Sync()
	}()

	// Load configuration
	config := BuildConfig()
	logger.Info("Configuration loaded",
		zap.String("listenAddr", config.ListenAddr),
		zap.String("databasePath", config.DatabasePath),
		zap.Int("retentionHours", config.RetentionHours),
		zap.Duration("tickInterval", config.TickInterval))

	// Initialize database
	db, err := NewDatabase(config.DatabasePath, logger)
	if err != nil {
		logger.Fatal("Failed to initialize database", zap.Error(err))
	}
	defer db.Close()

	// Create context for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start worker
	worker := NewWorker(db, logger, config.TickInterval, config.RetentionHours, config.DoveadmPath, config.UseSudo)
	go worker.Start(ctx)

	// Start HTTP server
	server := NewServer(config.WebhookSecret, db, logger)

	// Setup graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	// Start server in goroutine
	go func() {
		if err := server.Start(config.ListenAddr); err != nil {
			logger.Fatal("Failed to start server", zap.Error(err))
		}
	}()

	// Wait for shutdown signal
	<-sigChan
	logger.Info("Shutdown signal received, stopping...")
	cancel()
}
