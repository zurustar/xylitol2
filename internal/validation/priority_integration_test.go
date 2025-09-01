package validation

import (
	"testing"

	"github.com/zurustar/xylitol2/internal/parser"
)

// TestSessionTimerPriorityValidation tests that Session-Timer validation
// occurs before authentication validation according to RFC4028
func TestSessionTimerPriorityValidation(t *testing.T) {
	processor := NewMessageProcessor()
	
	// Add validators in reverse priority order to test sorting
	authValidator := NewAuthValidator(true, "example.com")
	sessionTimerValidator := NewSessionTimerValidator(90, 1800, true)
	syntaxValidator := NewSyntaxValidator()
	
	processor.AddValidator(authValidator)      // Priority 20
	processor.AddValidator(sessionTimerValidator) // Priority 10
	processor.AddValidator(syntaxValidator)    // Priority 1
	
	// Verify validators are in correct priority order
	validators := processor.GetValidators()
	if len(validators) != 3 {
		t.Fatalf("Expected 3 validators, got %d", len(validators))
	}
	
	// Check priority order: Syntax (1), SessionTimer (10), Auth (20)
	if validators[0].Name() != "SyntaxValidator" {
		t.Errorf("Expected SyntaxValidator first, got %s", validators[0].Name())
	}
	if validators[1].Name() != "SessionTimerValidator" {
		t.Errorf("Expected SessionTimerValidator second, got %s", validators[1].Name())
	}
	if validators[2].Name() != "AuthValidator" {
		t.Errorf("Expected AuthValidator third, got %s", validators[2].Name())
	}
}

// TestSessionTimerFailsBeforeAuth tests that Session-Timer validation failure
// prevents authentication validation from running
func TestSessionTimerFailsBeforeAuth(t *testing.T) {
	processor := NewMessageProcessor()
	
	// Add validators
	processor.AddValidator(NewAuthValidator(true, "example.com"))
	processor.AddValidator(NewSessionTimerValidator(90, 1800, true))
	processor.AddValidator(NewSyntaxValidator())
	
	// Create INVITE request without Session-Timer support
	req := parser.NewRequestMessage("INVITE", "sip:test@example.com")
	req.SetHeader("Via", "SIP/2.0/UDP 192.168.1.1:5060;branch=z9hG4bK123")
	req.SetHeader("From", "sip:alice@example.com;tag=abc123")
	req.SetHeader("To", "sip:bob@example.com")
	req.SetHeader("Call-ID", "call123@example.com")
	req.SetHeader("CSeq", "1 INVITE")
	// No Session-Expires header and no Supported: timer
	
	resp, err := processor.ProcessRequest(req)
	
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	
	if resp == nil {
		t.Fatal("Expected error response")
	}
	
	// Should get 421 Extension Required, not 401 Unauthorized
	if resp.GetStatusCode() != 421 {
		t.Errorf("Expected status code 421 (Extension Required), got %d", resp.GetStatusCode())
	}
	
	if resp.GetReasonPhrase() != "Extension Required" {
		t.Errorf("Expected 'Extension Required', got '%s'", resp.GetReasonPhrase())
	}
	
	// Should have Require header
	require := resp.GetHeader("Require")
	if require != "timer" {
		t.Errorf("Expected Require header 'timer', got '%s'", require)
	}
	
	// Should NOT have WWW-Authenticate header (auth didn't run)
	wwwAuth := resp.GetHeader("WWW-Authenticate")
	if wwwAuth != "" {
		t.Error("Expected no WWW-Authenticate header when Session-Timer validation fails first")
	}
}

// TestSessionTimerPassesAuthFails tests that when Session-Timer validation passes,
// authentication validation runs and can fail
func TestSessionTimerPassesAuthFails(t *testing.T) {
	processor := NewMessageProcessor()
	
	// Add validators
	processor.AddValidator(NewAuthValidator(true, "example.com"))
	processor.AddValidator(NewSessionTimerValidator(90, 1800, true))
	processor.AddValidator(NewSyntaxValidator())
	
	// Create INVITE request with Session-Timer support but no auth
	req := parser.NewRequestMessage("INVITE", "sip:test@example.com")
	req.SetHeader("Via", "SIP/2.0/UDP 192.168.1.1:5060;branch=z9hG4bK123")
	req.SetHeader("From", "sip:alice@example.com;tag=abc123")
	req.SetHeader("To", "sip:bob@example.com")
	req.SetHeader("Call-ID", "call123@example.com")
	req.SetHeader("CSeq", "1 INVITE")
	req.SetHeader("Supported", "timer")
	// No Authorization header
	
	resp, err := processor.ProcessRequest(req)
	
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	
	if resp == nil {
		t.Fatal("Expected error response")
	}
	
	// Should get 401 Unauthorized (auth validation ran and failed)
	if resp.GetStatusCode() != 401 {
		t.Errorf("Expected status code 401 (Unauthorized), got %d", resp.GetStatusCode())
	}
	
	if resp.GetReasonPhrase() != "Unauthorized" {
		t.Errorf("Expected 'Unauthorized', got '%s'", resp.GetReasonPhrase())
	}
	
	// Should have WWW-Authenticate header
	wwwAuth := resp.GetHeader("WWW-Authenticate")
	if wwwAuth == "" {
		t.Error("Expected WWW-Authenticate header for 401 response")
	}
}

// TestSessionTimerAndAuthBothPass tests that when both Session-Timer and
// authentication validation pass, processing continues
func TestSessionTimerAndAuthBothPass(t *testing.T) {
	processor := NewMessageProcessor()
	
	// Add validators
	processor.AddValidator(NewAuthValidator(true, "example.com"))
	processor.AddValidator(NewSessionTimerValidator(90, 1800, true))
	processor.AddValidator(NewSyntaxValidator())
	
	// Create INVITE request with both Session-Timer support and auth
	req := parser.NewRequestMessage("INVITE", "sip:test@example.com")
	req.SetHeader("Via", "SIP/2.0/UDP 192.168.1.1:5060;branch=z9hG4bK123")
	req.SetHeader("From", "sip:alice@example.com;tag=abc123")
	req.SetHeader("To", "sip:bob@example.com")
	req.SetHeader("Call-ID", "call123@example.com")
	req.SetHeader("CSeq", "1 INVITE")
	req.SetHeader("Supported", "timer")
	req.SetHeader("Authorization", `Digest username="alice", realm="example.com", nonce="abc123", uri="sip:test@example.com", response="def456"`)
	
	resp, err := processor.ProcessRequest(req)
	
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	
	// Should return nil (validation passed, continue processing)
	if resp != nil {
		t.Errorf("Expected nil response for successful validation, got status %d", resp.GetStatusCode())
	}
}

// TestNonInviteSkipsSessionTimer tests that non-INVITE requests skip
// Session-Timer validation but still go through auth validation
func TestNonInviteSkipsSessionTimer(t *testing.T) {
	processor := NewMessageProcessor()
	
	// Add validators
	processor.AddValidator(NewAuthValidator(true, "example.com"))
	processor.AddValidator(NewSessionTimerValidator(90, 1800, true))
	processor.AddValidator(NewSyntaxValidator())
	
	// Create REGISTER request (no Session-Timer required)
	req := parser.NewRequestMessage("REGISTER", "sip:example.com")
	req.SetHeader("Via", "SIP/2.0/UDP 192.168.1.1:5060;branch=z9hG4bK123")
	req.SetHeader("From", "sip:alice@example.com;tag=abc123")
	req.SetHeader("To", "sip:alice@example.com")
	req.SetHeader("Call-ID", "call123@example.com")
	req.SetHeader("CSeq", "1 REGISTER")
	// No Authorization header
	
	resp, err := processor.ProcessRequest(req)
	
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	
	if resp == nil {
		t.Fatal("Expected error response")
	}
	
	// Should get 401 Unauthorized (Session-Timer skipped, auth failed)
	if resp.GetStatusCode() != 401 {
		t.Errorf("Expected status code 401 (Unauthorized), got %d", resp.GetStatusCode())
	}
	
	// Should NOT have Require header (Session-Timer validation was skipped)
	require := resp.GetHeader("Require")
	if require != "" {
		t.Errorf("Expected no Require header for REGISTER, got '%s'", require)
	}
}

// TestSessionTimerTooSmall tests 422 Session Interval Too Small response
func TestSessionTimerTooSmall(t *testing.T) {
	processor := NewMessageProcessor()
	
	// Add validators
	processor.AddValidator(NewSessionTimerValidator(90, 1800, false))
	processor.AddValidator(NewSyntaxValidator())
	
	// Create INVITE request with Session-Expires too small
	req := parser.NewRequestMessage("INVITE", "sip:test@example.com")
	req.SetHeader("Via", "SIP/2.0/UDP 192.168.1.1:5060;branch=z9hG4bK123")
	req.SetHeader("From", "sip:alice@example.com;tag=abc123")
	req.SetHeader("To", "sip:bob@example.com")
	req.SetHeader("Call-ID", "call123@example.com")
	req.SetHeader("CSeq", "1 INVITE")
	req.SetHeader("Session-Expires", "60") // Less than minimum 90
	
	resp, err := processor.ProcessRequest(req)
	
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	
	if resp == nil {
		t.Fatal("Expected error response")
	}
	
	// Should get 422 Session Interval Too Small
	if resp.GetStatusCode() != 422 {
		t.Errorf("Expected status code 422, got %d", resp.GetStatusCode())
	}
	
	if resp.GetReasonPhrase() != "Session Interval Too Small" {
		t.Errorf("Expected 'Session Interval Too Small', got '%s'", resp.GetReasonPhrase())
	}
	
	// Should have Min-SE header
	minSE := resp.GetHeader("Min-SE")
	if minSE != "90" {
		t.Errorf("Expected Min-SE header '90', got '%s'", minSE)
	}
}

// TestValidationChainStopsOnFirstFailure tests that validation stops
// on the first failure and doesn't continue to subsequent validators
func TestValidationChainStopsOnFirstFailure(t *testing.T) {
	processor := NewMessageProcessor()
	
	// Add validators
	processor.AddValidator(NewAuthValidator(true, "example.com"))
	processor.AddValidator(NewSessionTimerValidator(90, 1800, true))
	
	// Create request that will fail Session-Timer validation first
	req := parser.NewRequestMessage("INVITE", "sip:test@example.com")
	req.SetHeader("Via", "SIP/2.0/UDP 192.168.1.1:5060;branch=z9hG4bK123")
	req.SetHeader("From", "sip:alice@example.com;tag=abc123")
	req.SetHeader("To", "sip:bob@example.com")
	req.SetHeader("Call-ID", "call123@example.com")
	req.SetHeader("CSeq", "1 INVITE")
	// Missing both Session-Timer support AND Authorization
	
	resp, err := processor.ProcessRequest(req)
	
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	
	if resp == nil {
		t.Fatal("Expected error response")
	}
	
	// Should fail on Session-Timer (priority 10) before Auth (priority 20)
	if resp.GetStatusCode() != 421 {
		t.Errorf("Expected status code 421 (Session-Timer failure), got %d", resp.GetStatusCode())
	}
	
	// Verify it's the Session-Timer error, not auth error
	require := resp.GetHeader("Require")
	if require != "timer" {
		t.Error("Expected Session-Timer validation failure, not auth failure")
	}
}