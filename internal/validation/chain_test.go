package validation

import (
	"testing"

	"github.com/zurustar/xylitol2/internal/parser"
)

// mockValidator is a mock validator for testing
type mockValidator struct {
	name     string
	priority int
	valid    bool
	applies  bool
	stop     bool
}

func (mv *mockValidator) Validate(req *parser.SIPMessage) ValidationResult {
	return ValidationResult{
		Valid:      mv.valid,
		ErrorCode:  400,
		ErrorReason: "Mock Error",
		Details:    "Mock validation failed",
		ShouldStop: mv.stop,
		Context: map[string]interface{}{
			"validator": mv.name,
		},
	}
}

func (mv *mockValidator) Priority() int {
	return mv.priority
}

func (mv *mockValidator) Name() string {
	return mv.name
}

func (mv *mockValidator) AppliesTo(req *parser.SIPMessage) bool {
	return mv.applies
}

func TestValidationChain_AddValidator(t *testing.T) {
	chain := NewValidationChain()
	
	// Add validators with different priorities
	v1 := &mockValidator{name: "validator1", priority: 20, applies: true, valid: true}
	v2 := &mockValidator{name: "validator2", priority: 10, applies: true, valid: true}
	v3 := &mockValidator{name: "validator3", priority: 30, applies: true, valid: true}
	
	chain.AddValidator(v1)
	chain.AddValidator(v2)
	chain.AddValidator(v3)
	
	validators := chain.GetValidators()
	if len(validators) != 3 {
		t.Errorf("Expected 3 validators, got %d", len(validators))
	}
	
	// Check priority order (v2=10, v1=20, v3=30)
	if validators[0].Name() != "validator2" {
		t.Errorf("Expected validator2 first, got %s", validators[0].Name())
	}
	if validators[1].Name() != "validator1" {
		t.Errorf("Expected validator1 second, got %s", validators[1].Name())
	}
	if validators[2].Name() != "validator3" {
		t.Errorf("Expected validator3 third, got %s", validators[2].Name())
	}
}

func TestValidationChain_RemoveValidator(t *testing.T) {
	chain := NewValidationChain()
	
	v1 := &mockValidator{name: "validator1", priority: 10, applies: true, valid: true}
	v2 := &mockValidator{name: "validator2", priority: 20, applies: true, valid: true}
	
	chain.AddValidator(v1)
	chain.AddValidator(v2)
	
	// Remove existing validator
	if !chain.RemoveValidator("validator1") {
		t.Error("Expected RemoveValidator to return true for existing validator")
	}
	
	validators := chain.GetValidators()
	if len(validators) != 1 {
		t.Errorf("Expected 1 validator after removal, got %d", len(validators))
	}
	
	if validators[0].Name() != "validator2" {
		t.Errorf("Expected validator2 to remain, got %s", validators[0].Name())
	}
	
	// Try to remove non-existing validator
	if chain.RemoveValidator("nonexistent") {
		t.Error("Expected RemoveValidator to return false for non-existing validator")
	}
}

func TestValidationChain_Validate_Success(t *testing.T) {
	chain := NewValidationChain()
	
	v1 := &mockValidator{name: "validator1", priority: 10, applies: true, valid: true}
	v2 := &mockValidator{name: "validator2", priority: 20, applies: true, valid: true}
	
	chain.AddValidator(v1)
	chain.AddValidator(v2)
	
	req := parser.NewRequestMessage("INVITE", "sip:test@example.com")
	result := chain.Validate(req)
	
	if !result.Valid {
		t.Error("Expected validation to pass when all validators pass")
	}
}

func TestValidationChain_Validate_Failure(t *testing.T) {
	chain := NewValidationChain()
	
	v1 := &mockValidator{name: "validator1", priority: 10, applies: true, valid: true}
	v2 := &mockValidator{name: "validator2", priority: 20, applies: true, valid: false}
	
	chain.AddValidator(v1)
	chain.AddValidator(v2)
	
	req := parser.NewRequestMessage("INVITE", "sip:test@example.com")
	result := chain.Validate(req)
	
	if result.Valid {
		t.Error("Expected validation to fail when a validator fails")
	}
	
	if result.ErrorCode != 400 {
		t.Errorf("Expected error code 400, got %d", result.ErrorCode)
	}
}

func TestValidationChain_Validate_ShouldStop(t *testing.T) {
	chain := NewValidationChain()
	
	v1 := &mockValidator{name: "validator1", priority: 10, applies: true, valid: true, stop: true}
	v2 := &mockValidator{name: "validator2", priority: 20, applies: true, valid: true}
	
	chain.AddValidator(v1)
	chain.AddValidator(v2)
	
	req := parser.NewRequestMessage("INVITE", "sip:test@example.com")
	result := chain.Validate(req)
	
	if !result.Valid {
		t.Error("Expected validation to pass")
	}
	
	// The result should come from validator1 (which has ShouldStop=true)
	if result.Context["validator"] != "validator1" {
		t.Error("Expected validation to stop after validator1")
	}
}

func TestValidationChain_Validate_NotApplicable(t *testing.T) {
	chain := NewValidationChain()
	
	v1 := &mockValidator{name: "validator1", priority: 10, applies: false, valid: false}
	v2 := &mockValidator{name: "validator2", priority: 20, applies: true, valid: true}
	
	chain.AddValidator(v1)
	chain.AddValidator(v2)
	
	req := parser.NewRequestMessage("INVITE", "sip:test@example.com")
	result := chain.Validate(req)
	
	if !result.Valid {
		t.Error("Expected validation to pass when non-applicable validator would fail")
	}
}

func TestCreateValidationError(t *testing.T) {
	result := ValidationResult{
		Valid:       false,
		ErrorCode:   421,
		ErrorReason: "Extension Required",
		Details:     "Session-Timer support required",
		Context: map[string]interface{}{
			"validator": "SessionTimerValidator",
		},
	}
	
	err := CreateValidationError(result)
	
	if err.Code != 421 {
		t.Errorf("Expected error code 421, got %d", err.Code)
	}
	
	if err.Reason != "Extension Required" {
		t.Errorf("Expected reason 'Extension Required', got %s", err.Reason)
	}
	
	if err.Details != "Session-Timer support required" {
		t.Errorf("Expected details 'Session-Timer support required', got %s", err.Details)
	}
	
	if err.Error() != "Session-Timer support required" {
		t.Errorf("Expected error message 'Session-Timer support required', got %s", err.Error())
	}
}