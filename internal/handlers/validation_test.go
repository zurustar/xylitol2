package handlers

import (
	"testing"

	"github.com/zurustar/xylitol2/internal/parser"
)

// MockValidator is a mock implementation of RequestValidator for testing
type MockValidator struct {
	name     string
	priority int
	applies  bool
	valid    bool
	response *parser.SIPMessage
	err      error
}

func (mv *MockValidator) Validate(req *parser.SIPMessage) ValidationResult {
	return ValidationResult{
		Valid:    mv.valid,
		Response: mv.response,
		Error:    mv.err,
	}
}

func (mv *MockValidator) Priority() int {
	return mv.priority
}

func (mv *MockValidator) Name() string {
	return mv.name
}

func (mv *MockValidator) AppliesTo(req *parser.SIPMessage) bool {
	return mv.applies
}

func TestValidationChain_AddValidator(t *testing.T) {
	chain := NewValidationChain()
	
	// Add validators with different priorities
	validator1 := &MockValidator{name: "validator1", priority: 20, applies: true, valid: true}
	validator2 := &MockValidator{name: "validator2", priority: 10, applies: true, valid: true}
	validator3 := &MockValidator{name: "validator3", priority: 30, applies: true, valid: true}
	
	chain.AddValidator(validator1)
	chain.AddValidator(validator2)
	chain.AddValidator(validator3)
	
	validators := chain.GetValidators()
	if len(validators) != 3 {
		t.Errorf("Expected 3 validators, got %d", len(validators))
	}
	
	// Check that validators are sorted by priority
	expectedOrder := []string{"validator2", "validator1", "validator3"}
	for i, validator := range validators {
		if validator.Name() != expectedOrder[i] {
			t.Errorf("Expected validator %s at position %d, got %s", expectedOrder[i], i, validator.Name())
		}
	}
}

func TestValidationChain_Validate_Success(t *testing.T) {
	chain := NewValidationChain()
	
	// Create a mock SIP message
	msg := &parser.SIPMessage{}
	
	// Add validators that all pass
	validator1 := &MockValidator{name: "validator1", priority: 10, applies: true, valid: true}
	validator2 := &MockValidator{name: "validator2", priority: 20, applies: true, valid: true}
	
	chain.AddValidator(validator1)
	chain.AddValidator(validator2)
	
	result := chain.Validate(msg)
	
	if !result.Valid {
		t.Errorf("Expected validation to pass, but it failed")
	}
	
	if result.Response != nil {
		t.Errorf("Expected no response for successful validation")
	}
	
	if result.Error != nil {
		t.Errorf("Expected no error for successful validation")
	}
}

func TestValidationChain_Validate_FirstValidatorFails(t *testing.T) {
	chain := NewValidationChain()
	
	// Create a mock SIP message
	msg := &parser.SIPMessage{}
	
	// Create mock response for failure
	mockResponse := &parser.SIPMessage{}
	mockError := &ValidationError{ValidatorName: "validator1", Code: 400, Reason: "Bad Request"}
	
	// Add validators where first one fails
	validator1 := &MockValidator{
		name:     "validator1",
		priority: 10,
		applies:  true,
		valid:    false,
		response: mockResponse,
		err:      mockError,
	}
	validator2 := &MockValidator{name: "validator2", priority: 20, applies: true, valid: true}
	
	chain.AddValidator(validator1)
	chain.AddValidator(validator2)
	
	result := chain.Validate(msg)
	
	if result.Valid {
		t.Errorf("Expected validation to fail, but it passed")
	}
	
	if result.Response != mockResponse {
		t.Errorf("Expected mock response to be returned")
	}
	
	if result.Error != mockError {
		t.Errorf("Expected mock error to be returned")
	}
}

func TestValidationChain_Validate_ValidatorNotApplicable(t *testing.T) {
	chain := NewValidationChain()
	
	// Create a mock SIP message
	msg := &parser.SIPMessage{}
	
	// Add validator that doesn't apply to this request
	validator1 := &MockValidator{
		name:     "validator1",
		priority: 10,
		applies:  false, // This validator doesn't apply
		valid:    false, // Would fail if applied
	}
	validator2 := &MockValidator{name: "validator2", priority: 20, applies: true, valid: true}
	
	chain.AddValidator(validator1)
	chain.AddValidator(validator2)
	
	result := chain.Validate(msg)
	
	if !result.Valid {
		t.Errorf("Expected validation to pass since first validator doesn't apply")
	}
}

func TestValidationChain_RemoveValidator(t *testing.T) {
	chain := NewValidationChain()
	
	validator1 := &MockValidator{name: "validator1", priority: 10, applies: true, valid: true}
	validator2 := &MockValidator{name: "validator2", priority: 20, applies: true, valid: true}
	
	chain.AddValidator(validator1)
	chain.AddValidator(validator2)
	
	// Remove validator1
	removed := chain.RemoveValidator("validator1")
	if !removed {
		t.Errorf("Expected validator1 to be removed")
	}
	
	validators := chain.GetValidators()
	if len(validators) != 1 {
		t.Errorf("Expected 1 validator after removal, got %d", len(validators))
	}
	
	if validators[0].Name() != "validator2" {
		t.Errorf("Expected validator2 to remain, got %s", validators[0].Name())
	}
	
	// Try to remove non-existent validator
	removed = chain.RemoveValidator("nonexistent")
	if removed {
		t.Errorf("Expected removal of non-existent validator to return false")
	}
}

func TestCreateErrorResponse(t *testing.T) {
	// Create a mock request
	req := parser.NewRequestMessage("INVITE", "sip:test@example.com")
	req.SetHeader(parser.HeaderVia, "SIP/2.0/UDP 192.168.1.1:5060")
	req.SetHeader(parser.HeaderFrom, "sip:caller@example.com;tag=123")
	req.SetHeader(parser.HeaderTo, "sip:test@example.com")
	req.SetHeader(parser.HeaderCallID, "call-123")
	req.SetHeader(parser.HeaderCSeq, "1 INVITE")
	
	// Create validation error
	validationError := &ValidationError{
		ValidatorName: "TestValidator",
		Code:          parser.StatusExtensionRequired,
		Reason:        "Extension Required",
		Details:       "Test error",
	}
	
	response := CreateErrorResponse(req, validationError)
	
	if response.GetStatusCode() != parser.StatusExtensionRequired {
		t.Errorf("Expected status code %d, got %d", parser.StatusExtensionRequired, response.GetStatusCode())
	}
	
	if response.GetReasonPhrase() != "Extension Required" {
		t.Errorf("Expected reason phrase 'Extension Required', got '%s'", response.GetReasonPhrase())
	}
	
	// Check that mandatory headers are copied
	if response.GetHeader(parser.HeaderVia) != req.GetHeader(parser.HeaderVia) {
		t.Errorf("Via header not copied correctly")
	}
	
	if response.GetHeader(parser.HeaderFrom) != req.GetHeader(parser.HeaderFrom) {
		t.Errorf("From header not copied correctly")
	}
	
	if response.GetHeader(parser.HeaderTo) != req.GetHeader(parser.HeaderTo) {
		t.Errorf("To header not copied correctly")
	}
	
	if response.GetHeader(parser.HeaderCallID) != req.GetHeader(parser.HeaderCallID) {
		t.Errorf("Call-ID header not copied correctly")
	}
	
	if response.GetHeader(parser.HeaderCSeq) != req.GetHeader(parser.HeaderCSeq) {
		t.Errorf("CSeq header not copied correctly")
	}
	
	// Check that specific headers are added for Extension Required
	if response.GetHeader(parser.HeaderRequire) != "timer" {
		t.Errorf("Expected Require header 'timer', got '%s'", response.GetHeader(parser.HeaderRequire))
	}
	
	if response.GetHeader(parser.HeaderSupported) != "timer" {
		t.Errorf("Expected Supported header 'timer', got '%s'", response.GetHeader(parser.HeaderSupported))
	}
}