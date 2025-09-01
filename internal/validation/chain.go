package validation

import (
	"sort"
	"sync"

	"github.com/zurustar/xylitol2/internal/parser"
)

// validationChain implements the ValidationChain interface
type validationChain struct {
	validators []RequestValidator
	mu         sync.RWMutex
}

// NewValidationChain creates a new validation chain
func NewValidationChain() ValidationChain {
	return &validationChain{
		validators: make([]RequestValidator, 0),
	}
}

// AddValidator adds a validator to the chain and sorts by priority
func (vc *validationChain) AddValidator(validator RequestValidator) {
	vc.mu.Lock()
	defer vc.mu.Unlock()
	
	vc.validators = append(vc.validators, validator)
	
	// Sort validators by priority (lower number = higher priority)
	sort.Slice(vc.validators, func(i, j int) bool {
		return vc.validators[i].Priority() < vc.validators[j].Priority()
	})
}

// RemoveValidator removes a validator from the chain by name
func (vc *validationChain) RemoveValidator(name string) bool {
	vc.mu.Lock()
	defer vc.mu.Unlock()
	
	for i, validator := range vc.validators {
		if validator.Name() == name {
			// Remove validator from slice
			vc.validators = append(vc.validators[:i], vc.validators[i+1:]...)
			return true
		}
	}
	return false
}

// Validate runs all applicable validators against the request in priority order
func (vc *validationChain) Validate(req *parser.SIPMessage) ValidationResult {
	vc.mu.RLock()
	defer vc.mu.RUnlock()
	
	for _, validator := range vc.validators {
		// Check if this validator applies to the request
		if !validator.AppliesTo(req) {
			continue
		}
		
		// Run the validation
		result := validator.Validate(req)
		
		// If validation failed or we should stop, return the result
		if !result.Valid || result.ShouldStop {
			return result
		}
	}
	
	// All validations passed
	return ValidationResult{
		Valid: true,
	}
}

// GetValidators returns all validators in priority order
func (vc *validationChain) GetValidators() []RequestValidator {
	vc.mu.RLock()
	defer vc.mu.RUnlock()
	
	// Return a copy to prevent external modification
	validators := make([]RequestValidator, len(vc.validators))
	copy(validators, vc.validators)
	return validators
}

// CreateValidationError creates a ValidationError from a ValidationResult
func CreateValidationError(result ValidationResult) ValidationError {
	return ValidationError{
		Code:    result.ErrorCode,
		Reason:  result.ErrorReason,
		Details: result.Details,
		Context: result.Context,
	}
}