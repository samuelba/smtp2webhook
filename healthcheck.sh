#!/bin/sh
# Health check script for SMTP Webhook Forwarder
# Checks TCP connection to the configured SMTP port

# Priority order:
# 1. SMTP_PORT environment variable (highest priority)
# 2. Port from config file
# 3. Default port 2525

PORT=2525

# Try to read port from config file first
if [ -n "$CONFIG_FILE" ] && [ -f "$CONFIG_FILE" ]; then
    CONFIG_PORT=$(grep -o '"port"[[:space:]]*:[[:space:]]*[0-9]*' "$CONFIG_FILE" | grep -o '[0-9]*' | head -1)
    if [ -n "$CONFIG_PORT" ]; then
        PORT=$CONFIG_PORT
    fi
fi

# Environment variable overrides config file
if [ -n "$SMTP_PORT" ]; then
    PORT=$SMTP_PORT
fi

# Check if SMTP port is listening
nc -z localhost "$PORT" || exit 1
