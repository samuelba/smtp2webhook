package webhook

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"strings"
	"testing"
)

// TestSign_StandardCompliance uses the logic from the standard-webhooks reference implementation
// to verify that our Signer produces compatible signatures when provided with a standard secret.
func TestSign_StandardCompliance(t *testing.T) {
	// Standard format secret: "whsec_" + base64("my-secret-key-123")
	// "my-secret-key-123" in base64 is "bXktc2VjcmV0LWtleS0xMjM="
	rawSecret := "my-secret-key-123"
	encodedSecret := "whsec_" + base64.StdEncoding.EncodeToString([]byte(rawSecret))

	msgID := "msg_test_123"
	timestamp := int64(1672531200) // 2023-01-01 00:00:00 UTC
	body := []byte(`{"event_type":"test.event","data":{"foo":"bar"}}`)

	// 1. Calculate Expected Signature using Reference Logic
	// Reference logic: Decode base64 secret (stripping whsec_ prefix)
	key, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(encodedSecret, "whsec_"))
	if err != nil {
		t.Fatalf("Failed to decode reference secret: %v", err)
	}

	payloadToSign := fmt.Sprintf("%s.%d.%s", msgID, timestamp, string(body))
	h := hmac.New(sha256.New, key)
	h.Write([]byte(payloadToSign))
	expectedSigBytes := h.Sum(nil)
	expectedBase64Sig := base64.StdEncoding.EncodeToString(expectedSigBytes)
	expectedFullSig := fmt.Sprintf("v1,%s", expectedBase64Sig)

	// 2. Calculate Actual Signature using our Signer
	signer := NewSigner()
	actualSig, err := signer.Sign(encodedSecret, msgID, timestamp, body)
	if err != nil {
		t.Fatalf("Signer.Sign returned error: %v", err)
	}

	// 3. Compare
	if actualSig != expectedFullSig {
		t.Errorf("Standard Compliance Mismatch!\nSecret Provided: %s\nExpected (Spec): %s\nActual (Impl):   %s\n\nExplanation: The spec requires secrets to be base64-encoded (prefixed with whsec_). \nOur implementation likely treats the secret as a raw string instead of decoding it.",
			encodedSecret, expectedFullSig, actualSig)
	}
}
