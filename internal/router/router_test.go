package router

import (
	"testing"

	"smtp-webhook-forwarder/internal/config"
)

func TestRouter_ExactMatch(t *testing.T) {
	routes := []config.Route{
		{
			Pattern: "user@example.com",
			Webhook: config.Webhook{URL: "https://webhook1.example.com", Timeout: 30},
		},
		{
			Pattern: "admin@example.com",
			Webhook: config.Webhook{URL: "https://webhook2.example.com", Timeout: 30},
		},
	}

	r := New(routes, nil)

	// Test exact match
	matches := r.Match("user@example.com")
	if len(matches) != 1 {
		t.Errorf("Expected 1 match, got %d", len(matches))
	}
	if len(matches) > 0 && matches[0].URL != "https://webhook1.example.com" {
		t.Errorf("Expected webhook1, got %s", matches[0].URL)
	}

	// Test no match
	matches = r.Match("other@example.com")
	if len(matches) != 0 {
		t.Errorf("Expected 0 matches, got %d", len(matches))
	}
}

func TestRouter_WildcardMatch(t *testing.T) {
	routes := []config.Route{
		{
			Pattern: "*@example.com",
			Webhook: config.Webhook{URL: "https://catchall.example.com", Timeout: 30},
		},
	}

	r := New(routes, nil)

	// Test wildcard match
	matches := r.Match("anyone@example.com")
	if len(matches) != 1 {
		t.Errorf("Expected 1 match, got %d", len(matches))
	}
	if len(matches) > 0 && matches[0].URL != "https://catchall.example.com" {
		t.Errorf("Expected catchall webhook, got %s", matches[0].URL)
	}

	// Test no match for different domain
	matches = r.Match("user@other.com")
	if len(matches) != 0 {
		t.Errorf("Expected 0 matches, got %d", len(matches))
	}
}

func TestRouter_AccumulativeMatching(t *testing.T) {
	routes := []config.Route{
		{
			Pattern: "support@example.com",
			Webhook: config.Webhook{URL: "https://support.example.com", Timeout: 30},
		},
		{
			Pattern: "*@example.com",
			Webhook: config.Webhook{URL: "https://catchall.example.com", Timeout: 30},
		},
	}

	r := New(routes, nil)

	// Test accumulative matching - should match both exact and wildcard
	matches := r.Match("support@example.com")
	if len(matches) != 2 {
		t.Errorf("Expected 2 matches (exact + wildcard), got %d", len(matches))
	}

	// Verify both webhooks are present
	urls := make(map[string]bool)
	for _, match := range matches {
		urls[match.URL] = true
	}

	if !urls["https://support.example.com"] {
		t.Error("Expected support webhook in matches")
	}
	if !urls["https://catchall.example.com"] {
		t.Error("Expected catchall webhook in matches")
	}
}

func TestRouter_DefaultWebhook(t *testing.T) {
	defaultWebhook := &config.Webhook{
		URL:     "https://default.example.com",
		Timeout: 30,
	}

	routes := []config.Route{
		{
			Pattern: "user@example.com",
			Webhook: config.Webhook{URL: "https://webhook1.example.com", Timeout: 30},
		},
	}

	r := New(routes, defaultWebhook)

	// Test GetDefault returns the configured default
	def := r.GetDefault()
	if def == nil {
		t.Fatal("Expected default webhook, got nil")
	}
	if def.URL != "https://default.example.com" {
		t.Errorf("Expected default webhook URL, got %s", def.URL)
	}

	// Test no match scenario (caller would use default)
	matches := r.Match("nomatch@example.com")
	if len(matches) != 0 {
		t.Errorf("Expected 0 matches, got %d", len(matches))
	}
}

func TestRouter_CaseInsensitive(t *testing.T) {
	routes := []config.Route{
		{
			Pattern: "User@Example.COM",
			Webhook: config.Webhook{URL: "https://webhook1.example.com", Timeout: 30},
		},
	}

	r := New(routes, nil)

	// Test case insensitive matching
	matches := r.Match("user@example.com")
	if len(matches) != 1 {
		t.Errorf("Expected 1 match (case insensitive), got %d", len(matches))
	}

	matches = r.Match("USER@EXAMPLE.COM")
	if len(matches) != 1 {
		t.Errorf("Expected 1 match (case insensitive), got %d", len(matches))
	}
}

func TestRouter_EmptyRoutes(t *testing.T) {
	r := New([]config.Route{}, nil)

	matches := r.Match("user@example.com")
	if len(matches) != 0 {
		t.Errorf("Expected 0 matches with empty routes, got %d", len(matches))
	}

	def := r.GetDefault()
	if def != nil {
		t.Error("Expected nil default webhook")
	}
}

func TestRouter_WhitespaceHandling(t *testing.T) {
	routes := []config.Route{
		{
			Pattern: "  user@example.com  ",
			Webhook: config.Webhook{URL: "https://webhook1.example.com", Timeout: 30},
		},
	}

	r := New(routes, nil)

	// Test whitespace is trimmed
	matches := r.Match("  user@example.com  ")
	if len(matches) != 1 {
		t.Errorf("Expected 1 match (whitespace trimmed), got %d", len(matches))
	}
}
