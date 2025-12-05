package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"smtp-webhook-forwarder/internal/auth"
	"smtp-webhook-forwarder/internal/config"
	"smtp-webhook-forwarder/internal/logger"
	"smtp-webhook-forwarder/internal/parser"
	"smtp-webhook-forwarder/internal/router"
	"smtp-webhook-forwarder/internal/session"
	"smtp-webhook-forwarder/internal/shutdown"
	"smtp-webhook-forwarder/internal/smtp"
	"smtp-webhook-forwarder/internal/webhook"
)

func main() {
	// Get configuration file path from environment variable or use default
	configPath := os.Getenv("CONFIG_FILE")
	if configPath == "" {
		configPath = "/etc/smtp-forwarder/config.json"
	}

	// Get log level from environment variable or use default
	logLevel := os.Getenv("LOG_LEVEL")
	if logLevel == "" {
		logLevel = "info"
	}

	// Initialize logger with configured log level
	log := logger.NewWithLevel(os.Stdout, logLevel)

	log.Info("SMTP Webhook Forwarder starting",
		logger.Str("version", "0.1.0"),
		logger.Str("config_file", configPath),
		logger.Str("log_level", logLevel),
	)

	// Load configuration
	cfg, err := config.Load(configPath)
	if err != nil {
		log.Error("failed to load configuration", err,
			logger.Str("config_file", configPath),
		)
		fmt.Fprintf(os.Stderr, "Error: Failed to load configuration from %s: %v\n", configPath, err)
		os.Exit(1)
	}

	log.Info("configuration loaded successfully")

	// Log startup configuration
	authStatus := "disabled"
	if cfg.Auth.Enabled {
		authStatus = "enabled"
	}

	log.Info("server configuration",
		logger.Int("port", cfg.Server.Port),
		logger.Str("hostname", cfg.Server.Hostname),
		logger.Str("security_mode", cfg.Server.SecurityMode),
		logger.Str("auth_status", authStatus),
		logger.Int64("max_email_size", cfg.Server.MaxEmailSize),
		logger.Int("max_concurrent_connections", cfg.Server.MaxConcurrentConnections),
	)

	// Initialize modules
	log.Info("initializing modules")

	// Initialize parser
	emailParser := parser.NewParser()

	// Initialize router
	emailRouter := router.New(cfg.Routes, cfg.Defaults.Webhook)

	// Initialize webhook signer
	signer := webhook.NewSigner()

	// Initialize webhook client
	webhookClient := webhook.NewClient(signer)

	// Initialize session generator
	sessionGen := session.NewGenerator()

	// Initialize authenticator (if auth is enabled)
	var authenticator auth.Authenticator
	if cfg.Auth.Enabled {
		credentials := make([]auth.Credential, len(cfg.Auth.Credentials))
		for i, cred := range cfg.Auth.Credentials {
			credentials[i] = auth.Credential{
				Username: cred.Username,
				Password: cred.Password,
			}
		}
		authenticator = auth.NewAuthenticator(credentials)
		log.Info("authentication enabled",
			logger.Int("credential_count", len(credentials)),
		)
	} else {
		log.Info("authentication disabled")
	}

	// Initialize SMTP message handler
	handler := smtp.NewHandler(
		emailParser,
		emailRouter,
		webhookClient,
		sessionGen,
		log,
		cfg.Server.MaxEmailSize,
	)

	// Initialize SMTP server
	server, err := smtp.NewServer(
		&cfg.Server,
		handler,
		authenticator,
		log,
	)
	if err != nil {
		log.Error("failed to initialize SMTP server", err)
		fmt.Fprintf(os.Stderr, "Error: Failed to initialize SMTP server: %v\n", err)
		os.Exit(1)
	}

	log.Info("modules initialized successfully")

	// Set up graceful shutdown handler
	shutdownHandler := shutdown.NewHandler(30 * time.Second)

	// Start SMTP server in a goroutine
	serverErr := make(chan error, 1)
	go func() {
		if err := server.Start(); err != nil {
			serverErr <- err
		}
	}()

	// Wait for either server error or shutdown signal
	select {
	case err := <-serverErr:
		log.Error("SMTP server error", err)
		fmt.Fprintf(os.Stderr, "Error: SMTP server failed: %v\n", err)
		os.Exit(1)

	case sig := <-waitForSignal(shutdownHandler):
		log.Info("received shutdown signal",
			logger.Str("signal", sig.String()),
		)

		// Perform graceful shutdown
		ctx := context.Background()
		logAdapter := &loggerAdapter{log: log}
		if err := shutdownHandler.Shutdown(ctx, server, logAdapter); err != nil {
			log.Error("shutdown error", err)
			fmt.Fprintf(os.Stderr, "Error: Shutdown failed: %v\n", err)
			os.Exit(1)
		}

		log.Info("server stopped successfully")
	}
}

// waitForSignal waits for a shutdown signal and returns it via a channel
func waitForSignal(handler *shutdown.Handler) <-chan os.Signal {
	sigChan := make(chan os.Signal, 1)
	go func() {
		sig := handler.Wait()
		sigChan <- sig
	}()
	return sigChan
}

// loggerAdapter adapts our logger.Logger to the shutdown.Logger interface
type loggerAdapter struct {
	log logger.Logger
}

func (l *loggerAdapter) Info(msg string, fields ...interface{}) {
	// Convert interface{} fields to logger.Field
	// For simplicity, we'll just log the message without fields
	l.log.Info(msg)
}

func (l *loggerAdapter) Error(msg string, err error, fields ...interface{}) {
	// Convert interface{} fields to logger.Field
	// For simplicity, we'll just log the message and error
	l.log.Error(msg, err)
}

func (l *loggerAdapter) Flush() error {
	return l.log.Flush()
}
