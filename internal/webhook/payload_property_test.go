package webhook

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"smtp-webhook-forwarder/internal/parser"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/prop"
)

// **Feature: smtp-webhook-forwarder, Property 10: Session ID in webhook payload**
// For any webhook request, the JSON payload should include the session identifier.
// Validates: Requirements 12.3
func TestProperty_SessionIDInWebhookPayload(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	properties.Property("webhook payload includes session ID", prop.ForAll(
		func(sessionID string, timestamp int64, email *parser.ParsedEmail, secret string, timeout int) bool {
			// Create a test HTTP server to capture the payload
			var capturedPayload WebhookPayload
			var mu sync.Mutex
			var payloadReceived bool

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Read the request body
				body, err := io.ReadAll(r.Body)
				if err != nil {
					t.Logf("Failed to read request body: %v", err)
					w.WriteHeader(http.StatusInternalServerError)
					return
				}
				r.Body.Close()

				// Parse the JSON payload
				var payload WebhookPayload
				err = json.Unmarshal(body, &payload)
				if err != nil {
					t.Logf("Failed to unmarshal payload: %v", err)
					w.WriteHeader(http.StatusInternalServerError)
					return
				}

				mu.Lock()
				capturedPayload = payload
				payloadReceived = true
				mu.Unlock()

				w.WriteHeader(http.StatusOK)
			}))
			defer server.Close()

			// Create webhook config
			webhook := WebhookConfig{
				URL:     server.URL,
				Secret:  secret,
				Timeout: timeout,
			}

			// Create payload with the session ID
			payload := &WebhookPayload{
				SessionID: sessionID,
				Timestamp: timestamp,
				Email:     email,
			}

			// Create client and send request
			signer := NewSigner()
			client := NewClient(signer)

			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			resp, err := client.Send(ctx, webhook, payload)
			if err != nil {
				t.Logf("Send error: %v", err)
				return false
			}

			if resp.Error != nil {
				t.Logf("Response error: %v", resp.Error)
				return false
			}

			// Verify the payload was received
			mu.Lock()
			received := payloadReceived
			captured := capturedPayload
			mu.Unlock()

			if !received {
				t.Logf("Payload was not received by server")
				return false
			}

			// Verify session ID is present in the payload
			if captured.SessionID == "" {
				t.Logf("Session ID is empty in payload")
				return false
			}

			// Verify session ID matches the one we sent
			if captured.SessionID != sessionID {
				t.Logf("Session ID mismatch: expected %q, got %q", sessionID, captured.SessionID)
				return false
			}

			// Verify the payload structure is complete
			if captured.Email == nil {
				t.Logf("Email is nil in payload")
				return false
			}

			// Verify timestamp is present
			if captured.Timestamp == 0 {
				t.Logf("Timestamp is zero in payload")
				return false
			}

			return true
		},
		genSessionID(),   // sessionID
		genTimestamp(),   // timestamp
		genParsedEmail(), // email
		genSecret(),      // secret (can be empty)
		genTimeout(),     // timeout
	))

	properties.TestingRun(t)
}
