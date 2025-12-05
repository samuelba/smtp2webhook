package parser

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"strings"
	"testing"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
)

// **Feature: smtp-webhook-forwarder, Property 1: Email parsing completeness**
// For any valid email message with headers, body, and attachments, parsing should extract
// all sender, recipient, subject, and message content fields, and encode attachments as Base64.
// Validates: Requirements 1.2, 1.3, 2.1
func TestProperty_EmailParsingCompleteness(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	properties.Property("parsing extracts all email fields and Base64-encodes attachments", prop.ForAll(
		func(email *generatedEmail) bool {
			// Parse the generated email
			parser := NewParser()
			parsed, err := parser.Parse(email.raw)
			if err != nil {
				t.Logf("Parse error: %v", err)
				return false
			}

			// Verify From field
			if parsed.From != email.from {
				t.Logf("From mismatch: expected %q, got %q", email.from, parsed.From)
				return false
			}

			// Verify To field
			if len(parsed.To) != len(email.to) {
				t.Logf("To length mismatch: expected %d, got %d", len(email.to), len(parsed.To))
				return false
			}
			for i, addr := range email.to {
				if parsed.To[i] != addr {
					t.Logf("To[%d] mismatch: expected %q, got %q", i, addr, parsed.To[i])
					return false
				}
			}

			// Verify Subject field
			if parsed.Subject != email.subject {
				t.Logf("Subject mismatch: expected %q, got %q", email.subject, parsed.Subject)
				return false
			}

			// Verify text body (if present)
			if email.textBody != "" && parsed.TextBody != email.textBody {
				t.Logf("TextBody mismatch: expected %q, got %q", email.textBody, parsed.TextBody)
				return false
			}

			// Verify HTML body (if present)
			if email.htmlBody != "" && parsed.HTMLBody != email.htmlBody {
				t.Logf("HTMLBody mismatch: expected %q, got %q", email.htmlBody, parsed.HTMLBody)
				return false
			}

			// Verify attachments
			if len(parsed.Attachments) != len(email.attachments) {
				t.Logf("Attachment count mismatch: expected %d, got %d", len(email.attachments), len(parsed.Attachments))
				return false
			}

			for i, att := range email.attachments {
				parsedAtt := parsed.Attachments[i]

				// Verify filename
				if parsedAtt.Filename != att.filename {
					t.Logf("Attachment[%d] filename mismatch: expected %q, got %q", i, att.filename, parsedAtt.Filename)
					return false
				}

				// Verify content type
				if parsedAtt.ContentType != att.contentType {
					t.Logf("Attachment[%d] content type mismatch: expected %q, got %q", i, att.contentType, parsedAtt.ContentType)
					return false
				}

				// Verify data is Base64 encoded and matches original
				decoded, err := base64.StdEncoding.DecodeString(parsedAtt.Data)
				if err != nil {
					t.Logf("Attachment[%d] data is not valid Base64: %v", i, err)
					return false
				}

				if !bytes.Equal(decoded, att.data) {
					t.Logf("Attachment[%d] data mismatch after Base64 decode", i)
					return false
				}

				// Verify size
				if parsedAtt.Size != len(att.data) {
					t.Logf("Attachment[%d] size mismatch: expected %d, got %d", i, len(att.data), parsedAtt.Size)
					return false
				}
			}

			return true
		},
		genEmail(),
	))

	properties.TestingRun(t)
}

// generatedEmail represents a generated email for testing
type generatedEmail struct {
	from        string
	to          []string
	subject     string
	textBody    string
	htmlBody    string
	attachments []generatedAttachment
	raw         []byte
}

// generatedAttachment represents a generated attachment
type generatedAttachment struct {
	filename    string
	contentType string
	data        []byte
}

// genEmail generates random valid emails
func genEmail() gopter.Gen {
	return gopter.CombineGens(
		genEmailAddress(), // from
		genEmailAddress(), // to1
		genEmailAddress(), // to2
		genSubject(),      // subject
		genBody(),         // text body
		genBody(),         // html body
		gen.Bool(),        // has attachment
		genAttachment(),   // attachment
	).Map(func(values []interface{}) *generatedEmail {
		from := values[0].(string)
		to1 := values[1].(string)
		to2 := values[2].(string)
		subject := values[3].(string)
		textBody := values[4].(string)
		htmlBody := values[5].(string)
		hasAttachment := values[6].(bool)
		attachment := values[7].(generatedAttachment)

		to := []string{to1, to2}
		var attachments []generatedAttachment
		if hasAttachment {
			attachments = []generatedAttachment{attachment}
		} else {
			attachments = []generatedAttachment{}
		}

		email := &generatedEmail{
			from:        from,
			to:          to,
			subject:     subject,
			textBody:    textBody,
			htmlBody:    htmlBody,
			attachments: attachments,
		}

		// Build the raw email
		email.raw = buildRawEmail(email)

		return email
	})
}

// genEmailAddress generates a valid email address
func genEmailAddress() gopter.Gen {
	return gopter.CombineGens(
		gen.Identifier(),
		gen.Identifier(),
		gen.OneConstOf("com", "org", "net", "io"),
	).Map(func(values []interface{}) string {
		user := strings.ToLower(values[0].(string))
		domain := strings.ToLower(values[1].(string))
		tld := values[2].(string)
		if user == "" {
			user = "user"
		}
		if domain == "" {
			domain = "example"
		}
		return fmt.Sprintf("%s@%s.%s", user, domain, tld)
	})
}

// genSubject generates an email subject
func genSubject() gopter.Gen {
	return gen.Identifier().Map(func(s string) string {
		if len(s) > 100 {
			return s[:100]
		}
		if len(s) == 0 {
			return "Subject"
		}
		return s
	})
}

// genBody generates a body (may be empty)
func genBody() gopter.Gen {
	return gen.Identifier().Map(func(s string) string {
		if len(s) > 200 {
			return s[:200]
		}
		return s
	})
}

// genAttachment generates a single attachment with fixed-size data (50 bytes)
func genAttachment() gopter.Gen {
	// Generate 50 bytes of data
	dataGens := make([]gopter.Gen, 50)
	for i := 0; i < 50; i++ {
		dataGens[i] = gen.UInt8()
	}

	return gopter.CombineGens(
		gen.Identifier().Map(func(s string) string {
			if len(s) > 20 {
				return s[:20]
			}
			if len(s) == 0 {
				return "file"
			}
			return s
		}),
		gen.OneConstOf("image/png", "application/pdf", "text/plain", "application/octet-stream"),
		gopter.CombineGens(dataGens...).Map(func(values []interface{}) []uint8 {
			result := make([]uint8, len(values))
			for i, v := range values {
				result[i] = v.(uint8)
			}
			return result
		}),
	).Map(func(values []interface{}) generatedAttachment {
		filename := values[0].(string) + ".dat"
		contentType := values[1].(string)
		data := values[2].([]uint8)
		return generatedAttachment{
			filename:    filename,
			contentType: contentType,
			data:        data,
		}
	})
}

// buildRawEmail constructs a raw RFC 5322 email from the generated components
func buildRawEmail(email *generatedEmail) []byte {
	var buf bytes.Buffer

	// Write headers
	buf.WriteString(fmt.Sprintf("From: <%s>\r\n", email.from))

	toAddrs := make([]string, len(email.to))
	for i, addr := range email.to {
		toAddrs[i] = fmt.Sprintf("<%s>", addr)
	}
	buf.WriteString(fmt.Sprintf("To: %s\r\n", strings.Join(toAddrs, ", ")))
	buf.WriteString(fmt.Sprintf("Subject: %s\r\n", email.subject))
	buf.WriteString("MIME-Version: 1.0\r\n")

	// Determine if we need multipart
	hasMultipleParts := false
	partCount := 0
	if email.textBody != "" {
		partCount++
	}
	if email.htmlBody != "" {
		partCount++
	}
	if len(email.attachments) > 0 {
		partCount += len(email.attachments)
	}
	hasMultipleParts = partCount > 1

	if !hasMultipleParts {
		// Simple message with single part
		if email.textBody != "" {
			buf.WriteString("Content-Type: text/plain; charset=utf-8\r\n")
			buf.WriteString("\r\n")
			buf.WriteString(email.textBody)
		} else if email.htmlBody != "" {
			buf.WriteString("Content-Type: text/html; charset=utf-8\r\n")
			buf.WriteString("\r\n")
			buf.WriteString(email.htmlBody)
		} else if len(email.attachments) == 1 {
			att := email.attachments[0]
			buf.WriteString(fmt.Sprintf("Content-Type: %s\r\n", att.contentType))
			buf.WriteString(fmt.Sprintf("Content-Disposition: attachment; filename=\"%s\"\r\n", att.filename))
			buf.WriteString("Content-Transfer-Encoding: base64\r\n")
			buf.WriteString("\r\n")
			buf.WriteString(base64.StdEncoding.EncodeToString(att.data))
		}
	} else {
		// Multipart message
		boundary := "boundary123456789"
		buf.WriteString(fmt.Sprintf("Content-Type: multipart/mixed; boundary=\"%s\"\r\n", boundary))
		buf.WriteString("\r\n")

		// Text body part
		if email.textBody != "" {
			buf.WriteString(fmt.Sprintf("--%s\r\n", boundary))
			buf.WriteString("Content-Type: text/plain; charset=utf-8\r\n")
			buf.WriteString("\r\n")
			buf.WriteString(email.textBody)
			buf.WriteString("\r\n")
		}

		// HTML body part
		if email.htmlBody != "" {
			buf.WriteString(fmt.Sprintf("--%s\r\n", boundary))
			buf.WriteString("Content-Type: text/html; charset=utf-8\r\n")
			buf.WriteString("\r\n")
			buf.WriteString(email.htmlBody)
			buf.WriteString("\r\n")
		}

		// Attachment parts
		for _, att := range email.attachments {
			buf.WriteString(fmt.Sprintf("--%s\r\n", boundary))
			buf.WriteString(fmt.Sprintf("Content-Type: %s\r\n", att.contentType))
			buf.WriteString(fmt.Sprintf("Content-Disposition: attachment; filename=\"%s\"\r\n", att.filename))
			buf.WriteString("Content-Transfer-Encoding: base64\r\n")
			buf.WriteString("\r\n")
			buf.WriteString(base64.StdEncoding.EncodeToString(att.data))
			buf.WriteString("\r\n")
		}

		// End boundary
		buf.WriteString(fmt.Sprintf("--%s--\r\n", boundary))
	}

	return buf.Bytes()
}
