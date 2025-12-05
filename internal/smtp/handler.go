package smtp

import (
	"context"
	"fmt"
	"io"
	"time"

	"smtp-webhook-forwarder/internal/config"
	"smtp-webhook-forwarder/internal/logger"
	"smtp-webhook-forwarder/internal/parser"
	"smtp-webhook-forwarder/internal/router"
	"smtp-webhook-forwarder/internal/session"
	"smtp-webhook-forwarder/internal/webhook"
)

// Handler coordinates the email processing pipeline:
// parse → route → webhook → map status
type Handler struct {
	parser        parser.Parser
	router        router.Router
	webhookClient webhook.Client
	sessionGen    session.Generator
	logger        logger.Logger
	maxEmailSize  int64
}

// NewHandler creates a new message handler
func NewHandler(
	parser parser.Parser,
	router router.Router,
	webhookClient webhook.Client,
	sessionGen session.Generator,
	logger logger.Logger,
	maxEmailSize int64,
) *Handler {
	return &Handler{
		parser:        parser,
		router:        router,
		webhookClient: webhookClient,
		sessionGen:    sessionGen,
		logger:        logger,
		maxEmailSize:  maxEmailSize,
	}
}

// Handle processes an incoming email message
// Requirements: 1.6, 2.1, 2.2, 2.3, 2.4, 2.5, 2.6, 10.3, 10.4, 11.1, 11.2, 12.1, 12.2, 12.3, 12.4
func (h *Handler) Handle(ctx context.Context, from string, to []string, data io.Reader) error {
	// Generate session ID for this message
	sessionID := h.sessionGen.Generate()
	sessionLogger := h.logger.WithSession(sessionID)

	sessionLogger.Info("received email",
		logger.Str("from", from),
		logger.Strs("to", to),
	)

	// Read email data
	emailData, err := io.ReadAll(data)
	if err != nil {
		sessionLogger.Error("failed to read email data", err)
		return &SMTPError{Code: 451, Message: "Failed to read message"}
	}

	// Validate email size
	emailSize := int64(len(emailData))
	if emailSize > h.maxEmailSize {
		sessionLogger.Error("email size exceeds limit", nil,
			logger.Int64("size", emailSize),
			logger.Int64("limit", h.maxEmailSize),
		)
		return &SMTPError{Code: 552, Message: "Message size exceeds fixed limit"}
	}

	// Log email metadata
	sessionLogger.Info("processing email",
		logger.Str("from", from),
		logger.Strs("recipients", to),
		logger.Int64("size", emailSize),
	)

	// Parse email
	parsedEmail, err := h.parser.Parse(emailData)
	if err != nil {
		sessionLogger.Error("failed to parse email", err)
		return &SMTPError{Code: 500, Message: "Failed to parse message"}
	}

	sessionLogger.Debug("email parsed successfully",
		logger.Str("subject", parsedEmail.Subject),
		logger.Int("attachments", len(parsedEmail.Attachments)),
	)

	// Match recipients to webhooks
	var allWebhooks []config.Webhook
	webhookMap := make(map[string]bool) // For deduplication by URL

	for _, recipient := range to {
		matches := h.router.Match(recipient)
		for _, wh := range matches {
			// Deduplicate webhooks by URL
			if !webhookMap[wh.URL] {
				webhookMap[wh.URL] = true
				allWebhooks = append(allWebhooks, wh)
			}
		}
	}

	// If no matches found, use default webhook
	if len(allWebhooks) == 0 {
		defaultWebhook := h.router.GetDefault()
		if defaultWebhook == nil {
			sessionLogger.Error("no matching routes and no default webhook", nil,
				logger.Strs("recipients", to),
			)
			return &SMTPError{Code: 550, Message: "No route to recipient"}
		}
		allWebhooks = append(allWebhooks, *defaultWebhook)
		sessionLogger.Info("using default webhook")
	}

	sessionLogger.Info("matched webhooks",
		logger.Int("count", len(allWebhooks)),
	)

	// Send to all matching webhooks
	// Track the "worst" SMTP code to return (550 > 451 > 250)
	worstSMTPCode := 250

	for _, wh := range allWebhooks {
		smtpCode, _ := h.sendToWebhook(ctx, sessionID, sessionLogger, wh, parsedEmail)

		// Update worst SMTP code
		if smtpCode == 550 {
			worstSMTPCode = 550
		} else if smtpCode == 451 && worstSMTPCode != 550 {
			worstSMTPCode = 451
		}
	}

	// Return appropriate SMTP response based on worst code
	if worstSMTPCode == 250 {
		sessionLogger.Info("email processed successfully")
		return nil
	} else if worstSMTPCode == 451 {
		return &SMTPError{Code: 451, Message: "Temporary failure"}
	} else {
		return &SMTPError{Code: 550, Message: "Permanent failure"}
	}
}

// sendToWebhook sends the email to a single webhook endpoint
func (h *Handler) sendToWebhook(
	ctx context.Context,
	sessionID string,
	sessionLogger logger.Logger,
	wh config.Webhook,
	parsedEmail *parser.ParsedEmail,
) (int, error) {
	// Create webhook payload
	payload := &webhook.WebhookPayload{
		SessionID: sessionID,
		Timestamp: time.Now().Unix(),
		Email:     parsedEmail,
	}

	// Convert config.Webhook to webhook.WebhookConfig
	webhookConfig := webhook.WebhookConfig{
		URL:     wh.URL,
		Secret:  wh.Secret,
		Timeout: wh.Timeout,
	}

	sessionLogger.Info("sending to webhook",
		logger.Str("url", wh.URL),
	)

	// Send webhook request
	resp, _ := h.webhookClient.Send(ctx, webhookConfig, payload)

	// Handle webhook response
	if resp.Error != nil {
		sessionLogger.Error("webhook delivery failed", resp.Error,
			logger.Str("url", wh.URL),
			logger.Str("error", resp.Error.Error()),
		)
		// Map error to SMTP code
		smtpCode := MapHTTPToSMTP(0, resp.Error)
		return smtpCode, resp.Error
	}

	sessionLogger.Info("webhook response received",
		logger.Str("url", wh.URL),
		logger.Int("status_code", resp.StatusCode),
	)

	// Map HTTP status to SMTP code
	smtpCode := MapHTTPToSMTP(resp.StatusCode, nil)

	if smtpCode != 250 {
		sessionLogger.Error("webhook returned error status", nil,
			logger.Str("url", wh.URL),
			logger.Int("http_status", resp.StatusCode),
			logger.Int("smtp_code", smtpCode),
		)
		return smtpCode, fmt.Errorf("webhook returned status %d", resp.StatusCode)
	}

	return smtpCode, nil
}

// SMTPError represents an SMTP protocol error
type SMTPError struct {
	Code    int
	Message string
}

// Error implements the error interface
func (e *SMTPError) Error() string {
	return fmt.Sprintf("%d %s", e.Code, e.Message)
}
