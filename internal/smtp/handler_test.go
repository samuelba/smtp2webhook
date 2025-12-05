package smtp

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"smtp-webhook-forwarder/internal/config"
	"smtp-webhook-forwarder/internal/logger"
	"smtp-webhook-forwarder/internal/parser"
	"smtp-webhook-forwarder/internal/webhook"
)

// Mock implementations for testing

// mockParser implements parser.Parser
type mockParser struct {
	parseFunc func(data []byte) (*parser.ParsedEmail, error)
}

func (m *mockParser) Parse(data []byte) (*parser.ParsedEmail, error) {
	if m.parseFunc != nil {
		return m.parseFunc(data)
	}
	return &parser.ParsedEmail{
		From:    "sender@example.com",
		To:      []string{"recipient@example.com"},
		Subject: "Test Email",
		Headers: map[string][]string{
			"From": {"sender@example.com"},
			"To":   {"recipient@example.com"},
		},
		TextBody:    "Test body",
		HTMLBody:    "",
		Attachments: []parser.Attachment{},
	}, nil
}

// mockRouter implements router.Router
type mockRouter struct {
	matchFunc      func(recipient string) []config.Webhook
	getDefaultFunc func() *config.Webhook
}

func (m *mockRouter) Match(recipient string) []config.Webhook {
	if m.matchFunc != nil {
		return m.matchFunc(recipient)
	}
	return []config.Webhook{
		{URL: "https://example.com/webhook", Secret: "", Timeout: 30},
	}
}

func (m *mockRouter) GetDefault() *config.Webhook {
	if m.getDefaultFunc != nil {
		return m.getDefaultFunc()
	}
	return nil
}

// mockWebhookClient implements webhook.Client
type mockWebhookClient struct {
	sendFunc func(ctx context.Context, webhook webhook.WebhookConfig, payload *webhook.WebhookPayload) (*webhook.WebhookResponse, error)
}

func (m *mockWebhookClient) Send(ctx context.Context, wh webhook.WebhookConfig, payload *webhook.WebhookPayload) (*webhook.WebhookResponse, error) {
	if m.sendFunc != nil {
		return m.sendFunc(ctx, wh, payload)
	}
	return &webhook.WebhookResponse{
		StatusCode: 200,
		Body:       []byte("OK"),
		Error:      nil,
	}, nil
}

// mockSessionGenerator implements session.Generator
type mockSessionGenerator struct {
	generateFunc func() string
}

func (m *mockSessionGenerator) Generate() string {
	if m.generateFunc != nil {
		return m.generateFunc()
	}
	return "test-session-id-12345"
}

// TestHandler_EmailSizeValidation tests that emails exceeding max size are rejected
// Requirements: 1.6
func TestHandler_EmailSizeValidation(t *testing.T) {
	tests := []struct {
		name         string
		emailSize    int
		maxEmailSize int64
		wantCode     int
		wantMessage  string
	}{
		{
			name:         "email within limit",
			emailSize:    100,
			maxEmailSize: 1000,
			wantCode:     0, // success, no error
			wantMessage:  "",
		},
		{
			name:         "email at exact limit",
			emailSize:    1000,
			maxEmailSize: 1000,
			wantCode:     0, // success, no error
			wantMessage:  "",
		},
		{
			name:         "email exceeds limit by 1 byte",
			emailSize:    1001,
			maxEmailSize: 1000,
			wantCode:     552,
			wantMessage:  "Message size exceeds fixed limit",
		},
		{
			name:         "email far exceeds limit",
			emailSize:    10000,
			maxEmailSize: 1000,
			wantCode:     552,
			wantMessage:  "Message size exceeds fixed limit",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test email data
			emailData := bytes.Repeat([]byte("a"), tt.emailSize)

			// Create handler with mocks
			handler := NewHandler(
				&mockParser{},
				&mockRouter{},
				&mockWebhookClient{},
				&mockSessionGenerator{},
				logger.New(io.Discard),
				tt.maxEmailSize,
			)

			// Handle the message
			err := handler.Handle(context.Background(), "sender@example.com", []string{"recipient@example.com"}, bytes.NewReader(emailData))

			if tt.wantCode == 0 {
				// Should succeed
				if err != nil {
					t.Errorf("Handle() error = %v, want nil", err)
				}
			} else {
				// Should fail with specific SMTP error
				if err == nil {
					t.Errorf("Handle() error = nil, want SMTP error %d", tt.wantCode)
					return
				}

				smtpErr, ok := err.(*SMTPError)
				if !ok {
					t.Errorf("Handle() error type = %T, want *SMTPError", err)
					return
				}

				if smtpErr.Code != tt.wantCode {
					t.Errorf("Handle() SMTP code = %d, want %d", smtpErr.Code, tt.wantCode)
				}

				if !strings.Contains(smtpErr.Message, tt.wantMessage) {
					t.Errorf("Handle() SMTP message = %q, want to contain %q", smtpErr.Message, tt.wantMessage)
				}
			}
		})
	}
}

// TestHandler_SuccessfulMessageFlow tests the complete successful message processing flow
// Requirements: 10.3, 10.4
func TestHandler_SuccessfulMessageFlow(t *testing.T) {
	emailData := []byte("From: sender@example.com\r\nTo: recipient@example.com\r\nSubject: Test\r\n\r\nTest body")

	// Track that all components were called
	parserCalled := false
	routerCalled := false
	webhookCalled := false

	handler := NewHandler(
		&mockParser{
			parseFunc: func(data []byte) (*parser.ParsedEmail, error) {
				parserCalled = true
				return &parser.ParsedEmail{
					From:        "sender@example.com",
					To:          []string{"recipient@example.com"},
					Subject:     "Test",
					Headers:     map[string][]string{},
					TextBody:    "Test body",
					HTMLBody:    "",
					Attachments: []parser.Attachment{},
				}, nil
			},
		},
		&mockRouter{
			matchFunc: func(recipient string) []config.Webhook {
				routerCalled = true
				return []config.Webhook{
					{URL: "https://example.com/webhook", Secret: "", Timeout: 30},
				}
			},
		},
		&mockWebhookClient{
			sendFunc: func(ctx context.Context, wh webhook.WebhookConfig, payload *webhook.WebhookPayload) (*webhook.WebhookResponse, error) {
				webhookCalled = true
				return &webhook.WebhookResponse{
					StatusCode: 200,
					Body:       []byte("OK"),
					Error:      nil,
				}, nil
			},
		},
		&mockSessionGenerator{},
		logger.New(io.Discard),
		10000,
	)

	err := handler.Handle(context.Background(), "sender@example.com", []string{"recipient@example.com"}, bytes.NewReader(emailData))

	if err != nil {
		t.Errorf("Handle() error = %v, want nil", err)
	}

	if !parserCalled {
		t.Error("Parser was not called")
	}

	if !routerCalled {
		t.Error("Router was not called")
	}

	if !webhookCalled {
		t.Error("Webhook client was not called")
	}
}

// TestHandler_ErrorHandlingWithoutCrashes tests that various errors are handled gracefully
// Requirements: 10.3, 10.4
func TestHandler_ErrorHandlingWithoutCrashes(t *testing.T) {
	tests := []struct {
		name         string
		setupHandler func() *Handler
		wantCode     int
		wantErr      bool
	}{
		{
			name: "parser error",
			setupHandler: func() *Handler {
				return NewHandler(
					&mockParser{
						parseFunc: func(data []byte) (*parser.ParsedEmail, error) {
							return nil, errors.New("parse error")
						},
					},
					&mockRouter{},
					&mockWebhookClient{},
					&mockSessionGenerator{},
					logger.New(io.Discard),
					10000,
				)
			},
			wantCode: 500,
			wantErr:  true,
		},
		{
			name: "no matching routes and no default",
			setupHandler: func() *Handler {
				return NewHandler(
					&mockParser{},
					&mockRouter{
						matchFunc: func(recipient string) []config.Webhook {
							return []config.Webhook{} // No matches
						},
						getDefaultFunc: func() *config.Webhook {
							return nil // No default
						},
					},
					&mockWebhookClient{},
					&mockSessionGenerator{},
					logger.New(io.Discard),
					10000,
				)
			},
			wantCode: 550,
			wantErr:  true,
		},
		{
			name: "webhook returns 500",
			setupHandler: func() *Handler {
				return NewHandler(
					&mockParser{},
					&mockRouter{},
					&mockWebhookClient{
						sendFunc: func(ctx context.Context, wh webhook.WebhookConfig, payload *webhook.WebhookPayload) (*webhook.WebhookResponse, error) {
							return &webhook.WebhookResponse{
								StatusCode: 500,
								Body:       []byte("Internal Server Error"),
								Error:      nil,
							}, nil
						},
					},
					&mockSessionGenerator{},
					logger.New(io.Discard),
					10000,
				)
			},
			wantCode: 451,
			wantErr:  true,
		},
		{
			name: "webhook returns 429",
			setupHandler: func() *Handler {
				return NewHandler(
					&mockParser{},
					&mockRouter{},
					&mockWebhookClient{
						sendFunc: func(ctx context.Context, wh webhook.WebhookConfig, payload *webhook.WebhookPayload) (*webhook.WebhookResponse, error) {
							return &webhook.WebhookResponse{
								StatusCode: 429,
								Body:       []byte("Too Many Requests"),
								Error:      nil,
							}, nil
						},
					},
					&mockSessionGenerator{},
					logger.New(io.Discard),
					10000,
				)
			},
			wantCode: 451,
			wantErr:  true,
		},
		{
			name: "webhook returns 404",
			setupHandler: func() *Handler {
				return NewHandler(
					&mockParser{},
					&mockRouter{},
					&mockWebhookClient{
						sendFunc: func(ctx context.Context, wh webhook.WebhookConfig, payload *webhook.WebhookPayload) (*webhook.WebhookResponse, error) {
							return &webhook.WebhookResponse{
								StatusCode: 404,
								Body:       []byte("Not Found"),
								Error:      nil,
							}, nil
						},
					},
					&mockSessionGenerator{},
					logger.New(io.Discard),
					10000,
				)
			},
			wantCode: 550,
			wantErr:  true,
		},
		{
			name: "webhook connection error",
			setupHandler: func() *Handler {
				return NewHandler(
					&mockParser{},
					&mockRouter{},
					&mockWebhookClient{
						sendFunc: func(ctx context.Context, wh webhook.WebhookConfig, payload *webhook.WebhookPayload) (*webhook.WebhookResponse, error) {
							return &webhook.WebhookResponse{
								StatusCode: 0,
								Body:       nil,
								Error:      errors.New("connection refused"),
							}, nil
						},
					},
					&mockSessionGenerator{},
					logger.New(io.Discard),
					10000,
				)
			},
			wantCode: 451,
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := tt.setupHandler()
			emailData := []byte("From: sender@example.com\r\nTo: recipient@example.com\r\n\r\nTest")

			// This should not panic
			err := handler.Handle(context.Background(), "sender@example.com", []string{"recipient@example.com"}, bytes.NewReader(emailData))

			if tt.wantErr {
				if err == nil {
					t.Errorf("Handle() error = nil, want error")
					return
				}

				smtpErr, ok := err.(*SMTPError)
				if !ok {
					t.Errorf("Handle() error type = %T, want *SMTPError", err)
					return
				}

				if smtpErr.Code != tt.wantCode {
					t.Errorf("Handle() SMTP code = %d, want %d", smtpErr.Code, tt.wantCode)
				}
			} else {
				if err != nil {
					t.Errorf("Handle() error = %v, want nil", err)
				}
			}
		})
	}
}

// TestHandler_SessionIDPropagation tests that session IDs are properly generated and used
// Requirements: 12.1
func TestHandler_SessionIDPropagation(t *testing.T) {
	expectedSessionID := "test-session-abc123"
	var capturedSessionID string

	handler := NewHandler(
		&mockParser{},
		&mockRouter{},
		&mockWebhookClient{
			sendFunc: func(ctx context.Context, wh webhook.WebhookConfig, payload *webhook.WebhookPayload) (*webhook.WebhookResponse, error) {
				// Capture the session ID from the webhook payload
				capturedSessionID = payload.SessionID
				return &webhook.WebhookResponse{
					StatusCode: 200,
					Body:       []byte("OK"),
					Error:      nil,
				}, nil
			},
		},
		&mockSessionGenerator{
			generateFunc: func() string {
				return expectedSessionID
			},
		},
		logger.New(io.Discard),
		10000,
	)

	emailData := []byte("From: sender@example.com\r\nTo: recipient@example.com\r\n\r\nTest")
	err := handler.Handle(context.Background(), "sender@example.com", []string{"recipient@example.com"}, bytes.NewReader(emailData))

	if err != nil {
		t.Errorf("Handle() error = %v, want nil", err)
	}

	if capturedSessionID != expectedSessionID {
		t.Errorf("Session ID in webhook payload = %q, want %q", capturedSessionID, expectedSessionID)
	}
}

// TestHandler_MultipleWebhooks tests that emails are sent to all matching webhooks
func TestHandler_MultipleWebhooks(t *testing.T) {
	webhooksCalled := make(map[string]bool)

	handler := NewHandler(
		&mockParser{},
		&mockRouter{
			matchFunc: func(recipient string) []config.Webhook {
				return []config.Webhook{
					{URL: "https://example.com/webhook1", Secret: "", Timeout: 30},
					{URL: "https://example.com/webhook2", Secret: "", Timeout: 30},
					{URL: "https://example.com/webhook3", Secret: "", Timeout: 30},
				}
			},
		},
		&mockWebhookClient{
			sendFunc: func(ctx context.Context, wh webhook.WebhookConfig, payload *webhook.WebhookPayload) (*webhook.WebhookResponse, error) {
				webhooksCalled[wh.URL] = true
				return &webhook.WebhookResponse{
					StatusCode: 200,
					Body:       []byte("OK"),
					Error:      nil,
				}, nil
			},
		},
		&mockSessionGenerator{},
		logger.New(io.Discard),
		10000,
	)

	emailData := []byte("From: sender@example.com\r\nTo: recipient@example.com\r\n\r\nTest")
	err := handler.Handle(context.Background(), "sender@example.com", []string{"recipient@example.com"}, bytes.NewReader(emailData))

	if err != nil {
		t.Errorf("Handle() error = %v, want nil", err)
	}

	expectedWebhooks := []string{
		"https://example.com/webhook1",
		"https://example.com/webhook2",
		"https://example.com/webhook3",
	}

	for _, url := range expectedWebhooks {
		if !webhooksCalled[url] {
			t.Errorf("Webhook %s was not called", url)
		}
	}

	if len(webhooksCalled) != len(expectedWebhooks) {
		t.Errorf("Expected %d webhooks to be called, got %d", len(expectedWebhooks), len(webhooksCalled))
	}
}

// TestHandler_WorstSMTPCodeReturned tests that the worst SMTP code is returned when multiple webhooks fail
func TestHandler_WorstSMTPCodeReturned(t *testing.T) {
	tests := []struct {
		name             string
		webhookResponses []int
		wantCode         int
	}{
		{
			name:             "all success",
			webhookResponses: []int{200, 200, 200},
			wantCode:         0, // no error
		},
		{
			name:             "one temporary failure",
			webhookResponses: []int{200, 500, 200},
			wantCode:         451,
		},
		{
			name:             "one permanent failure",
			webhookResponses: []int{200, 404, 200},
			wantCode:         550,
		},
		{
			name:             "temporary and permanent failures",
			webhookResponses: []int{500, 404, 200},
			wantCode:         550, // 550 is worse than 451
		},
		{
			name:             "all temporary failures",
			webhookResponses: []int{500, 503, 429},
			wantCode:         451,
		},
		{
			name:             "all permanent failures",
			webhookResponses: []int{404, 400, 403},
			wantCode:         550,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			responseIndex := 0

			handler := NewHandler(
				&mockParser{},
				&mockRouter{
					matchFunc: func(recipient string) []config.Webhook {
						webhooks := make([]config.Webhook, len(tt.webhookResponses))
						for i := range webhooks {
							webhooks[i] = config.Webhook{
								URL:     "https://example.com/webhook" + string(rune('1'+i)),
								Secret:  "",
								Timeout: 30,
							}
						}
						return webhooks
					},
				},
				&mockWebhookClient{
					sendFunc: func(ctx context.Context, wh webhook.WebhookConfig, payload *webhook.WebhookPayload) (*webhook.WebhookResponse, error) {
						statusCode := tt.webhookResponses[responseIndex]
						responseIndex++
						return &webhook.WebhookResponse{
							StatusCode: statusCode,
							Body:       []byte("Response"),
							Error:      nil,
						}, nil
					},
				},
				&mockSessionGenerator{},
				logger.New(io.Discard),
				10000,
			)

			emailData := []byte("From: sender@example.com\r\nTo: recipient@example.com\r\n\r\nTest")
			err := handler.Handle(context.Background(), "sender@example.com", []string{"recipient@example.com"}, bytes.NewReader(emailData))

			if tt.wantCode == 0 {
				if err != nil {
					t.Errorf("Handle() error = %v, want nil", err)
				}
			} else {
				if err == nil {
					t.Errorf("Handle() error = nil, want SMTP error %d", tt.wantCode)
					return
				}

				smtpErr, ok := err.(*SMTPError)
				if !ok {
					t.Errorf("Handle() error type = %T, want *SMTPError", err)
					return
				}

				if smtpErr.Code != tt.wantCode {
					t.Errorf("Handle() SMTP code = %d, want %d", smtpErr.Code, tt.wantCode)
				}
			}
		})
	}
}

// TestHandler_DefaultWebhookUsed tests that default webhook is used when no routes match
func TestHandler_DefaultWebhookUsed(t *testing.T) {
	defaultWebhookCalled := false

	handler := NewHandler(
		&mockParser{},
		&mockRouter{
			matchFunc: func(recipient string) []config.Webhook {
				return []config.Webhook{} // No matches
			},
			getDefaultFunc: func() *config.Webhook {
				return &config.Webhook{
					URL:     "https://example.com/default",
					Secret:  "",
					Timeout: 30,
				}
			},
		},
		&mockWebhookClient{
			sendFunc: func(ctx context.Context, wh webhook.WebhookConfig, payload *webhook.WebhookPayload) (*webhook.WebhookResponse, error) {
				if wh.URL == "https://example.com/default" {
					defaultWebhookCalled = true
				}
				return &webhook.WebhookResponse{
					StatusCode: 200,
					Body:       []byte("OK"),
					Error:      nil,
				}, nil
			},
		},
		&mockSessionGenerator{},
		logger.New(io.Discard),
		10000,
	)

	emailData := []byte("From: sender@example.com\r\nTo: recipient@example.com\r\n\r\nTest")
	err := handler.Handle(context.Background(), "sender@example.com", []string{"recipient@example.com"}, bytes.NewReader(emailData))

	if err != nil {
		t.Errorf("Handle() error = %v, want nil", err)
	}

	if !defaultWebhookCalled {
		t.Error("Default webhook was not called")
	}
}
