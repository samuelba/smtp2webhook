package parser

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"io"
	"mime"
	"strings"

	"github.com/emersion/go-message/mail"
)

// ParsedEmail represents a parsed email message
type ParsedEmail struct {
	From        string              `json:"from"`
	To          []string            `json:"to"`
	Subject     string              `json:"subject"`
	Headers     map[string][]string `json:"headers"`
	TextBody    string              `json:"text_body"`
	HTMLBody    string              `json:"html_body"`
	Attachments []Attachment        `json:"attachments"`
}

// Attachment represents an email attachment
type Attachment struct {
	Filename    string `json:"filename"`
	ContentType string `json:"content_type"`
	Data        string `json:"data"` // Base64 encoded
	Size        int    `json:"size"`
}

// Parser defines the interface for email parsing
type Parser interface {
	Parse(data []byte) (*ParsedEmail, error)
}

// EmailParser implements the Parser interface
type EmailParser struct{}

// NewParser creates a new email parser
func NewParser() *EmailParser {
	return &EmailParser{}
}

// Parse parses raw email data into a structured ParsedEmail
func (p *EmailParser) Parse(data []byte) (*ParsedEmail, error) {
	reader := bytes.NewReader(data)

	// Create mail reader
	mr, err := mail.CreateReader(reader)
	if err != nil {
		return nil, fmt.Errorf("failed to create mail reader: %w", err)
	}

	parsed := &ParsedEmail{
		Headers:     make(map[string][]string),
		Attachments: make([]Attachment, 0),
	}

	// Extract headers
	header := mr.Header

	// Extract From
	if from, err := header.AddressList("From"); err == nil && len(from) > 0 {
		parsed.From = from[0].Address
	}

	// Extract To
	if to, err := header.AddressList("To"); err == nil {
		parsed.To = make([]string, len(to))
		for i, addr := range to {
			parsed.To[i] = addr.Address
		}
	}

	// Extract Subject
	if subject, err := header.Subject(); err == nil {
		parsed.Subject = subject
	}

	// Extract all headers
	fields := header.Fields()
	for fields.Next() {
		key := fields.Key()
		value := fields.Value()
		parsed.Headers[key] = append(parsed.Headers[key], value)
	}

	// Parse message parts
	for {
		part, err := mr.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("failed to read message part: %w", err)
		}

		if err := p.processPart(part, parsed); err != nil {
			return nil, fmt.Errorf("failed to process message part: %w", err)
		}
	}

	return parsed, nil
}

// processPart processes a single message part (body or attachment)
func (p *EmailParser) processPart(part *mail.Part, parsed *ParsedEmail) error {
	// Get content type from header
	contentTypeHeader := part.Header.Get("Content-Type")
	contentType, params, err := mime.ParseMediaType(contentTypeHeader)
	if err != nil {
		contentType = "text/plain"
		params = make(map[string]string)
	}

	// Check if this is an attachment
	dispositionHeader := part.Header.Get("Content-Disposition")
	disposition, dispParams, _ := mime.ParseMediaType(dispositionHeader)
	isAttachment := disposition == "attachment" || disposition == "inline"

	// If it has a filename in Content-Disposition or Content-Type, treat as attachment
	filename := dispParams["filename"]
	if filename == "" {
		filename = params["name"]
	}

	if isAttachment || filename != "" {
		// Process as attachment
		return p.processAttachment(part, parsed, contentType, filename)
	}

	// Process as body part
	switch contentType {
	case "text/plain":
		body, err := io.ReadAll(part.Body)
		if err != nil {
			return fmt.Errorf("failed to read text body: %w", err)
		}
		parsed.TextBody = string(body)

	case "text/html":
		body, err := io.ReadAll(part.Body)
		if err != nil {
			return fmt.Errorf("failed to read HTML body: %w", err)
		}
		parsed.HTMLBody = string(body)

	default:
		// For other content types in body, treat as attachment
		if filename == "" {
			// Generate a filename if none exists
			filename = fmt.Sprintf("attachment_%s", sanitizeContentType(contentType))
		}
		return p.processAttachment(part, parsed, contentType, filename)
	}

	return nil
}

// processAttachment processes an attachment part
func (p *EmailParser) processAttachment(part *mail.Part, parsed *ParsedEmail, contentType, filename string) error {
	// Read attachment data
	data, err := io.ReadAll(part.Body)
	if err != nil {
		return fmt.Errorf("failed to read attachment: %w", err)
	}

	// Decode filename if it's MIME encoded
	decodedFilename, err := decodeRFC2047(filename)
	if err == nil {
		filename = decodedFilename
	}

	// Base64 encode the attachment data
	encoded := base64.StdEncoding.EncodeToString(data)

	attachment := Attachment{
		Filename:    filename,
		ContentType: contentType,
		Data:        encoded,
		Size:        len(data),
	}

	parsed.Attachments = append(parsed.Attachments, attachment)
	return nil
}

// sanitizeContentType creates a safe filename from a content type
func sanitizeContentType(contentType string) string {
	// Remove parameters
	ct := strings.Split(contentType, ";")[0]
	// Replace / with _
	ct = strings.ReplaceAll(ct, "/", "_")
	return ct
}

// decodeRFC2047 decodes MIME encoded-word strings
func decodeRFC2047(s string) (string, error) {
	dec := new(mime.WordDecoder)
	return dec.DecodeHeader(s)
}
