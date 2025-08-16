package handlers

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/zurustar/xylitol2/internal/parser"
	"github.com/zurustar/xylitol2/internal/sessiontimer"
)

// SessionTimerValidator validates Session-Timer requirements for SIP requests
type SessionTimerValidator struct {
	sessionTimerMgr sessiontimer.SessionTimerManager
	minSE           int // Minimum session expires value
	maxSE           int // Maximum session expires value
}

// NewSessionTimerValidator creates a new Session-Timer validator
func NewSessionTimerValidator(sessionTimerMgr sessiontimer.SessionTimerManager, minSE, maxSE int) *SessionTimerValidator {
	return &SessionTimerValidator{
		sessionTimerMgr: sessionTimerMgr,
		minSE:           minSE,
		maxSE:           maxSE,
	}
}

// Priority returns the priority of this validator (lower numbers = higher priority)
// Session-Timer validation should have high priority (before authentication)
func (stv *SessionTimerValidator) Priority() int {
	return 10 // High priority, before authentication (which typically has priority 20)
}

// Name returns the name of this validator
func (stv *SessionTimerValidator) Name() string {
	return "SessionTimerValidator"
}

// AppliesTo returns true if this validator should be applied to the given request
func (stv *SessionTimerValidator) AppliesTo(req *parser.SIPMessage) bool {
	method := req.GetMethod()
	
	// Session-Timer validation applies to INVITE requests
	// and re-INVITE requests (INVITE within a dialog)
	return method == parser.MethodINVITE
}

// Validate performs Session-Timer validation on a SIP request
func (stv *SessionTimerValidator) Validate(req *parser.SIPMessage) ValidationResult {
	// Check if Session-Timer is required by the server
	sessionTimerRequired := stv.sessionTimerMgr.IsSessionTimerRequired(req)
	sessionExpiresHeader := req.GetHeader(parser.HeaderSessionExpires)
	supportedHeader := req.GetHeader(parser.HeaderSupported)
	requireHeader := req.GetHeader(parser.HeaderRequire)
	
	// Check if client supports Session-Timer
	clientSupportsTimer := stv.clientSupportsSessionTimer(supportedHeader, requireHeader)
	
	// If server requires Session-Timer but client doesn't support it
	if sessionTimerRequired && !clientSupportsTimer {
		validationError := &ValidationError{
			ValidatorName: stv.Name(),
			Code:          parser.StatusExtensionRequired,
			Reason:        "Extension Required",
			Details:       "Session-Timer extension is required but not supported by client",
			Suggestions:   []string{"Add 'timer' to Supported header", "Add 'timer' to Require header"},
		}
		
		response := CreateErrorResponse(req, validationError)
		return ValidationResult{
			Valid:    false,
			Response: response,
			Error:    validationError,
		}
	}
	
	// If Session-Timer is supported/required, validate the Session-Expires header
	if clientSupportsTimer || sessionExpiresHeader != "" {
		return stv.validateSessionExpires(req, sessionExpiresHeader)
	}
	
	// If neither server requires nor client supports Session-Timer, validation passes
	return ValidationResult{Valid: true}
}

// validateSessionExpires validates the Session-Expires header value
func (stv *SessionTimerValidator) validateSessionExpires(req *parser.SIPMessage, sessionExpiresHeader string) ValidationResult {
	// If Session-Timer is being used, Session-Expires header is mandatory
	if sessionExpiresHeader == "" {
		validationError := &ValidationError{
			ValidatorName: stv.Name(),
			Code:          parser.StatusBadRequest,
			Reason:        "Bad Request",
			Header:        parser.HeaderSessionExpires,
			Details:       "Session-Expires header is required when using Session-Timer",
			Suggestions:   []string{"Add Session-Expires header with appropriate value"},
		}
		
		response := CreateErrorResponse(req, validationError)
		return ValidationResult{
			Valid:    false,
			Response: response,
			Error:    validationError,
		}
	}
	
	// Parse Session-Expires value
	sessionExpires, err := stv.parseSessionExpires(sessionExpiresHeader)
	if err != nil {
		validationError := &ValidationError{
			ValidatorName: stv.Name(),
			Code:          parser.StatusBadRequest,
			Reason:        "Bad Request",
			Header:        parser.HeaderSessionExpires,
			Details:       fmt.Sprintf("Invalid Session-Expires header format: %v", err),
			Suggestions:   []string{"Use format: Session-Expires: <seconds>[;refresher=<uac|uas>]"},
		}
		
		response := CreateErrorResponse(req, validationError)
		return ValidationResult{
			Valid:    false,
			Response: response,
			Error:    validationError,
		}
	}
	
	// Validate minimum session expires (Min-SE)
	minSE := stv.getMinSE(req)
	if sessionExpires < minSE {
		validationError := &ValidationError{
			ValidatorName: stv.Name(),
			Code:          parser.StatusIntervalTooBrief,
			Reason:        "Session Interval Too Small",
			Header:        strconv.Itoa(minSE),
			Details:       fmt.Sprintf("Session-Expires value %d is below minimum %d", sessionExpires, minSE),
			Suggestions:   []string{fmt.Sprintf("Use Session-Expires value >= %d", minSE)},
		}
		
		response := CreateErrorResponse(req, validationError)
		return ValidationResult{
			Valid:    false,
			Response: response,
			Error:    validationError,
		}
	}
	
	// Validate maximum session expires if configured
	if stv.maxSE > 0 && sessionExpires > stv.maxSE {
		validationError := &ValidationError{
			ValidatorName: stv.Name(),
			Code:          parser.StatusBadRequest,
			Reason:        "Bad Request",
			Header:        parser.HeaderSessionExpires,
			Details:       fmt.Sprintf("Session-Expires value %d exceeds maximum %d", sessionExpires, stv.maxSE),
			Suggestions:   []string{fmt.Sprintf("Use Session-Expires value <= %d", stv.maxSE)},
		}
		
		response := CreateErrorResponse(req, validationError)
		return ValidationResult{
			Valid:    false,
			Response: response,
			Error:    validationError,
		}
	}
	
	// All Session-Timer validations passed
	return ValidationResult{Valid: true}
}

// clientSupportsSessionTimer checks if the client supports Session-Timer extension
func (stv *SessionTimerValidator) clientSupportsSessionTimer(supportedHeader, requireHeader string) bool {
	// Check if "timer" is in Supported header
	if stv.headerContainsValue(supportedHeader, "timer") {
		return true
	}
	
	// Check if "timer" is in Require header
	if stv.headerContainsValue(requireHeader, "timer") {
		return true
	}
	
	return false
}

// headerContainsValue checks if a header contains a specific value
func (stv *SessionTimerValidator) headerContainsValue(header, value string) bool {
	if header == "" {
		return false
	}
	
	// Split by comma and check each value
	values := strings.Split(header, ",")
	for _, v := range values {
		if strings.TrimSpace(v) == value {
			return true
		}
	}
	
	return false
}

// parseSessionExpires parses the Session-Expires header value
func (stv *SessionTimerValidator) parseSessionExpires(header string) (int, error) {
	// Session-Expires header format: "1800" or "1800;refresher=uac"
	parts := strings.Split(header, ";")
	if len(parts) == 0 {
		return 0, fmt.Errorf("empty Session-Expires header")
	}
	
	expires, err := strconv.Atoi(strings.TrimSpace(parts[0]))
	if err != nil {
		return 0, fmt.Errorf("invalid Session-Expires value: %w", err)
	}
	
	if expires <= 0 {
		return 0, fmt.Errorf("Session-Expires must be positive")
	}
	
	return expires, nil
}

// getMinSE returns the minimum session expires value from the request or default
func (stv *SessionTimerValidator) getMinSE(req *parser.SIPMessage) int {
	minSEHeader := req.GetHeader(parser.HeaderMinSE)
	if minSEHeader == "" {
		return stv.minSE // Use configured default
	}
	
	minSE, err := strconv.Atoi(strings.TrimSpace(minSEHeader))
	if err != nil || minSE <= 0 {
		return stv.minSE // Use configured default on parse error
	}
	
	// Use the larger of the configured minimum and the client's Min-SE
	if minSE > stv.minSE {
		return minSE
	}
	
	return stv.minSE
}