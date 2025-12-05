package webhook

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"smtp-webhook-forwarder/internal/parser"
)

// WebhookPayload represents the JSON payload sent to webhook endpoints
type WebhookPayload struct {
	SessionID string              `json:"session_id"`
	Timestamp int64               `json:"timestamp"`
	Email     *parser.ParsedEmail `json:"email"`
}

// WebhookResponse represents the response from a webhook endpoint
type WebhookResponse struct {
	StatusCode int
	Body       []byte
	Error      error
}

// WebhookConfig represents the configuration for a webhook endpoint
type WebhookConfig struct {
	URL     string
	Secret  string
	Timeout int // seconds
}

// Client defines the interface for sending webhook requests
type Client interface {
	Send(ctx context.Context, webhook WebhookConfig, payload *WebhookPayload) (*WebhookResponse, error)
}

// HTTPClient implements the Client interface using HTTP POST requests
type HTTPClient struct {
	signer     Signer
	httpClient *http.Client
}

// NewClient creates a new webhook HTTP client
func NewClient(signer Signer) *HTTPClient {
	return &HTTPClient{
		signer: signer,
		httpClient: &http.Client{
			// Default timeout, will be overridden per request
			Timeout: 30 * time.Second,
		},
	}
}

// Send sends a webhook request to the configured endpoint
// It implements the Standard Webhooks specification:
// - webhook-id: unique message identifier (session ID)
// - webhook-timestamp: Unix timestamp in seconds
// - webhook-signature: HMAC-SHA256 signature (if secret present)
// - Content-Type: application/json
//
// Requirements: 2.2, 3.1, 3.2, 3.3, 3.4, 3.6
func (c *HTTPClient) Send(ctx context.Context, webhook WebhookConfig, payload *WebhookPayload) (*WebhookResponse, error) {
	// Serialize payload to JSON
	bodyBytes, err := json.Marshal(payload)
	if err != nil {
		return &WebhookResponse{
			Error: fmt.Errorf("failed to marshal payload: %w", err),
		}, nil
	}

	// Create HTTP request
	req, err := http.NewRequestWithContext(ctx, "POST", webhook.URL, bytes.NewReader(bodyBytes))
	if err != nil {
		return &WebhookResponse{
			Error: fmt.Errorf("failed to create request: %w", err),
		}, nil
	}

	// Set Content-Type header
	req.Header.Set("Content-Type", "application/json")

	// Set Standard Webhooks headers
	// webhook-id: unique message identifier (session ID)
	req.Header.Set("webhook-id", payload.SessionID)

	// webhook-timestamp: Unix timestamp in seconds
	req.Header.Set("webhook-timestamp", fmt.Sprintf("%d", payload.Timestamp))

	// webhook-signature: HMAC-SHA256 signature (if secret present)
	if webhook.Secret != "" {
		signature, err := c.signer.Sign(webhook.Secret, payload.SessionID, payload.Timestamp, bodyBytes)
		if err != nil {
			return &WebhookResponse{
				Error: fmt.Errorf("failed to generate signature: %w", err),
			}, nil
		}
		req.Header.Set("webhook-signature", signature)
	}

	// Set webhook specification version header
	req.Header.Set("webhook-version", "1")

	// Create a client with the configured timeout
	timeout := time.Duration(webhook.Timeout) * time.Second
	if webhook.Timeout == 0 {
		timeout = 30 * time.Second // default timeout
	}

	client := &http.Client{
		Timeout: timeout,
	}

	// Send the request
	resp, err := client.Do(req)
	if err != nil {
		return &WebhookResponse{
			Error: fmt.Errorf("failed to send request: %w", err),
		}, nil
	}
	defer func() { _ = resp.Body.Close() }()

	// Read response body
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return &WebhookResponse{
			StatusCode: resp.StatusCode,
			Error:      fmt.Errorf("failed to read response body: %w", err),
		}, nil
	}

	return &WebhookResponse{
		StatusCode: resp.StatusCode,
		Body:       respBody,
		Error:      nil,
	}, nil
}
