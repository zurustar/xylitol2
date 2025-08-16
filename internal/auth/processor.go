package auth

import (
	"fmt"

	"github.com/zurustar/xylitol2/internal/database"
	"github.com/zurustar/xylitol2/internal/parser"
	"github.com/zurustar/xylitol2/internal/transaction"
)

// AuthenticatedMessageProcessor handles SIP message processing with authentication
type AuthenticatedMessageProcessor struct {
	authHandler AuthenticationHandler
	userManager database.UserManager
}

// NewAuthenticatedMessageProcessor creates a new authenticated message processor
func NewAuthenticatedMessageProcessor(userManager database.UserManager, realm string) *AuthenticatedMessageProcessor {
	return &AuthenticatedMessageProcessor{
		authHandler: NewAuthenticationMiddleware(userManager, realm),
		userManager: userManager,
	}
}

// ProcessIncomingRequest processes an incoming SIP request with authentication
func (p *AuthenticatedMessageProcessor) ProcessIncomingRequest(request *parser.SIPMessage, transaction transaction.Transaction) (*parser.SIPMessage, *database.User, error) {
	// Process authentication
	authResponse, authenticated, err := p.authHandler.ProcessRequest(request)
	if err != nil {
		return nil, nil, fmt.Errorf("authentication processing failed: %w", err)
	}

	// If authentication failed, return the auth response (401 or 403)
	if !authenticated {
		return authResponse, nil, nil
	}

	// If authentication succeeded and was required, get the authenticated user
	var authenticatedUser *database.User
	if p.authHandler.RequiresAuthentication(request.GetMethod()) {
		authHeader := request.GetHeader(parser.HeaderAuthorization)
		if authHeader != "" {
			user, err := p.authHandler.ValidateAuthorizationHeader(request, authHeader)
			if err != nil {
				// This shouldn't happen if ProcessRequest succeeded, but handle it
				response, createErr := p.createAuthFailureResponse(request)
				if createErr != nil {
					return nil, nil, fmt.Errorf("failed to create auth failure response: %w", createErr)
				}
				return response, nil, nil
			}
			authenticatedUser = user
		}
	}

	// Request is authenticated (or doesn't require auth), return nil response to continue processing
	return nil, authenticatedUser, nil
}

// ProcessREGISTERRequest specifically handles REGISTER requests with authentication
func (p *AuthenticatedMessageProcessor) ProcessREGISTERRequest(request *parser.SIPMessage, transaction transaction.Transaction) (*parser.SIPMessage, *database.User, error) {
	// REGISTER always requires authentication
	authHeader := request.GetHeader(parser.HeaderAuthorization)
	
	// If no authorization header, send challenge
	if authHeader == "" {
		challenge, err := p.createAuthChallenge(request)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to create auth challenge: %w", err)
		}
		return challenge, nil, nil
	}

	// Validate authorization header
	user, err := p.authHandler.ValidateAuthorizationHeader(request, authHeader)
	if err != nil {
		// Authentication failed, send 403 Forbidden
		response, createErr := p.createAuthFailureResponse(request)
		if createErr != nil {
			return nil, nil, fmt.Errorf("failed to create auth failure response: %w", createErr)
		}
		return response, nil, nil
	}

	// Authentication successful
	return nil, user, nil
}

// ProcessINVITERequest specifically handles INVITE requests with authentication
func (p *AuthenticatedMessageProcessor) ProcessINVITERequest(request *parser.SIPMessage, transaction transaction.Transaction) (*parser.SIPMessage, *database.User, error) {
	// INVITE requires authentication
	authHeader := request.GetHeader(parser.HeaderAuthorization)
	
	// If no authorization header, send challenge
	if authHeader == "" {
		challenge, err := p.createAuthChallenge(request)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to create auth challenge: %w", err)
		}
		return challenge, nil, nil
	}

	// Validate authorization header
	user, err := p.authHandler.ValidateAuthorizationHeader(request, authHeader)
	if err != nil {
		// Authentication failed, send 403 Forbidden
		response, createErr := p.createAuthFailureResponse(request)
		if createErr != nil {
			return nil, nil, fmt.Errorf("failed to create auth failure response: %w", createErr)
		}
		return response, nil, nil
	}

	// Authentication successful
	return nil, user, nil
}

// createAuthChallenge creates a 401 Unauthorized response with authentication challenge
func (p *AuthenticatedMessageProcessor) createAuthChallenge(request *parser.SIPMessage) (*parser.SIPMessage, error) {
	messageAuth := p.authHandler.(*AuthenticationMiddleware).messageAuth
	return messageAuth.CreateAuthChallenge(request, p.authHandler.GetRealm())
}

// createAuthFailureResponse creates a 403 Forbidden response for authentication failures
func (p *AuthenticatedMessageProcessor) createAuthFailureResponse(request *parser.SIPMessage) (*parser.SIPMessage, error) {
	messageAuth := p.authHandler.(*AuthenticationMiddleware).messageAuth
	return messageAuth.CreateAuthFailureResponse(request)
}

// GetAuthenticatedUser extracts the authenticated user from a request
func (p *AuthenticatedMessageProcessor) GetAuthenticatedUser(request *parser.SIPMessage) (*database.User, error) {
	if !p.authHandler.RequiresAuthentication(request.GetMethod()) {
		return nil, nil // No authentication required
	}

	authHeader := request.GetHeader(parser.HeaderAuthorization)
	if authHeader == "" {
		return nil, fmt.Errorf("no authorization header present")
	}

	return p.authHandler.ValidateAuthorizationHeader(request, authHeader)
}

// SetRealm updates the realm used for authentication
func (p *AuthenticatedMessageProcessor) SetRealm(realm string) {
	p.authHandler.SetRealm(realm)
}

// GetRealm returns the current realm
func (p *AuthenticatedMessageProcessor) GetRealm() string {
	return p.authHandler.GetRealm()
}

// MessageProcessor defines the interface for processing SIP messages with authentication
type MessageProcessor interface {
	// ProcessIncomingRequest processes an incoming SIP request with authentication
	ProcessIncomingRequest(request *parser.SIPMessage, transaction transaction.Transaction) (*parser.SIPMessage, *database.User, error)
	
	// ProcessREGISTERRequest specifically handles REGISTER requests with authentication
	ProcessREGISTERRequest(request *parser.SIPMessage, transaction transaction.Transaction) (*parser.SIPMessage, *database.User, error)
	
	// ProcessINVITERequest specifically handles INVITE requests with authentication
	ProcessINVITERequest(request *parser.SIPMessage, transaction transaction.Transaction) (*parser.SIPMessage, *database.User, error)
	
	// GetAuthenticatedUser extracts the authenticated user from a request
	GetAuthenticatedUser(request *parser.SIPMessage) (*database.User, error)
	
	// SetRealm updates the realm used for authentication
	SetRealm(realm string)
	
	// GetRealm returns the current realm
	GetRealm() string
}