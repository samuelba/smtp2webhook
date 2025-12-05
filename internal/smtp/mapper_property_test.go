package smtp

import (
	"errors"
	"net"
	"syscall"
	"testing"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
)

// **Feature: smtp-webhook-forwarder, Property 2: HTTP to SMTP status code mapping**
// For any webhook HTTP response status code, the service should map it to the correct
// SMTP response code: 2xx→250, 429→451, 5xx→451, 4xx (excluding 429)→550.
// Validates: Requirements 2.3, 2.4, 2.5
func TestProperty_HTTPToSMTPStatusCodeMapping(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	properties.Property("HTTP status codes map to correct SMTP codes", prop.ForAll(
		func(statusCode int) bool {
			// Map the status code
			smtpCode := MapHTTPToSMTP(statusCode, nil)

			// Verify the mapping follows the specification
			switch {
			case statusCode >= 200 && statusCode < 300:
				// 2xx → 250 (Success)
				if smtpCode != 250 {
					t.Logf("2xx status %d should map to 250, got %d", statusCode, smtpCode)
					return false
				}
			case statusCode == 429:
				// 429 → 451 (Temporary Failure - Rate Limited)
				if smtpCode != 451 {
					t.Logf("429 status should map to 451, got %d", smtpCode)
					return false
				}
			case statusCode >= 500 && statusCode < 600:
				// 5xx → 451 (Temporary Failure - Server Error)
				if smtpCode != 451 {
					t.Logf("5xx status %d should map to 451, got %d", statusCode, smtpCode)
					return false
				}
			case statusCode >= 400 && statusCode < 500:
				// 4xx (excluding 429) → 550 (Permanent Failure - Client Error)
				if smtpCode != 550 {
					t.Logf("4xx status %d (excluding 429) should map to 550, got %d", statusCode, smtpCode)
					return false
				}
			default:
				// For any unexpected status codes, treat as temporary failure (451)
				if smtpCode != 451 {
					t.Logf("Unexpected status %d should map to 451, got %d", statusCode, smtpCode)
					return false
				}
			}

			return true
		},
		genHTTPStatusCode(),
	))

	properties.TestingRun(t)
}

// **Feature: smtp-webhook-forwarder, Property 2: HTTP to SMTP status code mapping (with errors)**
// For any timeout or connection error, the service should map it to SMTP code 451.
// Validates: Requirements 2.6
func TestProperty_ErrorToSMTPMapping(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	properties.Property("timeout and connection errors map to SMTP 451", prop.ForAll(
		func(errorType int) bool {
			var err error

			// Generate different types of errors
			switch errorType % 5 {
			case 0:
				// Timeout error
				err = &timeoutError{}
			case 1:
				// Connection refused
				err = &net.OpError{
					Op:  "dial",
					Err: syscall.ECONNREFUSED,
				}
			case 2:
				// Connection reset
				err = syscall.ECONNRESET
			case 3:
				// Broken pipe
				err = syscall.EPIPE
			case 4:
				// Generic error (should also map to 451)
				err = errors.New("generic error")
			}

			// Map the error
			smtpCode := MapHTTPToSMTP(0, err)

			// All errors should map to 451 (Temporary Failure)
			if smtpCode != 451 {
				t.Logf("Error %v should map to 451, got %d", err, smtpCode)
				return false
			}

			return true
		},
		gen.IntRange(0, 1000),
	))

	properties.TestingRun(t)
}

// genHTTPStatusCode generates HTTP status codes
// Includes common status codes and edge cases
func genHTTPStatusCode() gopter.Gen {
	return gen.OneGenOf(
		// 2xx Success codes
		gen.IntRange(200, 299),
		// 4xx Client Error codes
		gen.IntRange(400, 499),
		// 5xx Server Error codes
		gen.IntRange(500, 599),
		// Edge cases: unusual status codes
		gen.IntRange(100, 199), // 1xx Informational
		gen.IntRange(300, 399), // 3xx Redirection
		gen.IntRange(600, 999), // Non-standard codes
	)
}

// timeoutError is a mock error that implements net.Error with Timeout() returning true
type timeoutError struct{}

func (e *timeoutError) Error() string   { return "timeout error" }
func (e *timeoutError) Timeout() bool   { return true }
func (e *timeoutError) Temporary() bool { return true }
