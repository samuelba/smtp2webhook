package smtp

import (
	"errors"
	"net"
	"syscall"
)

// MapHTTPToSMTP maps HTTP status codes and errors to SMTP response codes.
// According to requirements 2.3, 2.4, 2.5, 2.6:
// - 2xx → 250 (Success)
// - 429 → 451 (Temporary Failure - Rate Limited)
// - 5xx → 451 (Temporary Failure - Server Error)
// - 4xx (excluding 429) → 550 (Permanent Failure - Client Error)
// - Timeout/Connection errors → 451 (Temporary Failure)
func MapHTTPToSMTP(statusCode int, err error) int {
	// Handle connection and timeout errors first
	if err != nil {
		if isTimeoutOrConnectionError(err) {
			return 451 // Temporary Failure
		}
		// For other errors, treat as temporary failure
		return 451
	}

	// Map HTTP status codes to SMTP codes
	switch {
	case statusCode >= 200 && statusCode < 300:
		return 250 // Success
	case statusCode == 429:
		return 451 // Temporary Failure - Rate Limited
	case statusCode >= 500 && statusCode < 600:
		return 451 // Temporary Failure - Server Error
	case statusCode >= 400 && statusCode < 500:
		return 550 // Permanent Failure - Client Error
	default:
		// For any unexpected status codes, treat as temporary failure
		return 451
	}
}

// isTimeoutOrConnectionError checks if an error is a timeout or connection error
func isTimeoutOrConnectionError(err error) bool {
	if err == nil {
		return false
	}

	// Check for timeout errors
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return true
	}

	// Check for connection refused
	var opErr *net.OpError
	if errors.As(err, &opErr) {
		if errors.Is(opErr.Err, syscall.ECONNREFUSED) {
			return true
		}
	}

	// Check for common connection errors
	if errors.Is(err, syscall.ECONNREFUSED) ||
		errors.Is(err, syscall.ECONNRESET) ||
		errors.Is(err, syscall.EPIPE) {
		return true
	}

	return false
}
