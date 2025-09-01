package validation

import (
	"testing"

	"github.com/zurustar/xylitol2/internal/parser"
)

func TestSyntaxValidator_AppliesTo(t *testing.T) {
	validator := NewSyntaxValidator()
	
	// Test with request message
	req := parser.NewRequestMessage("INVITE", "sip:test@example.com")
	if !validator.AppliesTo(req) {
		t.Error("Expected SyntaxValidator to apply to request messages")
	}
	
	// Test with response message
	resp := parser.NewResponseMessage(200, "OK")
	if validator.AppliesTo(resp) {
		t.Error("Expected SyntaxValidator not to apply to response messages")
	}
}

func TestSyntaxValidator_ValidRequest(t *testing.T) {
	validator := NewSyntaxValidator()
	
	req := parser.NewRequestMessage("INVITE", "sip:test@example.com")
	req.SetHeader("Via", "SIP/2.0/UDP 192.168.1.1:5060;branch=z9hG4bK123")
	req.SetHeader("From", "sip:alice@example.com;tag=abc123")
	req.SetHeader("To", "sip:bob@example.com")
	req.SetHeader("Call-ID", "call123@example.com")
	req.SetHeader("CSeq", "1 INVITE")
	req.SetHeader("Content-Length", "0")
	
	result := validator.Validate(req)
	
	if !result.Valid {
		t.Errorf("Expected valid request to pass validation, got error: %s", result.Details)
	}
}

func TestSyntaxValidator_MissingMethod(t *testing.T) {
	validator := NewSyntaxValidator()
	
	// Create message with empty method
	req := parser.NewSIPMessage()
	req.StartLine = &parser.RequestLine{
		Method:     "",
		RequestURI: "sip:test@example.com",
		Version:    "SIP/2.0",
	}
	
	result := validator.Validate(req)
	
	if result.Valid {
		t.Error("Expected validation to fail for missing method")
	}
	
	if result.ErrorCode != 400 {
		t.Errorf("Expected error code 400, got %d", result.ErrorCode)
	}
	
	if result.Details != "Missing or empty method" {
		t.Errorf("Expected 'Missing or empty method', got '%s'", result.Details)
	}
}

func TestSyntaxValidator_InvalidMethodCharacters(t *testing.T) {
	validator := NewSyntaxValidator()
	
	req := parser.NewSIPMessage()
	req.StartLine = &parser.RequestLine{
		Method:     "INVITE TEST",
		RequestURI: "sip:test@example.com",
		Version:    "SIP/2.0",
	}
	
	result := validator.Validate(req)
	
	if result.Valid {
		t.Error("Expected validation to fail for method with invalid characters")
	}
	
	if result.ErrorCode != 400 {
		t.Errorf("Expected error code 400, got %d", result.ErrorCode)
	}
}

func TestSyntaxValidator_MissingRequestURI(t *testing.T) {
	validator := NewSyntaxValidator()
	
	req := parser.NewSIPMessage()
	req.StartLine = &parser.RequestLine{
		Method:     "INVITE",
		RequestURI: "",
		Version:    "SIP/2.0",
	}
	
	result := validator.Validate(req)
	
	if result.Valid {
		t.Error("Expected validation to fail for missing Request-URI")
	}
	
	if result.ErrorCode != 400 {
		t.Errorf("Expected error code 400, got %d", result.ErrorCode)
	}
}

func TestSyntaxValidator_MissingRequiredHeaders(t *testing.T) {
	validator := NewSyntaxValidator()
	
	req := parser.NewRequestMessage("INVITE", "sip:test@example.com")
	// Don't add required headers
	
	result := validator.Validate(req)
	
	if result.Valid {
		t.Error("Expected validation to fail for missing required headers")
	}
	
	if result.ErrorCode != 400 {
		t.Errorf("Expected error code 400, got %d", result.ErrorCode)
	}
}

func TestSyntaxValidator_InvalidCSeq(t *testing.T) {
	validator := NewSyntaxValidator()
	
	req := parser.NewRequestMessage("INVITE", "sip:test@example.com")
	req.SetHeader("Via", "SIP/2.0/UDP 192.168.1.1:5060;branch=z9hG4bK123")
	req.SetHeader("From", "sip:alice@example.com;tag=abc123")
	req.SetHeader("To", "sip:bob@example.com")
	req.SetHeader("Call-ID", "call123@example.com")
	req.SetHeader("CSeq", "invalid")
	
	result := validator.Validate(req)
	
	if result.Valid {
		t.Error("Expected validation to fail for invalid CSeq format")
	}
	
	if result.ErrorCode != 400 {
		t.Errorf("Expected error code 400, got %d", result.ErrorCode)
	}
}

func TestSyntaxValidator_InvalidContentLength(t *testing.T) {
	validator := NewSyntaxValidator()
	
	req := parser.NewRequestMessage("INVITE", "sip:test@example.com")
	req.SetHeader("Via", "SIP/2.0/UDP 192.168.1.1:5060;branch=z9hG4bK123")
	req.SetHeader("From", "sip:alice@example.com;tag=abc123")
	req.SetHeader("To", "sip:bob@example.com")
	req.SetHeader("Call-ID", "call123@example.com")
	req.SetHeader("CSeq", "1 INVITE")
	req.SetHeader("Content-Length", "abc")
	
	result := validator.Validate(req)
	
	if result.Valid {
		t.Error("Expected validation to fail for invalid Content-Length")
	}
	
	if result.ErrorCode != 400 {
		t.Errorf("Expected error code 400, got %d", result.ErrorCode)
	}
}