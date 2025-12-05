package logger

import (
	"bytes"
	"os"
	"testing"
)

// TestFlush_WithFile tests that Flush works with a file writer
func TestFlush_WithFile(t *testing.T) {
	// Create a temporary file
	tmpFile, err := os.CreateTemp("", "logger_test_*.log")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	defer tmpFile.Close()

	// Create logger with file writer
	logger := New(tmpFile)

	// Write some log entries
	logger.Info("test message 1")
	logger.Info("test message 2")

	// Flush should succeed
	err = logger.Flush()
	if err != nil {
		t.Errorf("expected no error from Flush, got: %v", err)
	}
}

// TestFlush_WithBuffer tests that Flush works with a buffer (no-op)
func TestFlush_WithBuffer(t *testing.T) {
	var buf bytes.Buffer
	logger := New(&buf)

	// Write some log entries
	logger.Info("test message 1")
	logger.Info("test message 2")

	// Flush should succeed (no-op for buffer)
	err := logger.Flush()
	if err != nil {
		t.Errorf("expected no error from Flush with buffer, got: %v", err)
	}

	// Verify logs were written
	output := buf.String()
	if len(output) == 0 {
		t.Error("expected log output, got empty string")
	}
}

// TestFlush_WithSessionLogger tests that Flush works with session logger
func TestFlush_WithSessionLogger(t *testing.T) {
	var buf bytes.Buffer
	logger := New(&buf)
	sessionLogger := logger.WithSession("test-session-123")

	// Write some log entries
	sessionLogger.Info("test message 1")
	sessionLogger.Info("test message 2")

	// Flush should succeed
	err := sessionLogger.Flush()
	if err != nil {
		t.Errorf("expected no error from Flush with session logger, got: %v", err)
	}

	// Verify logs were written
	output := buf.String()
	if len(output) == 0 {
		t.Error("expected log output, got empty string")
	}
}

// TestFlush_Stdout tests that Flush works with stdout
func TestFlush_Stdout(t *testing.T) {
	logger := New(os.Stdout)

	// Write some log entries
	logger.Info("test message 1")

	// Flush should succeed
	err := logger.Flush()
	if err != nil {
		t.Errorf("expected no error from Flush with stdout, got: %v", err)
	}
}
