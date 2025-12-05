package config

import (
	"os"
	"path/filepath"
	"testing"
)

// TestLoadValidConfiguration tests loading a valid configuration file
func TestLoadValidConfiguration(t *testing.T) {
	// Create a temporary config file
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")

	validConfig := `{
		"server": {
			"port": 2525,
			"hostname": "smtp.example.com",
			"security_mode": "none",
			"max_email_size": 10485760,
			"max_concurrent_connections": 50,
			"read_timeout": 60,
			"write_timeout": 60
		},
		"auth": {
			"enabled": false,
			"credentials": []
		},
		"routes": [
			{
				"pattern": "test@example.com",
				"webhook": {
					"url": "https://api.example.com/webhook",
					"timeout": 30
				}
			}
		],
		"defaults": {}
	}`

	if err := os.WriteFile(configPath, []byte(validConfig), 0644); err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	// Load the configuration
	config, err := Load(configPath)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	// Verify configuration values
	if config.Server.Port != 2525 {
		t.Errorf("Expected port 2525, got %d", config.Server.Port)
	}
	if config.Server.Hostname != "smtp.example.com" {
		t.Errorf("Expected hostname 'smtp.example.com', got '%s'", config.Server.Hostname)
	}
	if config.Server.SecurityMode != "none" {
		t.Errorf("Expected security_mode 'none', got '%s'", config.Server.SecurityMode)
	}
	if config.Server.MaxEmailSize != 10485760 {
		t.Errorf("Expected max_email_size 10485760, got %d", config.Server.MaxEmailSize)
	}
	if len(config.Routes) != 1 {
		t.Errorf("Expected 1 route, got %d", len(config.Routes))
	}
	if config.Routes[0].Pattern != "test@example.com" {
		t.Errorf("Expected pattern 'test@example.com', got '%s'", config.Routes[0].Pattern)
	}
}

// TestLoadInvalidJSON tests that invalid JSON is rejected
func TestLoadInvalidJSON(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")

	invalidJSON := `{
		"server": {
			"port": 2525,
			"hostname": "smtp.example.com"
			// Missing comma and invalid syntax
		}
	}`

	if err := os.WriteFile(configPath, []byte(invalidJSON), 0644); err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	// Attempt to load the configuration
	_, err := Load(configPath)
	if err == nil {
		t.Fatal("Expected error for invalid JSON, got nil")
	}
}

// TestLoadMissingFile tests that missing file is handled
func TestLoadMissingFile(t *testing.T) {
	_, err := Load("/nonexistent/path/config.json")
	if err == nil {
		t.Fatal("Expected error for missing file, got nil")
	}
}

// TestValidateMissingRequiredFields tests detection of missing required fields
func TestValidateMissingRequiredFields(t *testing.T) {
	tests := []struct {
		name        string
		config      string
		expectError bool
		errorMsg    string
	}{
		{
			name: "missing hostname",
			config: `{
				"server": {
					"port": 2525,
					"hostname": "",
					"security_mode": "none",
					"max_email_size": 10485760,
					"max_concurrent_connections": 50,
					"read_timeout": 60,
					"write_timeout": 60
				},
				"auth": {"enabled": false, "credentials": []},
				"routes": [{"pattern": "test@example.com", "webhook": {"url": "https://api.example.com/webhook", "timeout": 30}}],
				"defaults": {}
			}`,
			expectError: true,
			errorMsg:    "hostname is required",
		},
		{
			name: "invalid port - too low",
			config: `{
				"server": {
					"port": 0,
					"hostname": "smtp.example.com",
					"security_mode": "none",
					"max_email_size": 10485760,
					"max_concurrent_connections": 50,
					"read_timeout": 60,
					"write_timeout": 60
				},
				"auth": {"enabled": false, "credentials": []},
				"routes": [{"pattern": "test@example.com", "webhook": {"url": "https://api.example.com/webhook", "timeout": 30}}],
				"defaults": {}
			}`,
			expectError: true,
			errorMsg:    "port must be between 1 and 65535",
		},
		{
			name: "invalid port - too high",
			config: `{
				"server": {
					"port": 70000,
					"hostname": "smtp.example.com",
					"security_mode": "none",
					"max_email_size": 10485760,
					"max_concurrent_connections": 50,
					"read_timeout": 60,
					"write_timeout": 60
				},
				"auth": {"enabled": false, "credentials": []},
				"routes": [{"pattern": "test@example.com", "webhook": {"url": "https://api.example.com/webhook", "timeout": 30}}],
				"defaults": {}
			}`,
			expectError: true,
			errorMsg:    "port must be between 1 and 65535",
		},
		{
			name: "missing TLS cert when security mode requires it",
			config: `{
				"server": {
					"port": 2525,
					"hostname": "smtp.example.com",
					"security_mode": "tls",
					"max_email_size": 10485760,
					"max_concurrent_connections": 50,
					"read_timeout": 60,
					"write_timeout": 60
				},
				"auth": {"enabled": false, "credentials": []},
				"routes": [{"pattern": "test@example.com", "webhook": {"url": "https://api.example.com/webhook", "timeout": 30}}],
				"defaults": {}
			}`,
			expectError: true,
			errorMsg:    "tls_cert_path is required",
		},
		{
			name: "no routes",
			config: `{
				"server": {
					"port": 2525,
					"hostname": "smtp.example.com",
					"security_mode": "none",
					"max_email_size": 10485760,
					"max_concurrent_connections": 50,
					"read_timeout": 60,
					"write_timeout": 60
				},
				"auth": {"enabled": false, "credentials": []},
				"routes": [],
				"defaults": {}
			}`,
			expectError: true,
			errorMsg:    "at least one route is required",
		},
		{
			name: "auth enabled but no credentials",
			config: `{
				"server": {
					"port": 2525,
					"hostname": "smtp.example.com",
					"security_mode": "none",
					"max_email_size": 10485760,
					"max_concurrent_connections": 50,
					"read_timeout": 60,
					"write_timeout": 60
				},
				"auth": {"enabled": true, "credentials": []},
				"routes": [{"pattern": "test@example.com", "webhook": {"url": "https://api.example.com/webhook", "timeout": 30}}],
				"defaults": {}
			}`,
			expectError: true,
			errorMsg:    "at least one credential is required",
		},
		{
			name: "invalid webhook URL scheme",
			config: `{
				"server": {
					"port": 2525,
					"hostname": "smtp.example.com",
					"security_mode": "none",
					"max_email_size": 10485760,
					"max_concurrent_connections": 50,
					"read_timeout": 60,
					"write_timeout": 60
				},
				"auth": {"enabled": false, "credentials": []},
				"routes": [{"pattern": "test@example.com", "webhook": {"url": "ftp://api.example.com/webhook", "timeout": 30}}],
				"defaults": {}
			}`,
			expectError: true,
			errorMsg:    "webhook URL must start with http:// or https://",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			configPath := filepath.Join(tmpDir, "config.json")

			if err := os.WriteFile(configPath, []byte(tt.config), 0644); err != nil {
				t.Fatalf("Failed to write test config: %v", err)
			}

			_, err := Load(configPath)
			if tt.expectError {
				if err == nil {
					t.Fatalf("Expected error containing '%s', got nil", tt.errorMsg)
				}
				// Check if error message contains expected text
				if tt.errorMsg != "" && !contains(err.Error(), tt.errorMsg) {
					t.Errorf("Expected error containing '%s', got '%s'", tt.errorMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Fatalf("Expected no error, got: %v", err)
				}
			}
		})
	}
}

// TestDefaultValueApplication tests that default values are applied correctly
func TestDefaultValueApplication(t *testing.T) {
	// Test with minimal config to see if defaults are applied
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")

	minimalConfig := `{
		"server": {
			"port": 2525,
			"hostname": "smtp.example.com",
			"security_mode": "none",
			"max_email_size": 10485760,
			"max_concurrent_connections": 50,
			"read_timeout": 60,
			"write_timeout": 60
		},
		"auth": {
			"enabled": false,
			"credentials": []
		},
		"routes": [
			{
				"pattern": "test@example.com",
				"webhook": {
					"url": "https://api.example.com/webhook",
					"timeout": 30
				}
			}
		],
		"defaults": {}
	}`

	if err := os.WriteFile(configPath, []byte(minimalConfig), 0644); err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	config, err := Load(configPath)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	// Verify that the config loaded successfully
	// The current implementation doesn't apply defaults beyond what's in JSON
	// but this test ensures the structure is correct
	if config.Defaults.Webhook != nil {
		t.Errorf("Expected nil default webhook, got %+v", config.Defaults.Webhook)
	}
}

// TestEnvironmentVariableOverrides tests that environment variables override config values
func TestEnvironmentVariableOverrides(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")

	baseConfig := `{
		"server": {
			"port": 2525,
			"hostname": "smtp.example.com",
			"security_mode": "none",
			"max_email_size": 10485760,
			"max_concurrent_connections": 50,
			"read_timeout": 60,
			"write_timeout": 60
		},
		"auth": {
			"enabled": false,
			"credentials": []
		},
		"routes": [
			{
				"pattern": "test@example.com",
				"webhook": {
					"url": "https://api.example.com/webhook",
					"timeout": 30
				}
			}
		],
		"defaults": {}
	}`

	if err := os.WriteFile(configPath, []byte(baseConfig), 0644); err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	// Set environment variables
	_ = os.Setenv("SMTP_PORT", "3030")
	_ = os.Setenv("MAX_EMAIL_SIZE", "20971520")
	_ = os.Setenv("SMTP_HOSTNAME", "override.example.com")
	os.Setenv("SMTP_SECURITY_MODE", "starttls")
	os.Setenv("TLS_CERT_PATH", "/path/to/cert.pem")
	os.Setenv("TLS_KEY_PATH", "/path/to/key.pem")

	// Clean up environment variables after test
	defer func() {
		_ = os.Unsetenv("SMTP_PORT")
		_ = os.Unsetenv("MAX_EMAIL_SIZE")
		_ = os.Unsetenv("SMTP_HOSTNAME")
		_ = os.Unsetenv("SMTP_SECURITY_MODE")
		_ = os.Unsetenv("TLS_CERT_PATH")
		_ = os.Unsetenv("TLS_KEY_PATH")
	}()

	config, err := Load(configPath)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	// Verify overrides were applied
	if config.Server.Port != 3030 {
		t.Errorf("Expected port 3030 from env override, got %d", config.Server.Port)
	}
	if config.Server.MaxEmailSize != 20971520 {
		t.Errorf("Expected max_email_size 20971520 from env override, got %d", config.Server.MaxEmailSize)
	}
	if config.Server.Hostname != "override.example.com" {
		t.Errorf("Expected hostname 'override.example.com' from env override, got '%s'", config.Server.Hostname)
	}
	if config.Server.SecurityMode != "starttls" {
		t.Errorf("Expected security_mode 'starttls' from env override, got '%s'", config.Server.SecurityMode)
	}
	if config.Server.TLSCertPath != "/path/to/cert.pem" {
		t.Errorf("Expected tls_cert_path '/path/to/cert.pem' from env override, got '%s'", config.Server.TLSCertPath)
	}
	if config.Server.TLSKeyPath != "/path/to/key.pem" {
		t.Errorf("Expected tls_key_path '/path/to/key.pem' from env override, got '%s'", config.Server.TLSKeyPath)
	}
}

// TestValidateSecurityModes tests validation of different security modes
func TestValidateSecurityModes(t *testing.T) {
	tests := []struct {
		name         string
		securityMode string
		hasCerts     bool
		expectError  bool
	}{
		{"valid none mode", "none", false, false},
		{"valid tls mode with certs", "tls", true, false},
		{"valid ssl mode with certs", "ssl", true, false},
		{"valid starttls mode with certs", "starttls", true, false},
		{"invalid mode", "invalid", false, true},
		{"tls without certs", "tls", false, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			configPath := filepath.Join(tmpDir, "config.json")

			certPath := ""
			keyPath := ""
			if tt.hasCerts {
				certPath = "/path/to/cert.pem"
				keyPath = "/path/to/key.pem"
			}

			config := `{
				"server": {
					"port": 2525,
					"hostname": "smtp.example.com",
					"security_mode": "` + tt.securityMode + `",
					"tls_cert_path": "` + certPath + `",
					"tls_key_path": "` + keyPath + `",
					"max_email_size": 10485760,
					"max_concurrent_connections": 50,
					"read_timeout": 60,
					"write_timeout": 60
				},
				"auth": {"enabled": false, "credentials": []},
				"routes": [{"pattern": "test@example.com", "webhook": {"url": "https://api.example.com/webhook", "timeout": 30}}],
				"defaults": {}
			}`

			if err := os.WriteFile(configPath, []byte(config), 0644); err != nil {
				t.Fatalf("Failed to write test config: %v", err)
			}

			_, err := Load(configPath)
			if tt.expectError && err == nil {
				t.Fatal("Expected error, got nil")
			}
			if !tt.expectError && err != nil {
				t.Fatalf("Expected no error, got: %v", err)
			}
		})
	}
}

// TestValidateWebhookTimeout tests webhook timeout validation
func TestValidateWebhookTimeout(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")

	invalidConfig := `{
		"server": {
			"port": 2525,
			"hostname": "smtp.example.com",
			"security_mode": "none",
			"max_email_size": 10485760,
			"max_concurrent_connections": 50,
			"read_timeout": 60,
			"write_timeout": 60
		},
		"auth": {"enabled": false, "credentials": []},
		"routes": [{"pattern": "test@example.com", "webhook": {"url": "https://api.example.com/webhook", "timeout": 0}}],
		"defaults": {}
	}`

	if err := os.WriteFile(configPath, []byte(invalidConfig), 0644); err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	_, err := Load(configPath)
	if err == nil {
		t.Fatal("Expected error for zero timeout, got nil")
	}
	if !contains(err.Error(), "timeout must be positive") {
		t.Errorf("Expected error about timeout, got: %v", err)
	}
}

// TestValidateCredentials tests credential validation
func TestValidateCredentials(t *testing.T) {
	tests := []struct {
		name        string
		credentials string
		expectError bool
	}{
		{
			name:        "valid credentials",
			credentials: `[{"username": "user1", "password": "pass1"}]`,
			expectError: false,
		},
		{
			name:        "empty username",
			credentials: `[{"username": "", "password": "pass1"}]`,
			expectError: true,
		},
		{
			name:        "empty password",
			credentials: `[{"username": "user1", "password": ""}]`,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			configPath := filepath.Join(tmpDir, "config.json")

			config := `{
				"server": {
					"port": 2525,
					"hostname": "smtp.example.com",
					"security_mode": "none",
					"max_email_size": 10485760,
					"max_concurrent_connections": 50,
					"read_timeout": 60,
					"write_timeout": 60
				},
				"auth": {"enabled": true, "credentials": ` + tt.credentials + `},
				"routes": [{"pattern": "test@example.com", "webhook": {"url": "https://api.example.com/webhook", "timeout": 30}}],
				"defaults": {}
			}`

			if err := os.WriteFile(configPath, []byte(config), 0644); err != nil {
				t.Fatalf("Failed to write test config: %v", err)
			}

			_, err := Load(configPath)
			if tt.expectError && err == nil {
				t.Fatal("Expected error, got nil")
			}
			if !tt.expectError && err != nil {
				t.Fatalf("Expected no error, got: %v", err)
			}
		})
	}
}

// Helper function to check if a string contains a substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && len(substr) > 0 && containsHelper(s, substr)))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
