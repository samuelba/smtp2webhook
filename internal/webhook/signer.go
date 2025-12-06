package webhook

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"strings"
)

// Signer generates Standard Webhooks signatures for webhook requests.
type Signer interface {
	Sign(secret string, msgID string, timestamp int64, body []byte) (string, error)
}

// HMACSigner implements the Signer interface using HMAC-SHA256.
type HMACSigner struct{}

// NewSigner creates a new HMACSigner instance.
func NewSigner() Signer {
	return &HMACSigner{}
}

// Sign generates a Standard Webhooks signature.
// The signature payload format is: {webhook-id}.{timestamp}.{body}
// The signature format is: v1,{base64(hmac-sha256(secret, payload))}
//
// Parameters:
//   - secret: The webhook secret used for signing. Can be raw string or "whsec_"{base64}
//   - msgID: The webhook message ID (session ID)
//   - timestamp: Unix timestamp in seconds
//   - body: The JSON body bytes
//
// Returns the formatted signature string or an error.
func (s *HMACSigner) Sign(secret string, msgID string, timestamp int64, body []byte) (string, error) {
	if secret == "" {
		return "", fmt.Errorf("secret cannot be empty")
	}
	if msgID == "" {
		return "", fmt.Errorf("msgID cannot be empty")
	}

	var key []byte
	if strings.HasPrefix(secret, "whsec_") {
		var err error
		key, err = base64.StdEncoding.DecodeString(strings.TrimPrefix(secret, "whsec_"))
		if err != nil {
			return "", fmt.Errorf("failed to decode whsec_ secret: %w", err)
		}
	} else {
		key = []byte(secret)
	}

	// Construct the signature payload: {webhook-id}.{timestamp}.{body}
	payload := fmt.Sprintf("%s.%d.%s", msgID, timestamp, string(body))

	// Compute HMAC-SHA256
	h := hmac.New(sha256.New, key)
	h.Write([]byte(payload))
	signature := h.Sum(nil)

	// Encode to base64
	encodedSignature := base64.StdEncoding.EncodeToString(signature)

	// Format according to Standard Webhooks spec: v1,{signature}
	return fmt.Sprintf("v1,%s", encodedSignature), nil
}
