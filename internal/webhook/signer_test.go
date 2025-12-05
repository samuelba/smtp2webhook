package webhook

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"strings"
	"testing"
)

// TestSign_KnownInputOutput tests known input/output pairs for HMAC-SHA256
// to ensure the signature generation is deterministic and correct.
func TestSign_KnownInputOutput(t *testing.T) {
	tests := []struct {
		name      string
		secret    string
		msgID     string
		timestamp int64
		body      []byte
		wantSig   string
	}{
		{
			name:      "simple payload",
			secret:    "test_secret",
			msgID:     "msg_123",
			timestamp: 1234567890,
			body:      []byte(`{"test":"data"}`),
			wantSig:   computeExpectedSignature("test_secret", "msg_123", 1234567890, []byte(`{"test":"data"}`)),
		},
		{
			name:      "empty body",
			secret:    "my_secret",
			msgID:     "msg_456",
			timestamp: 1000000000,
			body:      []byte(`{}`),
			wantSig:   computeExpectedSignature("my_secret", "msg_456", 1000000000, []byte(`{}`)),
		},
		{
			name:      "complex JSON body",
			secret:    "webhook_secret_123",
			msgID:     "session_abc_def",
			timestamp: 1700000000,
			body:      []byte(`{"email":{"from":"test@example.com","to":["recipient@example.com"],"subject":"Test"}}`),
			wantSig:   computeExpectedSignature("webhook_secret_123", "session_abc_def", 1700000000, []byte(`{"email":{"from":"test@example.com","to":["recipient@example.com"],"subject":"Test"}}`)),
		},
		{
			name:      "special characters in secret",
			secret:    "secret!@#$%^&*()",
			msgID:     "msg_789",
			timestamp: 1500000000,
			body:      []byte(`{"key":"value"}`),
			wantSig:   computeExpectedSignature("secret!@#$%^&*()", "msg_789", 1500000000, []byte(`{"key":"value"}`)),
		},
		{
			name:      "negative timestamp",
			secret:    "secret",
			msgID:     "msg_negative",
			timestamp: -1234567890,
			body:      []byte(`{"test":"negative"}`),
			wantSig:   computeExpectedSignature("secret", "msg_negative", -1234567890, []byte(`{"test":"negative"}`)),
		},
	}

	signer := NewSigner()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := signer.Sign(tt.secret, tt.msgID, tt.timestamp, tt.body)
			if err != nil {
				t.Fatalf("Sign() error = %v", err)
			}
			if got != tt.wantSig {
				t.Errorf("Sign() = %v, want %v", got, tt.wantSig)
			}
		})
	}
}

// TestSign_SignatureFormat tests that the signature format follows the Standard Webhooks spec.
func TestSign_SignatureFormat(t *testing.T) {
	signer := NewSigner()

	tests := []struct {
		name      string
		secret    string
		msgID     string
		timestamp int64
		body      []byte
	}{
		{
			name:      "basic format check",
			secret:    "secret",
			msgID:     "msg_1",
			timestamp: 1234567890,
			body:      []byte(`{"test":"data"}`),
		},
		{
			name:      "long secret",
			secret:    "very_long_secret_key_with_many_characters_1234567890",
			msgID:     "msg_2",
			timestamp: 1000000000,
			body:      []byte(`{"complex":"payload","with":["multiple","fields"]}`),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			signature, err := signer.Sign(tt.secret, tt.msgID, tt.timestamp, tt.body)
			if err != nil {
				t.Fatalf("Sign() error = %v", err)
			}

			// Verify signature starts with "v1,"
			if !strings.HasPrefix(signature, "v1,") {
				t.Errorf("Signature does not start with 'v1,': %q", signature)
			}

			// Verify signature has exactly one comma
			parts := strings.Split(signature, ",")
			if len(parts) != 2 {
				t.Errorf("Signature should have format 'v1,{signature}', got: %q", signature)
			}

			// Verify the signature part is valid base64
			encodedSig := parts[1]
			decoded, err := base64.StdEncoding.DecodeString(encodedSig)
			if err != nil {
				t.Errorf("Signature part is not valid base64: %v", err)
			}

			// Verify the decoded signature is 32 bytes (SHA256 output)
			if len(decoded) != 32 {
				t.Errorf("Decoded signature should be 32 bytes (SHA256), got %d bytes", len(decoded))
			}

			// Verify the signature is not empty
			if signature == "v1," {
				t.Errorf("Signature should not be empty after 'v1,'")
			}
		})
	}
}

// TestSign_EmptySecret tests that signing with an empty secret returns an error.
func TestSign_EmptySecret(t *testing.T) {
	signer := NewSigner()

	tests := []struct {
		name      string
		msgID     string
		timestamp int64
		body      []byte
	}{
		{
			name:      "empty secret with valid inputs",
			msgID:     "msg_123",
			timestamp: 1234567890,
			body:      []byte(`{"test":"data"}`),
		},
		{
			name:      "empty secret with empty body",
			msgID:     "msg_456",
			timestamp: 1000000000,
			body:      []byte(`{}`),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			signature, err := signer.Sign("", tt.msgID, tt.timestamp, tt.body)

			// Should return an error
			if err == nil {
				t.Errorf("Sign() with empty secret should return error, got signature: %q", signature)
			}

			// Error message should mention "secret"
			if err != nil && !strings.Contains(err.Error(), "secret") {
				t.Errorf("Error message should mention 'secret', got: %v", err)
			}

			// Signature should be empty when there's an error
			if signature != "" {
				t.Errorf("Sign() with error should return empty signature, got: %q", signature)
			}
		})
	}
}

// TestSign_EmptyMsgID tests that signing with an empty message ID returns an error.
func TestSign_EmptyMsgID(t *testing.T) {
	signer := NewSigner()

	signature, err := signer.Sign("secret", "", 1234567890, []byte(`{"test":"data"}`))

	// Should return an error
	if err == nil {
		t.Errorf("Sign() with empty msgID should return error, got signature: %q", signature)
	}

	// Error message should mention "msgID"
	if err != nil && !strings.Contains(err.Error(), "msgID") {
		t.Errorf("Error message should mention 'msgID', got: %v", err)
	}

	// Signature should be empty when there's an error
	if signature != "" {
		t.Errorf("Sign() with error should return empty signature, got: %q", signature)
	}
}

// TestSign_SignatureVerification tests that generated signatures can be verified.
func TestSign_SignatureVerification(t *testing.T) {
	signer := NewSigner()

	tests := []struct {
		name      string
		secret    string
		msgID     string
		timestamp int64
		body      []byte
	}{
		{
			name:      "verify simple signature",
			secret:    "test_secret",
			msgID:     "msg_123",
			timestamp: 1234567890,
			body:      []byte(`{"test":"data"}`),
		},
		{
			name:      "verify complex signature",
			secret:    "webhook_secret",
			msgID:     "session_xyz",
			timestamp: 1700000000,
			body:      []byte(`{"email":{"from":"sender@example.com","to":["recipient@example.com"],"subject":"Test Email","body":"Hello World"}}`),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			signature, err := signer.Sign(tt.secret, tt.msgID, tt.timestamp, tt.body)
			if err != nil {
				t.Fatalf("Sign() error = %v", err)
			}

			// Extract the base64 signature
			parts := strings.SplitN(signature, ",", 2)
			if len(parts) != 2 {
				t.Fatalf("Invalid signature format: %q", signature)
			}
			encodedSig := parts[1]

			// Decode the signature
			decodedSig, err := base64.StdEncoding.DecodeString(encodedSig)
			if err != nil {
				t.Fatalf("Failed to decode signature: %v", err)
			}

			// Recompute the signature
			payload := fmt.Sprintf("%s.%d.%s", tt.msgID, tt.timestamp, string(tt.body))
			h := hmac.New(sha256.New, []byte(tt.secret))
			h.Write([]byte(payload))
			expectedSig := h.Sum(nil)

			// Verify signatures match
			if !hmac.Equal(decodedSig, expectedSig) {
				t.Errorf("Signature verification failed")
			}
		})
	}
}

// computeExpectedSignature is a helper function to compute the expected signature
// for test cases with known inputs.
func computeExpectedSignature(secret, msgID string, timestamp int64, body []byte) string {
	payload := fmt.Sprintf("%s.%d.%s", msgID, timestamp, string(body))
	h := hmac.New(sha256.New, []byte(secret))
	h.Write([]byte(payload))
	signature := h.Sum(nil)
	encodedSignature := base64.StdEncoding.EncodeToString(signature)
	return fmt.Sprintf("v1,%s", encodedSignature)
}
