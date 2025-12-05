package auth

// Authenticator validates SMTP authentication credentials
type Authenticator interface {
	Authenticate(username, password string) bool
}

// Credential represents a username/password pair
type Credential struct {
	Username string
	Password string
}

// credentialAuthenticator implements the Authenticator interface
type credentialAuthenticator struct {
	credentials []Credential
}

// NewAuthenticator creates a new Authenticator with the given credentials
func NewAuthenticator(credentials []Credential) Authenticator {
	return &credentialAuthenticator{
		credentials: credentials,
	}
}

// Authenticate validates the provided username and password against configured credentials
// Returns true if the credentials match any configured credential pair (case-sensitive)
func (a *credentialAuthenticator) Authenticate(username, password string) bool {
	for _, cred := range a.credentials {
		// Case-sensitive comparison as per requirements 4.2, 4.3
		if cred.Username == username && cred.Password == password {
			return true
		}
	}
	return false
}
