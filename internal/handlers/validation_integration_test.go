package handlers

import (
	"testing"

	"github.com/zurustar/xylitol2/internal/database"
	"github.com/zurustar/xylitol2/internal/parser"
)

func TestSessionTimerValidationPriorityIntegration(t *testing.T) {
	// Test that Session-Timer validation happens before authentication
	chain := NewValidationChain()
	
	// Create mock dependencies
	sessionTimerMgr := &TestSessionTimerManager{requiresTimer: true}
	authProcessor := &MockAuthProcessor{shouldFail: true}
	userManager := &MockUserManager{}
	
	// Add validators
	sessionTimerValidator := NewSessionTimerValidator(sessionTimerMgr, 90, 7200)
	authValidator := NewAuthenticationValidator(authProcessor, userManager, "test.local")
	
	chain.AddValidator(sessionTimerValidator) // Priority 10
	chain.AddValidator(authValidator)         // Priority 20
	
	// Create INVITE without Session-Timer support (server requires it)
	msg := parser.NewRequestMessage(parser.MethodINVITE, "sip:test@example.com")
	msg.SetHeader(parser.HeaderVia, "SIP/2.0/UDP 192.168.1.1:5060")
	msg.SetHeader(parser.HeaderFrom, "sip:caller@example.com;tag=123")
	msg.SetHeader(parser.HeaderTo, "sip:test@example.com")
	msg.SetHeader(parser.HeaderCallID, "call-123")
	msg.SetHeader(parser.HeaderCSeq, "1 INVITE")
	
	result := chain.Validate(msg)
	
	// Should fail with Session-Timer error, not authentication error
	if result.Valid {
		t.Errorf("Expected validation to fail")
	}
	
	if result.Response == nil {
		t.Fatalf("Expected error response")
	}
	
	// Should be 421 Extension Required (Session-Timer), not 401 Unauthorized (Auth)
	if result.Response.GetStatusCode() != parser.StatusExtensionRequired {
		t.Errorf("Expected status code %d (Extension Required), got %d", 
			parser.StatusExtensionRequired, result.Response.GetStatusCode())
	}
	
	// Verify it's a Session-Timer error, not an authentication error
	if validationError, ok := result.Error.(*ValidationError); ok {
		if validationError.ValidatorName != "SessionTimerValidator" {
			t.Errorf("Expected error from SessionTimerValidator, got %s", validationError.ValidatorName)
		}
	} else {
		t.Errorf("Expected ValidationError type")
	}
}

func TestAuthenticationValidationAfterSessionTimer(t *testing.T) {
	// Test that authentication validation happens after Session-Timer validation passes
	chain := NewValidationChain()
	
	// Create mock dependencies
	sessionTimerMgr := &TestSessionTimerManager{requiresTimer: false}
	authProcessor := &MockAuthProcessor{shouldFail: true} // Auth will fail
	userManager := &MockUserManager{}
	
	// Add validators
	sessionTimerValidator := NewSessionTimerValidator(sessionTimerMgr, 90, 7200)
	authValidator := NewAuthenticationValidator(authProcessor, userManager, "test.local")
	
	chain.AddValidator(sessionTimerValidator) // Priority 10
	chain.AddValidator(authValidator)         // Priority 20
	
	// Create INVITE with valid Session-Timer but no authentication
	msg := parser.NewRequestMessage(parser.MethodINVITE, "sip:test@example.com")
	msg.SetHeader(parser.HeaderVia, "SIP/2.0/UDP 192.168.1.1:5060")
	msg.SetHeader(parser.HeaderFrom, "sip:caller@example.com;tag=123")
	msg.SetHeader(parser.HeaderTo, "sip:test@example.com")
	msg.SetHeader(parser.HeaderCallID, "call-123")
	msg.SetHeader(parser.HeaderCSeq, "1 INVITE")
	msg.SetHeader(parser.HeaderSupported, "timer")
	msg.SetHeader(parser.HeaderSessionExpires, "1800")
	
	result := chain.Validate(msg)
	
	// Should fail with authentication error (Session-Timer validation passed)
	if result.Valid {
		t.Errorf("Expected validation to fail")
	}
	
	if result.Response == nil {
		t.Fatalf("Expected error response")
	}
	
	// Should be 401 Unauthorized (Auth), not 421 Extension Required (Session-Timer)
	if result.Response.GetStatusCode() != parser.StatusUnauthorized {
		t.Errorf("Expected status code %d (Unauthorized), got %d", 
			parser.StatusUnauthorized, result.Response.GetStatusCode())
	}
	
	// Verify it's an authentication error, not a Session-Timer error
	if validationError, ok := result.Error.(*ValidationError); ok {
		if validationError.ValidatorName != "AuthenticationValidator" {
			t.Errorf("Expected error from AuthenticationValidator, got %s", validationError.ValidatorName)
		}
	} else {
		t.Errorf("Expected ValidationError type")
	}
}

func TestCompleteValidationChainSuccess(t *testing.T) {
	// Test that both validations pass when requirements are met
	chain := NewValidationChain()
	
	// Create mock dependencies
	sessionTimerMgr := &TestSessionTimerManager{requiresTimer: false}
	mockUser := &database.User{Username: "testuser", Realm: "test.local", Enabled: true}
	authProcessor := &MockAuthProcessor{shouldFail: false, user: mockUser}
	userManager := &MockUserManager{user: mockUser}
	
	// Add validators
	sessionTimerValidator := NewSessionTimerValidator(sessionTimerMgr, 90, 7200)
	authValidator := NewAuthenticationValidator(authProcessor, userManager, "test.local")
	
	chain.AddValidator(sessionTimerValidator) // Priority 10
	chain.AddValidator(authValidator)         // Priority 20
	
	// Create INVITE with valid Session-Timer and valid authentication
	msg := parser.NewRequestMessage(parser.MethodINVITE, "sip:test@example.com")
	msg.SetHeader(parser.HeaderVia, "SIP/2.0/UDP 192.168.1.1:5060")
	msg.SetHeader(parser.HeaderFrom, "sip:caller@example.com;tag=123")
	msg.SetHeader(parser.HeaderTo, "sip:test@example.com")
	msg.SetHeader(parser.HeaderCallID, "call-123")
	msg.SetHeader(parser.HeaderCSeq, "1 INVITE")
	msg.SetHeader(parser.HeaderSupported, "timer")
	msg.SetHeader(parser.HeaderSessionExpires, "1800")
	msg.SetHeader(parser.HeaderAuthorization, "Digest username=\"testuser\", realm=\"test.local\"")
	
	result := chain.Validate(msg)
	
	// Both validations should pass
	if !result.Valid {
		t.Errorf("Expected validation to pass")
	}
	
	if result.Response != nil {
		t.Errorf("Expected no response for successful validation")
	}
	
	if result.Error != nil {
		t.Errorf("Expected no error for successful validation, got: %v", result.Error)
	}
}