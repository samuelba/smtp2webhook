package router

import (
	"fmt"
	"strings"
	"testing"

	"smtp-webhook-forwarder/internal/config"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
)

// **Feature: smtp-webhook-forwarder, Property 7: Routing rule matching**
// For any recipient address and routing configuration, the router should return
// all webhooks whose patterns match the address (exact matches and wildcard domain matches).
// **Validates: Requirements 6.2, 6.4**
func TestProperty_RoutingRuleMatching(t *testing.T) {
	properties := gopter.NewProperties(nil)

	properties.Property("router returns all matching webhooks (exact and wildcard)", prop.ForAll(
		func(recipient string, routes []config.Route) bool {
			// Create router with the generated routes
			r := New(routes, nil)

			// Get matches from router
			matches := r.Match(recipient)

			// Normalize recipient for comparison (same as router does)
			normalizedRecipient := strings.ToLower(strings.TrimSpace(recipient))

			// Manually compute expected matches
			expectedMatches := make(map[string]bool)
			for _, route := range routes {
				normalizedPattern := strings.ToLower(strings.TrimSpace(route.Pattern))

				if matchesPattern(normalizedRecipient, normalizedPattern) {
					expectedMatches[route.Webhook.URL] = true
				}
			}

			// Verify all expected matches are present
			actualMatches := make(map[string]bool)
			for _, match := range matches {
				actualMatches[match.URL] = true
			}

			// Check that we have the right number of matches
			if len(actualMatches) != len(expectedMatches) {
				return false
			}

			// Check that all expected matches are in actual matches
			for url := range expectedMatches {
				if !actualMatches[url] {
					return false
				}
			}

			// Check that all actual matches are in expected matches
			for url := range actualMatches {
				if !expectedMatches[url] {
					return false
				}
			}

			return true
		},
		genEmailAddress(),
		genRoutes(),
	))

	properties.TestingRun(t, gopter.ConsoleReporter(false))
}

// genEmailAddress generates random email addresses
func genEmailAddress() gopter.Gen {
	return gen.OneGenOf(
		// Valid email addresses
		gen.Const("user@example.com"),
		gen.Const("admin@test.org"),
		gen.Const("support@company.net"),
		gen.Const("info@domain.io"),
		gen.Const("contact@site.co"),
		// Email addresses with various formats
		gen.Const("first.last@example.com"),
		gen.Const("user+tag@example.com"),
		gen.Const("123@numbers.com"),
		gen.Const("a@b.c"),
		// Edge cases
		gen.Const("  user@example.com  "), // with whitespace
		gen.Const("USER@EXAMPLE.COM"),     // uppercase
		gen.Const("User@Example.Com"),     // mixed case
		// Generate random combinations
		gen.AlphaString().SuchThat(func(s string) bool {
			return len(s) > 0 && len(s) < 20
		}).Map(func(local string) string {
			domains := []string{"example.com", "test.org", "domain.net", "site.io"}
			domain := domains[len(local)%len(domains)]
			return fmt.Sprintf("%s@%s", local, domain)
		}),
	)
}

// genRoutes generates random routing configurations
func genRoutes() gopter.Gen {
	return gen.SliceOf(genRoute()).SuchThat(func(routes []config.Route) bool {
		// Ensure routes are valid (non-empty patterns and URLs)
		for _, route := range routes {
			if strings.TrimSpace(route.Pattern) == "" {
				return false
			}
			if strings.TrimSpace(route.Webhook.URL) == "" {
				return false
			}
		}
		return true
	})
}

// genRoute generates a single route
func genRoute() gopter.Gen {
	return gen.OneGenOf(
		// Exact match patterns
		gen.Const(config.Route{
			Pattern: "user@example.com",
			Webhook: config.Webhook{URL: "https://webhook1.example.com", Timeout: 30},
		}),
		gen.Const(config.Route{
			Pattern: "admin@test.org",
			Webhook: config.Webhook{URL: "https://webhook2.example.com", Timeout: 30},
		}),
		gen.Const(config.Route{
			Pattern: "support@company.net",
			Webhook: config.Webhook{URL: "https://webhook3.example.com", Timeout: 30},
		}),
		// Wildcard patterns
		gen.Const(config.Route{
			Pattern: "*@example.com",
			Webhook: config.Webhook{URL: "https://catchall1.example.com", Timeout: 30},
		}),
		gen.Const(config.Route{
			Pattern: "*@test.org",
			Webhook: config.Webhook{URL: "https://catchall2.example.com", Timeout: 30},
		}),
		gen.Const(config.Route{
			Pattern: "*@domain.net",
			Webhook: config.Webhook{URL: "https://catchall3.example.com", Timeout: 30},
		}),
		// Mixed case patterns
		gen.Const(config.Route{
			Pattern: "User@Example.COM",
			Webhook: config.Webhook{URL: "https://webhook4.example.com", Timeout: 30},
		}),
		gen.Const(config.Route{
			Pattern: "*@Example.COM",
			Webhook: config.Webhook{URL: "https://catchall4.example.com", Timeout: 30},
		}),
		// Patterns with whitespace
		gen.Const(config.Route{
			Pattern: "  info@site.io  ",
			Webhook: config.Webhook{URL: "https://webhook5.example.com", Timeout: 30},
		}),
	)
}
