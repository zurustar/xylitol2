package validation

import (
	"strings"
	"testing"

	"github.com/zurustar/xylitol2/internal/parser"
)

func TestMessageProcessor_ProcessRequest_Success(t *testing.T) {
	processor := NewMessageProcessor()
	
	// Add a validator that always passes
	validator := &mockValidator{
		name:     "test",
		priority: 10,
		applies:  true,
		valid:    true,
	}
	processor.AddValidator(validator)
	
	req := parser.NewRequestMessage("INVITE", "sip:test@example.com")
	req.SetHeader("Via", "SIP/2.0/UDP 192.168.1.1:5060;branch=z9hG4bK123")
	req.SetHeader("From", "sip:alice@example.com;tag=abc123")
	req.SetHeader("To", "sip:bob@example.com")
	req.SetHeader("Call-ID", "call123@example.com")
	req.SetHeader("CSeq", "1 INVITE")
	
	resp, err := processor.ProcessRequest(req)
	
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	
	if resp != nil {
		t.Error("Expected nil response for successful validation")
	}
}

func TestMessageProcessor_ProcessRequest_ValidationFailure(t *testing.T) {
	processor := NewMessageProcessor()
	
	// Add a validator that always fails
	validator := &mockValidator{
		name:     "test",
		priority: 10,
		applies:  true,
		valid:    false,
	}
	processor.AddValidator(validator)
	
	req := parser.NewRequestMessage("INVITE", "sip:test@example.com")
	req.SetHeader("Via", "SIP/2.0/UDP 192.168.1.1:5060;branch=z9hG4bK123")
	req.SetHeader("From", "sip:alice@example.com;tag=abc123")
	req.SetHeader("To", "sip:bob@example.com")
	req.SetHeader("Call-ID", "call123@example.com")
	req.SetHeader("CSeq", "1 INVITE")
	
	resp, err := processor.ProcessRequest(req)
	
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	
	if resp == nil {
		t.Error("Expected error response for failed validation")
	}
	
	if resp.GetStatusCode() != 400 {
		t.Errorf("Expected status code 400, got %d", resp.GetStatusCode())
	}
}

func TestMessageProcessor_ProcessRequest_NonRequest(t *testing.T) {
	processor := NewMessageProcessor()
	
	resp := parser.NewResponseMessage(200, "OK")
	
	result, err := processor.ProcessRequest(resp)
	
	if err == nil {
		t.Error("Expected error for non-request message")
	}
	
	if result != nil {
		t.Error("Expected nil result for non-request message")
	}
}

func TestMessageProcessor_CreateErrorResponse_Basic(t *testing.T) {
	processor := NewMessageProcessor()
	
	req := parser.NewRequestMessage("INVITE", "sip:test@example.com")
	req.SetHeader("Via", "SIP/2.0/UDP 192.168.1.1:5060;branch=z9hG4bK123")
	req.SetHeader("From", "sip:alice@example.com;tag=abc123")
	req.SetHeader("To", "sip:bob@example.com")
	req.SetHeader("Call-ID", "call123@example.com")
	req.SetHeader("CSeq", "1 INVITE")
	
	result := ValidationResult{
		Valid:       false,
		ErrorCode:   400,
		ErrorReason: "Bad Request",
		Details:     "Test error",
	}
	
	resp := processor.createErrorResponse(req, result)
	
	if resp.GetStatusCode() != 400 {
		t.Errorf("Expected status code 400, got %d", resp.GetStatusCode())
	}
	
	if resp.GetReasonPhrase() != "Bad Request" {
		t.Errorf("Expected reason 'Bad Request', got '%s'", resp.GetReasonPhrase())
	}
	
	// Check that required headers are copied
	if resp.GetHeader("Via") != req.GetHeader("Via") {
		t.Error("Via header not copied correctly")
	}
	
	if resp.GetHeader("From") != req.GetHeader("From") {
		t.Error("From header not copied correctly")
	}
	
	if resp.GetHeader("Call-ID") != req.GetHeader("Call-ID") {
		t.Error("Call-ID header not copied correctly")
	}
	
	if resp.GetHeader("CSeq") != req.GetHeader("CSeq") {
		t.Error("CSeq header not copied correctly")
	}
}

func TestMessageProcessor_CreateErrorResponse_AddTag(t *testing.T) {
	processor := NewMessageProcessor()
	
	req := parser.NewRequestMessage("INVITE", "sip:test@example.com")
	req.SetHeader("To", "sip:bob@example.com") // No tag
	
	result := ValidationResult{
		Valid:       false,
		ErrorCode:   400,
		ErrorReason: "Bad Request",
	}
	
	resp := processor.createErrorResponse(req, result)
	
	toHeader := resp.GetHeader("To")
	if !containsTag(toHeader) {
		t.Error("Expected To header to contain tag parameter")
	}
}

func TestMessageProcessor_CreateErrorResponse_Unauthorized(t *testing.T) {
	processor := NewMessageProcessor()
	
	req := parser.NewRequestMessage("REGISTER", "sip:test@example.com")
	
	result := ValidationResult{
		Valid:       false,
		ErrorCode:   401,
		ErrorReason: "Unauthorized",
		Context: map[string]interface{}{
			"realm": "example.com",
		},
	}
	
	resp := processor.createErrorResponse(req, result)
	
	wwwAuth := resp.GetHeader("WWW-Authenticate")
	if wwwAuth == "" {
		t.Error("Expected WWW-Authenticate header for 401 response")
	}
	
	if !strings.Contains(wwwAuth, "realm=\"example.com\"") {
		t.Error("Expected realm in WWW-Authenticate header")
	}
}

func TestMessageProcessor_CreateErrorResponse_ExtensionRequired(t *testing.T) {
	processor := NewMessageProcessor()
	
	req := parser.NewRequestMessage("INVITE", "sip:test@example.com")
	
	result := ValidationResult{
		Valid:       false,
		ErrorCode:   421,
		ErrorReason: "Extension Required",
		Context: map[string]interface{}{
			"required": "timer",
		},
	}
	
	resp := processor.createErrorResponse(req, result)
	
	require := resp.GetHeader("Require")
	if require != "timer" {
		t.Errorf("Expected Require header 'timer', got '%s'", require)
	}
}

func TestMessageProcessor_CreateErrorResponse_SessionIntervalTooSmall(t *testing.T) {
	processor := NewMessageProcessor()
	
	req := parser.NewRequestMessage("INVITE", "sip:test@example.com")
	
	result := ValidationResult{
		Valid:       false,
		ErrorCode:   422,
		ErrorReason: "Session Interval Too Small",
		Context: map[string]interface{}{
			"min_se": 90,
		},
	}
	
	resp := processor.createErrorResponse(req, result)
	
	minSE := resp.GetHeader("Min-SE")
	if minSE != "90" {
		t.Errorf("Expected Min-SE header '90', got '%s'", minSE)
	}
}

func TestMessageProcessor_AddRemoveValidator(t *testing.T) {
	processor := NewMessageProcessor()
	
	validator := &mockValidator{
		name:     "test",
		priority: 10,
		applies:  true,
		valid:    true,
	}
	
	// Add validator
	processor.AddValidator(validator)
	validators := processor.GetValidators()
	if len(validators) != 1 {
		t.Errorf("Expected 1 validator, got %d", len(validators))
	}
	
	// Remove validator
	if !processor.RemoveValidator("test") {
		t.Error("Expected RemoveValidator to return true")
	}
	
	validators = processor.GetValidators()
	if len(validators) != 0 {
		t.Errorf("Expected 0 validators after removal, got %d", len(validators))
	}
}

func TestContainsTag(t *testing.T) {
	// Test with tag
	if !containsTag("sip:alice@example.com;tag=abc123") {
		t.Error("Expected containsTag to return true for header with tag")
	}
	
	// Test without tag
	if containsTag("sip:alice@example.com") {
		t.Error("Expected containsTag to return false for header without tag")
	}
}