package shutdown

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"
)

// Handler manages graceful shutdown of the application
type Handler struct {
	shutdownTimeout time.Duration
	signals         []os.Signal
}

// NewHandler creates a new shutdown handler with the specified timeout
// Requirements: 8.4
func NewHandler(timeout time.Duration) *Handler {
	if timeout == 0 {
		timeout = 30 * time.Second // Default 30 seconds
	}

	return &Handler{
		shutdownTimeout: timeout,
		signals:         []os.Signal{syscall.SIGTERM, syscall.SIGINT},
	}
}

// Wait blocks until a shutdown signal is received
// Requirements: 8.4
func (h *Handler) Wait() os.Signal {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, h.signals...)

	sig := <-sigChan
	return sig
}

// Shutdown performs graceful shutdown of the server
// Requirements: 8.4
// Steps:
// 1. Stop accepting new connections
// 2. Wait for in-flight webhook deliveries with timeout (30s default)
// 3. Close all connections
// 4. Flush logs
// 5. Exit cleanly
func (h *Handler) Shutdown(ctx context.Context, server Stoppable, logger Logger) error {
	logger.Info("initiating graceful shutdown")

	// Create a context with timeout for the shutdown process
	shutdownCtx, cancel := context.WithTimeout(ctx, h.shutdownTimeout)
	defer cancel()

	// Channel to signal when shutdown is complete
	done := make(chan error, 1)

	// Perform shutdown in a goroutine
	go func() {
		// Stop the server (stops accepting new connections, waits for in-flight requests)
		if err := server.Stop(); err != nil {
			logger.Error("error stopping server", err)
			done <- err
			return
		}

		// Flush logs
		if err := logger.Flush(); err != nil {
			logger.Error("error flushing logs", err)
			done <- err
			return
		}

		done <- nil
	}()

	// Wait for shutdown to complete or timeout
	select {
	case err := <-done:
		if err != nil {
			logger.Error("error during shutdown", err)
			return err
		}
		logger.Info("graceful shutdown completed successfully")
		return nil

	case <-shutdownCtx.Done():
		logger.Info("shutdown timeout reached, forcing exit")
		return shutdownCtx.Err()
	}
}

// Stoppable interface for components that can be stopped gracefully
type Stoppable interface {
	Stop() error
}

// Logger interface for logging during shutdown
type Logger interface {
	Info(msg string, fields ...interface{})
	Error(msg string, err error, fields ...interface{})
	Flush() error
}
