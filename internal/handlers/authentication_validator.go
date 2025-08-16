package handlers

import (
	"fmt"

	"github.com/zurustar/xylitol2/internal/auth"
	"github.com/zurustar/xylitol2/internal/database"
	"github.com/zurustar/xylitol2/internal/parser"
)

// AuthenticationValidator validates authentication requirements for SIP requests
type AuthenticationValidator struct {
	authProcessor auth.MessageProcessor
	userManager   database.UserManager
	realm         string
}

// NewAuthenticationValidator creates a new authentication validator
func NewAuthenticationValidator(authProcessor auth.MessageProcessor, userManager database.UserManager, realm string) *AuthenticationValidator {
	return &AuthenticationValidator{
		authProcessor: authProcessor,
		userManager:   userManager,
		realm:         realm,
	}
}

// Priority returns the priority of this validator (lower numbers = higher priority)
// Authentication validation should have lower priority than Session-Timer validation
func (av *AuthenticationValidator) Priority() int {
	return 20 // Lower priority than Session-Timer validation (which has priority 10)
}

// Name returns the name of this validator
func (av *AuthenticationValidator) Name() string {
	return "AuthenticationValidator"
}

// AppliesTo returns true if this validator should be applied to the given request
func (av *AuthenticationValidator) AppliesTo(req *parser.SIPMessage) bool {
	method := req.GetMethod()
	
	// Authentication applies to methods that require authentication
	return av.requiresAuthentication(method)
}

// Validate performs authentication validation on a SIP request
func (av *AuthenticationValidator) Validate(req *parser.SIPMessage) ValidationResult {
	// Process authentication using the existing auth processor
	authResponse, user, err := av.authProcessor.ProcessIncomingRequest(req, nil)
	if err != nil {
		validationError := &ValidationError{
			ValidatorName: av.Name(),
			Code:          parser.StatusServerInternalError,
			Reason:        "Internal Server Error",
			Details:       fmt.Sprintf("Authentication processing failed: %v", err),
		}
		
		response := CreateErrorResponse(req, validationError)
		return ValidationResult{
			Valid:    false,
			Response: response,
			Error:    validationError,
		}
	}
	
	// If authentication response is provided, authentication failed
	if authResponse != nil {
		// Determine the type of authentication failure
		authHeader := req.GetHeader(parser.HeaderAuthorization)
		if authHeader == "" {
			// No authentication provided, return 401 Unauthorized
			validationError := &ValidationError{
				ValidatorName: av.Name(),
				Code:          parser.StatusUnauthorized,
				Reason:        "Unauthorized",
				Details:       "Authentication required",
				Suggestions:   []string{"Provide Authorization header with valid credentials"},
			}
			
			response := av.createAuthChallengeResponse(req)
			return ValidationResult{
				Valid:    false,
				Response: response,
				Error:    validationError,
			}
		} else {
			// Authentication provided but invalid, return 403 Forbidden
			validationError := &ValidationError{
				ValidatorName: av.Name(),
				Code:          parser.StatusForbidden,
				Reason:        "Forbidden",
				Details:       "Invalid authentication credentials",
				Suggestions:   []string{"Check username and password", "Verify realm and nonce values"},
			}
			
			response := CreateErrorResponse(req, validationError)
			return ValidationResult{
				Valid:    false,
				Response: response,
				Error:    validationError,
			}
		}
	}
	
	// If user is nil but no auth response, authentication is not required
	if user == nil {
		return ValidationResult{Valid: true}
	}
	
	// Authentication successful
	return ValidationResult{Valid: true}
}

// requiresAuthentication checks if a SIP method requires authentication
func (av *AuthenticationValidator) requiresAuthentication(method string) bool {
	switch method {
	case parser.MethodREGISTER:
		return true // REGISTER always requires authentication
	case parser.MethodINVITE:
		return true // INVITE requires authentication for proxy requests
	case parser.MethodBYE:
		return true // BYE requires authentication
	case parser.MethodCANCEL:
		return true // CANCEL requires authentication
	case parser.MethodOPTIONS:
		// OPTIONS may or may not require authentication depending on context
		// For proxy requests, it requires authentication
		// For server capability requests, it doesn't
		return true // Default to requiring authentication for proxy requests
	case parser.MethodINFO:
		return true // INFO requires authentication
	case parser.MethodACK:
		return false // ACK doesn't require authentication
	default:
		return false // Unknown methods don't require authentication by default
	}
}

// createAuthChallengeResponse creates a 401 Unauthorized response with authentication challenge
func (av *AuthenticationValidator) createAuthChallengeResponse(req *parser.SIPMessage) *parser.SIPMessage {
	response := parser.NewResponseMessage(parser.StatusUnauthorized, "Unauthorized")
	
	// Copy mandatory headers from request
	copyResponseHeaders(req, response)
	
	// Add WWW-Authenticate header with digest challenge
	nonce := av.generateNonce()
	wwwAuth := fmt.Sprintf("Digest realm=\"%s\", nonce=\"%s\", algorithm=MD5, qop=\"auth\"", av.realm, nonce)
	response.SetHeader(parser.HeaderWWWAuthenticate, wwwAuth)
	
	return response
}

// generateNonce generates a nonce value for authentication challenges
func (av *AuthenticationValidator) generateNonce() string {
	// Simple nonce generation - in production, use proper random generation with timestamp
	// This should be replaced with the actual nonce generation from the auth processor
	return "abcdef123456789"
}

// GetAuthenticatedUser extracts the authenticated user from a validated request
// This method should be called after successful validation to get the user context
func (av *AuthenticationValidator) GetAuthenticatedUser(req *parser.SIPMessage) (*database.User, error) {
	authHeader := req.GetHeader(parser.HeaderAuthorization)
	if authHeader == "" {
		return nil, fmt.Errorf("no authentication header present")
	}
	
	// Use the auth processor to validate and extract user information
	authResponse, user, err := av.authProcessor.ProcessIncomingRequest(req, nil)
	if err != nil {
		return nil, fmt.Errorf("authentication processing failed: %w", err)
	}
	
	if authResponse != nil {
		return nil, fmt.Errorf("authentication failed")
	}
	
	return user, nil
}