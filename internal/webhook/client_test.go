package webhook

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"smtp-webhook-forwarder/internal/parser"
)

// TestHTTPPostRequestConstruction verifies that the webhook client constructs
// proper HTTP POST requests with correct method, URL, and body.
// Requirements: 2.2
func TestHTTPPostRequestConstruction(t *testing.T) {
	tests := []struct {
		name    string
		url     string
		payload *WebhookPayload
	}{
		{
			name: "basic POST request",
			url:  "/webhook",
			payload: &WebhookPayload{
				SessionID: "test-session-123",
				Timestamp: 1234567890,
				Email: &parser.ParsedEmail{
					From:    "sender@example.com",
					To:      []string{"recipient@example.com"},
					Subject: "Test Email",
				},
			},
		},
		{
			name: "POST request with complex email",
			url:  "/webhook/endpoint",
			payload: &WebhookPayload{
				SessionID: "session-456",
				Timestamp: 9876543210,
				Email: &parser.ParsedEmail{
					From:     "complex@example.com",
					To:       []string{"user1@test.com", "user2@test.com"},
					Subject:  "Complex Email",
					TextBody: "This is the text body",
					HTMLBody: "<p>This is the HTML body</p>",
					Headers: map[string][]string{
						"X-Custom": {"value1", "value2"},
					},
					Attachments: []parser.Attachment{
						{
							Filename:    "test.txt",
							ContentType: "text/plain",
							Data:        "dGVzdCBkYXRh", // base64 encoded
							Size:        9,
						},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test server to capture request
			var capturedMethod string
			var capturedURL string
			var capturedBody []byte

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				capturedMethod = r.Method
				capturedURL = r.URL.Path
				capturedBody, _ = io.ReadAll(r.Body)
				w.WriteHeader(http.StatusOK)
			}))
			defer server.Close()

			// Create webhook config
			webhook := WebhookConfig{
				URL:     server.URL + tt.url,
				Timeout: 5,
			}

			// Create client and send request
			signer := NewSigner()
			client := NewClient(signer)

			ctx := context.Background()
			resp, err := client.Send(ctx, webhook, tt.payload)

			// Verify no error
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if resp.Error != nil {
				t.Fatalf("unexpected response error: %v", resp.Error)
			}

			// Verify HTTP method is POST
			if capturedMethod != "POST" {
				t.Errorf("expected method POST, got %s", capturedMethod)
			}

			// Verify URL path
			if capturedURL != tt.url {
				t.Errorf("expected URL path %s, got %s", tt.url, capturedURL)
			}

			// Verify body is valid JSON and matches payload
			var receivedPayload WebhookPayload
			if err := json.Unmarshal(capturedBody, &receivedPayload); err != nil {
				t.Fatalf("failed to unmarshal body: %v", err)
			}

			if receivedPayload.SessionID != tt.payload.SessionID {
				t.Errorf("expected session ID %s, got %s", tt.payload.SessionID, receivedPayload.SessionID)
			}
			if receivedPayload.Timestamp != tt.payload.Timestamp {
				t.Errorf("expected timestamp %d, got %d", tt.payload.Timestamp, receivedPayload.Timestamp)
			}
		})
	}
}

// TestHeaderInclusion verifies that all required Standard Webhooks headers
// are included in the request.
// Requirements: 3.2, 3.3, 3.4
func TestHeaderInclusion(t *testing.T) {
	tests := []struct {
		name            string
		secret          string
		expectSignature bool
	}{
		{
			name:            "headers with signature",
			secret:          "test-secret-key",
			expectSignature: true,
		},
		{
			name:            "headers without signature",
			secret:          "",
			expectSignature: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test server to capture headers
			var capturedHeaders http.Header

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				capturedHeaders = r.Header.Clone()
				_, _ = io.ReadAll(r.Body)
				w.WriteHeader(http.StatusOK)
			}))
			defer server.Close()

			// Create webhook config
			webhook := WebhookConfig{
				URL:     server.URL,
				Secret:  tt.secret,
				Timeout: 5,
			}

			// Create payload
			payload := &WebhookPayload{
				SessionID: "test-session-789",
				Timestamp: 1234567890,
				Email: &parser.ParsedEmail{
					From:    "test@example.com",
					To:      []string{"recipient@example.com"},
					Subject: "Test",
				},
			}

			// Create client and send request
			signer := NewSigner()
			client := NewClient(signer)

			ctx := context.Background()
			resp, err := client.Send(ctx, webhook, payload)

			// Verify no error
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if resp.Error != nil {
				t.Fatalf("unexpected response error: %v", resp.Error)
			}

			// Verify Content-Type header
			contentType := capturedHeaders.Get("Content-Type")
			if contentType != "application/json" {
				t.Errorf("expected Content-Type 'application/json', got '%s'", contentType)
			}

			// Verify webhook-id header
			webhookID := capturedHeaders.Get("webhook-id")
			if webhookID != payload.SessionID {
				t.Errorf("expected webhook-id '%s', got '%s'", payload.SessionID, webhookID)
			}

			// Verify webhook-timestamp header
			webhookTimestamp := capturedHeaders.Get("webhook-timestamp")
			expectedTimestamp := "1234567890"
			if webhookTimestamp != expectedTimestamp {
				t.Errorf("expected webhook-timestamp '%s', got '%s'", expectedTimestamp, webhookTimestamp)
			}

			// Verify webhook-version header
			webhookVersion := capturedHeaders.Get("webhook-version")
			if webhookVersion != "1" {
				t.Errorf("expected webhook-version '1', got '%s'", webhookVersion)
			}

			// Verify webhook-signature header presence
			webhookSignature := capturedHeaders.Get("webhook-signature")
			if tt.expectSignature {
				if webhookSignature == "" {
					t.Error("expected webhook-signature header to be present")
				}
				// Verify signature format (should start with "v1,")
				if !strings.HasPrefix(webhookSignature, "v1,") {
					t.Errorf("expected signature to start with 'v1,', got '%s'", webhookSignature)
				}
			} else {
				if webhookSignature != "" {
					t.Errorf("expected no webhook-signature header, got '%s'", webhookSignature)
				}
			}
		})
	}
}

// TestTimeoutHandling verifies that the webhook client respects timeout
// configuration and handles timeout errors appropriately.
// Requirements: 2.2
func TestTimeoutHandling(t *testing.T) {
	tests := []struct {
		name          string
		serverDelay   time.Duration
		clientTimeout int
		expectTimeout bool
	}{
		{
			name:          "request completes before timeout",
			serverDelay:   100 * time.Millisecond,
			clientTimeout: 2,
			expectTimeout: false,
		},
		{
			name:          "request times out",
			serverDelay:   3 * time.Second,
			clientTimeout: 1,
			expectTimeout: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test server with delay
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_, _ = io.ReadAll(r.Body)
				time.Sleep(tt.serverDelay)
				w.WriteHeader(http.StatusOK)
			}))
			defer server.Close()

			// Create webhook config
			webhook := WebhookConfig{
				URL:     server.URL,
				Timeout: tt.clientTimeout,
			}

			// Create payload
			payload := &WebhookPayload{
				SessionID: "timeout-test",
				Timestamp: time.Now().Unix(),
				Email: &parser.ParsedEmail{
					From:    "test@example.com",
					To:      []string{"recipient@example.com"},
					Subject: "Timeout Test",
				},
			}

			// Create client and send request
			signer := NewSigner()
			client := NewClient(signer)

			ctx := context.Background()
			resp, err := client.Send(ctx, webhook, payload)

			// Verify error handling
			if err != nil {
				t.Fatalf("unexpected error from Send: %v", err)
			}

			if tt.expectTimeout {
				// Should have an error in the response
				if resp.Error == nil {
					t.Error("expected timeout error in response")
				}
				// Error message should indicate timeout
				if resp.Error != nil && !strings.Contains(resp.Error.Error(), "timeout") &&
					!strings.Contains(resp.Error.Error(), "deadline exceeded") {
					t.Errorf("expected timeout-related error, got: %v", resp.Error)
				}
			} else {
				// Should not have an error
				if resp.Error != nil {
					t.Errorf("unexpected error: %v", resp.Error)
				}
				// Should have successful status code
				if resp.StatusCode != http.StatusOK {
					t.Errorf("expected status 200, got %d", resp.StatusCode)
				}
			}
		})
	}
}

// TestResponseParsing verifies that the webhook client correctly parses
// HTTP responses including status codes and body content.
// Requirements: 2.2
func TestResponseParsing(t *testing.T) {
	tests := []struct {
		name               string
		serverStatusCode   int
		serverResponseBody string
		expectError        bool
	}{
		{
			name:               "successful response with body",
			serverStatusCode:   http.StatusOK,
			serverResponseBody: `{"status":"success"}`,
			expectError:        false,
		},
		{
			name:               "successful response without body",
			serverStatusCode:   http.StatusNoContent,
			serverResponseBody: "",
			expectError:        false,
		},
		{
			name:               "client error response",
			serverStatusCode:   http.StatusBadRequest,
			serverResponseBody: `{"error":"bad request"}`,
			expectError:        false,
		},
		{
			name:               "server error response",
			serverStatusCode:   http.StatusInternalServerError,
			serverResponseBody: `{"error":"internal error"}`,
			expectError:        false,
		},
		{
			name:               "rate limit response",
			serverStatusCode:   http.StatusTooManyRequests,
			serverResponseBody: `{"error":"rate limited"}`,
			expectError:        false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test server with configured response
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_, _ = io.ReadAll(r.Body)
				w.WriteHeader(tt.serverStatusCode)
				if tt.serverResponseBody != "" {
					_, _ = w.Write([]byte(tt.serverResponseBody))
				}
			}))
			defer server.Close()

			// Create webhook config
			webhook := WebhookConfig{
				URL:     server.URL,
				Timeout: 5,
			}

			// Create payload
			payload := &WebhookPayload{
				SessionID: "response-test",
				Timestamp: time.Now().Unix(),
				Email: &parser.ParsedEmail{
					From:    "test@example.com",
					To:      []string{"recipient@example.com"},
					Subject: "Response Test",
				},
			}

			// Create client and send request
			signer := NewSigner()
			client := NewClient(signer)

			ctx := context.Background()
			resp, err := client.Send(ctx, webhook, payload)

			// Verify no error from Send itself
			if err != nil {
				t.Fatalf("unexpected error from Send: %v", err)
			}

			// Verify response error matches expectation
			if tt.expectError && resp.Error == nil {
				t.Error("expected error in response")
			}
			if !tt.expectError && resp.Error != nil {
				t.Errorf("unexpected error in response: %v", resp.Error)
			}

			// Verify status code
			if resp.StatusCode != tt.serverStatusCode {
				t.Errorf("expected status code %d, got %d", tt.serverStatusCode, resp.StatusCode)
			}

			// Verify response body
			if tt.serverResponseBody != "" {
				if string(resp.Body) != tt.serverResponseBody {
					t.Errorf("expected body '%s', got '%s'", tt.serverResponseBody, string(resp.Body))
				}
			}
		})
	}
}

// TestDefaultTimeout verifies that the client uses a default timeout
// when none is configured.
func TestDefaultTimeout(t *testing.T) {
	// Create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	// Create webhook config with zero timeout (should use default)
	webhook := WebhookConfig{
		URL:     server.URL,
		Timeout: 0,
	}

	// Create payload
	payload := &WebhookPayload{
		SessionID: "default-timeout-test",
		Timestamp: time.Now().Unix(),
		Email: &parser.ParsedEmail{
			From:    "test@example.com",
			To:      []string{"recipient@example.com"},
			Subject: "Default Timeout Test",
		},
	}

	// Create client and send request
	signer := NewSigner()
	client := NewClient(signer)

	ctx := context.Background()
	resp, err := client.Send(ctx, webhook, payload)

	// Verify no error
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Error != nil {
		t.Fatalf("unexpected response error: %v", resp.Error)
	}

	// Verify successful response
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}
}

// TestContextCancellation verifies that the client respects context cancellation.
func TestContextCancellation(t *testing.T) {
	// Create test server with delay
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.ReadAll(r.Body)
		time.Sleep(2 * time.Second)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	// Create webhook config
	webhook := WebhookConfig{
		URL:     server.URL,
		Timeout: 10, // Long timeout
	}

	// Create payload
	payload := &WebhookPayload{
		SessionID: "context-cancel-test",
		Timestamp: time.Now().Unix(),
		Email: &parser.ParsedEmail{
			From:    "test@example.com",
			To:      []string{"recipient@example.com"},
			Subject: "Context Cancel Test",
		},
	}

	// Create client
	signer := NewSigner()
	client := NewClient(signer)

	// Create context with short timeout
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	resp, err := client.Send(ctx, webhook, payload)

	// Verify error handling
	if err != nil {
		t.Fatalf("unexpected error from Send: %v", err)
	}

	// Should have an error in the response due to context cancellation
	if resp.Error == nil {
		t.Error("expected error in response due to context cancellation")
	}
}
