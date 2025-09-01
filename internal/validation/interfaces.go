package validation

import (
	"github.com/zurustar/xylitol2/internal/parser"
)

// RequestValidator defines the interface for SIP request validators
type RequestValidator interface {
	// Validate performs validation on a SIP request
	Validate(req *parser.SIPMessage) ValidationResult
	
	// Priority returns the priority of this validator (lower number = higher priority)
	Priority() int
	
	// Name returns the name of this validator for logging
	Name() string
	
	// AppliesTo returns true if this validator should be applied to the given request
	AppliesTo(req *parser.SIPMessage) bool
}

// ValidationResult represents the result of a validation operation
type ValidationResult struct {
	// Valid indicates whether the validation passed
	Valid bool
	
	// ErrorCode is the SIP error code to return if validation failed
	ErrorCode int
	
	// ErrorReason is the reason phrase for the error response
	ErrorReason string
	
	// Details provides additional details about the validation failure
	Details string
	
	// ShouldStop indicates whether validation should stop after this validator
	ShouldStop bool
	
	// Context provides additional context information for logging/debugging
	Context map[string]interface{}
}

// ValidationChain manages a chain of validators that are applied in priority order
type ValidationChain interface {
	// AddValidator adds a validator to the chain
	AddValidator(validator RequestValidator)
	
	// RemoveValidator removes a validator from the chain by name
	RemoveValidator(name string) bool
	
	// Validate runs all applicable validators against the request in priority order
	Validate(req *parser.SIPMessage) ValidationResult
	
	// GetValidators returns all validators in priority order
	GetValidators() []RequestValidator
}

// ValidationError represents a validation error with detailed context
type ValidationError struct {
	Code        int
	Reason      string
	Header      string
	Details     string
	Suggestions []string
	Context     map[string]interface{}
}

// Error implements the error interface
func (e ValidationError) Error() string {
	if e.Details != "" {
		return e.Details
	}
	return e.Reason
}