# Docker Compose Setup for SMTP Webhook Forwarder

This directory contains Docker Compose configuration for running the SMTP Webhook Forwarder locally for testing and development.

## Quick Start

1. **Start the services:**
   ```bash
   docker compose up -d
   ```

2. **View logs:**
   ```bash
   docker compose logs -f smtp-forwarder
   ```

3. **Test sending an email:**
   ```bash
   # Using swaks (SMTP testing tool)
   swaks --to test@example.com \
         --from sender@test.com \
         --server localhost:2525 \
         --body "Test email body" \
         --header "Subject: Test Email"
   
   # Or using telnet
   telnet localhost 2525
   # Then type SMTP commands manually
   ```

4. **View webhook receiver logs:**
   ```bash
   docker compose logs -f webhook-receiver
   ```

5. **Stop the services:**
   ```bash
   docker compose down
   ```

## Configuration Files

### config.example.json
Basic configuration without authentication or TLS. Good for initial testing.
- No authentication required
- Plain text connections
- Routes to example webhook receiver

### config.tls.example.json
Advanced configuration with STARTTLS and authentication enabled.
- SMTP authentication required
- STARTTLS encryption
- Multiple routing patterns
- Webhook signatures enabled

To use the TLS example:
```bash
# Generate self-signed certificates for testing
mkdir -p certs
openssl req -x509 -newkey rsa:4096 -keyout certs/server.key -out certs/server.crt -days 365 -nodes -subj "/CN=smtp-forwarder"

# Update docker-compose.yml to use config.tls.example.json
# Then start the services
docker compose up -d
```

## Services

### smtp-forwarder
The main SMTP server that receives emails and forwards them to webhooks.

- **Port:** 2525 (SMTP)
- **Config:** Mounted from `./config.example.json`
- **Certs:** Mounted from `./certs` (if using TLS)
- **Environment Variables:**
  - `CONFIG_FILE`: Path to configuration file
  - `LOG_LEVEL`: Logging level (debug, info, error)
  - `SMTP_PORT`: Override SMTP port
  - `MAX_EMAIL_SIZE`: Override max email size

### webhook-receiver
A simple HTTP echo server that logs all incoming webhook requests. Perfect for testing.

- **HTTP Port:** 8080
- **HTTPS Port:** 8443
- **Endpoints:** Any path (e.g., `/webhooks/test`, `/webhooks/support`)

The webhook receiver will echo back all headers and body content, making it easy to verify:
- Standard Webhooks headers (webhook-id, webhook-timestamp, webhook-signature)
- Email payload structure
- Signature verification

## Testing Scenarios

### 1. Basic Email Forwarding
```bash
# Send to test@example.com (exact match)
swaks --to test@example.com --from sender@test.com --server localhost:2525 --body "Test"

# Check webhook receiver logs
docker compose logs webhook-receiver | grep "/webhooks/test"
```

### 2. Wildcard Routing
```bash
# Send to any address at example.com (wildcard match)
swaks --to anything@example.com --from sender@test.com --server localhost:2525 --body "Test"

# Should hit the catchall webhook
docker compose logs webhook-receiver | grep "/webhooks/catchall"
```

### 3. Multiple Recipients (Accumulative Matching)
```bash
# Send to support@example.com (matches both exact and wildcard)
swaks --to support@example.com --from sender@test.com --server localhost:2525 --body "Test"

# Should hit both /webhooks/support AND /webhooks/catchall
docker compose logs webhook-receiver
```

### 4. Default Webhook
```bash
# Send to unmatched address
swaks --to unknown@other.com --from sender@test.com --server localhost:2525 --body "Test"

# Should hit the default webhook
docker compose logs webhook-receiver | grep "/webhooks/default"
```

### 5. With Authentication (using config.tls.example.json)
```bash
swaks --to test@example.com \
      --from sender@test.com \
      --server localhost:2525 \
      --auth PLAIN \
      --auth-user testuser \
      --auth-password testpass123 \
      --body "Authenticated test"
```

### 6. Webhook Signature Verification
The webhook receiver will show the `webhook-signature` header when a secret is configured. You can verify the signature using:

```bash
# Extract values from webhook receiver logs
WEBHOOK_ID="<from webhook-id header>"
TIMESTAMP="<from webhook-timestamp header>"
BODY="<JSON payload>"
SECRET="whsec_test_secret_123"

# Calculate expected signature
echo -n "${WEBHOOK_ID}.${TIMESTAMP}.${BODY}" | openssl dgst -sha256 -hmac "${SECRET}" -binary | base64
```

## Troubleshooting

### SMTP Connection Refused
```bash
# Check if service is running
docker compose ps

# Check logs for errors
docker compose logs smtp-forwarder

# Verify port is exposed
netstat -an | grep 2525
```

### Webhook Not Receiving Requests
```bash
# Check network connectivity
docker compose exec smtp-forwarder ping webhook-receiver

# Verify webhook receiver is running
docker compose logs webhook-receiver

# Check SMTP forwarder logs for webhook errors
docker compose logs smtp-forwarder | grep -i webhook
```

### TLS Certificate Errors
```bash
# Verify certificates exist
ls -la certs/

# Check certificate validity
openssl x509 -in certs/server.crt -text -noout

# Regenerate if needed
openssl req -x509 -newkey rsa:4096 -keyout certs/server.key -out certs/server.crt -days 365 -nodes -subj "/CN=smtp-forwarder"
```

## Environment Variables

You can override configuration using environment variables in `docker-compose.yml`:

```yaml
environment:
  - CONFIG_FILE=/etc/smtp-forwarder/config.json
  - LOG_LEVEL=debug  # Change to debug for verbose logging
  - SMTP_PORT=2525
  - MAX_EMAIL_SIZE=10485760
```

## Health Checks

Both services include health checks:

```bash
# Check service health
docker compose ps

# Manually test SMTP health
nc -zv localhost 2525

# Manually test webhook receiver health
curl http://localhost:8080/health
```

## Production Considerations

This Docker Compose setup is designed for **local testing only**. For production:

1. Use proper TLS certificates (not self-signed)
2. Store secrets in a secure secret management system
3. Configure proper authentication credentials
4. Set up monitoring and alerting
5. Use a production-grade webhook endpoint
6. Configure resource limits
7. Set up log aggregation
8. Use orchestration platforms (Kubernetes, ECS, etc.)

## Cleaning Up

```bash
# Stop and remove containers
docker compose down

# Remove volumes and networks
docker compose down -v

# Remove images
docker compose down --rmi all
```
