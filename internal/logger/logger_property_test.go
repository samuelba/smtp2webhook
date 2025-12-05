package logger

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
)

// **Feature: smtp-webhook-forwarder, Property 12: Structured logging format**
// For any log entry, the output should be valid structured format (JSON) suitable for log aggregation.
// Validates: Requirements 11.5
func TestProperty_StructuredLoggingFormat(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	properties.Property("all log entries produce valid JSON output", prop.ForAll(
		func(entry *logEntry) bool {
			// Create a buffer to capture log output
			var buf bytes.Buffer
			logger := New(&buf)

			// Add session ID if present
			if entry.sessionID != "" {
				logger = logger.WithSession(entry.sessionID)
			}

			// Log based on level
			switch entry.level {
			case "info":
				logger.Info(entry.message, entry.fields...)
			case "error":
				logger.Error(entry.message, entry.err, entry.fields...)
			case "debug":
				logger.Debug(entry.message, entry.fields...)
			}

			// Get the output
			output := buf.String()

			// Verify output is not empty
			if len(output) == 0 {
				t.Logf("Empty log output for level=%s, message=%q", entry.level, entry.message)
				return false
			}

			// Each line should be valid JSON
			lines := strings.Split(strings.TrimSpace(output), "\n")
			for _, line := range lines {
				if line == "" {
					continue
				}

				// Try to parse as JSON
				var jsonData map[string]interface{}
				if err := json.Unmarshal([]byte(line), &jsonData); err != nil {
					t.Logf("Invalid JSON output: %s\nError: %v", line, err)
					return false
				}

				// Verify essential fields exist
				// Note: zerolog omits the "message" field when it's empty, which is valid behavior
				if entry.message != "" {
					if _, ok := jsonData["message"]; !ok {
						t.Logf("Missing 'message' field in JSON for non-empty message: %s", line)
						return false
					}
				}

				if _, ok := jsonData["level"]; !ok {
					t.Logf("Missing 'level' field in JSON: %s", line)
					return false
				}

				if _, ok := jsonData["time"]; !ok {
					t.Logf("Missing 'time' field in JSON: %s", line)
					return false
				}

				// If session ID was set, verify it's in the output
				if entry.sessionID != "" {
					if sessionIDVal, ok := jsonData["session_id"]; !ok {
						t.Logf("Missing 'session_id' field when session was set: %s", line)
						return false
					} else if sessionIDVal != entry.sessionID {
						t.Logf("Session ID mismatch: expected %q, got %q", entry.sessionID, sessionIDVal)
						return false
					}
				}

				// If error was logged, verify error field exists
				if entry.level == "error" && entry.err != nil {
					if _, ok := jsonData["error"]; !ok {
						t.Logf("Missing 'error' field for error log: %s", line)
						return false
					}
				}

				// Verify custom fields are present
				for _, field := range entry.fields {
					if _, ok := jsonData[field.Key]; !ok {
						t.Logf("Missing custom field %q in JSON: %s", field.Key, line)
						return false
					}
				}
			}

			return true
		},
		genLogEntry(),
	))

	properties.TestingRun(t)
}

// logEntry represents a log entry for testing
type logEntry struct {
	level     string
	message   string
	sessionID string
	err       error
	fields    []Field
}

// genLogEntry generates random log entries
func genLogEntry() gopter.Gen {
	return gopter.CombineGens(
		genLogLevel(),
		genLogMessage(),
		genSessionID(),
		genError(),
		genFields(),
	).Map(func(values []interface{}) *logEntry {
		return &logEntry{
			level:     values[0].(string),
			message:   values[1].(string),
			sessionID: values[2].(string),
			err:       values[3].(error),
			fields:    values[4].([]Field),
		}
	})
}

// genLogLevel generates random log levels
func genLogLevel() gopter.Gen {
	return gen.OneConstOf("info", "error", "debug")
}

// genLogMessage generates random log messages
func genLogMessage() gopter.Gen {
	return gen.OneGenOf(
		gen.Const("Email received"),
		gen.Const("Webhook sent"),
		gen.Const("Authentication failed"),
		gen.Const("Configuration loaded"),
		gen.Const("Server started"),
		gen.Const("Connection closed"),
		gen.AlphaString(),
	)
}

// genSessionID generates random session IDs (including empty)
func genSessionID() gopter.Gen {
	return gen.OneGenOf(
		gen.Const(""),
		gen.Identifier(),
		gen.AlphaString(),
	)
}

// genError generates random errors (including nil)
func genError() gopter.Gen {
	return gen.OneGenOf(
		gen.Const(error(nil)),
		gen.Const(testError("connection timeout")),
		gen.Const(testError("invalid format")),
		gen.Const(testError("webhook failed")),
	)
}

// testError is a simple error type for testing
type testError string

func (e testError) Error() string {
	return string(e)
}

// genFields generates random field slices
func genFields() gopter.Gen {
	return gen.SliceOfN(5, genField()).Map(func(fields []Field) []Field {
		return fields
	})
}

// genField generates random fields
func genField() gopter.Gen {
	return gopter.CombineGens(
		genFieldKey(),
		genFieldValue(),
	).Map(func(values []interface{}) Field {
		return Field{
			Key:   values[0].(string),
			Value: values[1],
		}
	})
}

// genFieldKey generates random field keys
func genFieldKey() gopter.Gen {
	return gen.OneGenOf(
		gen.Const("sender"),
		gen.Const("recipient"),
		gen.Const("size"),
		gen.Const("status_code"),
		gen.Const("url"),
		gen.Identifier(),
	)
}

// genFieldValue generates random field values
func genFieldValue() gopter.Gen {
	return gen.OneGenOf(
		gen.AlphaString(),
		gen.Int(),
		gen.Int64(),
		gen.Bool(),
	)
}

// **Feature: smtp-webhook-forwarder, Property 9: Session ID propagation in logs**
// For any operation within a session, all log entries should include the session identifier.
// Validates: Requirements 12.2
func TestProperty_SessionIDPropagationInLogs(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	properties.Property("all log entries within a session include the session ID", prop.ForAll(
		func(sessionID string, operations []logOperation) bool {
			// Skip empty session IDs - we're testing that when a session ID is set, it propagates
			if sessionID == "" {
				return true
			}

			// Skip empty operations
			if len(operations) == 0 {
				return true
			}

			// Create a buffer to capture log output
			var buf bytes.Buffer

			// Create logger with session ID
			logger := New(&buf)
			sessionLogger := logger.WithSession(sessionID)

			// Perform all operations with the session logger
			for _, op := range operations {
				switch op.level {
				case "info":
					sessionLogger.Info(op.message, op.fields...)
				case "error":
					sessionLogger.Error(op.message, op.err, op.fields...)
				case "debug":
					sessionLogger.Debug(op.message, op.fields...)
				}
			}

			// Get the output
			output := buf.String()

			// Verify output is not empty
			if len(output) == 0 {
				t.Logf("Empty log output for %d operations", len(operations))
				return false
			}

			// Parse each log line and verify session ID is present
			lines := strings.Split(strings.TrimSpace(output), "\n")
			for i, line := range lines {
				if line == "" {
					continue
				}

				// Parse JSON
				var jsonData map[string]interface{}
				if err := json.Unmarshal([]byte(line), &jsonData); err != nil {
					t.Logf("Invalid JSON output at line %d: %s\nError: %v", i, line, err)
					return false
				}

				// Verify session_id field exists and matches
				sessionIDVal, ok := jsonData["session_id"]
				if !ok {
					t.Logf("Missing 'session_id' field in log entry %d: %s", i, line)
					return false
				}

				if sessionIDVal != sessionID {
					t.Logf("Session ID mismatch at line %d: expected %q, got %q", i, sessionID, sessionIDVal)
					return false
				}
			}

			// Verify we got the expected number of log lines
			nonEmptyLines := 0
			for _, line := range lines {
				if line != "" {
					nonEmptyLines++
				}
			}

			if nonEmptyLines != len(operations) {
				t.Logf("Expected %d log lines, got %d", len(operations), nonEmptyLines)
				return false
			}

			return true
		},
		genNonEmptySessionID(),
		genLogOperations(),
	))

	properties.TestingRun(t)
}

// logOperation represents a single logging operation
type logOperation struct {
	level   string
	message string
	err     error
	fields  []Field
}

// genNonEmptySessionID generates random non-empty session IDs
func genNonEmptySessionID() gopter.Gen {
	return gen.Identifier().SuchThat(func(s string) bool {
		return s != ""
	})
}

// genLogOperations generates a slice of random log operations
func genLogOperations() gopter.Gen {
	return gen.SliceOfN(5, genLogOperation()).Map(func(ops []logOperation) []logOperation {
		// Ensure at least one operation
		if len(ops) == 0 {
			return []logOperation{{
				level:   "info",
				message: "test message",
				err:     nil,
				fields:  []Field{},
			}}
		}
		return ops
	})
}

// genLogOperation generates a single random log operation
func genLogOperation() gopter.Gen {
	return gopter.CombineGens(
		genLogLevel(),
		genLogMessage(),
		genError(),
		genFields(),
	).Map(func(values []interface{}) logOperation {
		return logOperation{
			level:   values[0].(string),
			message: values[1].(string),
			err:     values[2].(error),
			fields:  values[3].([]Field),
		}
	})
}

// **Feature: smtp-webhook-forwarder, Property 14: Email metadata logging**
// For any processed email, the log entry should include sender, recipients, and message size.
// Validates: Requirements 11.1
func TestProperty_EmailMetadataLogging(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	properties.Property("all email processing logs include sender, recipients, and message size", prop.ForAll(
		func(email *emailMetadata) bool {
			// Create a buffer to capture log output
			var buf bytes.Buffer
			logger := New(&buf)

			// Log email processing with metadata
			logger.Info("Email received",
				Str("sender", email.sender),
				Strs("recipients", email.recipients),
				Int64("size", email.size),
			)

			// Get the output
			output := buf.String()

			// Verify output is not empty
			if len(output) == 0 {
				t.Logf("Empty log output for email from %s", email.sender)
				return false
			}

			// Parse JSON
			var jsonData map[string]interface{}
			if err := json.Unmarshal([]byte(strings.TrimSpace(output)), &jsonData); err != nil {
				t.Logf("Invalid JSON output: %s\nError: %v", output, err)
				return false
			}

			// Verify sender field exists and matches
			senderVal, ok := jsonData["sender"]
			if !ok {
				t.Logf("Missing 'sender' field in log entry: %s", output)
				return false
			}
			if senderVal != email.sender {
				t.Logf("Sender mismatch: expected %q, got %q", email.sender, senderVal)
				return false
			}

			// Verify recipients field exists and matches
			recipientsVal, ok := jsonData["recipients"]
			if !ok {
				t.Logf("Missing 'recipients' field in log entry: %s", output)
				return false
			}

			// Convert recipients to []string for comparison
			recipientsSlice, ok := recipientsVal.([]interface{})
			if !ok {
				t.Logf("Recipients field is not an array: %v", recipientsVal)
				return false
			}

			if len(recipientsSlice) != len(email.recipients) {
				t.Logf("Recipients count mismatch: expected %d, got %d", len(email.recipients), len(recipientsSlice))
				return false
			}

			for i, recipient := range recipientsSlice {
				recipientStr, ok := recipient.(string)
				if !ok {
					t.Logf("Recipient at index %d is not a string: %v", i, recipient)
					return false
				}
				if recipientStr != email.recipients[i] {
					t.Logf("Recipient mismatch at index %d: expected %q, got %q", i, email.recipients[i], recipientStr)
					return false
				}
			}

			// Verify size field exists and matches
			sizeVal, ok := jsonData["size"]
			if !ok {
				t.Logf("Missing 'size' field in log entry: %s", output)
				return false
			}

			// JSON numbers are float64
			sizeFloat, ok := sizeVal.(float64)
			if !ok {
				t.Logf("Size field is not a number: %v", sizeVal)
				return false
			}

			if int64(sizeFloat) != email.size {
				t.Logf("Size mismatch: expected %d, got %f", email.size, sizeFloat)
				return false
			}

			return true
		},
		genEmailMetadata(),
	))

	properties.TestingRun(t)
}

// emailMetadata represents email metadata for testing
type emailMetadata struct {
	sender     string
	recipients []string
	size       int64
}

// genEmailMetadata generates random email metadata
func genEmailMetadata() gopter.Gen {
	return gopter.CombineGens(
		genEmailAddress(),
		genRecipientList(),
		genEmailSize(),
	).Map(func(values []interface{}) *emailMetadata {
		return &emailMetadata{
			sender:     values[0].(string),
			recipients: values[1].([]string),
			size:       values[2].(int64),
		}
	})
}

// genEmailAddress generates random email addresses
func genEmailAddress() gopter.Gen {
	return gen.OneGenOf(
		gen.Const("user@example.com"),
		gen.Const("admin@test.org"),
		gen.Const("support@company.net"),
		gen.Const("info@domain.io"),
		gopter.CombineGens(
			gen.Identifier(),
			gen.OneConstOf("example.com", "test.org", "domain.net", "mail.io"),
		).Map(func(values []interface{}) string {
			return values[0].(string) + "@" + values[1].(string)
		}),
	)
}

// genRecipientList generates random recipient lists
func genRecipientList() gopter.Gen {
	return gen.SliceOfN(5, genEmailAddress()).
		SuchThat(func(recipients []string) bool {
			// Ensure at least one recipient
			return len(recipients) > 0
		}).
		Map(func(recipients []string) []string {
			// Ensure we have at least one recipient
			if len(recipients) == 0 {
				return []string{"recipient@example.com"}
			}
			return recipients
		})
}

// genEmailSize generates random email sizes (in bytes)
func genEmailSize() gopter.Gen {
	return gen.Int64Range(0, 10485760) // 0 to 10MB
}

// **Feature: smtp-webhook-forwarder, Property 13: Webhook delivery failure logging**
// For any failed webhook delivery, the log entry should include the webhook URL, HTTP status code, and error message.
// Validates: Requirements 10.1, 11.2
func TestProperty_WebhookDeliveryFailureLogging(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	properties.Property("all webhook failure logs include URL, status code, and error message", prop.ForAll(
		func(failure *webhookFailure) bool {
			// Create a buffer to capture log output
			var buf bytes.Buffer
			logger := New(&buf)

			// Log webhook delivery failure with required fields
			logger.Error("Webhook delivery failed",
				failure.err,
				Str("webhook_url", failure.url),
				Int("status_code", failure.statusCode),
			)

			// Get the output
			output := buf.String()

			// Verify output is not empty
			if len(output) == 0 {
				t.Logf("Empty log output for webhook failure to %s", failure.url)
				return false
			}

			// Parse JSON
			var jsonData map[string]interface{}
			if err := json.Unmarshal([]byte(strings.TrimSpace(output)), &jsonData); err != nil {
				t.Logf("Invalid JSON output: %s\nError: %v", output, err)
				return false
			}

			// Verify webhook_url field exists and matches
			urlVal, ok := jsonData["webhook_url"]
			if !ok {
				t.Logf("Missing 'webhook_url' field in log entry: %s", output)
				return false
			}
			if urlVal != failure.url {
				t.Logf("URL mismatch: expected %q, got %q", failure.url, urlVal)
				return false
			}

			// Verify status_code field exists and matches
			statusCodeVal, ok := jsonData["status_code"]
			if !ok {
				t.Logf("Missing 'status_code' field in log entry: %s", output)
				return false
			}

			// JSON numbers are float64
			statusCodeFloat, ok := statusCodeVal.(float64)
			if !ok {
				t.Logf("Status code field is not a number: %v", statusCodeVal)
				return false
			}

			if int(statusCodeFloat) != failure.statusCode {
				t.Logf("Status code mismatch: expected %d, got %f", failure.statusCode, statusCodeFloat)
				return false
			}

			// Verify error field exists (error message)
			errorVal, ok := jsonData["error"]
			if !ok {
				t.Logf("Missing 'error' field in log entry: %s", output)
				return false
			}

			// Error should be a string
			errorStr, ok := errorVal.(string)
			if !ok {
				t.Logf("Error field is not a string: %v", errorVal)
				return false
			}

			// Verify error message is not empty
			if errorStr == "" {
				t.Logf("Error message is empty in log entry: %s", output)
				return false
			}

			// Verify error message matches the expected error
			if failure.err != nil && errorStr != failure.err.Error() {
				t.Logf("Error message mismatch: expected %q, got %q", failure.err.Error(), errorStr)
				return false
			}

			// Verify log level is "error"
			levelVal, ok := jsonData["level"]
			if !ok {
				t.Logf("Missing 'level' field in log entry: %s", output)
				return false
			}

			if levelVal != "error" {
				t.Logf("Log level should be 'error', got: %q", levelVal)
				return false
			}

			return true
		},
		genWebhookFailure(),
	))

	properties.TestingRun(t)
}

// webhookFailure represents a webhook delivery failure for testing
type webhookFailure struct {
	url        string
	statusCode int
	err        error
}

// genWebhookFailure generates random webhook failures
func genWebhookFailure() gopter.Gen {
	return gopter.CombineGens(
		genWebhookURL(),
		genFailureStatusCode(),
		genWebhookError(),
	).Map(func(values []interface{}) *webhookFailure {
		return &webhookFailure{
			url:        values[0].(string),
			statusCode: values[1].(int),
			err:        values[2].(error),
		}
	})
}

// genWebhookURL generates random webhook URLs
func genWebhookURL() gopter.Gen {
	return gen.OneGenOf(
		gen.Const("https://api.example.com/webhook"),
		gen.Const("https://hooks.slack.com/services/T00000000/B00000000/XXXXXXXXXXXXXXXXXXXX"),
		gen.Const("https://webhook.site/unique-id"),
		gen.Const("http://localhost:8080/webhook"),
		gopter.CombineGens(
			gen.OneConstOf("https", "http"),
			gen.Identifier(),
			gen.OneConstOf("com", "org", "net", "io"),
			gen.Identifier(),
		).Map(func(values []interface{}) string {
			scheme := values[0].(string)
			domain := values[1].(string)
			tld := values[2].(string)
			path := values[3].(string)

			if domain == "" {
				domain = "example"
			}
			if path == "" {
				path = "webhook"
			}

			return fmt.Sprintf("%s://%s.%s/%s", scheme, domain, tld, path)
		}),
	)
}

// genFailureStatusCode generates HTTP status codes that represent failures
func genFailureStatusCode() gopter.Gen {
	return gen.OneGenOf(
		// 4xx client errors
		gen.Const(400), // Bad Request
		gen.Const(401), // Unauthorized
		gen.Const(403), // Forbidden
		gen.Const(404), // Not Found
		gen.Const(408), // Request Timeout
		gen.Const(429), // Too Many Requests
		gen.Const(499), // Client Closed Request

		// 5xx server errors
		gen.Const(500), // Internal Server Error
		gen.Const(502), // Bad Gateway
		gen.Const(503), // Service Unavailable
		gen.Const(504), // Gateway Timeout

		// Random 4xx and 5xx codes
		gen.IntRange(400, 499),
		gen.IntRange(500, 599),
	)
}

// genWebhookError generates random webhook error messages
func genWebhookError() gopter.Gen {
	return gen.OneGenOf(
		gen.Const(testError("connection timeout")),
		gen.Const(testError("connection refused")),
		gen.Const(testError("webhook endpoint not found")),
		gen.Const(testError("internal server error")),
		gen.Const(testError("rate limit exceeded")),
		gen.Const(testError("bad gateway")),
		gen.Const(testError("service unavailable")),
		gen.Const(testError("gateway timeout")),
		gen.Const(testError("invalid response")),
		gen.Const(testError("network unreachable")),
		gen.AlphaString().Map(func(s string) error {
			if s == "" {
				return testError("unknown error")
			}
			return testError(s)
		}),
	)
}
