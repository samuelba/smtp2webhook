package integration

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/smtp"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"smtp-webhook-forwarder/internal/auth"
	"smtp-webhook-forwarder/internal/config"
	"smtp-webhook-forwarder/internal/logger"
	"smtp-webhook-forwarder/internal/parser"
	"smtp-webhook-forwarder/internal/router"
	"smtp-webhook-forwarder/internal/session"
	smtpserver "smtp-webhook-forwarder/internal/smtp"
	"smtp-webhook-forwarder/internal/webhook"
)

// TestEndToEndSMTPToWebhook tests the complete flow from SMTP to webhook
// Requirements: 1.1, 2.2
func TestEndToEndSMTPToWebhook(t *testing.T) {
	// Create a mock webhook server
	var receivedPayload *webhook.WebhookPayload
	var receivedHeaders http.Header
	webhookServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Read the payload
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &receivedPayload)
		receivedHeaders = r.Header.Clone()
		w.WriteHeader(http.StatusOK)
	}))
	defer webhookServer.Close()

	// Create test configuration
	cfg := &config.Config{
		Server: config.ServerConfig{
			Port:                     getFreePort(t),
			Hostname:                 "localhost",
			SecurityMode:             "none",
			MaxEmailSize:             10485760,
			MaxConcurrentConnections: 10,
			ReadTimeout:              60,
			WriteTimeout:             60,
		},
		Routes: []config.Route{
			{
				Pattern: "test@example.com",
				Webhook: config.Webhook{
					URL:     webhookServer.URL,
					Secret:  "test-secret",
					Timeout: 30,
				},
			},
		},
		Auth: config.AuthConfig{
			Enabled: false,
		},
	}

	// Start SMTP server
	smtpSrv := startTestSMTPServer(t, cfg)
	defer smtpSrv.Stop()

	// Give server time to start
	time.Sleep(100 * time.Millisecond)

	// Send test email
	err := sendTestEmail(t, cfg.Server.Port, "sender@example.com", []string{"test@example.com"}, "Test Subject", "Test Body")
	if err != nil {
		t.Fatalf("Failed to send email: %v", err)
	}

	// Wait for webhook to be called
	time.Sleep(200 * time.Millisecond)

	// Verify webhook was called
	if receivedPayload == nil {
		t.Fatal("Webhook was not called")
	}

	// Verify payload content
	if receivedPayload.Email.From != "sender@example.com" {
		t.Errorf("Expected from 'sender@example.com', got '%s'", receivedPayload.Email.From)
	}
	if len(receivedPayload.Email.To) != 1 || receivedPayload.Email.To[0] != "test@example.com" {
		t.Errorf("Expected to ['test@example.com'], got %v", receivedPayload.Email.To)
	}
	if receivedPayload.Email.Subject != "Test Subject" {
		t.Errorf("Expected subject 'Test Subject', got '%s'", receivedPayload.Email.Subject)
	}

	// Verify Standard Webhooks headers
	if receivedHeaders.Get("webhook-id") == "" {
		t.Error("Missing webhook-id header")
	}
	if receivedHeaders.Get("webhook-timestamp") == "" {
		t.Error("Missing webhook-timestamp header")
	}
	if receivedHeaders.Get("webhook-signature") == "" {
		t.Error("Missing webhook-signature header (secret was configured)")
	}
	if receivedHeaders.Get("webhook-version") != "1" {
		t.Errorf("Expected webhook-version '1', got '%s'", receivedHeaders.Get("webhook-version"))
	}
}

// TestAuthenticationEnabled tests SMTP authentication when enabled
// Requirements: 4.1, 4.2, 4.3
func TestAuthenticationEnabled(t *testing.T) {
	webhookServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer webhookServer.Close()

	cfg := &config.Config{
		Server: config.ServerConfig{
			Port:                     getFreePort(t),
			Hostname:                 "localhost",
			SecurityMode:             "none",
			MaxEmailSize:             10485760,
			MaxConcurrentConnections: 10,
			ReadTimeout:              60,
			WriteTimeout:             60,
		},
		Routes: []config.Route{
			{
				Pattern: "*@example.com",
				Webhook: config.Webhook{
					URL:     webhookServer.URL,
					Timeout: 30,
				},
			},
		},
		Auth: config.AuthConfig{
			Enabled: true,
			Credentials: []config.Credential{
				{Username: "testuser", Password: "testpass"},
			},
		},
	}

	smtpSrv := startTestSMTPServer(t, cfg)
	defer smtpSrv.Stop()
	time.Sleep(100 * time.Millisecond)

	// Test with valid credentials
	t.Run("ValidCredentials", func(t *testing.T) {
		err := sendAuthenticatedEmail(t, cfg.Server.Port, "testuser", "testpass", "sender@example.com", []string{"test@example.com"}, "Test", "Body")
		if err != nil {
			t.Errorf("Failed to send email with valid credentials: %v", err)
		}
	})

	// Test with invalid credentials
	t.Run("InvalidCredentials", func(t *testing.T) {
		err := sendAuthenticatedEmail(t, cfg.Server.Port, "testuser", "wrongpass", "sender@example.com", []string{"test@example.com"}, "Test", "Body")
		if err == nil {
			t.Error("Expected authentication to fail with invalid credentials")
		}
		if !strings.Contains(err.Error(), "535") {
			t.Errorf("Expected SMTP 535 error, got: %v", err)
		}
	})

	// Test without credentials when auth is required
	t.Run("NoCredentialsWhenRequired", func(t *testing.T) {
		err := sendTestEmail(t, cfg.Server.Port, "sender@example.com", []string{"test@example.com"}, "Test", "Body")
		if err == nil {
			t.Error("Expected email to be rejected when auth is required but not provided")
		}
		if !strings.Contains(err.Error(), "530") {
			t.Errorf("Expected SMTP 530 error (Authentication required), got: %v", err)
		}
	})
}

// TestAuthenticationDisabled tests SMTP without authentication
// Requirements: 4.4
func TestAuthenticationDisabled(t *testing.T) {
	webhookServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer webhookServer.Close()

	cfg := &config.Config{
		Server: config.ServerConfig{
			Port:                     getFreePort(t),
			Hostname:                 "localhost",
			SecurityMode:             "none",
			MaxEmailSize:             10485760,
			MaxConcurrentConnections: 10,
			ReadTimeout:              60,
			WriteTimeout:             60,
		},
		Routes: []config.Route{
			{
				Pattern: "*@example.com",
				Webhook: config.Webhook{
					URL:     webhookServer.URL,
					Timeout: 30,
				},
			},
		},
		Auth: config.AuthConfig{
			Enabled: false,
		},
	}

	smtpSrv := startTestSMTPServer(t, cfg)
	defer smtpSrv.Stop()
	time.Sleep(100 * time.Millisecond)

	// Should be able to send without authentication
	err := sendTestEmail(t, cfg.Server.Port, "sender@example.com", []string{"test@example.com"}, "Test", "Body")
	if err != nil {
		t.Errorf("Failed to send email without authentication: %v", err)
	}
}

// TestTLSMode tests SMTP with TLS encryption
// Requirements: 5.1, 5.3
func TestTLSMode(t *testing.T) {
	// Generate test certificates
	certFile, keyFile := generateTestCerts(t)
	defer os.Remove(certFile)
	defer os.Remove(keyFile)

	webhookServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer webhookServer.Close()

	cfg := &config.Config{
		Server: config.ServerConfig{
			Port:                     getFreePort(t),
			Hostname:                 "localhost",
			SecurityMode:             "starttls",
			TLSCertPath:              certFile,
			TLSKeyPath:               keyFile,
			MaxEmailSize:             10485760,
			MaxConcurrentConnections: 10,
			ReadTimeout:              60,
			WriteTimeout:             60,
		},
		Routes: []config.Route{
			{
				Pattern: "*@example.com",
				Webhook: config.Webhook{
					URL:     webhookServer.URL,
					Timeout: 30,
				},
			},
		},
		Auth: config.AuthConfig{
			Enabled: false,
		},
	}

	smtpSrv := startTestSMTPServer(t, cfg)
	defer smtpSrv.Stop()
	time.Sleep(100 * time.Millisecond)

	// Send email with STARTTLS
	err := sendEmailWithSTARTTLS(t, cfg.Server.Port, "sender@example.com", []string{"test@example.com"}, "Test", "Body")
	if err != nil {
		t.Errorf("Failed to send email with STARTTLS: %v", err)
	}
}

// TestRoutingExactMatch tests exact email address matching
// Requirements: 6.3
func TestRoutingExactMatch(t *testing.T) {
	var webhook1Called, webhook2Called bool
	var mu sync.Mutex

	webhook1 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		webhook1Called = true
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	defer webhook1.Close()

	webhook2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		webhook2Called = true
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	defer webhook2.Close()

	cfg := &config.Config{
		Server: config.ServerConfig{
			Port:                     getFreePort(t),
			Hostname:                 "localhost",
			SecurityMode:             "none",
			MaxEmailSize:             10485760,
			MaxConcurrentConnections: 10,
			ReadTimeout:              60,
			WriteTimeout:             60,
		},
		Routes: []config.Route{
			{
				Pattern: "exact@example.com",
				Webhook: config.Webhook{
					URL:     webhook1.URL,
					Timeout: 30,
				},
			},
			{
				Pattern: "other@example.com",
				Webhook: config.Webhook{
					URL:     webhook2.URL,
					Timeout: 30,
				},
			},
		},
		Auth: config.AuthConfig{
			Enabled: false,
		},
	}

	smtpSrv := startTestSMTPServer(t, cfg)
	defer smtpSrv.Stop()
	time.Sleep(100 * time.Millisecond)

	// Send to exact@example.com
	err := sendTestEmail(t, cfg.Server.Port, "sender@example.com", []string{"exact@example.com"}, "Test", "Body")
	if err != nil {
		t.Fatalf("Failed to send email: %v", err)
	}

	time.Sleep(200 * time.Millisecond)

	mu.Lock()
	if !webhook1Called {
		t.Error("Webhook 1 should have been called for exact match")
	}
	if webhook2Called {
		t.Error("Webhook 2 should not have been called")
	}
	mu.Unlock()
}

// TestRoutingWildcardMatch tests wildcard domain matching
// Requirements: 6.3
func TestRoutingWildcardMatch(t *testing.T) {
	var webhookCalled bool
	var mu sync.Mutex

	webhookServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		webhookCalled = true
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	defer webhookServer.Close()

	cfg := &config.Config{
		Server: config.ServerConfig{
			Port:                     getFreePort(t),
			Hostname:                 "localhost",
			SecurityMode:             "none",
			MaxEmailSize:             10485760,
			MaxConcurrentConnections: 10,
			ReadTimeout:              60,
			WriteTimeout:             60,
		},
		Routes: []config.Route{
			{
				Pattern: "*@example.com",
				Webhook: config.Webhook{
					URL:     webhookServer.URL,
					Timeout: 30,
				},
			},
		},
		Auth: config.AuthConfig{
			Enabled: false,
		},
	}

	smtpSrv := startTestSMTPServer(t, cfg)
	defer smtpSrv.Stop()
	time.Sleep(100 * time.Millisecond)

	// Send to any address at example.com
	err := sendTestEmail(t, cfg.Server.Port, "sender@example.com", []string{"anything@example.com"}, "Test", "Body")
	if err != nil {
		t.Fatalf("Failed to send email: %v", err)
	}

	time.Sleep(200 * time.Millisecond)

	mu.Lock()
	if !webhookCalled {
		t.Error("Webhook should have been called for wildcard match")
	}
	mu.Unlock()
}

// TestWebhookSignatureVerification tests webhook signature generation
// Requirements: 3.1, 3.5
func TestWebhookSignatureVerification(t *testing.T) {
	var receivedSignature string
	var receivedBody []byte
	var receivedID, receivedTimestamp string

	webhookServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedSignature = r.Header.Get("webhook-signature")
		receivedID = r.Header.Get("webhook-id")
		receivedTimestamp = r.Header.Get("webhook-timestamp")
		receivedBody, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusOK)
	}))
	defer webhookServer.Close()

	secret := "test-secret-key"
	cfg := &config.Config{
		Server: config.ServerConfig{
			Port:                     getFreePort(t),
			Hostname:                 "localhost",
			SecurityMode:             "none",
			MaxEmailSize:             10485760,
			MaxConcurrentConnections: 10,
			ReadTimeout:              60,
			WriteTimeout:             60,
		},
		Routes: []config.Route{
			{
				Pattern: "*@example.com",
				Webhook: config.Webhook{
					URL:     webhookServer.URL,
					Secret:  secret,
					Timeout: 30,
				},
			},
		},
		Auth: config.AuthConfig{
			Enabled: false,
		},
	}

	smtpSrv := startTestSMTPServer(t, cfg)
	defer smtpSrv.Stop()
	time.Sleep(100 * time.Millisecond)

	err := sendTestEmail(t, cfg.Server.Port, "sender@example.com", []string{"test@example.com"}, "Test", "Body")
	if err != nil {
		t.Fatalf("Failed to send email: %v", err)
	}

	time.Sleep(200 * time.Millisecond)

	// Verify signature is present
	if receivedSignature == "" {
		t.Fatal("Signature header is missing")
	}

	// Verify signature format (should start with "v1,")
	if !strings.HasPrefix(receivedSignature, "v1,") {
		t.Errorf("Signature should start with 'v1,', got: %s", receivedSignature)
	}

	// Verify we can validate the signature
	signer := webhook.NewSigner()
	timestamp := receivedTimestamp
	expectedSig, err := signer.Sign(secret, receivedID, mustParseInt64(timestamp), receivedBody)
	if err != nil {
		t.Fatalf("Failed to generate expected signature: %v", err)
	}

	if receivedSignature != expectedSig {
		t.Errorf("Signature mismatch.\nExpected: %s\nGot: %s", expectedSig, receivedSignature)
	}
}

// TestWebhookFailureHandling tests error scenarios
// Requirements: 2.3, 2.4, 2.5, 2.6
func TestWebhookFailureHandling(t *testing.T) {
	tests := []struct {
		name          string
		webhookStatus int
		expectedSMTP  string
	}{
		{"Success", http.StatusOK, "250"},
		{"RateLimited", http.StatusTooManyRequests, "451"},
		{"ServerError", http.StatusInternalServerError, "451"},
		{"BadRequest", http.StatusBadRequest, "550"},
		{"NotFound", http.StatusNotFound, "550"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			webhookServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.webhookStatus)
			}))
			defer webhookServer.Close()

			cfg := &config.Config{
				Server: config.ServerConfig{
					Port:                     getFreePort(t),
					Hostname:                 "localhost",
					SecurityMode:             "none",
					MaxEmailSize:             10485760,
					MaxConcurrentConnections: 10,
					ReadTimeout:              60,
					WriteTimeout:             60,
				},
				Routes: []config.Route{
					{
						Pattern: "*@example.com",
						Webhook: config.Webhook{
							URL:     webhookServer.URL,
							Timeout: 30,
						},
					},
				},
				Auth: config.AuthConfig{
					Enabled: false,
				},
			}

			smtpSrv := startTestSMTPServer(t, cfg)
			defer smtpSrv.Stop()
			time.Sleep(100 * time.Millisecond)

			err := sendTestEmail(t, cfg.Server.Port, "sender@example.com", []string{"test@example.com"}, "Test", "Body")

			if tt.expectedSMTP == "250" {
				if err != nil {
					t.Errorf("Expected success, got error: %v", err)
				}
			} else {
				if err == nil {
					t.Error("Expected error, got success")
				} else if !strings.Contains(err.Error(), tt.expectedSMTP) {
					t.Errorf("Expected SMTP code %s in error, got: %v", tt.expectedSMTP, err)
				}
			}
		})
	}
}

// TestWebhookTimeout tests timeout handling
// Requirements: 2.6
func TestWebhookTimeout(t *testing.T) {
	webhookServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Simulate slow webhook
		time.Sleep(3 * time.Second)
		w.WriteHeader(http.StatusOK)
	}))
	defer webhookServer.Close()

	cfg := &config.Config{
		Server: config.ServerConfig{
			Port:                     getFreePort(t),
			Hostname:                 "localhost",
			SecurityMode:             "none",
			MaxEmailSize:             10485760,
			MaxConcurrentConnections: 10,
			ReadTimeout:              60,
			WriteTimeout:             60,
		},
		Routes: []config.Route{
			{
				Pattern: "*@example.com",
				Webhook: config.Webhook{
					URL:     webhookServer.URL,
					Timeout: 1, // 1 second timeout
				},
			},
		},
		Auth: config.AuthConfig{
			Enabled: false,
		},
	}

	smtpSrv := startTestSMTPServer(t, cfg)
	defer smtpSrv.Stop()
	time.Sleep(100 * time.Millisecond)

	err := sendTestEmail(t, cfg.Server.Port, "sender@example.com", []string{"test@example.com"}, "Test", "Body")
	if err == nil {
		t.Error("Expected timeout error")
	}
	if !strings.Contains(err.Error(), "451") {
		t.Errorf("Expected SMTP 451 for timeout, got: %v", err)
	}
}

// TestConfigurationFromEnvironment tests environment variable overrides
// Requirements: 7.2, 8.2
func TestConfigurationFromEnvironment(t *testing.T) {
	// Create a temporary config file
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")

	webhookServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer webhookServer.Close()

	baseConfig := config.Config{
		Server: config.ServerConfig{
			Port:                     2525,
			Hostname:                 "original-hostname",
			SecurityMode:             "none",
			MaxEmailSize:             1048576,
			MaxConcurrentConnections: 10,
			ReadTimeout:              60,
			WriteTimeout:             60,
		},
		Routes: []config.Route{
			{
				Pattern: "*@example.com",
				Webhook: config.Webhook{
					URL:     webhookServer.URL,
					Timeout: 30,
				},
			},
		},
		Auth: config.AuthConfig{
			Enabled: false,
		},
	}

	configData, _ := json.MarshalIndent(baseConfig, "", "  ")
	os.WriteFile(configPath, configData, 0644)

	// Set environment variables
	newPort := getFreePort(t)
	os.Setenv("SMTP_PORT", fmt.Sprintf("%d", newPort))
	os.Setenv("SMTP_HOSTNAME", "env-hostname")
	os.Setenv("MAX_EMAIL_SIZE", "2097152")
	defer func() {
		os.Unsetenv("SMTP_PORT")
		os.Unsetenv("SMTP_HOSTNAME")
		os.Unsetenv("MAX_EMAIL_SIZE")
	}()

	// Load config (should apply env overrides)
	cfg, err := config.Load(configPath)
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	// Verify overrides were applied
	if cfg.Server.Port != newPort {
		t.Errorf("Expected port %d from env, got %d", newPort, cfg.Server.Port)
	}
	if cfg.Server.Hostname != "env-hostname" {
		t.Errorf("Expected hostname 'env-hostname' from env, got '%s'", cfg.Server.Hostname)
	}
	if cfg.Server.MaxEmailSize != 2097152 {
		t.Errorf("Expected max email size 2097152 from env, got %d", cfg.Server.MaxEmailSize)
	}
}

// Helper functions

func startTestSMTPServer(t *testing.T, cfg *config.Config) *smtpserver.Server {
	t.Helper()

	// Create logger
	logBuf := &bytes.Buffer{}
	log := logger.New(logBuf)

	// Create components
	emailParser := parser.NewParser()
	rtr := router.New(cfg.Routes, cfg.Defaults.Webhook)
	signer := webhook.NewSigner()
	webhookClient := webhook.NewClient(signer)
	sessionGen := session.NewGenerator()

	// Create handler
	handler := smtpserver.NewHandler(
		emailParser,
		rtr,
		webhookClient,
		sessionGen,
		log,
		cfg.Server.MaxEmailSize,
	)

	// Create authenticator if auth is enabled
	var authenticator auth.Authenticator
	if cfg.Auth.Enabled {
		creds := make([]auth.Credential, len(cfg.Auth.Credentials))
		for i, c := range cfg.Auth.Credentials {
			creds[i] = auth.Credential{Username: c.Username, Password: c.Password}
		}
		authenticator = auth.NewAuthenticator(creds)
	}

	// Create SMTP server
	srv, err := smtpserver.NewServer(&cfg.Server, handler, authenticator, log)
	if err != nil {
		t.Fatalf("Failed to create SMTP server: %v", err)
	}

	// Start server in background
	go func() {
		if err := srv.Start(); err != nil {
			t.Logf("SMTP server stopped: %v", err)
		}
	}()

	return srv
}

func sendTestEmail(t *testing.T, port int, from string, to []string, subject, body string) error {
	t.Helper()

	addr := fmt.Sprintf("localhost:%d", port)
	c, err := smtp.Dial(addr)
	if err != nil {
		return fmt.Errorf("dial failed: %w", err)
	}
	defer c.Close()

	if err := c.Mail(from); err != nil {
		return fmt.Errorf("MAIL FROM failed: %w", err)
	}

	for _, recipient := range to {
		if err := c.Rcpt(recipient); err != nil {
			return fmt.Errorf("RCPT TO failed: %w", err)
		}
	}

	wc, err := c.Data()
	if err != nil {
		return fmt.Errorf("DATA failed: %w", err)
	}

	msg := fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: %s\r\n\r\n%s\r\n", from, strings.Join(to, ","), subject, body)
	if _, err := wc.Write([]byte(msg)); err != nil {
		return fmt.Errorf("write failed: %w", err)
	}

	if err := wc.Close(); err != nil {
		return fmt.Errorf("close failed: %w", err)
	}

	return c.Quit()
}

func sendAuthenticatedEmail(t *testing.T, port int, username, password, from string, to []string, subject, body string) error {
	t.Helper()

	addr := fmt.Sprintf("localhost:%d", port)
	auth := smtp.PlainAuth("", username, password, "localhost")

	msg := fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: %s\r\n\r\n%s\r\n", from, strings.Join(to, ","), subject, body)

	return smtp.SendMail(addr, auth, from, to, []byte(msg))
}

func sendEmailWithSTARTTLS(t *testing.T, port int, from string, to []string, subject, body string) error {
	t.Helper()

	addr := fmt.Sprintf("localhost:%d", port)
	c, err := smtp.Dial(addr)
	if err != nil {
		return fmt.Errorf("dial failed: %w", err)
	}
	defer c.Close()

	// Start TLS
	tlsConfig := &tls.Config{
		InsecureSkipVerify: true, // For testing only
		ServerName:         "localhost",
	}
	if err := c.StartTLS(tlsConfig); err != nil {
		return fmt.Errorf("STARTTLS failed: %w", err)
	}

	if err := c.Mail(from); err != nil {
		return fmt.Errorf("MAIL FROM failed: %w", err)
	}

	for _, recipient := range to {
		if err := c.Rcpt(recipient); err != nil {
			return fmt.Errorf("RCPT TO failed: %w", err)
		}
	}

	wc, err := c.Data()
	if err != nil {
		return fmt.Errorf("DATA failed: %w", err)
	}

	msg := fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: %s\r\n\r\n%s\r\n", from, strings.Join(to, ","), subject, body)
	if _, err := wc.Write([]byte(msg)); err != nil {
		return fmt.Errorf("write failed: %w", err)
	}

	if err := wc.Close(); err != nil {
		return fmt.Errorf("close failed: %w", err)
	}

	return c.Quit()
}

func getFreePort(t *testing.T) int {
	t.Helper()
	listener, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		t.Fatalf("Failed to get free port: %v", err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	listener.Close()
	return port
}

func mustParseInt64(s string) int64 {
	var i int64
	fmt.Sscanf(s, "%d", &i)
	return i
}
