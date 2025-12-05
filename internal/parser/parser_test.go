package parser

import (
	"encoding/base64"
	"strings"
	"testing"
)

// Test parsing a simple plain text email
func TestParse_SimplePlainText(t *testing.T) {
	rawEmail := `From: sender@example.com
To: recipient@example.com
Subject: Test Subject
MIME-Version: 1.0
Content-Type: text/plain; charset=utf-8

This is a plain text email body.`

	parser := NewParser()
	parsed, err := parser.Parse([]byte(rawEmail))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	if parsed.From != "sender@example.com" {
		t.Errorf("From: expected %q, got %q", "sender@example.com", parsed.From)
	}

	if len(parsed.To) != 1 || parsed.To[0] != "recipient@example.com" {
		t.Errorf("To: expected [%q], got %v", "recipient@example.com", parsed.To)
	}

	if parsed.Subject != "Test Subject" {
		t.Errorf("Subject: expected %q, got %q", "Test Subject", parsed.Subject)
	}

	if parsed.TextBody != "This is a plain text email body." {
		t.Errorf("TextBody: expected %q, got %q", "This is a plain text email body.", parsed.TextBody)
	}

	if parsed.HTMLBody != "" {
		t.Errorf("HTMLBody: expected empty, got %q", parsed.HTMLBody)
	}

	if len(parsed.Attachments) != 0 {
		t.Errorf("Attachments: expected 0, got %d", len(parsed.Attachments))
	}
}

// Test parsing an HTML email
func TestParse_HTMLEmail(t *testing.T) {
	rawEmail := `From: sender@example.com
To: recipient@example.com
Subject: HTML Test
MIME-Version: 1.0
Content-Type: text/html; charset=utf-8

<html><body><h1>Hello World</h1></body></html>`

	parser := NewParser()
	parsed, err := parser.Parse([]byte(rawEmail))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	if parsed.HTMLBody != "<html><body><h1>Hello World</h1></body></html>" {
		t.Errorf("HTMLBody: expected HTML content, got %q", parsed.HTMLBody)
	}

	if parsed.TextBody != "" {
		t.Errorf("TextBody: expected empty, got %q", parsed.TextBody)
	}
}

// Test parsing a multipart email with text and HTML
func TestParse_MultipartTextAndHTML(t *testing.T) {
	rawEmail := `From: sender@example.com
To: recipient@example.com
Subject: Multipart Test
MIME-Version: 1.0
Content-Type: multipart/alternative; boundary="boundary123"

--boundary123
Content-Type: text/plain; charset=utf-8

Plain text version

--boundary123
Content-Type: text/html; charset=utf-8

<html><body>HTML version</body></html>

--boundary123--`

	parser := NewParser()
	parsed, err := parser.Parse([]byte(rawEmail))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	if parsed.TextBody != "Plain text version\n" {
		t.Errorf("TextBody: expected %q, got %q", "Plain text version\n", parsed.TextBody)
	}

	if parsed.HTMLBody != "<html><body>HTML version</body></html>\n" {
		t.Errorf("HTMLBody: expected HTML content, got %q", parsed.HTMLBody)
	}
}

// Test parsing email with attachment
func TestParse_WithAttachment(t *testing.T) {
	attachmentData := []byte("This is attachment content")
	encodedData := base64.StdEncoding.EncodeToString(attachmentData)

	rawEmail := `From: sender@example.com
To: recipient@example.com
Subject: Email with Attachment
MIME-Version: 1.0
Content-Type: multipart/mixed; boundary="boundary456"

--boundary456
Content-Type: text/plain; charset=utf-8

Email body with attachment

--boundary456
Content-Type: application/pdf
Content-Disposition: attachment; filename="document.pdf"
Content-Transfer-Encoding: base64

` + encodedData + `

--boundary456--`

	parser := NewParser()
	parsed, err := parser.Parse([]byte(rawEmail))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	if len(parsed.Attachments) != 1 {
		t.Fatalf("Attachments: expected 1, got %d", len(parsed.Attachments))
	}

	att := parsed.Attachments[0]
	if att.Filename != "document.pdf" {
		t.Errorf("Attachment filename: expected %q, got %q", "document.pdf", att.Filename)
	}

	if att.ContentType != "application/pdf" {
		t.Errorf("Attachment content type: expected %q, got %q", "application/pdf", att.ContentType)
	}

	// Verify Base64 encoding
	decoded, err := base64.StdEncoding.DecodeString(att.Data)
	if err != nil {
		t.Errorf("Attachment data is not valid Base64: %v", err)
	}

	if string(decoded) != string(attachmentData) {
		t.Errorf("Attachment data mismatch: expected %q, got %q", string(attachmentData), string(decoded))
	}

	if att.Size != len(attachmentData) {
		t.Errorf("Attachment size: expected %d, got %d", len(attachmentData), att.Size)
	}
}

// Test parsing email with multiple recipients
func TestParse_MultipleRecipients(t *testing.T) {
	rawEmail := `From: sender@example.com
To: recipient1@example.com, recipient2@example.com, recipient3@example.com
Subject: Multiple Recipients
MIME-Version: 1.0
Content-Type: text/plain; charset=utf-8

Email to multiple recipients`

	parser := NewParser()
	parsed, err := parser.Parse([]byte(rawEmail))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	expectedRecipients := []string{
		"recipient1@example.com",
		"recipient2@example.com",
		"recipient3@example.com",
	}

	if len(parsed.To) != len(expectedRecipients) {
		t.Fatalf("To: expected %d recipients, got %d", len(expectedRecipients), len(parsed.To))
	}

	for i, expected := range expectedRecipients {
		if parsed.To[i] != expected {
			t.Errorf("To[%d]: expected %q, got %q", i, expected, parsed.To[i])
		}
	}
}

// Test parsing email with empty body
func TestParse_EmptyBody(t *testing.T) {
	rawEmail := `From: sender@example.com
To: recipient@example.com
Subject: Empty Body
MIME-Version: 1.0
Content-Type: text/plain; charset=utf-8

`

	parser := NewParser()
	parsed, err := parser.Parse([]byte(rawEmail))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	if parsed.From != "sender@example.com" {
		t.Errorf("From: expected %q, got %q", "sender@example.com", parsed.From)
	}

	if parsed.Subject != "Empty Body" {
		t.Errorf("Subject: expected %q, got %q", "Empty Body", parsed.Subject)
	}

	// Empty body should result in empty string
	if parsed.TextBody != "" {
		t.Errorf("TextBody: expected empty string, got %q", parsed.TextBody)
	}
}

// Test parsing email with no attachments
func TestParse_NoAttachments(t *testing.T) {
	rawEmail := `From: sender@example.com
To: recipient@example.com
Subject: No Attachments
MIME-Version: 1.0
Content-Type: text/plain; charset=utf-8

This email has no attachments.`

	parser := NewParser()
	parsed, err := parser.Parse([]byte(rawEmail))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	if len(parsed.Attachments) != 0 {
		t.Errorf("Attachments: expected 0, got %d", len(parsed.Attachments))
	}
}

// Test parsing email with multiple attachments
func TestParse_MultipleAttachments(t *testing.T) {
	data1 := []byte("First attachment")
	data2 := []byte("Second attachment")
	encoded1 := base64.StdEncoding.EncodeToString(data1)
	encoded2 := base64.StdEncoding.EncodeToString(data2)

	rawEmail := `From: sender@example.com
To: recipient@example.com
Subject: Multiple Attachments
MIME-Version: 1.0
Content-Type: multipart/mixed; boundary="boundary789"

--boundary789
Content-Type: text/plain; charset=utf-8

Email with multiple attachments

--boundary789
Content-Type: image/png
Content-Disposition: attachment; filename="image.png"
Content-Transfer-Encoding: base64

` + encoded1 + `

--boundary789
Content-Type: application/pdf
Content-Disposition: attachment; filename="document.pdf"
Content-Transfer-Encoding: base64

` + encoded2 + `

--boundary789--`

	parser := NewParser()
	parsed, err := parser.Parse([]byte(rawEmail))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	if len(parsed.Attachments) != 2 {
		t.Fatalf("Attachments: expected 2, got %d", len(parsed.Attachments))
	}

	// Check first attachment
	if parsed.Attachments[0].Filename != "image.png" {
		t.Errorf("Attachment[0] filename: expected %q, got %q", "image.png", parsed.Attachments[0].Filename)
	}

	// Check second attachment
	if parsed.Attachments[1].Filename != "document.pdf" {
		t.Errorf("Attachment[1] filename: expected %q, got %q", "document.pdf", parsed.Attachments[1].Filename)
	}
}

// Test parsing invalid email format - malformed headers
func TestParse_InvalidFormat_MalformedHeaders(t *testing.T) {
	rawEmail := `This is not a valid email format
No proper headers
Just random text`

	parser := NewParser()
	_, err := parser.Parse([]byte(rawEmail))
	if err == nil {
		t.Error("Expected error for invalid email format, got nil")
	}

	// Verify error message indicates parsing failure
	if err != nil && !strings.Contains(err.Error(), "failed to create mail reader") {
		t.Errorf("Expected 'failed to create mail reader' error, got: %v", err)
	}
}

// Test parsing invalid email format - empty input
func TestParse_InvalidFormat_EmptyInput(t *testing.T) {
	rawEmail := []byte("")

	parser := NewParser()
	parsed, err := parser.Parse(rawEmail)

	// The parser may handle empty input gracefully or return an error
	// Both behaviors are acceptable for edge case handling
	if err != nil {
		// Error is acceptable for empty input
		return
	}

	// If it succeeds, verify it returns a valid but empty structure
	if parsed == nil {
		t.Error("Expected non-nil parsed result for empty input")
	}
}

// Test parsing invalid email format - missing required headers
func TestParse_InvalidFormat_MissingHeaders(t *testing.T) {
	// Email with no From header
	rawEmail := `To: recipient@example.com
Subject: Missing From

Body text`

	parser := NewParser()
	parsed, err := parser.Parse([]byte(rawEmail))

	// The parser should handle missing headers gracefully
	// It may succeed but with empty From field
	if err != nil {
		// If it errors, that's acceptable for invalid format
		return
	}

	// If it succeeds, From should be empty
	if parsed.From != "" {
		t.Errorf("Expected empty From for missing header, got %q", parsed.From)
	}
}

// Test parsing email with special characters in subject
func TestParse_SpecialCharactersInSubject(t *testing.T) {
	rawEmail := `From: sender@example.com
To: recipient@example.com
Subject: Special chars: !@#$%^&*()
MIME-Version: 1.0
Content-Type: text/plain; charset=utf-8

Body text`

	parser := NewParser()
	parsed, err := parser.Parse([]byte(rawEmail))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	if parsed.Subject != "Special chars: !@#$%^&*()" {
		t.Errorf("Subject: expected %q, got %q", "Special chars: !@#$%^&*()", parsed.Subject)
	}
}

// Test parsing email with headers map
func TestParse_HeadersMap(t *testing.T) {
	rawEmail := `From: sender@example.com
To: recipient@example.com
Subject: Headers Test
X-Custom-Header: CustomValue
MIME-Version: 1.0
Content-Type: text/plain; charset=utf-8

Body text`

	parser := NewParser()
	parsed, err := parser.Parse([]byte(rawEmail))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	if parsed.Headers == nil {
		t.Fatal("Headers map is nil")
	}

	// Check that headers are captured
	if len(parsed.Headers) == 0 {
		t.Error("Headers map is empty, expected headers to be captured")
	}

	// Check for custom header
	if customHeaders, ok := parsed.Headers["X-Custom-Header"]; ok {
		if len(customHeaders) == 0 || customHeaders[0] != "CustomValue" {
			t.Errorf("X-Custom-Header: expected %q, got %v", "CustomValue", customHeaders)
		}
	}
}

// Test parsing email with inline attachment
func TestParse_InlineAttachment(t *testing.T) {
	attachmentData := []byte("Inline image data")
	encodedData := base64.StdEncoding.EncodeToString(attachmentData)

	rawEmail := `From: sender@example.com
To: recipient@example.com
Subject: Inline Attachment
MIME-Version: 1.0
Content-Type: multipart/mixed; boundary="boundary999"

--boundary999
Content-Type: text/html; charset=utf-8

<html><body><img src="cid:image1"/></body></html>

--boundary999
Content-Type: image/png
Content-Disposition: inline; filename="inline.png"
Content-Transfer-Encoding: base64

` + encodedData + `

--boundary999--`

	parser := NewParser()
	parsed, err := parser.Parse([]byte(rawEmail))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	// Inline attachments should still be captured
	if len(parsed.Attachments) != 1 {
		t.Fatalf("Attachments: expected 1, got %d", len(parsed.Attachments))
	}

	if parsed.Attachments[0].Filename != "inline.png" {
		t.Errorf("Inline attachment filename: expected %q, got %q", "inline.png", parsed.Attachments[0].Filename)
	}
}
