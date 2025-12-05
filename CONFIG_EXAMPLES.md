# Configuration Examples

This directory contains various configuration examples for the SMTP Webhook Forwarder. Choose the example that best matches your use case and customize it for your needs.

## Quick Start

For a minimal working configuration:
```bash
cp config.minimal.json config.json
# Edit config.json with your webhook URL
```

## Available Examples

### 1. `config.minimal.json` - Minimal Configuration
**Use case:** Quick testing and development

The absolute minimum configuration required to run the service:
- No authentication
- No encryption
- Single wildcard route
- No default webhook

**Start here if:** You're just testing the service locally.

### 2. `config.example.json` - Complete Reference
**Use case:** Understanding all available options

Comprehensive example showing every configuration option with detailed inline comments:
- All security modes explained
- Multiple routing patterns
- Authentication examples
- Webhook secrets
- Environment variable overrides documented

**Start here if:** You want to understand all available configuration options.

### 3. `config.tls.example.json` - TLS/SSL Mode
**Use case:** Implicit TLS encryption from connection start

Configuration for implicit TLS (also called SSL mode):
- Port 465 (traditional SMTPS port)
- `security_mode: "tls"` or `"ssl"` (equivalent)
- Client must connect with TLS from the start
- Authentication enabled
- Webhook with secrets

**Start here if:** You need maximum security and your clients support implicit TLS.

### 4. `config.starttls.example.json` - STARTTLS Mode
**Use case:** Opportunistic TLS encryption (most common)

Configuration for STARTTLS (connection starts plaintext, upgrades to TLS):
- Port 587 (standard submission port)
- `security_mode: "starttls"`
- Connection starts plaintext, upgrades when client issues STARTTLS
- Authentication enabled
- Mixed webhook configurations (with and without secrets)

**Start here if:** You need broad client compatibility with good security.

### 5. `config.noauth.example.json` - No Authentication
**Use case:** Internal/trusted networks only

Configuration without SMTP authentication:
- No encryption (`security_mode: "none"`)
- No authentication required
- Suitable for internal networks only
- **WARNING:** Do not expose to public internet

**Start here if:** You're running in a trusted internal network and don't need authentication.

### 6. `config.nosecret.example.json` - Unsigned Webhooks
**Use case:** Webhook receivers that don't support signature verification

Configuration with webhooks that don't use HMAC signatures:
- STARTTLS encryption
- Authentication enabled
- Webhooks without `secret` field
- Standard Webhooks headers still included (id, timestamp)

**Start here if:** Your webhook receiver doesn't support or need signature verification.

### 7. `config.routing.example.json` - Advanced Routing
**Use case:** Complex routing scenarios

Demonstrates advanced routing patterns:
- Exact email address matching
- Wildcard domain matching
- **Accumulative matching** (one email → multiple webhooks)
- Multiple wildcards for the same domain
- Default webhook fallback
- Inline examples showing which emails go where

**Start here if:** You need to route emails to multiple webhooks or have complex routing requirements.

### 8. `config.production.example.json` - Production Ready
**Use case:** Production deployment

Recommended configuration for production environments:
- STARTTLS on port 587
- Authentication required
- All webhooks use secrets
- Larger resource limits
- Production checklist included
- Security best practices

**Start here if:** You're deploying to production.

## Configuration Structure

All configuration files follow this structure:

```json
{
  "server": {
    "port": 2525,
    "hostname": "smtp.example.com",
    "security_mode": "starttls|tls|ssl|none",
    "tls_cert_path": "/path/to/cert.pem",
    "tls_key_path": "/path/to/key.pem",
    "max_email_size": 10485760,
    "max_concurrent_connections": 50,
    "read_timeout": 60,
    "write_timeout": 60
  },
  "auth": {
    "enabled": true,
    "credentials": [
      {"username": "user", "password": "pass"}
    ]
  },
  "routes": [
    {
      "pattern": "user@domain.com",
      "webhook": {
        "url": "https://api.example.com/webhook",
        "secret": "whsec_...",
        "timeout": 30
      }
    }
  ],
  "defaults": {
    "webhook": {
      "url": "https://api.example.com/default",
      "timeout": 30
    }
  }
}
```

## Security Modes

| Mode | Description | Port | Use Case |
|------|-------------|------|----------|
| `none` | No encryption | 25, 2525 | Internal networks only |
| `starttls` | Opportunistic TLS | 587 | Most common, broad compatibility |
| `tls` or `ssl` | Implicit TLS | 465 | Maximum security |

## Routing Patterns

### Exact Match
```json
{"pattern": "support@example.com"}
```
Matches only `support@example.com`

### Wildcard Domain
```json
{"pattern": "*@example.com"}
```
Matches any address at `example.com` (e.g., `user@example.com`, `admin@example.com`)

### Accumulative Matching
If an email matches multiple patterns, it's sent to **all** matching webhooks:

```json
{
  "routes": [
    {"pattern": "support@example.com", "webhook": {"url": "https://api.example.com/support"}},
    {"pattern": "*@example.com", "webhook": {"url": "https://api.example.com/archive"}}
  ]
}
```

Email to `support@example.com` → sent to **both** webhooks

## Environment Variable Overrides

Configuration values can be overridden with environment variables:

| Environment Variable | Overrides | Example |
|---------------------|-----------|---------|
| `SMTP_PORT` | `server.port` | `SMTP_PORT=2525` |
| `SMTP_HOSTNAME` | `server.hostname` | `SMTP_HOSTNAME=smtp.example.com` |
| `SMTP_SECURITY_MODE` | `server.security_mode` | `SMTP_SECURITY_MODE=starttls` |
| `TLS_CERT_PATH` | `server.tls_cert_path` | `TLS_CERT_PATH=/certs/cert.pem` |
| `TLS_KEY_PATH` | `server.tls_key_path` | `TLS_KEY_PATH=/certs/key.pem` |
| `MAX_EMAIL_SIZE` | `server.max_email_size` | `MAX_EMAIL_SIZE=26214400` |

## Webhook Secrets

Webhook secrets enable HMAC-SHA256 signature verification (Standard Webhooks spec):

**With secret:**
```json
{
  "webhook": {
    "url": "https://api.example.com/webhook",
    "secret": "whsec_abc123",
    "timeout": 30
  }
}
```
Includes `webhook-signature` header with HMAC-SHA256 signature

**Without secret:**
```json
{
  "webhook": {
    "url": "https://api.example.com/webhook",
    "timeout": 30
  }
}
```
No `webhook-signature` header (still includes `webhook-id` and `webhook-timestamp`)

## Testing Your Configuration

Validate your configuration before starting the service:

```bash
# The service validates configuration on startup
./smtp-forwarder -config config.json

# Check logs for validation errors
# Valid config will show: "Configuration loaded successfully"
# Invalid config will show specific validation errors and exit
```

## Common Configuration Patterns

### Pattern 1: Single Application
Route all emails to one webhook:
```json
{
  "routes": [
    {"pattern": "*@yourdomain.com", "webhook": {"url": "https://api.example.com/mail", "timeout": 30}}
  ]
}
```

### Pattern 2: Department Routing
Route different addresses to different webhooks:
```json
{
  "routes": [
    {"pattern": "support@yourdomain.com", "webhook": {"url": "https://api.example.com/support", "timeout": 30}},
    {"pattern": "sales@yourdomain.com", "webhook": {"url": "https://api.example.com/sales", "timeout": 30}},
    {"pattern": "*@yourdomain.com", "webhook": {"url": "https://api.example.com/general", "timeout": 30}}
  ]
}
```

### Pattern 3: Redundancy
Send emails to multiple webhooks for backup:
```json
{
  "routes": [
    {"pattern": "*@yourdomain.com", "webhook": {"url": "https://primary.example.com/mail", "timeout": 30}},
    {"pattern": "*@yourdomain.com", "webhook": {"url": "https://backup.example.com/mail", "timeout": 30}}
  ]
}
```

### Pattern 4: Multi-Domain
Handle multiple domains:
```json
{
  "routes": [
    {"pattern": "*@domain1.com", "webhook": {"url": "https://api.domain1.com/mail", "timeout": 30}},
    {"pattern": "*@domain2.com", "webhook": {"url": "https://api.domain2.com/mail", "timeout": 30}},
    {"pattern": "*@domain3.com", "webhook": {"url": "https://api.domain3.com/mail", "timeout": 30}}
  ]
}
```

## Troubleshooting

### Configuration won't load
- Check JSON syntax (use `jq . < config.json` to validate)
- Ensure all required fields are present
- Check file permissions (service must be able to read the file)

### TLS errors
- Verify certificate and key files exist at specified paths
- Ensure certificate is valid and not expired
- Check file permissions on certificate files
- For testing, you can use self-signed certificates (not recommended for production)

### Authentication not working
- Verify `auth.enabled` is `true`
- Check credentials are correctly formatted
- Passwords are case-sensitive
- Ensure at least one credential is configured when auth is enabled

### Emails rejected with 550
- Check routing patterns match your recipient addresses
- Configure a default webhook to catch unmatched addresses
- Review logs for routing decisions

## Security Best Practices

1. **Always use authentication in production** (`auth.enabled: true`)
2. **Always use encryption in production** (`security_mode: "starttls"` or `"tls"`)
3. **Always use webhook secrets** for signature verification
4. **Use strong passwords** (32+ characters, randomly generated)
5. **Protect configuration file** (`chmod 600 config.json`)
6. **Use environment variables** for secrets in containerized deployments
7. **Keep TLS certificates up to date** (automate renewal with certbot)
8. **Set appropriate resource limits** based on expected load
9. **Monitor logs** for suspicious activity
10. **Don't expose to public internet** without authentication and encryption

## Docker Deployment

When deploying with Docker, mount your configuration file:

```bash
docker run -d \
  -v /path/to/config.json:/etc/smtp-forwarder/config.json:ro \
  -v /path/to/certs:/etc/smtp-forwarder/certs:ro \
  -p 2525:2525 \
  -e CONFIG_FILE=/etc/smtp-forwarder/config.json \
  smtp-webhook-forwarder
```

Or use environment variables to override settings:

```bash
docker run -d \
  -v /path/to/config.json:/etc/smtp-forwarder/config.json:ro \
  -p 587:587 \
  -e SMTP_PORT=587 \
  -e SMTP_HOSTNAME=smtp.example.com \
  smtp-webhook-forwarder
```

## Need Help?

- Review the main README.md for general documentation
- Check the design document at `.kiro/specs/smtp-webhook-forwarder/design.md`
- Review requirements at `.kiro/specs/smtp-webhook-forwarder/requirements.md`
- Check logs for detailed error messages
