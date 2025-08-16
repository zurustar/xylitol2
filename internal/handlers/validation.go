package handlers

import (
	"fmt"
	"sort"

	"github.com/zurustar/xylitol2/internal/parser"
)

// ValidationResult represents the result of a validation operation
type ValidationResult struct {
	Valid    bool
	Response *parser.SIPMessage // Response to send if validation fails
	Error    error              // Error details if validation fails
}

// RequestValidator defines the interface for validating SIP requests
type RequestValidator interface {
	// Validate performs validation on a SIP request
	Validate(req *parser.SIPMessage) ValidationResult
	
	// Priority returns the priority of this validator (lower numbers = higher priority)
	Priority() int
	
	// Name returns the name of this validator for logging purposes
	Name() string
	
	// AppliesTo returns true if this validator should be applied to the given request
	AppliesTo(req *parser.SIPMessage) bool
}

// ValidationChain manages a chain of request validators with priority-based execution
type ValidationChain struct {
	validators []RequestValidator
}

// NewValidationChain creates a new validation chain
func NewValidationChain() *ValidationChain {
	return &ValidationChain{
		validators: make([]RequestValidator, 0),
	}
}

// AddValidator adds a validator to the chain
func (vc *ValidationChain) AddValidator(validator RequestValidator) {
	vc.validators = append(vc.validators, validator)
	
	// Sort validators by priority (lower numbers first)
	sort.Slice(vc.validators, func(i, j int) bool {
		return vc.validators[i].Priority() < vc.validators[j].Priority()
	})
}

// Validate runs all applicable validators in priority order
// Returns the first validation failure, or success if all validators pass
func (vc *ValidationChain) Validate(req *parser.SIPMessage) ValidationResult {
	for _, validator := range vc.validators {
		// Check if this validator applies to the request
		if !validator.AppliesTo(req) {
			continue
		}
		
		// Run the validation
		result := validator.Validate(req)
		if !result.Valid {
			// Validation failed, return the failure result
			return result
		}
	}
	
	// All validations passed
	return ValidationResult{Valid: true}
}

// GetValidators returns all validators in priority order
func (vc *ValidationChain) GetValidators() []RequestValidator {
	return vc.validators
}

// RemoveValidator removes a validator from the chain
func (vc *ValidationChain) RemoveValidator(name string) bool {
	for i, validator := range vc.validators {
		if validator.Name() == name {
			vc.validators = append(vc.validators[:i], vc.validators[i+1:]...)
			return true
		}
	}
	return false
}

// ValidationError represents a validation error with detailed information
type ValidationError struct {
	ValidatorName string
	Code          int
	Reason        string
	Header        string
	Details       string
	Suggestions   []string
}

// Error implements the error interface
func (ve *ValidationError) Error() string {
	return fmt.Sprintf("validation failed in %s: %s", ve.ValidatorName, ve.Reason)
}

// CreateErrorResponse creates a SIP error response from a ValidationError
func CreateErrorResponse(req *parser.SIPMessage, validationError *ValidationError) *parser.SIPMessage {
	response := parser.NewResponseMessage(validationError.Code, validationError.Reason)
	
	// Copy mandatory headers from request
	copyResponseHeaders(req, response)
	
	// Add specific headers based on error type
	switch validationError.Code {
	case parser.StatusExtensionRequired:
		response.AddHeader(parser.HeaderRequire, "timer")
		response.AddHeader(parser.HeaderSupported, "timer")
	case parser.StatusIntervalTooBrief:
		if validationError.Header != "" {
			response.SetHeader(parser.HeaderMinSE, validationError.Header)
		}
	}
	
	return response
}

// copyResponseHeaders copies necessary headers from request to response
func copyResponseHeaders(req *parser.SIPMessage, resp *parser.SIPMessage) {
	// Copy mandatory headers for responses
	if via := req.GetHeader(parser.HeaderVia); via != "" {
		resp.SetHeader(parser.HeaderVia, via)
	}
	if from := req.GetHeader(parser.HeaderFrom); from != "" {
		resp.SetHeader(parser.HeaderFrom, from)
	}
	if to := req.GetHeader(parser.HeaderTo); to != "" {
		resp.SetHeader(parser.HeaderTo, to)
	}
	if callID := req.GetHeader(parser.HeaderCallID); callID != "" {
		resp.SetHeader(parser.HeaderCallID, callID)
	}
	if cseq := req.GetHeader(parser.HeaderCSeq); cseq != "" {
		resp.SetHeader(parser.HeaderCSeq, cseq)
	}
	
	// Set Content-Length to 0 for responses without body
	resp.SetHeader(parser.HeaderContentLength, "0")
}