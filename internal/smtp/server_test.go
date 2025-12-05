package smtp

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"testing"
	"time"

	"smtp-webhook-forwarder/internal/auth"
	"smtp-webhook-forwarder/internal/config"
	"smtp-webhook-forwarder/internal/logger"
	"smtp-webhook-forwarder/internal/parser"
	"smtp-webhook-forwarder/internal/router"
	"smtp-webhook-forwarder/internal/session"
	"smtp-webhook-forwarder/internal/webhook"
)

// createTestCertificates creates temporary TLS certificate and key files for testing
func createTestCertificates(t *testing.T) (certPath, keyPath string, cleanup func()) {
	t.Helper()

	// Create temporary directory
	tmpDir, err := os.MkdirTemp("", "smtp-test-certs-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}

	certPath = filepath.Join(tmpDir, "test.crt")
	keyPath = filepath.Join(tmpDir, "test.key")

	// Generate private key
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		_ = os.RemoveAll(tmpDir)
		t.Fatalf("failed to generate private key: %v", err)
	}

	// Create certificate template
	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			Organization: []string{"Test Org"},
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(24 * time.Hour),
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}

	// Create self-signed certificate
	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &privateKey.PublicKey, privateKey)
	if err != nil {
		_ = os.RemoveAll(tmpDir)
		t.Fatalf("failed to create certificate: %v", err)
	}

	// Write certificate to file
	certFile, err := os.Create(certPath)
	if err != nil {
		_ = os.RemoveAll(tmpDir)
		t.Fatalf("failed to create cert file: %v", err)
	}
	if err := pem.Encode(certFile, &pem.Block{Type: "CERTIFICATE", Bytes: certDER}); err != nil {
		_ = certFile.Close()
		_ = os.RemoveAll(tmpDir)
		t.Fatalf("failed to encode certificate: %v", err)
	}
	_ = certFile.Close()

	// Write private key to file
	keyFile, err := os.Create(keyPath)
	if err != nil {
		_ = os.RemoveAll(tmpDir)
		t.Fatalf("failed to create key file: %v", err)
	}
	privateKeyBytes := x509.MarshalPKCS1PrivateKey(privateKey)
	if err := pem.Encode(keyFile, &pem.Block{Type: "RSA PRIVATE KEY", Bytes: privateKeyBytes}); err != nil {
		_ = keyFile.Close()
		_ = os.RemoveAll(tmpDir)
		t.Fatalf("failed to encode private key: %v", err)
	}
	_ = keyFile.Close()

	cleanup = func() {
		_ = os.RemoveAll(tmpDir)
	}

	return certPath, keyPath, cleanup
}

// createTestHandler creates a minimal handler for testing
func createTestHandler(t *testing.T) *Handler {
	t.Helper()

	// Create minimal dependencies
	p := parser.NewParser()
	r := router.New([]config.Route{}, nil)
	signer := webhook.NewSigner()
	wc := webhook.NewClient(signer)
	sg := session.NewGenerator()
	log := logger.New(bytes.NewBuffer(nil))

	return NewHandler(p, r, wc, sg, log, 10*1024*1024)
}

// TestNewServer_SecurityModeNone tests server initialization with no encryption
func TestNewServer_SecurityModeNone(t *testing.T) {
	cfg := &config.ServerConfig{
		Port:                     2525,
		Hostname:                 "localhost",
		SecurityMode:             "none",
		MaxEmailSize:             10 * 1024 * 1024,
		MaxConcurrentConnections: 10,
		ReadTimeout:              60,
		WriteTimeout:             60,
	}

	handler := createTestHandler(t)
	log := logger.New(bytes.NewBuffer(nil))

	server, err := NewServer(cfg, handler, nil, log)
	if err != nil {
		t.Fatalf("NewServer failed: %v", err)
	}

	if server == nil {
		t.Fatal("expected server to be created")
	}

	if server.smtpServer.Addr != ":2525" {
		t.Errorf("expected addr :2525, got %s", server.smtpServer.Addr)
	}

	if server.smtpServer.Domain != "localhost" {
		t.Errorf("expected domain localhost, got %s", server.smtpServer.Domain)
	}

	if server.smtpServer.ReadTimeout != 60*time.Second {
		t.Errorf("expected read timeout 60s, got %v", server.smtpServer.ReadTimeout)
	}

	if server.smtpServer.WriteTimeout != 60*time.Second {
		t.Errorf("expected write timeout 60s, got %v", server.smtpServer.WriteTimeout)
	}

	if server.smtpServer.MaxMessageBytes != 10*1024*1024 {
		t.Errorf("expected max message bytes 10MB, got %d", server.smtpServer.MaxMessageBytes)
	}

	if !server.smtpServer.AllowInsecureAuth {
		t.Error("expected AllowInsecureAuth to be true for security mode 'none'")
	}

	if server.smtpServer.TLSConfig != nil {
		t.Error("expected TLSConfig to be nil for security mode 'none'")
	}
}

// TestNewServer_SecurityModeTLS tests server initialization with TLS encryption
func TestNewServer_SecurityModeTLS(t *testing.T) {
	certPath, keyPath, cleanup := createTestCertificates(t)
	defer cleanup()

	cfg := &config.ServerConfig{
		Port:                     2525,
		Hostname:                 "localhost",
		SecurityMode:             "tls",
		TLSCertPath:              certPath,
		TLSKeyPath:               keyPath,
		MaxEmailSize:             10 * 1024 * 1024,
		MaxConcurrentConnections: 10,
		ReadTimeout:              60,
		WriteTimeout:             60,
	}

	handler := createTestHandler(t)
	log := logger.New(bytes.NewBuffer(nil))

	server, err := NewServer(cfg, handler, nil, log)
	if err != nil {
		t.Fatalf("NewServer failed: %v", err)
	}

	if server == nil {
		t.Fatal("expected server to be created")
	}

	if server.smtpServer.TLSConfig == nil {
		t.Fatal("expected TLSConfig to be set for TLS mode")
	}

	if len(server.smtpServer.TLSConfig.Certificates) != 1 {
		t.Errorf("expected 1 certificate, got %d", len(server.smtpServer.TLSConfig.Certificates))
	}

	if server.smtpServer.AllowInsecureAuth {
		t.Error("expected AllowInsecureAuth to be false for TLS mode")
	}
}

// TestNewServer_SecurityModeSSL tests server initialization with SSL encryption
func TestNewServer_SecurityModeSSL(t *testing.T) {
	certPath, keyPath, cleanup := createTestCertificates(t)
	defer cleanup()

	cfg := &config.ServerConfig{
		Port:                     2525,
		Hostname:                 "localhost",
		SecurityMode:             "ssl",
		TLSCertPath:              certPath,
		TLSKeyPath:               keyPath,
		MaxEmailSize:             10 * 1024 * 1024,
		MaxConcurrentConnections: 10,
		ReadTimeout:              60,
		WriteTimeout:             60,
	}

	handler := createTestHandler(t)
	log := logger.New(bytes.NewBuffer(nil))

	server, err := NewServer(cfg, handler, nil, log)
	if err != nil {
		t.Fatalf("NewServer failed: %v", err)
	}

	if server == nil {
		t.Fatal("expected server to be created")
	}

	if server.smtpServer.TLSConfig == nil {
		t.Fatal("expected TLSConfig to be set for SSL mode")
	}

	if len(server.smtpServer.TLSConfig.Certificates) != 1 {
		t.Errorf("expected 1 certificate, got %d", len(server.smtpServer.TLSConfig.Certificates))
	}
}

// TestNewServer_SecurityModeSTARTTLS tests server initialization with STARTTLS
func TestNewServer_SecurityModeSTARTTLS(t *testing.T) {
	certPath, keyPath, cleanup := createTestCertificates(t)
	defer cleanup()

	cfg := &config.ServerConfig{
		Port:                     2525,
		Hostname:                 "localhost",
		SecurityMode:             "starttls",
		TLSCertPath:              certPath,
		TLSKeyPath:               keyPath,
		MaxEmailSize:             10 * 1024 * 1024,
		MaxConcurrentConnections: 10,
		ReadTimeout:              60,
		WriteTimeout:             60,
	}

	handler := createTestHandler(t)
	log := logger.New(bytes.NewBuffer(nil))

	server, err := NewServer(cfg, handler, nil, log)
	if err != nil {
		t.Fatalf("NewServer failed: %v", err)
	}

	if server == nil {
		t.Fatal("expected server to be created")
	}

	if server.smtpServer.TLSConfig == nil {
		t.Fatal("expected TLSConfig to be set for STARTTLS mode")
	}

	if len(server.smtpServer.TLSConfig.Certificates) != 1 {
		t.Errorf("expected 1 certificate, got %d", len(server.smtpServer.TLSConfig.Certificates))
	}
}

// TestNewServer_TLSCertificateLoadingFailure tests that server creation fails with invalid certificate paths
func TestNewServer_TLSCertificateLoadingFailure(t *testing.T) {
	cfg := &config.ServerConfig{
		Port:                     2525,
		Hostname:                 "localhost",
		SecurityMode:             "tls",
		TLSCertPath:              "/nonexistent/cert.pem",
		TLSKeyPath:               "/nonexistent/key.pem",
		MaxEmailSize:             10 * 1024 * 1024,
		MaxConcurrentConnections: 10,
		ReadTimeout:              60,
		WriteTimeout:             60,
	}

	handler := createTestHandler(t)
	log := logger.New(bytes.NewBuffer(nil))

	server, err := NewServer(cfg, handler, nil, log)
	if err == nil {
		t.Fatal("expected error when loading nonexistent certificates")
	}

	if server != nil {
		t.Error("expected server to be nil when certificate loading fails")
	}
}

// TestNewServer_WithAuthentication tests server initialization with authentication enabled
func TestNewServer_WithAuthentication(t *testing.T) {
	cfg := &config.ServerConfig{
		Port:                     2525,
		Hostname:                 "localhost",
		SecurityMode:             "none",
		MaxEmailSize:             10 * 1024 * 1024,
		MaxConcurrentConnections: 10,
		ReadTimeout:              60,
		WriteTimeout:             60,
	}

	handler := createTestHandler(t)
	log := logger.New(bytes.NewBuffer(nil))

	// Create authenticator with test credentials
	authenticator := auth.NewAuthenticator([]auth.Credential{
		{Username: "testuser", Password: "testpass"},
	})

	server, err := NewServer(cfg, handler, authenticator, log)
	if err != nil {
		t.Fatalf("NewServer failed: %v", err)
	}

	if server == nil {
		t.Fatal("expected server to be created")
	}

	if server.authenticator == nil {
		t.Error("expected authenticator to be set")
	}
}

// TestNewServer_WithoutAuthentication tests server initialization without authentication
func TestNewServer_WithoutAuthentication(t *testing.T) {
	cfg := &config.ServerConfig{
		Port:                     2525,
		Hostname:                 "localhost",
		SecurityMode:             "none",
		MaxEmailSize:             10 * 1024 * 1024,
		MaxConcurrentConnections: 10,
		ReadTimeout:              60,
		WriteTimeout:             60,
	}

	handler := createTestHandler(t)
	log := logger.New(bytes.NewBuffer(nil))

	server, err := NewServer(cfg, handler, nil, log)
	if err != nil {
		t.Fatalf("NewServer failed: %v", err)
	}

	if server == nil {
		t.Fatal("expected server to be created")
	}

	if server.authenticator != nil {
		t.Error("expected authenticator to be nil when not provided")
	}
}

// TestNewServer_TimeoutConfiguration tests that timeouts are properly configured
func TestNewServer_TimeoutConfiguration(t *testing.T) {
	tests := []struct {
		name         string
		readTimeout  int
		writeTimeout int
	}{
		{"default timeouts", 60, 60},
		{"custom timeouts", 30, 45},
		{"short timeouts", 10, 10},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.ServerConfig{
				Port:                     2525,
				Hostname:                 "localhost",
				SecurityMode:             "none",
				MaxEmailSize:             10 * 1024 * 1024,
				MaxConcurrentConnections: 10,
				ReadTimeout:              tt.readTimeout,
				WriteTimeout:             tt.writeTimeout,
			}

			handler := createTestHandler(t)
			log := logger.New(bytes.NewBuffer(nil))

			server, err := NewServer(cfg, handler, nil, log)
			if err != nil {
				t.Fatalf("NewServer failed: %v", err)
			}

			expectedReadTimeout := time.Duration(tt.readTimeout) * time.Second
			if server.smtpServer.ReadTimeout != expectedReadTimeout {
				t.Errorf("expected read timeout %v, got %v", expectedReadTimeout, server.smtpServer.ReadTimeout)
			}

			expectedWriteTimeout := time.Duration(tt.writeTimeout) * time.Second
			if server.smtpServer.WriteTimeout != expectedWriteTimeout {
				t.Errorf("expected write timeout %v, got %v", expectedWriteTimeout, server.smtpServer.WriteTimeout)
			}
		})
	}
}

// TestNewServer_InvalidSecurityMode tests that server creation fails with invalid security mode
func TestNewServer_InvalidSecurityMode(t *testing.T) {
	cfg := &config.ServerConfig{
		Port:                     2525,
		Hostname:                 "localhost",
		SecurityMode:             "invalid",
		MaxEmailSize:             10 * 1024 * 1024,
		MaxConcurrentConnections: 10,
		ReadTimeout:              60,
		WriteTimeout:             60,
	}

	handler := createTestHandler(t)
	log := logger.New(bytes.NewBuffer(nil))

	server, err := NewServer(cfg, handler, nil, log)
	if err == nil {
		t.Fatal("expected error with invalid security mode")
	}

	if server != nil {
		t.Error("expected server to be nil with invalid security mode")
	}
}

// TestNewServer_ConcurrentConnectionLimit tests that concurrent connection limit is properly set
func TestNewServer_ConcurrentConnectionLimit(t *testing.T) {
	tests := []struct {
		name     string
		maxConns int
	}{
		{"low limit", 5},
		{"medium limit", 50},
		{"high limit", 100},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.ServerConfig{
				Port:                     2525,
				Hostname:                 "localhost",
				SecurityMode:             "none",
				MaxEmailSize:             10 * 1024 * 1024,
				MaxConcurrentConnections: tt.maxConns,
				ReadTimeout:              60,
				WriteTimeout:             60,
			}

			handler := createTestHandler(t)
			log := logger.New(bytes.NewBuffer(nil))

			server, err := NewServer(cfg, handler, nil, log)
			if err != nil {
				t.Fatalf("NewServer failed: %v", err)
			}

			if cap(server.semaphore) != tt.maxConns {
				t.Errorf("expected semaphore capacity %d, got %d", tt.maxConns, cap(server.semaphore))
			}
		})
	}
}
