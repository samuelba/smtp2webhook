package config

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// Config represents the complete service configuration
type Config struct {
	Server   ServerConfig  `json:"server"`
	Routes   []Route       `json:"routes"`
	Auth     AuthConfig    `json:"auth"`
	Defaults DefaultConfig `json:"defaults"`
}

// ServerConfig contains SMTP server configuration
type ServerConfig struct {
	Port                     int    `json:"port"`
	Hostname                 string `json:"hostname"`
	SecurityMode             string `json:"security_mode"` // "tls", "ssl", "starttls", "none"
	TLSCertPath              string `json:"tls_cert_path,omitempty"`
	TLSKeyPath               string `json:"tls_key_path,omitempty"`
	MaxEmailSize             int64  `json:"max_email_size"`             // bytes
	MaxConcurrentConnections int    `json:"max_concurrent_connections"` // limit concurrent SMTP sessions
	ReadTimeout              int    `json:"read_timeout"`               // seconds
	WriteTimeout             int    `json:"write_timeout"`              // seconds
}

// Route defines an email routing rule
type Route struct {
	Pattern string  `json:"pattern"` // email pattern: "user@domain.com" or "*@domain.com"
	Webhook Webhook `json:"webhook"`
}

// Webhook defines a webhook endpoint configuration
type Webhook struct {
	URL     string `json:"url"`
	Secret  string `json:"secret,omitempty"`
	Timeout int    `json:"timeout"` // seconds
}

// AuthConfig contains authentication configuration
type AuthConfig struct {
	Enabled     bool         `json:"enabled"`
	Credentials []Credential `json:"credentials"`
}

// Credential represents a username/password pair
type Credential struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// DefaultConfig contains default webhook configuration
type DefaultConfig struct {
	Webhook *Webhook `json:"webhook,omitempty"`
}

// Load reads and parses the configuration file from the given path
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config Config
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config JSON: %w", err)
	}

	// Apply environment variable overrides
	applyEnvOverrides(&config)

	// Validate the configuration
	if err := Validate(&config); err != nil {
		return nil, fmt.Errorf("config validation failed: %w", err)
	}

	return &config, nil
}

// applyEnvOverrides applies environment variable overrides to the configuration
func applyEnvOverrides(config *Config) {
	// Override SMTP port
	if portStr := os.Getenv("SMTP_PORT"); portStr != "" {
		if port, err := strconv.Atoi(portStr); err == nil {
			config.Server.Port = port
		}
	}

	// Override max email size
	if sizeStr := os.Getenv("MAX_EMAIL_SIZE"); sizeStr != "" {
		if size, err := strconv.ParseInt(sizeStr, 10, 64); err == nil {
			config.Server.MaxEmailSize = size
		}
	}

	// Override hostname
	if hostname := os.Getenv("SMTP_HOSTNAME"); hostname != "" {
		config.Server.Hostname = hostname
	}

	// Override security mode
	if secMode := os.Getenv("SMTP_SECURITY_MODE"); secMode != "" {
		config.Server.SecurityMode = secMode
	}

	// Override TLS cert path
	if certPath := os.Getenv("TLS_CERT_PATH"); certPath != "" {
		config.Server.TLSCertPath = certPath
	}

	// Override TLS key path
	if keyPath := os.Getenv("TLS_KEY_PATH"); keyPath != "" {
		config.Server.TLSKeyPath = keyPath
	}
}

// Validate checks if the configuration is valid
func Validate(config *Config) error {
	// Validate server configuration
	if err := validateServer(&config.Server); err != nil {
		return fmt.Errorf("server config: %w", err)
	}

	// Validate routes
	if err := validateRoutes(config.Routes); err != nil {
		return fmt.Errorf("routes config: %w", err)
	}

	// Validate auth configuration
	if err := validateAuth(&config.Auth); err != nil {
		return fmt.Errorf("auth config: %w", err)
	}

	// Validate defaults
	if err := validateDefaults(&config.Defaults); err != nil {
		return fmt.Errorf("defaults config: %w", err)
	}

	return nil
}

// validateServer validates server configuration
func validateServer(server *ServerConfig) error {
	// Validate port
	if server.Port < 1 || server.Port > 65535 {
		return fmt.Errorf("port must be between 1 and 65535, got %d", server.Port)
	}

	// Validate hostname (required)
	if strings.TrimSpace(server.Hostname) == "" {
		return fmt.Errorf("hostname is required")
	}

	// Validate security mode
	validModes := map[string]bool{
		"tls":      true,
		"ssl":      true,
		"starttls": true,
		"none":     true,
	}
	if !validModes[server.SecurityMode] {
		return fmt.Errorf("security_mode must be one of: tls, ssl, starttls, none; got %s", server.SecurityMode)
	}

	// Validate TLS configuration
	if server.SecurityMode != "none" {
		if strings.TrimSpace(server.TLSCertPath) == "" {
			return fmt.Errorf("tls_cert_path is required when security_mode is %s", server.SecurityMode)
		}
		if strings.TrimSpace(server.TLSKeyPath) == "" {
			return fmt.Errorf("tls_key_path is required when security_mode is %s", server.SecurityMode)
		}
	}

	// Validate max email size (must be positive)
	if server.MaxEmailSize <= 0 {
		return fmt.Errorf("max_email_size must be positive, got %d", server.MaxEmailSize)
	}

	// Validate max concurrent connections (must be positive)
	if server.MaxConcurrentConnections <= 0 {
		return fmt.Errorf("max_concurrent_connections must be positive, got %d", server.MaxConcurrentConnections)
	}

	// Validate timeouts (must be positive)
	if server.ReadTimeout <= 0 {
		return fmt.Errorf("read_timeout must be positive, got %d", server.ReadTimeout)
	}
	if server.WriteTimeout <= 0 {
		return fmt.Errorf("write_timeout must be positive, got %d", server.WriteTimeout)
	}

	return nil
}

// validateRoutes validates routing configuration
func validateRoutes(routes []Route) error {
	if len(routes) == 0 {
		return fmt.Errorf("at least one route is required")
	}

	for i, route := range routes {
		if strings.TrimSpace(route.Pattern) == "" {
			return fmt.Errorf("route %d: pattern is required", i)
		}

		if err := validateWebhook(&route.Webhook); err != nil {
			return fmt.Errorf("route %d: %w", i, err)
		}
	}

	return nil
}

// validateWebhook validates webhook configuration
func validateWebhook(webhook *Webhook) error {
	if strings.TrimSpace(webhook.URL) == "" {
		return fmt.Errorf("webhook URL is required")
	}

	// Validate URL scheme
	if !strings.HasPrefix(webhook.URL, "http://") && !strings.HasPrefix(webhook.URL, "https://") {
		return fmt.Errorf("webhook URL must start with http:// or https://, got %s", webhook.URL)
	}

	// Validate timeout (must be positive)
	if webhook.Timeout <= 0 {
		return fmt.Errorf("webhook timeout must be positive, got %d", webhook.Timeout)
	}

	return nil
}

// validateAuth validates authentication configuration
func validateAuth(auth *AuthConfig) error {
	if auth.Enabled {
		if len(auth.Credentials) == 0 {
			return fmt.Errorf("at least one credential is required when auth is enabled")
		}

		for i, cred := range auth.Credentials {
			if strings.TrimSpace(cred.Username) == "" {
				return fmt.Errorf("credential %d: username is required", i)
			}
			if strings.TrimSpace(cred.Password) == "" {
				return fmt.Errorf("credential %d: password is required", i)
			}
		}
	}

	return nil
}

// validateDefaults validates default configuration
func validateDefaults(defaults *DefaultConfig) error {
	if defaults.Webhook != nil {
		if err := validateWebhook(defaults.Webhook); err != nil {
			return fmt.Errorf("default webhook: %w", err)
		}
	}

	return nil
}
