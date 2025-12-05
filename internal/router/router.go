package router

import (
	"strings"

	"smtp-webhook-forwarder/internal/config"
)

// Router matches recipient addresses to webhook endpoints
type Router interface {
	Match(recipient string) []config.Webhook
	GetDefault() *config.Webhook
}

// router implements the Router interface
type router struct {
	routes         []config.Route
	defaultWebhook *config.Webhook
}

// New creates a new Router with the given routes and default webhook
func New(routes []config.Route, defaultWebhook *config.Webhook) Router {
	return &router{
		routes:         routes,
		defaultWebhook: defaultWebhook,
	}
}

// Match returns all webhooks whose patterns match the recipient address
// Implements accumulative matching: returns all matching webhooks (exact and wildcard)
func (r *router) Match(recipient string) []config.Webhook {
	var matches []config.Webhook
	recipient = strings.ToLower(strings.TrimSpace(recipient))

	for _, route := range r.routes {
		pattern := strings.ToLower(strings.TrimSpace(route.Pattern))

		if matchesPattern(recipient, pattern) {
			matches = append(matches, route.Webhook)
		}
	}

	return matches
}

// GetDefault returns the default webhook if configured
func (r *router) GetDefault() *config.Webhook {
	return r.defaultWebhook
}

// matchesPattern checks if a recipient matches a pattern
// Supports exact matching and wildcard domain matching (*@domain.com)
func matchesPattern(recipient, pattern string) bool {
	// Exact match
	if recipient == pattern {
		return true
	}

	// Wildcard domain match: *@domain.com
	if strings.HasPrefix(pattern, "*@") {
		domain := pattern[2:] // Remove "*@" prefix
		if strings.Contains(recipient, "@") {
			recipientDomain := recipient[strings.Index(recipient, "@")+1:]
			return recipientDomain == domain
		}
	}

	return false
}
