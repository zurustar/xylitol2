package auth

import (
	"fmt"
	"strings"

	"github.com/zurustar/xylitol2/internal/database"
	"github.com/zurustar/xylitol2/internal/parser"
)

// SIPMessageAuthenticator implements MessageAuthenticator for SIP messages
type SIPMessageAuthenticator struct {
	digestAuth DigestAuthenticator
	realm      string
}

// NewSIPMessageAuthenticator creates a new SIP message authenticator
func NewSIPMessageAuthenticator(realm string) *SIPMessageAuthenticator {
	return &SIPMessageAuthenticator{
		digestAuth: NewSIPDigestAuthenticator(),
		realm:      realm,
	}
}

// NewSIPMessageAuthenticatorWithDigest creates a new SIP message authenticator with custom digest authenticator
func NewSIPMessageAuthenticatorWithDigest(realm string, digestAuth DigestAuthenticator) *SIPMessageAuthenticator {
	return &SIPMessageAuthenticator{
		digestAuth: digestAuth,
		realm:      realm,
	}
}

// AuthenticateRequest checks if a SIP request is properly authenticated
func (a *SIPMessageAuthenticator) AuthenticateRequest(msg *parser.SIPMessage, userManager database.UserManager) (*AuthResult, error) {
	// Only authenticate certain methods
	method := msg.GetMethod()
	if !a.requiresAuthentication(method) {
		return &AuthResult{
			Authenticated: true,
			RequiresAuth:  false,
		}, nil
	}

	// Check if Authorization header is present
	authHeader := msg.GetHeader(parser.HeaderAuthorization)
	if authHeader == "" {
		return &AuthResult{
			Authenticated: false,
			RequiresAuth:  true,
		}, nil
	}

	// Parse authorization header
	creds, err := a.digestAuth.ParseAuthorizationHeader(authHeader)
	if err != nil {
		return &AuthResult{
			Authenticated: false,
			Error:         fmt.Errorf("failed to parse authorization header: %w", err),
			RequiresAuth:  true,
		}, nil
	}

	// Get user from database
	user, err := userManager.GetUser(creds.Username, creds.Realm)
	if err != nil {
		return &AuthResult{
			Authenticated: false,
			Error:         fmt.Errorf("user not found: %w", err),
			RequiresAuth:  true,
		}, nil
	}

	// Check if user is enabled
	if !user.Enabled {
		return &AuthResult{
			Authenticated: false,
			Error:         fmt.Errorf("user account disabled"),
			RequiresAuth:  true,
		}, nil
	}

	// Validate credentials
	valid, err := a.digestAuth.ValidateCredentials(authHeader, method, user)
	if err != nil {
		return &AuthResult{
			Authenticated: false,
			Error:         fmt.Errorf("credential validation failed: %w", err),
			RequiresAuth:  true,
		}, nil
	}

	if !valid {
		return &AuthResult{
			Authenticated: false,
			Error:         fmt.Errorf("invalid credentials"),
			RequiresAuth:  true,
		}, nil
	}

	return &AuthResult{
		Authenticated: true,
		User:          user,
		RequiresAuth:  true,
	}, nil
}

// CreateAuthChallenge creates a 401 Unauthorized response with authentication challenge
func (a *SIPMessageAuthenticator) CreateAuthChallenge(request *parser.SIPMessage, realm string) (*parser.SIPMessage, error) {
	// Generate authentication challenge
	challenge, err := a.digestAuth.GenerateChallenge(realm)
	if err != nil {
		return nil, fmt.Errorf("failed to generate challenge: %w", err)
	}

	// Create 401 Unauthorized response
	response := parser.NewResponseMessage(parser.StatusUnauthorized, parser.GetReasonPhraseForCode(parser.StatusUnauthorized))

	// Copy required headers from request
	a.copyRequiredHeaders(request, response)

	// Add WWW-Authenticate header
	response.SetHeader(parser.HeaderWWWAuthenticate, challenge)

	// Add Content-Length header
	response.SetHeader(parser.HeaderContentLength, "0")

	return response, nil
}

// CreateAuthFailureResponse creates a 403 Forbidden response for authentication failures
func (a *SIPMessageAuthenticator) CreateAuthFailureResponse(request *parser.SIPMessage) (*parser.SIPMessage, error) {
	// Create 403 Forbidden response
	response := parser.NewResponseMessage(parser.StatusForbidden, parser.GetReasonPhraseForCode(parser.StatusForbidden))

	// Copy required headers from request
	a.copyRequiredHeaders(request, response)

	// Add Content-Length header
	response.SetHeader(parser.HeaderContentLength, "0")

	return response, nil
}

// requiresAuthentication checks if a SIP method requires authentication
func (a *SIPMessageAuthenticator) requiresAuthentication(method string) bool {
	switch method {
	case parser.MethodREGISTER, parser.MethodINVITE:
		return true
	default:
		return false
	}
}

// copyRequiredHeaders copies required headers from request to response
func (a *SIPMessageAuthenticator) copyRequiredHeaders(request, response *parser.SIPMessage) {
	// Copy Via headers (all of them)
	viaHeaders := request.GetHeaders(parser.HeaderVia)
	for _, via := range viaHeaders {
		response.AddHeader(parser.HeaderVia, via)
	}

	// Copy From header
	if from := request.GetHeader(parser.HeaderFrom); from != "" {
		response.SetHeader(parser.HeaderFrom, from)
	}

	// Copy To header (may need to add tag)
	if to := request.GetHeader(parser.HeaderTo); to != "" {
		// Add tag if not present
		if !strings.Contains(to, "tag=") {
			tag, err := a.generateTag()
			if err == nil {
				to = to + ";tag=" + tag
			}
		}
		response.SetHeader(parser.HeaderTo, to)
	}

	// Copy Call-ID header
	if callID := request.GetHeader(parser.HeaderCallID); callID != "" {
		response.SetHeader(parser.HeaderCallID, callID)
	}

	// Copy CSeq header
	if cseq := request.GetHeader(parser.HeaderCSeq); cseq != "" {
		response.SetHeader(parser.HeaderCSeq, cseq)
	}

	// Add Server header
	response.SetHeader(parser.HeaderServer, "SIP-Server/1.0")
}

// generateTag generates a random tag for To header
func (a *SIPMessageAuthenticator) generateTag() (string, error) {
	// Use the same nonce generation logic for tags
	return a.digestAuth.GenerateNonce()
}

// SetRealm updates the realm used for authentication
func (a *SIPMessageAuthenticator) SetRealm(realm string) {
	a.realm = realm
}

// GetRealm returns the current realm
func (a *SIPMessageAuthenticator) GetRealm() string {
	return a.realm
}