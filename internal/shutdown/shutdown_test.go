package shutdown

import (
	"context"
	"errors"
	"testing"
	"time"
)

// mockServer implements the Stoppable interface for testing
type mockServer struct {
	stopCalled bool
	stopDelay  time.Duration
	stopError  error
}

func (m *mockServer) Stop() error {
	if m.stopDelay > 0 {
		time.Sleep(m.stopDelay)
	}
	m.stopCalled = true
	return m.stopError
}

// mockLogger implements the Logger interface for testing
type mockLogger struct {
	infoMessages  []string
	errorMessages []string
	flushCalled   bool
	flushError    error
}

func (m *mockLogger) Info(msg string, fields ...interface{}) {
	m.infoMessages = append(m.infoMessages, msg)
}

func (m *mockLogger) Error(msg string, err error, fields ...interface{}) {
	m.errorMessages = append(m.errorMessages, msg)
}

func (m *mockLogger) Flush() error {
	m.flushCalled = true
	return m.flushError
}

func TestNewHandler(t *testing.T) {
	tests := []struct {
		name            string
		timeout         time.Duration
		expectedTimeout time.Duration
	}{
		{
			name:            "default timeout",
			timeout:         0,
			expectedTimeout: 30 * time.Second,
		},
		{
			name:            "custom timeout",
			timeout:         10 * time.Second,
			expectedTimeout: 10 * time.Second,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := NewHandler(tt.timeout)
			if handler.shutdownTimeout != tt.expectedTimeout {
				t.Errorf("expected timeout %v, got %v", tt.expectedTimeout, handler.shutdownTimeout)
			}
		})
	}
}

func TestShutdown_Success(t *testing.T) {
	handler := NewHandler(5 * time.Second)
	server := &mockServer{}
	logger := &mockLogger{}

	ctx := context.Background()
	err := handler.Shutdown(ctx, server, logger)

	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}

	if !server.stopCalled {
		t.Error("expected server.Stop() to be called")
	}

	if !logger.flushCalled {
		t.Error("expected logger.Flush() to be called")
	}

	// Check that appropriate log messages were recorded
	foundInitiating := false
	foundCompleted := false
	for _, msg := range logger.infoMessages {
		if msg == "initiating graceful shutdown" {
			foundInitiating = true
		}
		if msg == "graceful shutdown completed successfully" {
			foundCompleted = true
		}
	}

	if !foundInitiating {
		t.Error("expected 'initiating graceful shutdown' log message")
	}

	if !foundCompleted {
		t.Error("expected 'graceful shutdown completed successfully' log message")
	}
}

func TestShutdown_ServerStopError(t *testing.T) {
	handler := NewHandler(5 * time.Second)
	expectedError := errors.New("server stop failed")
	server := &mockServer{stopError: expectedError}
	logger := &mockLogger{}

	ctx := context.Background()
	err := handler.Shutdown(ctx, server, logger)

	if err != expectedError {
		t.Errorf("expected error %v, got %v", expectedError, err)
	}

	if !server.stopCalled {
		t.Error("expected server.Stop() to be called")
	}

	// Logger flush should not be called if server stop fails
	if logger.flushCalled {
		t.Error("expected logger.Flush() not to be called when server stop fails")
	}

	// Check that error was logged
	foundError := false
	for _, msg := range logger.errorMessages {
		if msg == "error stopping server" || msg == "error during shutdown" {
			foundError = true
			break
		}
	}

	if !foundError {
		t.Error("expected error to be logged")
	}
}

func TestShutdown_LoggerFlushError(t *testing.T) {
	handler := NewHandler(5 * time.Second)
	expectedError := errors.New("flush failed")
	server := &mockServer{}
	logger := &mockLogger{flushError: expectedError}

	ctx := context.Background()
	err := handler.Shutdown(ctx, server, logger)

	if err != expectedError {
		t.Errorf("expected error %v, got %v", expectedError, err)
	}

	if !server.stopCalled {
		t.Error("expected server.Stop() to be called")
	}

	if !logger.flushCalled {
		t.Error("expected logger.Flush() to be called")
	}

	// Check that error was logged
	foundError := false
	for _, msg := range logger.errorMessages {
		if msg == "error flushing logs" || msg == "error during shutdown" {
			foundError = true
			break
		}
	}

	if !foundError {
		t.Error("expected error to be logged")
	}
}

func TestShutdown_Timeout(t *testing.T) {
	// Set a very short timeout
	handler := NewHandler(100 * time.Millisecond)

	// Server that takes longer than the timeout to stop
	server := &mockServer{stopDelay: 500 * time.Millisecond}
	logger := &mockLogger{}

	ctx := context.Background()
	err := handler.Shutdown(ctx, server, logger)

	if err == nil {
		t.Error("expected timeout error, got nil")
	}

	// Check that timeout message was logged
	foundTimeout := false
	for _, msg := range logger.infoMessages {
		if msg == "shutdown timeout reached, forcing exit" {
			foundTimeout = true
			break
		}
	}

	if !foundTimeout {
		t.Error("expected 'shutdown timeout reached' log message")
	}
}

func TestShutdown_ContextCancellation(t *testing.T) {
	handler := NewHandler(5 * time.Second)
	server := &mockServer{}
	logger := &mockLogger{}

	// Create a context that's already cancelled
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := handler.Shutdown(ctx, server, logger)

	// When parent context is cancelled, the timeout context inherits that
	// So we expect an error
	if err == nil {
		t.Error("expected context canceled error, got nil")
	}
}
