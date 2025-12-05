# Graceful Shutdown Package

This package provides graceful shutdown functionality for the SMTP Webhook Forwarder service.

## Overview

The shutdown package implements graceful shutdown handling that ensures:
1. No new connections are accepted after shutdown is initiated
2. In-flight webhook deliveries are allowed to complete (with timeout)
3. All connections are properly closed
4. Logs are flushed before exit
5. Clean exit with proper signal handling

## Requirements

Implements **Requirement 8.4**: Graceful shutdown functionality

## Usage

```go
import (
    "context"
    "time"
    "smtp-webhook-forwarder/internal/shutdown"
)

func main() {
    // Create shutdown handler with 30 second timeout
    shutdownHandler := shutdown.NewHandler(30 * time.Second)
    
    // Start your server in a goroutine
    go func() {
        if err := server.Start(); err != nil {
            logger.Error("server error", err)
            os.Exit(1)
        }
    }()
    
    // Wait for shutdown signal (SIGTERM or SIGINT)
    sig := shutdownHandler.Wait()
    logger.Info("received shutdown signal", logger.Str("signal", sig.String()))
    
    // Perform graceful shutdown
    ctx := context.Background()
    if err := shutdownHandler.Shutdown(ctx, server, logger); err != nil {
        logger.Error("shutdown error", err)
        os.Exit(1)
    }
    
    logger.Info("server stopped successfully")
}
```

## Shutdown Process

When a shutdown signal (SIGTERM or SIGINT) is received, the handler:

1. **Stops accepting new connections**: Calls `server.Stop()` which closes the SMTP listener
2. **Waits for in-flight requests**: The server's Stop() method waits for all active connections to complete their webhook deliveries
3. **Enforces timeout**: If connections don't complete within the configured timeout (default 30s), the shutdown proceeds anyway
4. **Flushes logs**: Ensures all log entries are written to disk
5. **Returns control**: Allows the application to exit cleanly

## Timeout Behavior

The shutdown timeout controls how long the system will wait for in-flight requests to complete:

- **Default**: 30 seconds
- **Configurable**: Pass a custom duration to `NewHandler()`
- **Behavior**: If the timeout is reached, the shutdown proceeds immediately, potentially interrupting in-flight webhook deliveries

## Signal Handling

The handler listens for the following signals:
- **SIGTERM**: Graceful termination signal (sent by `kill` or container orchestrators)
- **SIGINT**: Interrupt signal (sent by Ctrl+C)

## Interfaces

### Stoppable

Components that can be gracefully stopped must implement:

```go
type Stoppable interface {
    Stop() error
}
```

The `Stop()` method should:
- Stop accepting new connections
- Wait for in-flight operations to complete
- Clean up resources
- Return any errors encountered

### Logger

Loggers used during shutdown must implement:

```go
type Logger interface {
    Info(msg string, fields ...interface{})
    Error(msg string, err error, fields ...interface{})
    Flush() error
}
```

The `Flush()` method should:
- Write any buffered log entries to disk
- Sync the underlying writer if supported
- Return any errors encountered

## Testing

The package includes comprehensive unit tests covering:
- Successful shutdown
- Server stop errors
- Logger flush errors
- Timeout scenarios
- Context cancellation

Run tests with:
```bash
go test ./internal/shutdown/...
```

## Integration with SMTP Server

The SMTP server's `Stop()` method already implements the required graceful shutdown behavior:

```go
func (s *Server) Stop() error {
    // Cancel context to signal shutdown
    s.cancel()
    
    // Close the SMTP server (stops accepting new connections)
    s.smtpServer.Close()
    
    // Wait for in-flight connections to complete (with 30s timeout)
    done := make(chan struct{})
    go func() {
        s.wg.Wait()
        close(done)
    }()
    
    select {
    case <-done:
        // All connections closed gracefully
    case <-time.After(30 * time.Second):
        // Timeout reached, force close
    }
    
    return nil
}
```

## Best Practices

1. **Set appropriate timeouts**: Consider your typical webhook response times when setting the shutdown timeout
2. **Log shutdown events**: Always log when shutdown is initiated and completed
3. **Handle errors**: Check for errors from `Shutdown()` and log them appropriately
4. **Test in production-like environments**: Verify shutdown behavior under load
5. **Monitor shutdown duration**: Track how long shutdowns take to identify issues

## Example: Docker Container

When running in Docker, the container runtime sends SIGTERM when stopping:

```bash
# Graceful shutdown with 30s timeout
docker stop my-smtp-forwarder

# Force kill after 10s (overrides default 10s Docker timeout)
docker stop -t 10 my-smtp-forwarder
```

Ensure your Docker stop timeout is longer than your application shutdown timeout to allow graceful shutdown to complete.

## Example: Kubernetes

In Kubernetes, configure `terminationGracePeriodSeconds` to allow enough time for graceful shutdown:

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: smtp-forwarder
spec:
  terminationGracePeriodSeconds: 45  # Allow 45s for shutdown (30s app + 15s buffer)
  containers:
  - name: smtp-forwarder
    image: smtp-forwarder:latest
```
