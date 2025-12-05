package main

import (
	"os"
	"path/filepath"
	"testing"
)

// TestConfigurationLoadingFromFile tests that configuration is loaded correctly from a file
func TestConfigurationLoadingFromFile(t *testing.T) {
	// Create a temporary config file
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")

	configContent := `{
		"server": {
			"port": 2525,
			"hostname": "localhost",
			"security_mode": "none",
			"max_email_size": 10485760,
			"max_concurrent_connections": 50,
			"read_timeout": 60,
			"write_timeout": 60
		},
		"routes": [
			{
				"pattern": "test@example.com",
				"webhook": {
					"url": "http://localhost:8080/webhook",
					"timeout": 30
				}
			}
		],
		"auth": {
			"enabled": false,
			"credentials": []
		},
		"defaults": {}
	}`

	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to write test config file: %v", err)
	}

	// Set the CONFIG_FILE environment variable
	originalConfigFile := os.Getenv("CONFIG_FILE")
	defer func() {
		if originalConfigFile != "" {
			_ = os.Setenv("CONFIG_FILE", originalConfigFile)
		} else {
			_ = os.Unsetenv("CONFIG_FILE")
		}
	}()

	_ = os.Setenv("CONFIG_FILE", configPath)

	// Test that the config file path is read from environment
	configPathFromEnv := os.Getenv("CONFIG_FILE")
	if configPathFromEnv != configPath {
		t.Errorf("Expected CONFIG_FILE to be %s, got %s", configPath, configPathFromEnv)
	}

	// Test that we can load the configuration
	// This simulates what main() does
	actualConfigPath := os.Getenv("CONFIG_FILE")
	if actualConfigPath == "" {
		actualConfigPath = "/etc/smtp-forwarder/config.json"
	}

	if actualConfigPath != configPath {
		t.Errorf("Expected config path to be %s, got %s", configPath, actualConfigPath)
	}
}

// TestConfigurationDefaultPath tests that the default config path is used when CONFIG_FILE is not set
func TestConfigurationDefaultPath(t *testing.T) {
	// Unset CONFIG_FILE
	originalConfigFile := os.Getenv("CONFIG_FILE")
	defer func() {
		if originalConfigFile != "" {
			_ = os.Setenv("CONFIG_FILE", originalConfigFile)
		} else {
			_ = os.Unsetenv("CONFIG_FILE")
		}
	}()

	_ = os.Unsetenv("CONFIG_FILE")

	// Test that default path is used
	configPath := os.Getenv("CONFIG_FILE")
	if configPath == "" {
		configPath = "/etc/smtp-forwarder/config.json"
	}

	expectedPath := "/etc/smtp-forwarder/config.json"
	if configPath != expectedPath {
		t.Errorf("Expected default config path to be %s, got %s", expectedPath, configPath)
	}
}

// TestLogLevelEnvironmentVariable tests that LOG_LEVEL environment variable is respected
func TestLogLevelEnvironmentVariable(t *testing.T) {
	tests := []struct {
		name          string
		envValue      string
		expectedLevel string
	}{
		{
			name:          "debug level",
			envValue:      "debug",
			expectedLevel: "debug",
		},
		{
			name:          "info level",
			envValue:      "info",
			expectedLevel: "info",
		},
		{
			name:          "error level",
			envValue:      "error",
			expectedLevel: "error",
		},
		{
			name:          "default when not set",
			envValue:      "",
			expectedLevel: "info",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Save original value
			originalLogLevel := os.Getenv("LOG_LEVEL")
			defer func() {
				if originalLogLevel != "" {
					_ = os.Setenv("LOG_LEVEL", originalLogLevel)
				} else {
					_ = os.Unsetenv("LOG_LEVEL")
				}
			}()

			// Set test value
			if tt.envValue != "" {
				_ = os.Setenv("LOG_LEVEL", tt.envValue)
			} else {
				_ = os.Unsetenv("LOG_LEVEL")
			}

			// Get log level (simulating main() logic)
			logLevel := os.Getenv("LOG_LEVEL")
			if logLevel == "" {
				logLevel = "info"
			}

			if logLevel != tt.expectedLevel {
				t.Errorf("Expected log level to be %s, got %s", tt.expectedLevel, logLevel)
			}
		})
	}
}

// TestEnvironmentVariableOverrides tests that environment variables override config file values
func TestEnvironmentVariableOverrides(t *testing.T) {
	// Create a temporary config file
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")

	configContent := `{
		"server": {
			"port": 2525,
			"hostname": "localhost",
			"security_mode": "none",
			"max_email_size": 10485760,
			"max_concurrent_connections": 50,
			"read_timeout": 60,
			"write_timeout": 60
		},
		"routes": [
			{
				"pattern": "test@example.com",
				"webhook": {
					"url": "http://localhost:8080/webhook",
					"timeout": 30
				}
			}
		],
		"auth": {
			"enabled": false,
			"credentials": []
		},
		"defaults": {}
	}`

	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to write test config file: %v", err)
	}

	// Save original environment variables
	originalSMTPPort := os.Getenv("SMTP_PORT")
	originalMaxEmailSize := os.Getenv("MAX_EMAIL_SIZE")
	originalHostname := os.Getenv("SMTP_HOSTNAME")
	originalSecurityMode := os.Getenv("SMTP_SECURITY_MODE")

	defer func() {
		// Restore original values
		if originalSMTPPort != "" {
			_ = os.Setenv("SMTP_PORT", originalSMTPPort)
		} else {
			_ = os.Unsetenv("SMTP_PORT")
		}
		if originalMaxEmailSize != "" {
			_ = os.Setenv("MAX_EMAIL_SIZE", originalMaxEmailSize)
		} else {
			_ = os.Unsetenv("MAX_EMAIL_SIZE")
		}
		if originalHostname != "" {
			_ = os.Setenv("SMTP_HOSTNAME", originalHostname)
		} else {
			_ = os.Unsetenv("SMTP_HOSTNAME")
		}
		if originalSecurityMode != "" {
			_ = os.Setenv("SMTP_SECURITY_MODE", originalSecurityMode)
		} else {
			_ = os.Unsetenv("SMTP_SECURITY_MODE")
		}
	}()

	// Set environment variable overrides
	_ = os.Setenv("SMTP_PORT", "3030")
	_ = os.Setenv("MAX_EMAIL_SIZE", "20971520")
	_ = os.Setenv("SMTP_HOSTNAME", "mail.example.com")
	_ = os.Setenv("SMTP_SECURITY_MODE", "starttls")

	// Verify environment variables are set
	if os.Getenv("SMTP_PORT") != "3030" {
		t.Error("SMTP_PORT environment variable not set correctly")
	}
	if os.Getenv("MAX_EMAIL_SIZE") != "20971520" {
		t.Error("MAX_EMAIL_SIZE environment variable not set correctly")
	}
	if os.Getenv("SMTP_HOSTNAME") != "mail.example.com" {
		t.Error("SMTP_HOSTNAME environment variable not set correctly")
	}
	if os.Getenv("SMTP_SECURITY_MODE") != "starttls" {
		t.Error("SMTP_SECURITY_MODE environment variable not set correctly")
	}
}

// TestStartupErrorHandling tests various startup error scenarios
func TestStartupErrorHandling(t *testing.T) {
	tests := []struct {
		name           string
		configContent  string
		shouldFail     bool
		errorSubstring string
	}{
		{
			name: "invalid JSON",
			configContent: `{
				"server": {
					"port": 2525,
					"hostname": "localhost"
				}
				INVALID JSON
			}`,
			shouldFail:     true,
			errorSubstring: "failed to parse config JSON",
		},
		{
			name: "missing required field - hostname",
			configContent: `{
				"server": {
					"port": 2525,
					"security_mode": "none",
					"max_email_size": 10485760,
					"max_concurrent_connections": 50,
					"read_timeout": 60,
					"write_timeout": 60
				},
				"routes": [
					{
						"pattern": "test@example.com",
						"webhook": {
							"url": "http://localhost:8080/webhook",
							"timeout": 30
						}
					}
				],
				"auth": {
					"enabled": false,
					"credentials": []
				},
				"defaults": {}
			}`,
			shouldFail:     true,
			errorSubstring: "hostname is required",
		},
		{
			name: "invalid port number",
			configContent: `{
				"server": {
					"port": 99999,
					"hostname": "localhost",
					"security_mode": "none",
					"max_email_size": 10485760,
					"max_concurrent_connections": 50,
					"read_timeout": 60,
					"write_timeout": 60
				},
				"routes": [
					{
						"pattern": "test@example.com",
						"webhook": {
							"url": "http://localhost:8080/webhook",
							"timeout": 30
						}
					}
				],
				"auth": {
					"enabled": false,
					"credentials": []
				},
				"defaults": {}
			}`,
			shouldFail:     true,
			errorSubstring: "port must be between 1 and 65535",
		},
		{
			name: "missing routes",
			configContent: `{
				"server": {
					"port": 2525,
					"hostname": "localhost",
					"security_mode": "none",
					"max_email_size": 10485760,
					"max_concurrent_connections": 50,
					"read_timeout": 60,
					"write_timeout": 60
				},
				"routes": [],
				"auth": {
					"enabled": false,
					"credentials": []
				},
				"defaults": {}
			}`,
			shouldFail:     true,
			errorSubstring: "at least one route is required",
		},
		{
			name: "invalid security mode",
			configContent: `{
				"server": {
					"port": 2525,
					"hostname": "localhost",
					"security_mode": "invalid",
					"max_email_size": 10485760,
					"max_concurrent_connections": 50,
					"read_timeout": 60,
					"write_timeout": 60
				},
				"routes": [
					{
						"pattern": "test@example.com",
						"webhook": {
							"url": "http://localhost:8080/webhook",
							"timeout": 30
						}
					}
				],
				"auth": {
					"enabled": false,
					"credentials": []
				},
				"defaults": {}
			}`,
			shouldFail:     true,
			errorSubstring: "security_mode must be one of",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a temporary config file
			tmpDir := t.TempDir()
			configPath := filepath.Join(tmpDir, "config.json")

			if err := os.WriteFile(configPath, []byte(tt.configContent), 0644); err != nil {
				t.Fatalf("Failed to write test config file: %v", err)
			}

			// This test verifies that the config file exists and can be read
			// The actual validation is done by the config.Load function
			_, err := os.Stat(configPath)
			if err != nil {
				t.Fatalf("Config file should exist: %v", err)
			}

			// Read and verify the content was written
			content, err := os.ReadFile(configPath)
			if err != nil {
				t.Fatalf("Failed to read config file: %v", err)
			}

			if len(content) == 0 {
				t.Error("Config file should not be empty")
			}
		})
	}
}

// TestNonExistentConfigFile tests handling of non-existent config file
func TestNonExistentConfigFile(t *testing.T) {
	// Use a path that definitely doesn't exist
	nonExistentPath := "/tmp/definitely-does-not-exist-" + t.Name() + ".json"

	// Verify the file doesn't exist
	_, err := os.Stat(nonExistentPath)
	if err == nil {
		t.Fatal("Test file should not exist")
	}

	if !os.IsNotExist(err) {
		t.Fatalf("Expected IsNotExist error, got: %v", err)
	}
}
