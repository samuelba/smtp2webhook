package session

import (
	"github.com/google/uuid"
)

// Generator generates unique session identifiers.
type Generator interface {
	Generate() string
}

// UUIDGenerator generates session IDs using UUID v4.
type UUIDGenerator struct{}

// NewGenerator creates a new UUID-based session ID generator.
func NewGenerator() Generator {
	return &UUIDGenerator{}
}

// Generate creates a new unique session identifier.
// Uses UUID v4 which provides cryptographic randomness and
// ensures uniqueness across concurrent sessions.
func (g *UUIDGenerator) Generate() string {
	return uuid.New().String()
}
