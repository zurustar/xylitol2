package validation

import (
	"fmt"
	"strings"

	"github.com/zurustar/xylitol2/internal/parser"
)

// MessageProcessor processes SIP messages through a validation chain
type MessageProcessor struct {
	chain ValidationChain
}

// NewMessageProcessor creates a new message processor with validation chain
func NewMessageProcessor() *MessageProcessor {
	return &MessageProcessor{
		chain: NewValidationChain(),
	}
}

// AddValidator adds a validator to the processing chain
func (mp *MessageProcessor) AddValidator(validator RequestValidator) {
	mp.chain.AddValidator(validator)
}

// RemoveValidator removes a validator from the processing chain
func (mp *MessageProcessor) RemoveValidator(name string) bool {
	return mp.chain.RemoveValidator(name)
}

// ProcessRequest processes a SIP request through the validation chain
func (mp *MessageProcessor) ProcessRequest(req *parser.SIPMessage) (*parser.SIPMessage, error) {
	// Only process request messages
	if !req.IsRequest() {
		return nil, fmt.Errorf("message is not a request")
	}

	// Run validation chain
	result := mp.chain.Validate(req)
	
	// If validation failed, create error response
	if !result.Valid {
		return mp.createErrorResponse(req, result), nil
	}
	
	// Validation passed, return nil to indicate processing should continue
	return nil, nil
}

// createErrorResponse creates an error response based on validation result
func (mp *MessageProcessor) createErrorResponse(req *parser.SIPMessage, result ValidationResult) *parser.SIPMessage {
	// Create response message
	resp := parser.NewResponseMessage(result.ErrorCode, result.ErrorReason)
	
	// Copy required headers from request
	if via := req.GetHeader("Via"); via != "" {
		resp.SetHeader("Via", via)
	}
	
	if from := req.GetHeader("From"); from != "" {
		resp.SetHeader("From", from)
	}
	
	if to := req.GetHeader("To"); to != "" {
		// Add tag to To header if not present
		if !containsTag(to) {
			to += ";tag=" + generateTag()
		}
		resp.SetHeader("To", to)
	}
	
	if callID := req.GetHeader("Call-ID"); callID != "" {
		resp.SetHeader("Call-ID", callID)
	}
	
	if cseq := req.GetHeader("CSeq"); cseq != "" {
		resp.SetHeader("CSeq", cseq)
	}
	
	// Add specific headers based on error type
	switch result.ErrorCode {
	case 401: // Unauthorized
		if realm, ok := result.Context["realm"].(string); ok {
			wwwAuth := fmt.Sprintf(`Digest realm="%s", nonce="%s", algorithm=MD5`, realm, generateNonce())
			resp.SetHeader("WWW-Authenticate", wwwAuth)
		}
	case 421: // Extension Required
		if required, ok := result.Context["required"].(string); ok {
			resp.SetHeader("Require", required)
		}
	case 422: // Session Interval Too Small
		if minSE, ok := result.Context["min_se"].(int); ok {
			resp.SetHeader("Min-SE", fmt.Sprintf("%d", minSE))
		}
	}
	
	// Set Content-Length
	resp.SetHeader("Content-Length", "0")
	
	// Add error details to body if present
	if result.Details != "" {
		body := result.Details
		resp.Body = []byte(body)
		resp.SetHeader("Content-Type", "text/plain")
		resp.SetHeader("Content-Length", fmt.Sprintf("%d", len(body)))
	}
	
	return resp
}

// GetValidators returns all validators in the chain
func (mp *MessageProcessor) GetValidators() []RequestValidator {
	return mp.chain.GetValidators()
}

// Helper functions

// containsTag checks if a header contains a tag parameter
func containsTag(header string) bool {
	return strings.Contains(header, "tag=")
}

// generateTag generates a random tag for To header
func generateTag() string {
	return fmt.Sprintf("tag-%d", generateRandomNumber())
}

// generateNonce generates a random nonce for authentication
func generateNonce() string {
	return fmt.Sprintf("nonce-%d", generateRandomNumber())
}

// generateRandomNumber generates a random number (simplified implementation)
func generateRandomNumber() int64 {
	// This is a simplified implementation
	// In production, use crypto/rand for better randomness
	return 123456789
}