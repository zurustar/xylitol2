package handlers

import (
	"strings"
	"testing"

	"github.com/zurustar/xylitol2/internal/parser"
)

func TestNewEnhancedRequestValidator(t *testing.T) {
	validator := NewEnhancedRequestValidator()
	
	if validator == nil {
		t.Fatal("NewEnhancedRequestValidator should not return nil")
	}
	
	if validator.errorGenerator == nil {
		t.Error("Validator should have error generator")
	}
}

func TestEnhancedRequestValidator_ValidateBasicSIPMessage(t *testing.T) {
	validator := NewEnhancedRequestValidator()
	
	// Test with nil message
	result := validator.ValidateBasicSIPMessage(nil)
	if result.Valid {
		t.Error("Validation should fail for nil message")
	}
	if result.Response.GetStatusCode() != parser.StatusBadRequest {
		t.Error("Should return 400 Bad Request for nil message")
	}
	
	// Test with valid message
	req := parser.NewRequestMessage(parser.MethodINVITE, "sip:test@example.com")
	req.SetHeader(parser.HeaderVia, "SIP/2.0/UDP client.example.com:5060")
	req.SetHeader(parser.HeaderFrom, "sip:alice@example.com;tag=123")
	req.SetHeader(parser.HeaderTo, "sip:bob@example.com")
	req.SetHeader(parser.HeaderCallID, "test-call-id")
	req.SetHeader(parser.HeaderCSeq, "1 INVITE")
	
	result = validator.ValidateBasicSIPMessage(req)
	if !result.Valid {
		t.Errorf("Validation should pass for valid message: %v", result.Error)
	}
	
	// Test with missing headers
	incompleteReq := parser.NewRequestMessage(parser.MethodINVITE, "sip:test@example.com")
	incompleteReq.SetHeader(parser.HeaderVia, "SIP/2.0/UDP client.example.com:5060")
	// Missing From, To, Call-ID, CSeq
	
	result = validator.ValidateBasicSIPMessage(incompleteReq)
	if result.Valid {
		t.Error("Validation should fail for message with missing headers")
	}
	if result.Response.GetStatusCode() != parser.StatusBadRequest {
		t.Error("Should return 400 Bad Request for missing headers")
	}
	
	// Check that response body contains helpful information
	if len(result.Response.Body) == 0 {
		t.Error("Response should have body with error details")
	}
}

func TestEnhancedRequestValidator_ValidateBasicSIPMessage_InvalidHeaders(t *testing.T) {
	validator := NewEnhancedRequestValidator()
	
	req := parser.NewRequestMessage(parser.MethodINVITE, "sip:test@example.com")
	req.SetHeader(parser.HeaderVia, "SIP/2.0/UDP client.example.com:5060")
	req.SetHeader(parser.HeaderFrom, "sip:alice@example.com;tag=123")
	req.SetHeader(parser.HeaderTo, "sip:bob@example.com")
	req.SetHeader(parser.HeaderCallID, "test-call-id")
	req.SetHeader(parser.HeaderCSeq, "invalid cseq format") // Invalid CSeq
	req.SetHeader(parser.HeaderContentLength, "not-a-number") // Invalid Content-Length
	
	result := validator.ValidateBasicSIPMessage(req)
	if result.Valid {
		t.Error("Validation should fail for message with invalid headers")
	}
	if result.Response.GetStatusCode() != parser.StatusBadRequest {
		t.Error("Should return 400 Bad Request for invalid headers")
	}
	
	// Check that response contains details about invalid headers
	bodyText := string(result.Response.Body)
	if !strings.Contains(bodyText, "CSeq") || !strings.Contains(bodyText, "Content-Length") {
		t.Error("Response should contain information about invalid headers")
	}
}

func TestEnhancedRequestValidator_ValidateMethodSupport(t *testing.T) {
	validator := NewEnhancedRequestValidator()
	supportedMethods := []string{parser.MethodINVITE, parser.MethodREGISTER, parser.MethodOPTIONS}
	
	// Test with nil message
	result := validator.ValidateMethodSupport(nil, supportedMethods)
	if result.Valid {
		t.Error("Validation should fail for nil message")
	}
	
	// Test with supported method
	req := parser.NewRequestMessage(parser.MethodINVITE, "sip:test@example.com")
	result = validator.ValidateMethodSupport(req, supportedMethods)
	if !result.Valid {
		t.Error("Validation should pass for supported method")
	}
	
	// Test with unsupported method
	unsupportedReq := parser.NewRequestMessage("UNKNOWN", "sip:test@example.com")
	result = validator.ValidateMethodSupport(unsupportedReq, supportedMethods)
	if result.Valid {
		t.Error("Validation should fail for unsupported method")
	}
	if result.Response.GetStatusCode() != parser.StatusMethodNotAllowed {
		t.Error("Should return 405 Method Not Allowed for unsupported method")
	}
	
	// Check Allow header
	allowHeader := result.Response.GetHeader(parser.HeaderAllow)
	if !strings.Contains(allowHeader, parser.MethodINVITE) {
		t.Error("Response should have Allow header with supported methods")
	}
}

func TestEnhancedRequestValidator_ValidateSessionTimer(t *testing.T) {
	validator := NewEnhancedRequestValidator()
	minSE := 90
	
	// Test with non-INVITE method (should pass)
	registerReq := parser.NewRequestMessage(parser.MethodREGISTER, "sip:test@example.com")
	result := validator.ValidateSessionTimer(registerReq, minSE, true)
	if !result.Valid {
		t.Error("Session timer validation should pass for non-INVITE methods")
	}
	
	// Test INVITE without Session-Expires when required
	inviteReq := parser.NewRequestMessage(parser.MethodINVITE, "sip:test@example.com")
	result = validator.ValidateSessionTimer(inviteReq, minSE, true)
	if result.Valid {
		t.Error("Validation should fail for INVITE without Session-Expires when required")
	}
	if result.Response.GetStatusCode() != parser.StatusExtensionRequired {
		t.Error("Should return 421 Extension Required for missing Session-Timer")
	}
	
	// Test INVITE with valid Session-Expires
	validInvite := parser.NewRequestMessage(parser.MethodINVITE, "sip:test@example.com")
	validInvite.SetHeader(parser.HeaderSessionExpires, "1800")
	result = validator.ValidateSessionTimer(validInvite, minSE, true)
	if !result.Valid {
		t.Errorf("Validation should pass for valid Session-Expires: %v", result.Error)
	}
	
	// Test INVITE with Session-Expires too small
	smallInvite := parser.NewRequestMessage(parser.MethodINVITE, "sip:test@example.com")
	smallInvite.SetHeader(parser.HeaderSessionExpires, "30") // Less than minSE (90)
	result = validator.ValidateSessionTimer(smallInvite, minSE, true)
	if result.Valid {
		t.Error("Validation should fail for Session-Expires smaller than Min-SE")
	}
	if result.Response.GetStatusCode() != parser.StatusIntervalTooBrief {
		t.Error("Should return 422 Session Interval Too Small")
	}
	
	// Check Min-SE header in response
	if result.Response.GetHeader(parser.HeaderMinSE) != "90" {
		t.Error("Response should have Min-SE header with minimum value")
	}
}

func TestEnhancedRequestValidator_ValidateSessionTimer_InvalidHeaders(t *testing.T) {
	validator := NewEnhancedRequestValidator()
	minSE := 90
	
	// Test with invalid Session-Expires
	inviteReq := parser.NewRequestMessage(parser.MethodINVITE, "sip:test@example.com")
	inviteReq.SetHeader(parser.HeaderSessionExpires, "not-a-number")
	result := validator.ValidateSessionTimer(inviteReq, minSE, true)
	if result.Valid {
		t.Error("Validation should fail for invalid Session-Expires")
	}
	if result.Response.GetStatusCode() != parser.StatusBadRequest {
		t.Error("Should return 400 Bad Request for invalid Session-Expires")
	}
	
	// Test with invalid Min-SE
	minSEReq := parser.NewRequestMessage(parser.MethodINVITE, "sip:test@example.com")
	minSEReq.SetHeader(parser.HeaderSessionExpires, "1800")
	minSEReq.SetHeader(parser.HeaderMinSE, "invalid")
	result = validator.ValidateSessionTimer(minSEReq, minSE, true)
	if result.Valid {
		t.Error("Validation should fail for invalid Min-SE")
	}
	if result.Response.GetStatusCode() != parser.StatusBadRequest {
		t.Error("Should return 400 Bad Request for invalid Min-SE")
	}
}

func TestEnhancedRequestValidator_ValidateRegistrationRequest(t *testing.T) {
	validator := NewEnhancedRequestValidator()
	
	// Test with non-REGISTER method (should pass)
	inviteReq := parser.NewRequestMessage(parser.MethodINVITE, "sip:test@example.com")
	result := validator.ValidateRegistrationRequest(inviteReq)
	if !result.Valid {
		t.Error("Registration validation should pass for non-REGISTER methods")
	}
	
	// Test REGISTER without Contact header
	registerReq := parser.NewRequestMessage(parser.MethodREGISTER, "sip:test@example.com")
	result = validator.ValidateRegistrationRequest(registerReq)
	if result.Valid {
		t.Error("Validation should fail for REGISTER without Contact header")
	}
	if result.Response.GetStatusCode() != parser.StatusBadRequest {
		t.Error("Should return 400 Bad Request for missing Contact header")
	}
	
	// Test valid REGISTER request
	validRegister := parser.NewRequestMessage(parser.MethodREGISTER, "sip:test@example.com")
	validRegister.SetHeader(parser.HeaderContact, "<sip:user@client.example.com>")
	result = validator.ValidateRegistrationRequest(validRegister)
	if !result.Valid {
		t.Errorf("Validation should pass for valid REGISTER: %v", result.Error)
	}
	
	// Test REGISTER with invalid Expires
	invalidExpires := parser.NewRequestMessage(parser.MethodREGISTER, "sip:test@example.com")
	invalidExpires.SetHeader(parser.HeaderContact, "<sip:user@client.example.com>")
	invalidExpires.SetHeader(parser.HeaderExpires, "invalid")
	result = validator.ValidateRegistrationRequest(invalidExpires)
	if result.Valid {
		t.Error("Validation should fail for invalid Expires header")
	}
	if result.Response.GetStatusCode() != parser.StatusBadRequest {
		t.Error("Should return 400 Bad Request for invalid Expires")
	}
}

func TestEnhancedRequestValidator_validateCSeqHeader(t *testing.T) {
	validator := NewEnhancedRequestValidator()
	
	tests := []struct {
		cseq     string
		method   string
		shouldFail bool
	}{
		{"1 INVITE", "INVITE", false},
		{"123 REGISTER", "REGISTER", false},
		{"invalid", "INVITE", true},
		{"1", "INVITE", true},
		{"1 INVITE EXTRA", "INVITE", true},
		{"0 INVITE", "INVITE", true}, // Zero sequence number
		{"1 REGISTER", "INVITE", true}, // Method mismatch
	}
	
	for _, test := range tests {
		err := validator.validateCSeqHeader(test.cseq, test.method)
		if test.shouldFail && err == nil {
			t.Errorf("CSeq validation should fail for '%s' with method '%s'", test.cseq, test.method)
		}
		if !test.shouldFail && err != nil {
			t.Errorf("CSeq validation should pass for '%s' with method '%s': %v", test.cseq, test.method, err)
		}
	}
}

func TestEnhancedRequestValidator_validateContentLengthHeader(t *testing.T) {
	validator := NewEnhancedRequestValidator()
	
	tests := []struct {
		contentLength string
		shouldFail    bool
	}{
		{"0", false},
		{"123", false},
		{"1000", false},
		{"-1", true},
		{"invalid", true},
		{"", true},
	}
	
	for _, test := range tests {
		err := validator.validateContentLengthHeader(test.contentLength)
		if test.shouldFail && err == nil {
			t.Errorf("Content-Length validation should fail for '%s'", test.contentLength)
		}
		if !test.shouldFail && err != nil {
			t.Errorf("Content-Length validation should pass for '%s': %v", test.contentLength, err)
		}
	}
}

func TestCreateEnhancedValidationChain(t *testing.T) {
	supportedMethods := []string{parser.MethodINVITE, parser.MethodREGISTER}
	minSE := 90
	requireSessionTimer := true
	
	chain := CreateEnhancedValidationChain(supportedMethods, minSE, requireSessionTimer)
	
	if chain == nil {
		t.Fatal("CreateEnhancedValidationChain should not return nil")
	}
	
	validators := chain.GetValidators()
	if len(validators) != 4 {
		t.Errorf("Expected 4 validators, got %d", len(validators))
	}
	
	// Check validator names and priorities
	expectedValidators := []string{
		"BasicMessageValidator",
		"MethodSupportValidator", 
		"EnhancedSessionTimerValidator",
		"RegistrationValidator",
	}
	
	for i, validator := range validators {
		if validator.Name() != expectedValidators[i] {
			t.Errorf("Expected validator %d to be %s, got %s", i, expectedValidators[i], validator.Name())
		}
	}
	
	// Test the chain with a complete validation scenario
	req := parser.NewRequestMessage(parser.MethodINVITE, "sip:test@example.com")
	req.SetHeader(parser.HeaderVia, "SIP/2.0/UDP client.example.com:5060")
	req.SetHeader(parser.HeaderFrom, "sip:alice@example.com;tag=123")
	req.SetHeader(parser.HeaderTo, "sip:bob@example.com")
	req.SetHeader(parser.HeaderCallID, "test-call-id")
	req.SetHeader(parser.HeaderCSeq, "1 INVITE")
	req.SetHeader(parser.HeaderSessionExpires, "1800")
	
	result := chain.Validate(req)
	if !result.Valid {
		t.Errorf("Validation chain should pass for valid request: %v", result.Error)
	}
}

func TestBasicMessageValidator(t *testing.T) {
	validator := &BasicMessageValidator{
		validator: NewEnhancedRequestValidator(),
	}
	
	if validator.Priority() != 10 {
		t.Errorf("BasicMessageValidator priority should be 10, got %d", validator.Priority())
	}
	
	if validator.Name() != "BasicMessageValidator" {
		t.Errorf("BasicMessageValidator name should be 'BasicMessageValidator', got %s", validator.Name())
	}
	
	if !validator.AppliesTo(nil) {
		t.Error("BasicMessageValidator should apply to all requests")
	}
}

func TestEnhancedSessionTimerValidator(t *testing.T) {
	validator := &EnhancedSessionTimerValidator{
		validator:            NewEnhancedRequestValidator(),
		minSE:               90,
		requireSessionTimer: true,
	}
	
	if validator.Priority() != 30 {
		t.Errorf("EnhancedSessionTimerValidator priority should be 30, got %d", validator.Priority())
	}
	
	if validator.Name() != "EnhancedSessionTimerValidator" {
		t.Errorf("EnhancedSessionTimerValidator name should be 'EnhancedSessionTimerValidator', got %s", validator.Name())
	}
	
	// Should only apply to INVITE requests
	inviteReq := parser.NewRequestMessage(parser.MethodINVITE, "sip:test@example.com")
	if !validator.AppliesTo(inviteReq) {
		t.Error("EnhancedSessionTimerValidator should apply to INVITE requests")
	}
	
	registerReq := parser.NewRequestMessage(parser.MethodREGISTER, "sip:test@example.com")
	if validator.AppliesTo(registerReq) {
		t.Error("EnhancedSessionTimerValidator should not apply to REGISTER requests")
	}
}