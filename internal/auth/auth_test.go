package auth

import "testing"

// Test valid credential acceptance
// Requirements: 4.2, 4.3
func TestAuthenticator_ValidCredentials(t *testing.T) {
	credentials := []Credential{
		{Username: "user1", Password: "pass1"},
		{Username: "user2", Password: "pass2"},
		{Username: "admin", Password: "secret123"},
	}

	auth := NewAuthenticator(credentials)

	// Test each valid credential
	for _, cred := range credentials {
		if !auth.Authenticate(cred.Username, cred.Password) {
			t.Errorf("Expected authentication to succeed for username=%q, password=%q", cred.Username, cred.Password)
		}
	}
}

// Test invalid credential rejection
// Requirements: 4.2, 4.3
func TestAuthenticator_InvalidCredentials(t *testing.T) {
	credentials := []Credential{
		{Username: "user1", Password: "pass1"},
		{Username: "user2", Password: "pass2"},
	}

	auth := NewAuthenticator(credentials)

	testCases := []struct {
		name     string
		username string
		password string
	}{
		{"wrong username", "wronguser", "pass1"},
		{"wrong password", "user1", "wrongpass"},
		{"both wrong", "wronguser", "wrongpass"},
		{"username exists but wrong password", "user2", "pass1"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if auth.Authenticate(tc.username, tc.password) {
				t.Errorf("Expected authentication to fail for username=%q, password=%q", tc.username, tc.password)
			}
		})
	}
}

// Test case sensitivity
// Requirements: 4.2, 4.3
func TestAuthenticator_CaseSensitivity(t *testing.T) {
	credentials := []Credential{
		{Username: "User1", Password: "Pass1"},
		{Username: "ADMIN", Password: "SECRET"},
	}

	auth := NewAuthenticator(credentials)

	testCases := []struct {
		name     string
		username string
		password string
		expected bool
	}{
		{"exact match - User1", "User1", "Pass1", true},
		{"exact match - ADMIN", "ADMIN", "SECRET", true},
		{"lowercase username", "user1", "Pass1", false},
		{"lowercase password", "User1", "pass1", false},
		{"uppercase username", "USER1", "Pass1", false},
		{"uppercase password", "User1", "PASS1", false},
		{"all lowercase", "user1", "pass1", false},
		{"all uppercase", "USER1", "PASS1", false},
		{"lowercase admin username", "admin", "SECRET", false},
		{"lowercase admin password", "ADMIN", "secret", false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := auth.Authenticate(tc.username, tc.password)
			if result != tc.expected {
				t.Errorf("Expected authentication result=%v for username=%q, password=%q, got=%v",
					tc.expected, tc.username, tc.password, result)
			}
		})
	}
}

// Test empty credentials
// Requirements: 4.2, 4.3
func TestAuthenticator_EmptyCredentials(t *testing.T) {
	t.Run("empty username and password in config", func(t *testing.T) {
		credentials := []Credential{
			{Username: "", Password: ""},
			{Username: "user1", Password: "pass1"},
		}

		auth := NewAuthenticator(credentials)

		// Empty credentials should match if configured
		if !auth.Authenticate("", "") {
			t.Error("Expected authentication to succeed for empty credentials when configured")
		}

		// Non-empty credentials should not match empty
		if auth.Authenticate("user1", "") {
			t.Error("Expected authentication to fail for username with empty password")
		}

		if auth.Authenticate("", "pass1") {
			t.Error("Expected authentication to fail for empty username with password")
		}
	})

	t.Run("empty username in attempt", func(t *testing.T) {
		credentials := []Credential{
			{Username: "user1", Password: "pass1"},
		}

		auth := NewAuthenticator(credentials)

		if auth.Authenticate("", "pass1") {
			t.Error("Expected authentication to fail for empty username")
		}

		if auth.Authenticate("", "") {
			t.Error("Expected authentication to fail for empty username and password")
		}
	})

	t.Run("empty password in attempt", func(t *testing.T) {
		credentials := []Credential{
			{Username: "user1", Password: "pass1"},
		}

		auth := NewAuthenticator(credentials)

		if auth.Authenticate("user1", "") {
			t.Error("Expected authentication to fail for empty password")
		}
	})

	t.Run("no credentials configured", func(t *testing.T) {
		credentials := []Credential{}

		auth := NewAuthenticator(credentials)

		if auth.Authenticate("user1", "pass1") {
			t.Error("Expected authentication to fail when no credentials configured")
		}

		if auth.Authenticate("", "") {
			t.Error("Expected authentication to fail for empty credentials when none configured")
		}
	})
}
