package auth

import (
	"reflect"
	"testing"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
)

// **Feature: smtp-webhook-forwarder, Property 6: Authentication validation**
// For any credential pair, the authentication function should return true if and only if
// the credentials match a configured credential pair.
// Validates: Requirements 4.2, 4.3
func TestProperty_AuthenticationValidation(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	properties.Property("authentication returns true for valid credentials and false for invalid", prop.ForAll(
		func(config *authTestConfig) bool {
			// Create authenticator with configured credentials
			authenticator := NewAuthenticator(config.configuredCredentials)

			// Test valid credentials - should return true
			for _, validCred := range config.configuredCredentials {
				if !authenticator.Authenticate(validCred.Username, validCred.Password) {
					t.Logf("Valid credential rejected: username=%q", validCred.Username)
					return false
				}
			}

			// Test invalid credentials - should return false
			for _, invalidCred := range config.invalidCredentials {
				if authenticator.Authenticate(invalidCred.Username, invalidCred.Password) {
					t.Logf("Invalid credential accepted: username=%q, password=%q", invalidCred.Username, invalidCred.Password)
					return false
				}
			}

			return true
		},
		genAuthTestConfig(),
	))

	properties.TestingRun(t)
}

// authTestConfig represents a test configuration with valid and invalid credentials
type authTestConfig struct {
	configuredCredentials []Credential
	invalidCredentials    []Credential
}

// genAuthTestConfig generates random authentication test configurations
func genAuthTestConfig() gopter.Gen {
	return gopter.CombineGens(
		genCredentialList(1, 5),  // 1-5 configured credentials
		genCredentialList(1, 10), // 1-10 test credentials
	).Map(func(values []interface{}) *authTestConfig {
		configuredCreds := values[0].([]Credential)
		testCreds := values[1].([]Credential)

		// Separate test credentials into invalid ones (those not in configured list)
		invalidCreds := make([]Credential, 0)
		for _, testCred := range testCreds {
			isValid := false
			for _, configCred := range configuredCreds {
				// Case-sensitive comparison as per requirements
				if testCred.Username == configCred.Username && testCred.Password == configCred.Password {
					isValid = true
					break
				}
			}
			if !isValid {
				invalidCreds = append(invalidCreds, testCred)
			}
		}

		return &authTestConfig{
			configuredCredentials: configuredCreds,
			invalidCredentials:    invalidCreds,
		}
	})
}

// genCredentialList generates a list of random credentials
func genCredentialList(minSize, maxSize int) gopter.Gen {
	return gen.IntRange(minSize, maxSize).FlatMap(func(n interface{}) gopter.Gen {
		size := n.(int)
		gens := make([]gopter.Gen, size)
		for i := 0; i < size; i++ {
			gens[i] = genCredential()
		}
		return gopter.CombineGens(gens...).Map(func(values []interface{}) []Credential {
			creds := make([]Credential, len(values))
			for i, v := range values {
				creds[i] = v.(Credential)
			}
			// Deduplicate credentials to ensure uniqueness
			seen := make(map[string]bool)
			unique := make([]Credential, 0)
			for _, cred := range creds {
				key := cred.Username + ":" + cred.Password
				if !seen[key] {
					seen[key] = true
					unique = append(unique, cred)
				}
			}
			return unique
		})
	}, reflect.TypeOf([]Credential{}))
}

// genCredential generates a random credential
func genCredential() gopter.Gen {
	return gopter.CombineGens(
		genUsername(),
		genPassword(),
	).Map(func(values []interface{}) Credential {
		return Credential{
			Username: values[0].(string),
			Password: values[1].(string),
		}
	})
}

// genUsername generates a random username
func genUsername() gopter.Gen {
	return gen.Identifier().Map(func(s string) string {
		if len(s) > 20 {
			return s[:20]
		}
		if len(s) == 0 {
			return "user"
		}
		return s
	})
}

// genPassword generates a random password
func genPassword() gopter.Gen {
	return gen.Identifier().Map(func(s string) string {
		if len(s) > 30 {
			return s[:30]
		}
		if len(s) == 0 {
			return "pass"
		}
		return s
	})
}
