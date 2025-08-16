package handlers

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/zurustar/xylitol2/internal/parser"
)

// EnhancedRequestValidator provides detailed error responses for validation failures
type EnhancedRequestValidator struct {
	errorGenerator *DetailedErrorResponseGenerator
}

// NewEnhancedRequestValidator creates a new enhanced request validator
func NewEnhancedRequestValidator() *EnhancedRequestValidator {
	errorHandler := NewDefaultErrorHandler()
	errorGenerator := NewDetailedErrorResponseGenerator(errorHandler)
	
	return &EnhancedRequestValidator{
		errorGenerator: errorGenerator,
	}
}

// ValidateBasicSIPMessage validates basic SIP message structure and required headers
func (erv *EnhancedRequestValidator) ValidateBasicSIPMessage(req *parser.SIPMessage) ValidationResult {
	if req == nil {
		response := erv.errorGenerator.GenerateBadRequestResponse(nil, "Null SIP message", nil, nil)
		return ValidationResult{
			Valid:    false,
			Response: response,
			Error:    fmt.Errorf("null SIP message"),
		}
	}
	
	var missingHeaders []string
	var invalidHeaders = make(map[string]string)
	
	// Check required headers for all requests
	requiredHeaders := []string{
		parser.HeaderVia,
		parser.HeaderFrom,
		parser.HeaderTo,
		parser.HeaderCallID,
		parser.HeaderCSeq,
	}
	
	for _, header := range requiredHeaders {
		if !req.HasHeader(header) {
			missingHeaders = append(missingHeaders, header)
		}
	}
	
	// Validate header formats
	if req.HasHeader(parser.HeaderCSeq) {
		if err := erv.validateCSeqHeader(req.GetHeader(parser.HeaderCSeq), req.GetMethod()); err != nil {
			invalidHeaders[parser.HeaderCSeq] = err.Error()
		}
	}
	
	if req.HasHeader(parser.HeaderContentLength) {
		if err := erv.validateContentLengthHeader(req.GetHeader(parser.HeaderContentLength)); err != nil {
			invalidHeaders[parser.HeaderContentLength] = err.Error()
		}
	}
	
	if req.HasHeader(parser.HeaderMaxForwards) {
		if err := erv.validateMaxForwardsHeader(req.GetHeader(parser.HeaderMaxForwards)); err != nil {
			invalidHeaders[parser.HeaderMaxForwards] = err.Error()
		}
	}
	
	// If there are validation errors, return detailed response
	if len(missingHeaders) > 0 || len(invalidHeaders) > 0 {
		response := erv.errorGenerator.GenerateBadRequestResponse(
			req, 
			"SIP message validation failed", 
			missingHeaders, 
			invalidHeaders,
		)
		return ValidationResult{
			Valid:    false,
			Response: response,
			Error:    fmt.Errorf("SIP message validation failed"),
		}
	}
	
	return ValidationResult{Valid: true}
}

// ValidateMethodSupport validates that the SIP method is supported
func (erv *EnhancedRequestValidator) ValidateMethodSupport(req *parser.SIPMessage, supportedMethods []string) ValidationResult {
	if req == nil {
		response := erv.errorGenerator.GenerateBadRequestResponse(nil, "Null SIP message", nil, nil)
		return ValidationResult{
			Valid:    false,
			Response: response,
			Error:    fmt.Errorf("null SIP message"),
		}
	}
	
	method := req.GetMethod()
	if method == "" {
		response := erv.errorGenerator.GenerateBadRequestResponse(req, "Missing or empty method", nil, nil)
		return ValidationResult{
			Valid:    false,
			Response: response,
			Error:    fmt.Errorf("missing or empty method"),
		}
	}
	
	// Check if method is supported
	for _, supportedMethod := range supportedMethods {
		if method == supportedMethod {
			return ValidationResult{Valid: true}
		}
	}
	
	// Method not supported
	response := erv.errorGenerator.GenerateMethodNotAllowedResponse(req, supportedMethods)
	return ValidationResult{
		Valid:    false,
		Response: response,
		Error:    fmt.Errorf("method %s not supported", method),
	}
}

// ValidateSessionTimer validates Session-Timer related headers
func (erv *EnhancedRequestValidator) ValidateSessionTimer(req *parser.SIPMessage, minSE int, requireSessionTimer bool) ValidationResult {
	if req == nil {
		response := erv.errorGenerator.GenerateBadRequestResponse(nil, "Null SIP message", nil, nil)
		return ValidationResult{
			Valid:    false,
			Response: response,
			Error:    fmt.Errorf("null SIP message"),
		}
	}
	
	// Only validate session timer for INVITE requests
	if req.GetMethod() != parser.MethodINVITE {
		return ValidationResult{Valid: true}
	}
	
	hasSessionExpires := req.HasHeader(parser.HeaderSessionExpires)
	hasSupported := req.HasHeader(parser.HeaderSupported)
	
	// Check if Session-Timer is required but not present
	if requireSessionTimer && !hasSessionExpires {
		// Check if client supports timer extension
		if hasSupported {
			supported := req.GetHeader(parser.HeaderSupported)
			if !strings.Contains(strings.ToLower(supported), "timer") {
				response := erv.errorGenerator.GenerateExtensionRequiredResponse(req, "timer")
				return ValidationResult{
					Valid:    false,
					Response: response,
					Error:    fmt.Errorf("session timer extension required"),
				}
			}
		} else {
			response := erv.errorGenerator.GenerateExtensionRequiredResponse(req, "timer")
			return ValidationResult{
				Valid:    false,
				Response: response,
				Error:    fmt.Errorf("session timer extension required"),
			}
		}
	}
	
	// If Session-Expires is present, validate its value
	if hasSessionExpires {
		sessionExpiresStr := req.GetHeader(parser.HeaderSessionExpires)
		sessionExpires, err := strconv.Atoi(sessionExpiresStr)
		if err != nil {
			invalidHeaders := map[string]string{
				parser.HeaderSessionExpires: "must be a positive integer",
			}
			response := erv.errorGenerator.GenerateInvalidHeaderResponse(req, invalidHeaders, "SessionTimerValidator")
			return ValidationResult{
				Valid:    false,
				Response: response,
				Error:    fmt.Errorf("invalid Session-Expires header: %v", err),
			}
		}
		
		// Check if session expires is too small
		if sessionExpires < minSE {
			response := erv.errorGenerator.GenerateIntervalTooBriefResponse(req, minSE, sessionExpires)
			return ValidationResult{
				Valid:    false,
				Response: response,
				Error:    fmt.Errorf("session expires %d is less than minimum %d", sessionExpires, minSE),
			}
		}
	}
	
	// Validate Min-SE header if present
	if req.HasHeader(parser.HeaderMinSE) {
		minSEStr := req.GetHeader(parser.HeaderMinSE)
		minSEValue, err := strconv.Atoi(minSEStr)
		if err != nil || minSEValue <= 0 {
			invalidHeaders := map[string]string{
				parser.HeaderMinSE: "must be a positive integer",
			}
			response := erv.errorGenerator.GenerateInvalidHeaderResponse(req, invalidHeaders, "SessionTimerValidator")
			return ValidationResult{
				Valid:    false,
				Response: response,
				Error:    fmt.Errorf("invalid Min-SE header"),
			}
		}
	}
	
	return ValidationResult{Valid: true}
}

// ValidateRegistrationRequest validates REGISTER-specific requirements
func (erv *EnhancedRequestValidator) ValidateRegistrationRequest(req *parser.SIPMessage) ValidationResult {
	if req == nil || req.GetMethod() != parser.MethodREGISTER {
		return ValidationResult{Valid: true}
	}
	
	var missingHeaders []string
	var invalidHeaders = make(map[string]string)
	
	// REGISTER requests require Contact header
	if !req.HasHeader(parser.HeaderContact) {
		missingHeaders = append(missingHeaders, parser.HeaderContact)
	}
	
	// Validate Expires header if present
	if req.HasHeader(parser.HeaderExpires) {
		expiresStr := req.GetHeader(parser.HeaderExpires)
		expires, err := strconv.Atoi(expiresStr)
		if err != nil || expires < 0 {
			invalidHeaders[parser.HeaderExpires] = "must be a non-negative integer"
		}
	}
	
	// If there are validation errors, return detailed response
	if len(missingHeaders) > 0 || len(invalidHeaders) > 0 {
		response := erv.errorGenerator.GenerateBadRequestResponse(
			req, 
			"REGISTER request validation failed", 
			missingHeaders, 
			invalidHeaders,
		)
		return ValidationResult{
			Valid:    false,
			Response: response,
			Error:    fmt.Errorf("REGISTER request validation failed"),
		}
	}
	
	return ValidationResult{Valid: true}
}

// validateCSeqHeader validates the CSeq header format
func (erv *EnhancedRequestValidator) validateCSeqHeader(cseq, method string) error {
	parts := strings.Fields(cseq)
	if len(parts) != 2 {
		return fmt.Errorf("must contain sequence number and method")
	}
	
	// Validate sequence number
	seqNum, err := strconv.ParseUint(parts[0], 10, 32)
	if err != nil || seqNum == 0 {
		return fmt.Errorf("sequence number must be a positive integer")
	}
	
	// Validate method matches request method
	if parts[1] != method {
		return fmt.Errorf("method must match request method (%s)", method)
	}
	
	return nil
}

// validateContentLengthHeader validates the Content-Length header
func (erv *EnhancedRequestValidator) validateContentLengthHeader(contentLength string) error {
	length, err := strconv.Atoi(contentLength)
	if err != nil {
		return fmt.Errorf("must be a valid integer")
	}
	
	if length < 0 {
		return fmt.Errorf("must be non-negative")
	}
	
	return nil
}

// validateMaxForwardsHeader validates the Max-Forwards header
func (erv *EnhancedRequestValidator) validateMaxForwardsHeader(maxForwards string) error {
	forwards, err := strconv.Atoi(maxForwards)
	if err != nil {
		return fmt.Errorf("must be a valid integer")
	}
	
	if forwards < 0 {
		return fmt.Errorf("must be non-negative")
	}
	
	return nil
}

// CreateEnhancedValidationChain creates a validation chain with enhanced error responses
func CreateEnhancedValidationChain(supportedMethods []string, minSE int, requireSessionTimer bool) *ValidationChain {
	chain := NewValidationChain()
	validator := NewEnhancedRequestValidator()
	
	// Add basic message validator
	chain.AddValidator(&BasicMessageValidator{
		validator: validator,
	})
	
	// Add method support validator
	chain.AddValidator(&MethodSupportValidator{
		validator:        validator,
		supportedMethods: supportedMethods,
	})
	
	// Add session timer validator
	chain.AddValidator(&EnhancedSessionTimerValidator{
		validator:            validator,
		minSE:               minSE,
		requireSessionTimer: requireSessionTimer,
	})
	
	// Add registration validator
	chain.AddValidator(&RegistrationValidator{
		validator: validator,
	})
	
	return chain
}

// BasicMessageValidator validates basic SIP message structure
type BasicMessageValidator struct {
	validator *EnhancedRequestValidator
}

func (bmv *BasicMessageValidator) Validate(req *parser.SIPMessage) ValidationResult {
	return bmv.validator.ValidateBasicSIPMessage(req)
}

func (bmv *BasicMessageValidator) Priority() int {
	return 10 // High priority - basic validation first
}

func (bmv *BasicMessageValidator) Name() string {
	return "BasicMessageValidator"
}

func (bmv *BasicMessageValidator) AppliesTo(req *parser.SIPMessage) bool {
	return true // Applies to all requests
}

// MethodSupportValidator validates method support
type MethodSupportValidator struct {
	validator        *EnhancedRequestValidator
	supportedMethods []string
}

func (msv *MethodSupportValidator) Validate(req *parser.SIPMessage) ValidationResult {
	return msv.validator.ValidateMethodSupport(req, msv.supportedMethods)
}

func (msv *MethodSupportValidator) Priority() int {
	return 20 // After basic validation
}

func (msv *MethodSupportValidator) Name() string {
	return "MethodSupportValidator"
}

func (msv *MethodSupportValidator) AppliesTo(req *parser.SIPMessage) bool {
	return true // Applies to all requests
}

// EnhancedSessionTimerValidator validates session timer requirements
type EnhancedSessionTimerValidator struct {
	validator            *EnhancedRequestValidator
	minSE               int
	requireSessionTimer bool
}

func (estv *EnhancedSessionTimerValidator) Validate(req *parser.SIPMessage) ValidationResult {
	return estv.validator.ValidateSessionTimer(req, estv.minSE, estv.requireSessionTimer)
}

func (estv *EnhancedSessionTimerValidator) Priority() int {
	return 30 // After method validation, before authentication
}

func (estv *EnhancedSessionTimerValidator) Name() string {
	return "EnhancedSessionTimerValidator"
}

func (estv *EnhancedSessionTimerValidator) AppliesTo(req *parser.SIPMessage) bool {
	return req != nil && req.GetMethod() == parser.MethodINVITE
}

// RegistrationValidator validates REGISTER requests
type RegistrationValidator struct {
	validator *EnhancedRequestValidator
}

func (rv *RegistrationValidator) Validate(req *parser.SIPMessage) ValidationResult {
	return rv.validator.ValidateRegistrationRequest(req)
}

func (rv *RegistrationValidator) Priority() int {
	return 40 // After session timer validation
}

func (rv *RegistrationValidator) Name() string {
	return "RegistrationValidator"
}

func (rv *RegistrationValidator) AppliesTo(req *parser.SIPMessage) bool {
	return req != nil && req.GetMethod() == parser.MethodREGISTER
}