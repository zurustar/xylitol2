package handlers

import (
	"testing"

	"github.com/zurustar/xylitol2/internal/database"
	"github.com/zurustar/xylitol2/internal/parser"
)



func TestValidationChain_SessionTimerBeforeAuthentication(t *testing.T) {
	// Create validation chain
	chain := NewValidationChain()
	
	// Create mock dependencies
	mockSTM := &TestSessionTimerManager{requiresTimer: true}
	mockAuthProcessor := &MockAuthProcessor{shouldFail: true}
	mockUserManager := &MockUserManager{}
	
	// Add validators (they should be sorted by priority automatically)
	authValidator := NewAuthenticationValidator(mockAuthProcessor, mockUserManager, "test.local")
	sessionTimerValidator := NewSessionTimerValidator(mockSTM, 90, 7200)
	
	// Add in reverse priority order to test sorting
	chain.AddValidator(authValidator)      // Priority 20
	chain.AddValidator(sessionTimerValidator) // Priority 10
	
	// Verify that Session-Timer validator comes first
	validators := chain.GetValidators()
	if len(validators) != 2 {
		t.Fatalf("Expected 2 validators, got %d", len(validators))
	}
	
	if validators[0].Name() != "SessionTimerValidator" {
		t.Errorf("Expected SessionTimerValidator first, got %s", validators[0].Name())
	}
	
	if validators[1].Name() != "AuthenticationValidator" {
		t.Errorf("Expected AuthenticationValidator second, got %s", validators[1].Name())
	}
	
	// Test that Session-Timer validation fails before authentication is checked
	// Create INVITE without Session-Timer support (server requires it)
	msg := parser.NewRequestMessage(parser.MethodINVITE, "sip:test@example.com")
	msg.SetHeader(parser.HeaderVia, "SIP/2.0/UDP 192.168.1.1:5060")
	msg.SetHeader(parser.HeaderFrom, "sip:caller@example.com;tag=123")
	msg.SetHeader(parser.HeaderTo, "sip:test@example.com")
	msg.SetHeader(parser.HeaderCallID, "call-123")
	msg.SetHeader(parser.HeaderCSeq, "1 INVITE")
	// No Session-Timer support headers
	
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

func TestValidationChain_AuthenticationAfterSessionTimer(t *testing.T) {
	// Create validation chain
	chain := NewValidationChain()
	
	// Create mock dependencies
	mockSTM := &TestSessionTimerManager{requiresTimer: false}
	mockAuthProcessor := &MockAuthProcessor{shouldFail: true} // Auth will fail
	mockUserManager := &MockUserManager{}
	
	// Add validators
	authValidator := NewAuthenticationValidator(mockAuthProcessor, mockUserManager, "test.local")
	sessionTimerValidator := NewSessionTimerValidator(mockSTM, 90, 7200)
	
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
	// No Authorization header
	
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

func TestValidationChain_BothValidationsPass(t *testing.T) {
	// Create validation chain
	chain := NewValidationChain()
	
	// Create mock dependencies
	mockSTM := &TestSessionTimerManager{requiresTimer: false}
	mockUser := &database.User{Username: "testuser", Realm: "test.local", Enabled: true}
	mockAuthProcessor := &MockAuthProcessor{shouldFail: false, user: mockUser}
	mockUserManager := &MockUserManager{user: mockUser}
	
	// Add validators
	authValidator := NewAuthenticationValidator(mockAuthProcessor, mockUserManager, "test.local")
	sessionTimerValidator := NewSessionTimerValidator(mockSTM, 90, 7200)
	
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

func TestValidationChain_NonINVITERequest(t *testing.T) {
	// Create validation chain
	chain := NewValidationChain()
	
	// Create mock dependencies
	mockSTM := &TestSessionTimerManager{requiresTimer: true}
	mockUser := &database.User{Username: "testuser", Realm: "test.local", Enabled: true}
	mockAuthProcessor := &MockAuthProcessor{shouldFail: false, user: mockUser}
	mockUserManager := &MockUserManager{user: mockUser}
	
	// Add validators
	authValidator := NewAuthenticationValidator(mockAuthProcessor, mockUserManager, "test.local")
	sessionTimerValidator := NewSessionTimerValidator(mockSTM, 90, 7200)
	
	chain.AddValidator(sessionTimerValidator) // Priority 10
	chain.AddValidator(authValidator)         // Priority 20
	
	// Create REGISTER request (Session-Timer doesn't apply, but auth does)
	msg := parser.NewRequestMessage(parser.MethodREGISTER, "sip:test@example.com")
	msg.SetHeader(parser.HeaderVia, "SIP/2.0/UDP 192.168.1.1:5060")
	msg.SetHeader(parser.HeaderFrom, "sip:caller@example.com;tag=123")
	msg.SetHeader(parser.HeaderTo, "sip:test@example.com")
	msg.SetHeader(parser.HeaderCallID, "call-123")
	msg.SetHeader(parser.HeaderCSeq, "1 REGISTER")
	msg.SetHeader(parser.HeaderAuthorization, "Digest username=\"testuser\", realm=\"test.local\"")
	
	result := chain.Validate(msg)
	
	// Should pass (Session-Timer doesn't apply, auth passes)
	if !result.Valid {
		t.Errorf("Expected validation to pass for REGISTER request")
	}
	
	if result.Response != nil {
		t.Errorf("Expected no response for successful validation")
	}
	
	if result.Error != nil {
		t.Errorf("Expected no error for successful validation, got: %v", result.Error)
	}
}