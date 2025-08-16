package auth

import (
	"github.com/zurustar/xylitol2/internal/database"
	"github.com/zurustar/xylitol2/internal/parser"
)

// DigestAuthenticator defines the interface for SIP digest authentication
type DigestAuthenticator interface {
	// GenerateChallenge creates a WWW-Authenticate header value for digest authentication
	GenerateChallenge(realm string) (string, error)
	
	// ValidateCredentials validates Authorization header credentials against user database
	ValidateCredentials(authHeader string, method string, user *database.User) (bool, error)
	
	// ParseAuthorizationHeader parses an Authorization header and returns digest parameters
	ParseAuthorizationHeader(authHeader string) (*DigestCredentials, error)
	
	// GenerateNonce creates a new nonce value for authentication challenges
	GenerateNonce() (string, error)
	
	// ValidateNonce checks if a nonce is valid and not expired
	ValidateNonce(nonce string) bool
}

// MessageAuthenticator defines the interface for authenticating SIP messages
type MessageAuthenticator interface {
	// AuthenticateRequest checks if a SIP request is properly authenticated
	AuthenticateRequest(msg *parser.SIPMessage, userManager database.UserManager) (*AuthResult, error)
	
	// CreateAuthChallenge creates a 401 Unauthorized response with authentication challenge
	CreateAuthChallenge(request *parser.SIPMessage, realm string) (*parser.SIPMessage, error)
	
	// CreateAuthFailureResponse creates a 403 Forbidden response for authentication failures
	CreateAuthFailureResponse(request *parser.SIPMessage) (*parser.SIPMessage, error)
	
	// SetRealm updates the realm used for authentication
	SetRealm(realm string)
	
	// GetRealm returns the current realm
	GetRealm() string
}

// DigestCredentials represents parsed digest authentication credentials
type DigestCredentials struct {
	Username  string
	Realm     string
	Nonce     string
	URI       string
	Response  string
	Algorithm string
	Opaque    string
	QOP       string
	NC        string
	CNonce    string
}

// AuthResult represents the result of authentication
type AuthResult struct {
	Authenticated bool
	User          *database.User
	Error         error
	RequiresAuth  bool
}

// NonceStore defines the interface for storing and validating nonces
type NonceStore interface {
	// StoreNonce stores a nonce with expiration time
	StoreNonce(nonce string) error
	
	// ValidateNonce checks if a nonce exists and is not expired
	ValidateNonce(nonce string) bool
	
	// CleanupExpired removes expired nonces
	CleanupExpired()
}