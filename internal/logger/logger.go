package logger

import (
	"io"
	"os"

	"github.com/rs/zerolog"
)

// Logger provides structured logging with session tracking
type Logger interface {
	WithSession(sessionID string) Logger
	Info(msg string, fields ...Field)
	Error(msg string, err error, fields ...Field)
	Debug(msg string, fields ...Field)
	Flush() error
}

// Field represents a key-value pair for structured logging
type Field struct {
	Key   string
	Value interface{}
}

// logger implements the Logger interface using zerolog
type logger struct {
	zlog   zerolog.Logger
	writer io.Writer
}

// New creates a new Logger instance with JSON output
func New(w io.Writer) Logger {
	if w == nil {
		w = os.Stdout
	}

	zlog := zerolog.New(w).With().Timestamp().Logger()

	return &logger{
		zlog:   zlog,
		writer: w,
	}
}

// NewWithLevel creates a new Logger instance with a specific log level
func NewWithLevel(w io.Writer, level string) Logger {
	if w == nil {
		w = os.Stdout
	}

	zlog := zerolog.New(w).With().Timestamp().Logger()

	// Set log level
	switch level {
	case "debug":
		zlog = zlog.Level(zerolog.DebugLevel)
	case "info":
		zlog = zlog.Level(zerolog.InfoLevel)
	case "error":
		zlog = zlog.Level(zerolog.ErrorLevel)
	default:
		zlog = zlog.Level(zerolog.InfoLevel)
	}

	return &logger{
		zlog:   zlog,
		writer: w,
	}
}

// WithSession returns a new Logger with the session ID included in all log entries
func (l *logger) WithSession(sessionID string) Logger {
	return &logger{
		zlog:   l.zlog.With().Str("session_id", sessionID).Logger(),
		writer: l.writer,
	}
}

// Info logs an informational message with optional fields
func (l *logger) Info(msg string, fields ...Field) {
	event := l.zlog.Info()
	for _, field := range fields {
		event = event.Interface(field.Key, field.Value)
	}
	event.Msg(msg)
}

// Error logs an error message with optional fields
func (l *logger) Error(msg string, err error, fields ...Field) {
	event := l.zlog.Error()
	if err != nil {
		event = event.Err(err)
	}
	for _, field := range fields {
		event = event.Interface(field.Key, field.Value)
	}
	event.Msg(msg)
}

// Debug logs a debug message with optional fields
func (l *logger) Debug(msg string, fields ...Field) {
	event := l.zlog.Debug()
	for _, field := range fields {
		event = event.Interface(field.Key, field.Value)
	}
	event.Msg(msg)
}

// Str creates a string field
func Str(key, value string) Field {
	return Field{Key: key, Value: value}
}

// Int creates an integer field
func Int(key string, value int) Field {
	return Field{Key: key, Value: value}
}

// Int64 creates an int64 field
func Int64(key string, value int64) Field {
	return Field{Key: key, Value: value}
}

// Strs creates a string slice field
func Strs(key string, value []string) Field {
	return Field{Key: key, Value: value}
}

// Any creates a field with any value
func Any(key string, value interface{}) Field {
	return Field{Key: key, Value: value}
}

// Flush flushes any buffered log entries
// If the underlying writer supports flushing (e.g., *os.File), it will be flushed
func (l *logger) Flush() error {
	// Check if the writer implements Sync (like *os.File)
	if syncer, ok := l.writer.(interface{ Sync() error }); ok {
		err := syncer.Sync()
		// Ignore "invalid argument" errors which can occur with stdout/stderr
		// in certain environments (e.g., when redirected to /dev/null or pipes)
		if err != nil && !isInvalidArgumentError(err) {
			return err
		}
	}
	// If the writer doesn't support syncing, that's okay
	return nil
}

// isInvalidArgumentError checks if an error is an "invalid argument" error
func isInvalidArgumentError(err error) bool {
	if err == nil {
		return false
	}
	// Check for common "invalid argument" error messages
	errMsg := err.Error()
	return errMsg == "invalid argument" ||
		errMsg == "sync /dev/stdout: invalid argument" ||
		errMsg == "sync /dev/stderr: invalid argument"
}
