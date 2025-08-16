package handlers

import (
	"testing"

	"github.com/zurustar/xylitol2/internal/parser"
)



func TestSessionTimerValidator_Priority(t *testing.T) {
	mockSTM := &TestSessionTimerManager{}
	validator := NewSessionTimerValidator(mockSTM, 90, 7200)
	
	priority := validator.Priority()
	if priority != 10 {
		t.Errorf("Expected priority 10, got %d", priority)
	}
}

func TestSessionTimerValidator_Name(t *testing.T) {
	mockSTM := &TestSessionTimerManager{}
	validator := NewSessionTimerValidator(mockSTM, 90, 7200)
	
	name := validator.Name()
	if name != "SessionTimerValidator" {
		t.Errorf("Expected name 'SessionTimerValidator', got '%s'", name)
	}
}

func TestSessionTimerValidator_AppliesTo(t *testing.T) {
	mockSTM := &TestSessionTimerManager{}
	validator := NewSessionTimerValidator(mockSTM, 90, 7200)
	
	// Test INVITE request
	inviteMsg := parser.NewRequestMessage(parser.MethodINVITE, "sip:test@example.com")
	if !validator.AppliesTo(inviteMsg) {
		t.Errorf("Expected validator to apply to INVITE requests")
	}
	
	// Test REGISTER request
	registerMsg := parser.NewRequestMessage(parser.MethodREGISTER, "sip:test@example.com")
	if validator.AppliesTo(registerMsg) {
		t.Errorf("Expected validator not to apply to REGISTER requests")
	}
	
	// Test OPTIONS request
	optionsMsg := parser.NewRequestMessage(parser.MethodOPTIONS, "sip:test@example.com")
	if validator.AppliesTo(optionsMsg) {
		t.Errorf("Expected validator not to apply to OPTIONS requests")
	}
}

func TestSessionTimerValidator_Validate_ExtensionRequired(t *testing.T) {
	// Server requires Session-Timer but client doesn't support it
	mockSTM := &TestSessionTimerManager{requiresTimer: true}
	validator := NewSessionTimerValidator(mockSTM, 90, 7200)
	
	// Create INVITE without Session-Timer support
	msg := parser.NewRequestMessage(parser.MethodINVITE, "sip:test@example.com")
	// No Supported or Require headers with "timer"
	
	result := validator.Validate(msg)
	
	if result.Valid {
		t.Errorf("Expected validation to fail for missing Session-Timer support")
	}
	
	if result.Response == nil {
		t.Errorf("Expected error response to be provided")
	}
	
	if result.Response.GetStatusCode() != parser.StatusExtensionRequired {
		t.Errorf("Expected status code %d, got %d", parser.StatusExtensionRequired, result.Response.GetStatusCode())
	}
	
	// Check that Require and Supported headers are added
	if result.Response.GetHeader(parser.HeaderRequire) != "timer" {
		t.Errorf("Expected Require header 'timer', got '%s'", result.Response.GetHeader(parser.HeaderRequire))
	}
	
	if result.Response.GetHeader(parser.HeaderSupported) != "timer" {
		t.Errorf("Expected Supported header 'timer', got '%s'", result.Response.GetHeader(parser.HeaderSupported))
	}
}

func TestSessionTimerValidator_Validate_MissingSessionExpires(t *testing.T) {
	mockSTM := &TestSessionTimerManager{requiresTimer: false}
	validator := NewSessionTimerValidator(mockSTM, 90, 7200)
	
	// Create INVITE with Session-Timer support but no Session-Expires header
	msg := parser.NewRequestMessage(parser.MethodINVITE, "sip:test@example.com")
	msg.SetHeader(parser.HeaderSupported, "timer")
	// No Session-Expires header
	
	result := validator.Validate(msg)
	
	if result.Valid {
		t.Errorf("Expected validation to fail for missing Session-Expires header")
	}
	
	if result.Response == nil {
		t.Errorf("Expected error response to be provided")
	}
	
	if result.Response.GetStatusCode() != parser.StatusBadRequest {
		t.Errorf("Expected status code %d, got %d", parser.StatusBadRequest, result.Response.GetStatusCode())
	}
}

func TestSessionTimerValidator_Validate_InvalidSessionExpires(t *testing.T) {
	mockSTM := &TestSessionTimerManager{requiresTimer: false}
	validator := NewSessionTimerValidator(mockSTM, 90, 7200)
	
	// Create INVITE with invalid Session-Expires header
	msg := parser.NewRequestMessage(parser.MethodINVITE, "sip:test@example.com")
	msg.SetHeader(parser.HeaderSupported, "timer")
	msg.SetHeader(parser.HeaderSessionExpires, "invalid")
	
	result := validator.Validate(msg)
	
	if result.Valid {
		t.Errorf("Expected validation to fail for invalid Session-Expires header")
	}
	
	if result.Response == nil {
		t.Errorf("Expected error response to be provided")
	}
	
	if result.Response.GetStatusCode() != parser.StatusBadRequest {
		t.Errorf("Expected status code %d, got %d", parser.StatusBadRequest, result.Response.GetStatusCode())
	}
}

func TestSessionTimerValidator_Validate_IntervalTooSmall(t *testing.T) {
	mockSTM := &TestSessionTimerManager{requiresTimer: false}
	validator := NewSessionTimerValidator(mockSTM, 90, 7200)
	
	// Create INVITE with Session-Expires value below minimum
	msg := parser.NewRequestMessage(parser.MethodINVITE, "sip:test@example.com")
	msg.SetHeader(parser.HeaderSupported, "timer")
	msg.SetHeader(parser.HeaderSessionExpires, "60") // Below minimum of 90
	
	result := validator.Validate(msg)
	
	if result.Valid {
		t.Errorf("Expected validation to fail for Session-Expires below minimum")
	}
	
	if result.Response == nil {
		t.Errorf("Expected error response to be provided")
	}
	
	if result.Response.GetStatusCode() != parser.StatusIntervalTooBrief {
		t.Errorf("Expected status code %d, got %d", parser.StatusIntervalTooBrief, result.Response.GetStatusCode())
	}
	
	// Check that Min-SE header is set
	if result.Response.GetHeader(parser.HeaderMinSE) != "90" {
		t.Errorf("Expected Min-SE header '90', got '%s'", result.Response.GetHeader(parser.HeaderMinSE))
	}
}

func TestSessionTimerValidator_Validate_IntervalTooLarge(t *testing.T) {
	mockSTM := &TestSessionTimerManager{requiresTimer: false}
	validator := NewSessionTimerValidator(mockSTM, 90, 7200)
	
	// Create INVITE with Session-Expires value above maximum
	msg := parser.NewRequestMessage(parser.MethodINVITE, "sip:test@example.com")
	msg.SetHeader(parser.HeaderSupported, "timer")
	msg.SetHeader(parser.HeaderSessionExpires, "10000") // Above maximum of 7200
	
	result := validator.Validate(msg)
	
	if result.Valid {
		t.Errorf("Expected validation to fail for Session-Expires above maximum")
	}
	
	if result.Response == nil {
		t.Errorf("Expected error response to be provided")
	}
	
	if result.Response.GetStatusCode() != parser.StatusBadRequest {
		t.Errorf("Expected status code %d, got %d", parser.StatusBadRequest, result.Response.GetStatusCode())
	}
}

func TestSessionTimerValidator_Validate_Success(t *testing.T) {
	mockSTM := &TestSessionTimerManager{requiresTimer: false}
	validator := NewSessionTimerValidator(mockSTM, 90, 7200)
	
	// Create valid INVITE with proper Session-Timer support
	msg := parser.NewRequestMessage(parser.MethodINVITE, "sip:test@example.com")
	msg.SetHeader(parser.HeaderSupported, "timer")
	msg.SetHeader(parser.HeaderSessionExpires, "1800") // Valid value
	
	result := validator.Validate(msg)
	
	if !result.Valid {
		t.Errorf("Expected validation to pass for valid Session-Timer configuration")
	}
	
	if result.Response != nil {
		t.Errorf("Expected no response for successful validation")
	}
	
	if result.Error != nil {
		t.Errorf("Expected no error for successful validation")
	}
}

func TestSessionTimerValidator_Validate_NoSessionTimerNeeded(t *testing.T) {
	mockSTM := &TestSessionTimerManager{requiresTimer: false}
	validator := NewSessionTimerValidator(mockSTM, 90, 7200)
	
	// Create INVITE without Session-Timer support (and server doesn't require it)
	msg := parser.NewRequestMessage(parser.MethodINVITE, "sip:test@example.com")
	// No Session-Timer related headers
	
	result := validator.Validate(msg)
	
	if !result.Valid {
		t.Errorf("Expected validation to pass when Session-Timer is not needed")
	}
	
	if result.Response != nil {
		t.Errorf("Expected no response for successful validation")
	}
	
	if result.Error != nil {
		t.Errorf("Expected no error for successful validation")
	}
}

func TestSessionTimerValidator_ClientSupportsSessionTimer(t *testing.T) {
	mockSTM := &TestSessionTimerManager{}
	validator := NewSessionTimerValidator(mockSTM, 90, 7200)
	
	// Test with Supported header
	if !validator.clientSupportsSessionTimer("timer", "") {
		t.Errorf("Expected client to support Session-Timer with 'timer' in Supported header")
	}
	
	// Test with Require header
	if !validator.clientSupportsSessionTimer("", "timer") {
		t.Errorf("Expected client to support Session-Timer with 'timer' in Require header")
	}
	
	// Test with multiple values in Supported header
	if !validator.clientSupportsSessionTimer("replaces, timer, 100rel", "") {
		t.Errorf("Expected client to support Session-Timer with 'timer' among multiple values")
	}
	
	// Test without Session-Timer support
	if validator.clientSupportsSessionTimer("replaces, 100rel", "") {
		t.Errorf("Expected client not to support Session-Timer without 'timer' value")
	}
	
	// Test with empty headers
	if validator.clientSupportsSessionTimer("", "") {
		t.Errorf("Expected client not to support Session-Timer with empty headers")
	}
}