package webhook

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"strings"
	"testing"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
)

// **Feature: smtp-webhook-forwarder, Property 4: Signature generation with secret**
// For any webhook request with a configured secret, the request should include a webhook-signature
// header computed using HMAC-SHA256 of the message ID, timestamp, and body.
// Validates: Requirements 3.1, 3.5, 6.7
func TestProperty_SignatureGenerationWithSecret(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	properties.Property("signature is present and can be validated with HMAC-SHA256", prop.ForAll(
		func(secret string, msgID string, timestamp int64, body []byte) bool {
			// Create signer
			signer := NewSigner()

			// Generate signature
			signature, err := signer.Sign(secret, msgID, timestamp, body)
			if err != nil {
				t.Logf("Sign error: %v", err)
				return false
			}

			// Verify signature is present (not empty)
			if signature == "" {
				t.Logf("Signature is empty")
				return false
			}

			// Verify signature format: v1,{base64_signature}
			if !strings.HasPrefix(signature, "v1,") {
				t.Logf("Signature does not have v1 prefix: %q", signature)
				return false
			}

			// Extract the base64 signature part
			parts := strings.SplitN(signature, ",", 2)
			if len(parts) != 2 {
				t.Logf("Signature format invalid: %q", signature)
				return false
			}
			encodedSig := parts[1]

			// Decode the signature
			decodedSig, err := base64.StdEncoding.DecodeString(encodedSig)
			if err != nil {
				t.Logf("Failed to decode signature: %v", err)
				return false
			}

			// Verify the signature by recomputing it
			payload := fmt.Sprintf("%s.%d.%s", msgID, timestamp, string(body))
			h := hmac.New(sha256.New, []byte(secret))
			h.Write([]byte(payload))
			expectedSig := h.Sum(nil)

			// Compare signatures
			if !hmac.Equal(decodedSig, expectedSig) {
				t.Logf("Signature validation failed")
				return false
			}

			return true
		},
		genNonEmptyString(), // secret
		genNonEmptyString(), // msgID
		gen.Int64(),         // timestamp
		genJSONBody(),       // body
	))

	properties.TestingRun(t)
}

// genNonEmptyString generates non-empty strings
func genNonEmptyString() gopter.Gen {
	return gen.Identifier().SuchThat(func(s string) bool {
		return s != ""
	}).Map(func(s string) string {
		if len(s) > 100 {
			return s[:100]
		}
		return s
	})
}

// **Feature: smtp-webhook-forwarder, Property 5: Signature omission without secret**
// For any webhook request without a configured secret, the request should not include
// a webhook-signature header.
// Validates: Requirements 3.6, 6.8
func TestProperty_SignatureOmissionWithoutSecret(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	properties.Property("signature is omitted when secret is empty", prop.ForAll(
		func(msgID string, timestamp int64, body []byte) bool {
			// Create signer
			signer := NewSigner()

			// Attempt to sign with empty secret
			signature, err := signer.Sign("", msgID, timestamp, body)

			// When there's no secret, we expect an error (current implementation)
			// or an empty signature (alternative implementation)
			// The key property is: no valid signature should be generated
			if err == nil && signature != "" {
				t.Logf("Expected no signature or error with empty secret, got signature: %q", signature)
				return false
			}

			// Verify that if there's an error, it's about the empty secret
			if err != nil {
				errMsg := err.Error()
				if !strings.Contains(errMsg, "secret") {
					t.Logf("Error message doesn't mention secret: %v", err)
					return false
				}
			}

			return true
		},
		genNonEmptyString(), // msgID
		gen.Int64(),         // timestamp
		genJSONBody(),       // body
	))

	properties.TestingRun(t)
}

// genJSONBody generates simple JSON payloads
func genJSONBody() gopter.Gen {
	return gopter.CombineGens(
		gen.Identifier(),
		gen.Identifier(),
		gen.Int(),
	).Map(func(values []interface{}) []byte {
		key1 := values[0].(string)
		val1 := values[1].(string)
		val2 := values[2].(int)

		if key1 == "" {
			key1 = "field"
		}
		if val1 == "" {
			val1 = "value"
		}

		json := fmt.Sprintf(`{"key":"%s","value":"%s","number":%d}`, key1, val1, val2)
		return []byte(json)
	})
}
