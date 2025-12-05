package smtp

import (
	"errors"
	"net"
	"syscall"
	"testing"
)

// TestMapHTTPToSMTP_2xxSuccess tests that 2xx status codes map to SMTP 250
func TestMapHTTPToSMTP_2xxSuccess(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		want       int
	}{
		{"200 OK", 200, 250},
		{"201 Created", 201, 250},
		{"202 Accepted", 202, 250},
		{"204 No Content", 204, 250},
		{"299 Edge of 2xx", 299, 250},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := MapHTTPToSMTP(tt.statusCode, nil)
			if got != tt.want {
				t.Errorf("MapHTTPToSMTP(%d, nil) = %d, want %d", tt.statusCode, got, tt.want)
			}
		})
	}
}

// TestMapHTTPToSMTP_429RateLimit tests that 429 maps to SMTP 451
func TestMapHTTPToSMTP_429RateLimit(t *testing.T) {
	got := MapHTTPToSMTP(429, nil)
	want := 451
	if got != want {
		t.Errorf("MapHTTPToSMTP(429, nil) = %d, want %d", got, want)
	}
}

// TestMapHTTPToSMTP_5xxServerError tests that 5xx status codes map to SMTP 451
func TestMapHTTPToSMTP_5xxServerError(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		want       int
	}{
		{"500 Internal Server Error", 500, 451},
		{"502 Bad Gateway", 502, 451},
		{"503 Service Unavailable", 503, 451},
		{"504 Gateway Timeout", 504, 451},
		{"599 Edge of 5xx", 599, 451},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := MapHTTPToSMTP(tt.statusCode, nil)
			if got != tt.want {
				t.Errorf("MapHTTPToSMTP(%d, nil) = %d, want %d", tt.statusCode, got, tt.want)
			}
		})
	}
}

// TestMapHTTPToSMTP_4xxClientError tests that 4xx (excluding 429) maps to SMTP 550
func TestMapHTTPToSMTP_4xxClientError(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		want       int
	}{
		{"400 Bad Request", 400, 550},
		{"401 Unauthorized", 401, 550},
		{"403 Forbidden", 403, 550},
		{"404 Not Found", 404, 550},
		{"405 Method Not Allowed", 405, 550},
		{"410 Gone", 410, 550},
		{"418 I'm a teapot", 418, 550}, // Unusual but valid 4xx
		{"422 Unprocessable Entity", 422, 550},
		{"451 Unavailable For Legal Reasons", 451, 550}, // Unusual 4xx
		{"499 Edge of 4xx", 499, 550},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := MapHTTPToSMTP(tt.statusCode, nil)
			if got != tt.want {
				t.Errorf("MapHTTPToSMTP(%d, nil) = %d, want %d", tt.statusCode, got, tt.want)
			}
		})
	}
}

// TestMapHTTPToSMTP_UnusualStatusCodes tests edge cases with unusual status codes
func TestMapHTTPToSMTP_UnusualStatusCodes(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		want       int
	}{
		{"100 Continue", 100, 451},
		{"101 Switching Protocols", 101, 451},
		{"300 Multiple Choices", 300, 451},
		{"301 Moved Permanently", 301, 451},
		{"600 Non-standard", 600, 451},
		{"999 Non-standard", 999, 451},
		{"0 Invalid", 0, 451},
		{"-1 Invalid", -1, 451},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := MapHTTPToSMTP(tt.statusCode, nil)
			if got != tt.want {
				t.Errorf("MapHTTPToSMTP(%d, nil) = %d, want %d", tt.statusCode, got, tt.want)
			}
		})
	}
}

// TestMapHTTPToSMTP_TimeoutError tests that timeout errors map to SMTP 451
func TestMapHTTPToSMTP_TimeoutError(t *testing.T) {
	// Create a timeout error
	timeoutErr := &timeoutError{}

	got := MapHTTPToSMTP(0, timeoutErr)
	want := 451
	if got != want {
		t.Errorf("MapHTTPToSMTP(0, timeoutError) = %d, want %d", got, want)
	}
}

// TestMapHTTPToSMTP_ConnectionRefused tests that connection refused errors map to SMTP 451
func TestMapHTTPToSMTP_ConnectionRefused(t *testing.T) {
	// Create a connection refused error
	connRefusedErr := &net.OpError{
		Op:  "dial",
		Err: syscall.ECONNREFUSED,
	}

	got := MapHTTPToSMTP(0, connRefusedErr)
	want := 451
	if got != want {
		t.Errorf("MapHTTPToSMTP(0, ECONNREFUSED) = %d, want %d", got, want)
	}
}

// TestMapHTTPToSMTP_ConnectionReset tests that connection reset errors map to SMTP 451
func TestMapHTTPToSMTP_ConnectionReset(t *testing.T) {
	got := MapHTTPToSMTP(0, syscall.ECONNRESET)
	want := 451
	if got != want {
		t.Errorf("MapHTTPToSMTP(0, ECONNRESET) = %d, want %d", got, want)
	}
}

// TestMapHTTPToSMTP_BrokenPipe tests that broken pipe errors map to SMTP 451
func TestMapHTTPToSMTP_BrokenPipe(t *testing.T) {
	got := MapHTTPToSMTP(0, syscall.EPIPE)
	want := 451
	if got != want {
		t.Errorf("MapHTTPToSMTP(0, EPIPE) = %d, want %d", got, want)
	}
}

// TestMapHTTPToSMTP_GenericError tests that generic errors map to SMTP 451
func TestMapHTTPToSMTP_GenericError(t *testing.T) {
	genericErr := errors.New("generic error")

	got := MapHTTPToSMTP(0, genericErr)
	want := 451
	if got != want {
		t.Errorf("MapHTTPToSMTP(0, generic error) = %d, want %d", got, want)
	}
}

// TestMapHTTPToSMTP_ErrorWithSuccessStatus tests that errors take precedence over status codes
func TestMapHTTPToSMTP_ErrorWithSuccessStatus(t *testing.T) {
	// Even with a 200 status code, if there's an error, it should map to 451
	err := errors.New("some error")

	got := MapHTTPToSMTP(200, err)
	want := 451
	if got != want {
		t.Errorf("MapHTTPToSMTP(200, error) = %d, want %d (errors should take precedence)", got, want)
	}
}
