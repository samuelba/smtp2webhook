# Multi-stage Dockerfile for SMTP Webhook Forwarder

# Builder stage
# Note: Using latest Go version to match go.mod requirements
FROM golang:alpine AS builder

# Set working directory
WORKDIR /build

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build static binary with CGO disabled
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -installsuffix cgo -ldflags="-w -s" -o smtp-forwarder ./cmd/smtp-forwarder

# Runtime stage - using Alpine for minimal image with health check support
FROM alpine:latest

# Install ca-certificates for HTTPS webhook calls
RUN apk --no-cache add ca-certificates

# Copy static binary from builder
COPY --from=builder /build/smtp-forwarder /usr/local/bin/smtp-forwarder

# Copy health check script
COPY healthcheck.sh /usr/local/bin/healthcheck.sh

# Make health check script executable and owned by nonroot user
RUN chmod +x /usr/local/bin/healthcheck.sh

# Create non-root user
RUN addgroup -g 65532 -S nonroot && adduser -u 65532 -S nonroot -G nonroot

# Switch to non-root user
USER nonroot

# Expose SMTP port (default 2525, can be overridden via config)
EXPOSE 2525

# Set entrypoint to the binary
ENTRYPOINT ["/usr/local/bin/smtp-forwarder"]

# Health check - uses script that reads port from SMTP_PORT env var or config file
# Supports dynamic port configuration via environment variables or config
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
  CMD ["/usr/local/bin/healthcheck.sh"]
