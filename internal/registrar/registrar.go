package registrar

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/zurustar/xylitol2/internal/auth"
	"github.com/zurustar/xylitol2/internal/database"
	"github.com/zurustar/xylitol2/internal/parser"
)

// SIPRegistrar implements the Registrar interface
type SIPRegistrar struct {
	storage         database.RegistrationDB
	authenticator   auth.MessageAuthenticator
	userManager     database.UserManager
	realm           string
	defaultExpires  int
	maxExpires      int
	minExpires      int
}

// NewSIPRegistrar creates a new SIP registrar instance
func NewSIPRegistrar(storage database.RegistrationDB, authenticator auth.MessageAuthenticator, userManager database.UserManager, realm string) *SIPRegistrar {
	return &SIPRegistrar{
		storage:        storage,
		authenticator:  authenticator,
		userManager:    userManager,
		realm:          realm,
		defaultExpires: 3600, // 1 hour default
		maxExpires:     7200, // 2 hours max
		minExpires:     60,   // 1 minute min
	}
}

// ProcessRegisterRequest processes a REGISTER request and returns a response
func (r *SIPRegistrar) ProcessRegisterRequest(request *parser.SIPMessage) (*parser.SIPMessage, error) {
	if request.GetMethod() != parser.MethodREGISTER {
		return nil, fmt.Errorf("not a REGISTER request")
	}

	// Authenticate the request
	authResult, err := r.authenticator.AuthenticateRequest(request, r.userManager)
	if err != nil {
		return nil, fmt.Errorf("authentication error: %w", err)
	}

	if !authResult.Authenticated {
		if authResult.RequiresAuth {
			// Return 401 Unauthorized with challenge
			return r.authenticator.CreateAuthChallenge(request, r.realm)
		} else {
			// Return 403 Forbidden
			return r.authenticator.CreateAuthFailureResponse(request)
		}
	}

	// Extract AOR from To header
	toHeader := request.GetHeader(parser.HeaderTo)
	if toHeader == "" {
		return r.createErrorResponse(request, parser.StatusBadRequest, "Missing To header")
	}

	aor, err := r.extractAOR(toHeader)
	if err != nil {
		return r.createErrorResponse(request, parser.StatusBadRequest, "Invalid To header")
	}

	// Process Contact headers
	contactHeaders := request.GetHeaders(parser.HeaderContact)
	if len(contactHeaders) == 0 {
		// Query registration - return current contacts
		return r.handleRegistrationQuery(request, aor)
	}

	// Handle registration/deregistration
	return r.handleRegistrationUpdate(request, aor, contactHeaders)
}

// Register registers a contact (implements Registrar interface)
func (r *SIPRegistrar) Register(contact *database.RegistrarContact, expires int) error {
	// Validate expires value
	if expires < 0 {
		return fmt.Errorf("invalid expires value: %d", expires)
	}

	if expires == 0 {
		// Deregister - remove the contact
		return r.storage.Delete(contact.AOR, contact.URI)
	}

	// Apply expires limits
	if expires > r.maxExpires {
		expires = r.maxExpires
	}
	if expires < r.minExpires {
		expires = r.minExpires
	}

	// Set expiration time
	contact.Expires = time.Now().UTC().Add(time.Duration(expires) * time.Second)

	// Store the contact
	return r.storage.Store(contact)
}

// Unregister removes all contacts for an AOR (implements Registrar interface)
func (r *SIPRegistrar) Unregister(aor string) error {
	// Get all contacts for the AOR
	contacts, err := r.storage.Retrieve(aor)
	if err != nil {
		return fmt.Errorf("failed to retrieve contacts for unregistration: %w", err)
	}

	// Delete each contact
	for _, contact := range contacts {
		if err := r.storage.Delete(aor, contact.URI); err != nil {
			// Log error but continue with other contacts
			fmt.Printf("Warning: failed to delete contact %s for AOR %s: %v\n", contact.URI, aor, err)
		}
	}

	return nil
}

// FindContacts retrieves all registered contacts for an AOR (implements Registrar interface)
func (r *SIPRegistrar) FindContacts(aor string) ([]*database.RegistrarContact, error) {
	return r.storage.Retrieve(aor)
}

// CleanupExpired removes expired registrations (implements Registrar interface)
func (r *SIPRegistrar) CleanupExpired() {
	if err := r.storage.CleanupExpired(); err != nil {
		fmt.Printf("Warning: failed to cleanup expired contacts: %v\n", err)
	}
}

// handleRegistrationQuery handles REGISTER requests without Contact headers (queries)
func (r *SIPRegistrar) handleRegistrationQuery(request *parser.SIPMessage, aor string) (*parser.SIPMessage, error) {
	// Get current contacts
	contacts, err := r.FindContacts(aor)
	if err != nil {
		return r.createErrorResponse(request, parser.StatusServerInternalError, "Failed to retrieve contacts")
	}

	// Create 200 OK response
	response := r.createSuccessResponse(request)

	// Add Contact headers for each registered contact
	for _, contact := range contacts {
		expires := int(time.Until(contact.Expires).Seconds())
		if expires < 0 {
			expires = 0
		}
		contactValue := fmt.Sprintf("<%s>;expires=%d", contact.URI, expires)
		response.AddHeader(parser.HeaderContact, contactValue)
	}

	return response, nil
}

// handleRegistrationUpdate handles REGISTER requests with Contact headers (registration/deregistration)
func (r *SIPRegistrar) handleRegistrationUpdate(request *parser.SIPMessage, aor string, contactHeaders []string) (*parser.SIPMessage, error) {
	callID := request.GetHeader(parser.HeaderCallID)
	if callID == "" {
		return r.createErrorResponse(request, parser.StatusBadRequest, "Missing Call-ID header")
	}

	cseqHeader := request.GetHeader(parser.HeaderCSeq)
	if cseqHeader == "" {
		return r.createErrorResponse(request, parser.StatusBadRequest, "Missing CSeq header")
	}

	cseq, err := r.parseCSeq(cseqHeader)
	if err != nil {
		return r.createErrorResponse(request, parser.StatusBadRequest, "Invalid CSeq header")
	}

	// Get default expires from Expires header or use default
	expires := r.defaultExpires
	if expiresHeader := request.GetHeader(parser.HeaderExpires); expiresHeader != "" {
		if parsedExpires, err := strconv.Atoi(expiresHeader); err == nil {
			expires = parsedExpires
		}
	}

	// Process each Contact header
	var processedContacts []string
	for _, contactHeader := range contactHeaders {
		contactURI, contactExpires, err := r.parseContactHeader(contactHeader, expires)
		if err != nil {
			return r.createErrorResponse(request, parser.StatusBadRequest, "Invalid Contact header")
		}

		// Handle wildcard contact for deregistration
		if contactURI == "*" {
			if contactExpires != 0 {
				return r.createErrorResponse(request, parser.StatusBadRequest, "Wildcard contact must have expires=0")
			}
			// Deregister all contacts for this AOR
			if err := r.Unregister(aor); err != nil {
				return r.createErrorResponse(request, parser.StatusServerInternalError, "Failed to deregister contacts")
			}
			// Return success response with no contacts
			return r.createSuccessResponse(request), nil
		}

		// Create contact record
		contact := &database.RegistrarContact{
			AOR:    aor,
			URI:    contactURI,
			CallID: callID,
			CSeq:   cseq,
		}

		// Register or deregister the contact
		if err := r.Register(contact, contactExpires); err != nil {
			return r.createErrorResponse(request, parser.StatusServerInternalError, "Failed to process registration")
		}

		// Add to processed contacts for response
		if contactExpires > 0 {
			// Apply expires limits for response
			if contactExpires > r.maxExpires {
				contactExpires = r.maxExpires
			}
			if contactExpires < r.minExpires {
				contactExpires = r.minExpires
			}
			processedContacts = append(processedContacts, fmt.Sprintf("<%s>;expires=%d", contactURI, contactExpires))
		}
	}

	// Create success response
	response := r.createSuccessResponse(request)

	// Add processed contacts to response
	for _, contact := range processedContacts {
		response.AddHeader(parser.HeaderContact, contact)
	}

	return response, nil
}

// extractAOR extracts the Address of Record from a To header
func (r *SIPRegistrar) extractAOR(toHeader string) (string, error) {
	// Simple extraction - look for URI between < and > or the entire value
	toHeader = strings.TrimSpace(toHeader)
	
	// Remove display name and parameters
	if idx := strings.Index(toHeader, "<"); idx >= 0 {
		end := strings.Index(toHeader[idx:], ">")
		if end < 0 {
			return "", fmt.Errorf("malformed To header: missing closing >")
		}
		return toHeader[idx+1 : idx+end], nil
	}
	
	// No angle brackets, take everything before first semicolon (parameters)
	if idx := strings.Index(toHeader, ";"); idx >= 0 {
		return strings.TrimSpace(toHeader[:idx]), nil
	}
	
	return toHeader, nil
}

// parseContactHeader parses a Contact header and returns URI and expires value
func (r *SIPRegistrar) parseContactHeader(contactHeader string, defaultExpires int) (string, int, error) {
	contactHeader = strings.TrimSpace(contactHeader)
	
	// Handle wildcard
	if contactHeader == "*" {
		return "*", 0, nil
	}
	
	// Extract URI
	var uri string
	if idx := strings.Index(contactHeader, "<"); idx >= 0 {
		end := strings.Index(contactHeader[idx:], ">")
		if end < 0 {
			return "", 0, fmt.Errorf("malformed Contact header: missing closing >")
		}
		uri = contactHeader[idx+1 : idx+end]
		contactHeader = contactHeader[idx+end+1:] // Keep parameters part
	} else {
		// No angle brackets, split at first semicolon
		parts := strings.SplitN(contactHeader, ";", 2)
		uri = strings.TrimSpace(parts[0])
		if len(parts) > 1 {
			contactHeader = ";" + parts[1]
		} else {
			contactHeader = ""
		}
	}
	
	// Parse expires parameter
	expires := defaultExpires
	if strings.Contains(contactHeader, "expires=") {
		// Simple parameter parsing
		params := strings.Split(contactHeader, ";")
		for _, param := range params {
			param = strings.TrimSpace(param)
			if strings.HasPrefix(param, "expires=") {
				if val, err := strconv.Atoi(param[8:]); err == nil {
					expires = val
				}
			}
		}
	}
	
	return uri, expires, nil
}

// parseCSeq parses a CSeq header and returns the sequence number
func (r *SIPRegistrar) parseCSeq(cseqHeader string) (uint32, error) {
	parts := strings.Fields(cseqHeader)
	if len(parts) < 2 {
		return 0, fmt.Errorf("invalid CSeq header format")
	}
	
	cseq, err := strconv.ParseUint(parts[0], 10, 32)
	if err != nil {
		return 0, fmt.Errorf("invalid CSeq number: %w", err)
	}
	
	return uint32(cseq), nil
}

// createSuccessResponse creates a 200 OK response for a REGISTER request
func (r *SIPRegistrar) createSuccessResponse(request *parser.SIPMessage) *parser.SIPMessage {
	response := parser.NewResponseMessage(parser.StatusOK, parser.GetReasonPhraseForCode(parser.StatusOK))
	
	// Copy required headers from request
	r.copyRequiredHeaders(request, response)
	
	// Add server header
	response.SetHeader(parser.HeaderServer, "SIP-Server/1.0")
	
	// Add Date header
	response.SetHeader("Date", time.Now().UTC().Format(time.RFC1123))
	
	return response
}

// createErrorResponse creates an error response for a REGISTER request
func (r *SIPRegistrar) createErrorResponse(request *parser.SIPMessage, statusCode int, reason string) (*parser.SIPMessage, error) {
	reasonPhrase := parser.GetReasonPhraseForCode(statusCode)
	if reason != "" {
		reasonPhrase = reason
	}
	
	response := parser.NewResponseMessage(statusCode, reasonPhrase)
	
	// Copy required headers from request
	r.copyRequiredHeaders(request, response)
	
	// Add server header
	response.SetHeader(parser.HeaderServer, "SIP-Server/1.0")
	
	return response, nil
}

// copyRequiredHeaders copies required headers from request to response
func (r *SIPRegistrar) copyRequiredHeaders(request, response *parser.SIPMessage) {
	// Copy Via headers (in reverse order for responses)
	viaHeaders := request.GetHeaders(parser.HeaderVia)
	for i := len(viaHeaders) - 1; i >= 0; i-- {
		response.AddHeader(parser.HeaderVia, viaHeaders[i])
	}
	
	// Copy From header
	if from := request.GetHeader(parser.HeaderFrom); from != "" {
		response.SetHeader(parser.HeaderFrom, from)
	}
	
	// Copy To header (may add tag parameter in real implementation)
	if to := request.GetHeader(parser.HeaderTo); to != "" {
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
	
	// Set Content-Length to 0 for responses without body
	response.SetHeader(parser.HeaderContentLength, "0")
}

// SetDefaultExpires sets the default expiration time in seconds
func (r *SIPRegistrar) SetDefaultExpires(expires int) {
	r.defaultExpires = expires
}

// SetMaxExpires sets the maximum allowed expiration time in seconds
func (r *SIPRegistrar) SetMaxExpires(expires int) {
	r.maxExpires = expires
}

// SetMinExpires sets the minimum allowed expiration time in seconds
func (r *SIPRegistrar) SetMinExpires(expires int) {
	r.minExpires = expires
}

// GetDefaultExpires returns the default expiration time in seconds
func (r *SIPRegistrar) GetDefaultExpires() int {
	return r.defaultExpires
}

// GetMaxExpires returns the maximum allowed expiration time in seconds
func (r *SIPRegistrar) GetMaxExpires() int {
	return r.maxExpires
}

// GetMinExpires returns the minimum allowed expiration time in seconds
func (r *SIPRegistrar) GetMinExpires() int {
	return r.minExpires
}