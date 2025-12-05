package webhook

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync"
	"testing"
	"time"

	"smtp-webhook-forwarder/internal/parser"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
)

// **Feature: smtp-webhook-forwarder, Property 3: Standard Webhooks headers presence**
// For any webhook request, the request headers should include webhook-id, webhook-timestamp,
// and webhook specification version.
// Validates: Requirements 3.2, 3.3, 3.4
func TestProperty_StandardWebhooksHeadersPresence(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	properties.Property("all required Standard Webhooks headers are present", prop.ForAll(
		func(sessionID string, timestamp int64, email *parser.ParsedEmail, secret string, timeout int) bool {
			// Create a test HTTP server to capture headers
			var capturedHeaders http.Header
			var mu sync.Mutex

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				mu.Lock()
				capturedHeaders = r.Header.Clone()
				mu.Unlock()

				// Read and discard body
				_, _ = io.ReadAll(r.Body)
				_ = r.Body.Close()

				w.WriteHeader(http.StatusOK)
			}))
			defer server.Close()

			// Create webhook config
			webhook := WebhookConfig{
				URL:     server.URL,
				Secret:  secret,
				Timeout: timeout,
			}

			// Create payload
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

			// Verify all required headers are present
			mu.Lock()
			headers := capturedHeaders
			mu.Unlock()

			// Check webhook-id header
			webhookID := headers.Get("webhook-id")
			if webhookID == "" {
				t.Logf("Missing webhook-id header")
				return false
			}

			// Verify webhook-id matches session ID
			if webhookID != sessionID {
				t.Logf("webhook-id header (%q) does not match session ID (%q)", webhookID, sessionID)
				return false
			}

			// Check webhook-timestamp header
			webhookTimestamp := headers.Get("webhook-timestamp")
			if webhookTimestamp == "" {
				t.Logf("Missing webhook-timestamp header")
				return false
			}

			// Verify webhook-timestamp is a valid integer and matches the payload timestamp
			parsedTimestamp, err := strconv.ParseInt(webhookTimestamp, 10, 64)
			if err != nil {
				t.Logf("webhook-timestamp header is not a valid integer: %q", webhookTimestamp)
				return false
			}

			if parsedTimestamp != timestamp {
				t.Logf("webhook-timestamp header (%d) does not match payload timestamp (%d)", parsedTimestamp, timestamp)
				return false
			}

			// Check webhook-version header (specification version)
			webhookVersion := headers.Get("webhook-version")
			if webhookVersion == "" {
				t.Logf("Missing webhook-version header")
				return false
			}

			// Verify version is "1" as per Standard Webhooks spec
			if webhookVersion != "1" {
				t.Logf("webhook-version header should be '1', got: %q", webhookVersion)
				return false
			}

			// Check Content-Type header
			contentType := headers.Get("Content-Type")
			if contentType == "" {
				t.Logf("Missing Content-Type header")
				return false
			}

			if contentType != "application/json" {
				t.Logf("Content-Type should be 'application/json', got: %q", contentType)
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

// genSessionID generates session IDs (non-empty strings)
func genSessionID() gopter.Gen {
	return gen.Identifier().SuchThat(func(s string) bool {
		return s != ""
	}).Map(func(s string) string {
		// Limit length to reasonable size
		if len(s) > 50 {
			return s[:50]
		}
		return s
	})
}

// genTimestamp generates Unix timestamps
func genTimestamp() gopter.Gen {
	// Generate timestamps in a reasonable range (year 2000 to 2100)
	return gen.Int64Range(946684800, 4102444800)
}

// genParsedEmail generates random ParsedEmail instances
func genParsedEmail() gopter.Gen {
	return gopter.CombineGens(
		genEmailAddress(),     // from
		genEmailAddressList(), // to
		gen.Identifier(),      // subject
		genHeaders(),          // headers
		gen.AlphaString(),     // text body
		gen.AlphaString(),     // html body
		genAttachments(),      // attachments
	).Map(func(values []interface{}) *parser.ParsedEmail {
		from := values[0].(string)
		to := values[1].([]string)
		subject := values[2].(string)
		headers := values[3].(map[string][]string)
		textBody := values[4].(string)
		htmlBody := values[5].(string)
		attachments := values[6].([]parser.Attachment)

		return &parser.ParsedEmail{
			From:        from,
			To:          to,
			Subject:     subject,
			Headers:     headers,
			TextBody:    textBody,
			HTMLBody:    htmlBody,
			Attachments: attachments,
		}
	})
}

// genEmailAddress generates email addresses
func genEmailAddress() gopter.Gen {
	return gopter.CombineGens(
		gen.Identifier(),
		gen.Identifier(),
	).Map(func(values []interface{}) string {
		user := values[0].(string)
		domain := values[1].(string)

		if user == "" {
			user = "user"
		}
		if domain == "" {
			domain = "example.com"
		}

		return fmt.Sprintf("%s@%s", user, domain)
	})
}

// genEmailAddressList generates lists of email addresses
func genEmailAddressList() gopter.Gen {
	return gen.SliceOfN(3, genEmailAddress())
}

// genHeaders generates email headers
func genHeaders() gopter.Gen {
	return gen.MapOf(
		gen.Identifier(),
		gen.SliceOf(gen.Identifier()),
	).Map(func(m map[string][]string) map[string][]string {
		// Ensure at least one header
		if len(m) == 0 {
			m["X-Test"] = []string{"test"}
		}
		return m
	})
}

// genAttachments generates attachment lists
func genAttachments() gopter.Gen {
	return gen.SliceOfN(2, genAttachment())
}

// genAttachment generates a single attachment
func genAttachment() gopter.Gen {
	return gopter.CombineGens(
		gen.Identifier(),  // filename
		gen.Identifier(),  // content type
		gen.AlphaString(), // data (will be base64 encoded)
	).Map(func(values []interface{}) parser.Attachment {
		filename := values[0].(string)
		contentType := values[1].(string)
		data := values[2].(string)

		if filename == "" {
			filename = "file.txt"
		}
		if contentType == "" {
			contentType = "text/plain"
		}

		// Simulate base64 encoding
		encoded := data // In real scenario, this would be base64 encoded

		return parser.Attachment{
			Filename:    filename,
			ContentType: contentType,
			Data:        encoded,
			Size:        len(data),
		}
	})
}

// genSecret generates webhook secrets (can be empty)
func genSecret() gopter.Gen {
	return gen.OneGenOf(
		gen.Const(""),    // Empty secret (no signature)
		gen.Identifier(), // Non-empty secret
	)
}

// genTimeout generates timeout values in seconds
func genTimeout() gopter.Gen {
	return gen.IntRange(1, 60)
}

// **Feature: smtp-webhook-forwarder, Property 11: Session ID in webhook headers**
// For any webhook request, the webhook-id header should contain the session identifier.
// Validates: Requirements 12.4
func TestProperty_SessionIDInWebhookHeaders(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	properties.Property("webhook-id header contains session identifier", prop.ForAll(
		func(sessionID string, timestamp int64, email *parser.ParsedEmail, secret string, timeout int) bool {
			// Create a test HTTP server to capture headers
			var capturedHeaders http.Header
			var mu sync.Mutex

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				mu.Lock()
				capturedHeaders = r.Header.Clone()
				mu.Unlock()

				// Read and discard body
				_, _ = io.ReadAll(r.Body)
				_ = r.Body.Close()

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

			// Verify webhook-id header is present
			mu.Lock()
			headers := capturedHeaders
			mu.Unlock()

			webhookID := headers.Get("webhook-id")
			if webhookID == "" {
				t.Logf("Missing webhook-id header")
				return false
			}

			// Verify webhook-id header contains the session identifier
			if webhookID != sessionID {
				t.Logf("webhook-id header (%q) does not match session ID (%q)", webhookID, sessionID)
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
