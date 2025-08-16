package auth

import (
	"fmt"

	"github.com/zurustar/xylitol2/internal/database"
	"github.com/zurustar/xylitol2/internal/parser"
)

// AuthenticationMiddleware handles authentication for SIP messages
type AuthenticationMiddleware struct {
	messageAuth MessageAuthenticator
	userManager database.UserManager
	realm       string
}

// NewAuthenticationMiddleware creates a new authentication middleware
func NewAuthenticationMiddleware(userManager database.UserManager, realm string) *AuthenticationMiddleware {
	return &AuthenticationMiddleware{
		messageAuth: NewSIPMessageAuthenticator(realm),
		userManager: userManager,
		realm:       realm,
	}
}

// ProcessRequest processes a SIP request and handles authentication
// Returns:
// - response: authentication challenge or error response if authentication fails
// - authenticated: true if request is authenticated or doesn't require auth
// - error: any processing error
func (m *AuthenticationMiddleware) ProcessRequest(request *parser.SIPMessage) (*parser.SIPMessage, bool, error) {
	// Authenticate the request
	authResult, err := m.messageAuth.AuthenticateRequest(request, m.userManager)
	if err != nil {
		return nil, false, fmt.Errorf("authentication processing failed: %w", err)
	}

	// If authentication is not required, allow the request
	if !authResult.RequiresAuth {
		return nil, true, nil
	}

	// If authenticated successfully, allow the request
	if authResult.Authenticated {
		return nil, true, nil
	}

	// If authentication failed with an error, return 403 Forbidden
	if authResult.Error != nil {
		response, err := m.messageAuth.CreateAuthFailureResponse(request)
		if err != nil {
			return nil, false, fmt.Errorf("failed to create auth failure response: %w", err)
		}
		return response, false, nil
	}

	// If no authentication provided, return 401 Unauthorized with challenge
	response, err := m.messageAuth.CreateAuthChallenge(request, m.realm)
	if err != nil {
		return nil, false, fmt.Errorf("failed to create auth challenge: %w", err)
	}

	return response, false, nil
}

// ValidateAuthorizationHeader validates an Authorization header for a specific request
func (m *AuthenticationMiddleware) ValidateAuthorizationHeader(request *parser.SIPMessage, authHeader string) (*database.User, error) {
	// Parse authorization header
	digestAuth := m.messageAuth.(*SIPMessageAuthenticator).digestAuth
	creds, err := digestAuth.ParseAuthorizationHeader(authHeader)
	if err != nil {
		return nil, fmt.Errorf("failed to parse authorization header: %w", err)
	}

	// Get user from database
	user, err := m.userManager.GetUser(creds.Username, creds.Realm)
	if err != nil {
		return nil, fmt.Errorf("user not found: %w", err)
	}

	// Check if user is enabled
	if !user.Enabled {
		return nil, fmt.Errorf("user account disabled")
	}

	// Validate credentials
	method := request.GetMethod()
	valid, err := digestAuth.ValidateCredentials(authHeader, method, user)
	if err != nil {
		return nil, fmt.Errorf("credential validation failed: %w", err)
	}

	if !valid {
		return nil, fmt.Errorf("invalid credentials")
	}

	return user, nil
}

// RequiresAuthentication checks if a SIP method requires authentication
func (m *AuthenticationMiddleware) RequiresAuthentication(method string) bool {
	return m.messageAuth.(*SIPMessageAuthenticator).requiresAuthentication(method)
}

// SetRealm updates the realm used for authentication
func (m *AuthenticationMiddleware) SetRealm(realm string) {
	m.realm = realm
	m.messageAuth.SetRealm(realm)
}

// GetRealm returns the current realm
func (m *AuthenticationMiddleware) GetRealm() string {
	return m.realm
}

// AuthenticationHandler defines the interface for handling authentication in message processing
type AuthenticationHandler interface {
	// ProcessRequest processes a SIP request and handles authentication
	ProcessRequest(request *parser.SIPMessage) (*parser.SIPMessage, bool, error)
	
	// ValidateAuthorizationHeader validates an Authorization header for a specific request
	ValidateAuthorizationHeader(request *parser.SIPMessage, authHeader string) (*database.User, error)
	
	// RequiresAuthentication checks if a SIP method requires authentication
	RequiresAuthentication(method string) bool
	
	// SetRealm updates the realm used for authentication
	SetRealm(realm string)
	
	// GetRealm returns the current realm
	GetRealm() string
}