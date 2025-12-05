package session

import (
	"sync"
	"testing"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
)

// **Feature: smtp-webhook-forwarder, Property 8: Session ID uniqueness**
// For any set of concurrent SMTP sessions, all generated session identifiers should be unique.
// Validates: Requirements 12.1
func TestProperty_SessionIDUniqueness(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	properties.Property("all session IDs are unique across concurrent generation", prop.ForAll(
		func(numSessions int) bool {
			generator := NewGenerator()

			// Generate session IDs concurrently
			sessionIDs := make([]string, numSessions)
			var wg sync.WaitGroup
			wg.Add(numSessions)

			for i := 0; i < numSessions; i++ {
				go func(index int) {
					defer wg.Done()
					sessionIDs[index] = generator.Generate()
				}(i)
			}

			wg.Wait()

			// Check for uniqueness
			seen := make(map[string]bool)
			for _, id := range sessionIDs {
				if seen[id] {
					t.Logf("Duplicate session ID found: %s", id)
					return false
				}
				seen[id] = true
			}

			return true
		},
		gen.IntRange(1, 1000), // Test with 1 to 1000 concurrent sessions
	))

	properties.TestingRun(t)
}
