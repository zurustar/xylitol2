package validation

import (
	"testing"

	"github.com/zurustar/xylitol2/internal/parser"
)

func TestSessionTimerValidator_AppliesTo(t *testing.T) {
	validator := NewSessionTimerValidator(90, 1800, true)
	
	// Test with INVITE request
	invite := parser.NewRequestMessage("INVITE", "sip:test@example.com")
	if !validator.AppliesTo(invite) {
		t.Error("Expected SessionTimerValidator to apply to INVITE requests")
	}
	
	// Test with non-INVITE request
	register := parser.NewRequestMessage("REGISTER", "sip:test@example.com")
	if validator.AppliesTo(register) {
		t.Error("Expected SessionTimerValidator not to apply to non-INVITE requests")
	}
}

func TestSessionTimerValidator_RequiredSupport_Missing(t *testing.T) {
	validator := NewSessionTimerValidator(90, 1800, true)
	
	req := parser.NewRequestMessage("INVITE", "sip:test@example.com")
	// No Session-Expires header and no timer support
	
	result := validator.Validate(req)
	
	if result.Valid {
		t.Error("Expected validation to fail when Session-Timer support is required but missing")
	}
	
	if result.ErrorCode != 421 {
		t.Errorf("Expected error code 421, got %d", result.ErrorCode)
	}
	
	if result.ErrorReason != "Extension Required" {
		t.Errorf("Expected 'Extension Required', got '%s'", result.ErrorReason)
	}
}

func TestSessionTimerValidator_RequiredSupport_WithSupported(t *testing.T) {
	validator := NewSessionTimerValidator(90, 1800, true)
	
	req := parser.NewRequestMessage("INVITE", "sip:test@example.com")
	req.SetHeader("Supported", "timer")
	
	result := validator.Validate(req)
	
	if !result.Valid {
		t.Errorf("Expected validation to pass when timer is supported, got error: %s", result.Details)
	}
}

func TestSessionTimerValidator_ValidSessionExpires(t *testing.T) {
	validator := NewSessionTimerValidator(90, 1800, false)
	
	req := parser.NewRequestMessage("INVITE", "sip:test@example.com")
	req.SetHeader("Session-Expires", "1800")
	
	result := validator.Validate(req)
	
	if !result.Valid {
		t.Errorf("Expected validation to pass for valid Session-Expires, got error: %s", result.Details)
	}
}

func TestSessionTimerValidator_SessionExpiresTooSmall(t *testing.T) {
	validator := NewSessionTimerValidator(90, 1800, false)
	
	req := parser.NewRequestMessage("INVITE", "sip:test@example.com")
	req.SetHeader("Session-Expires", "60")
	
	result := validator.Validate(req)
	
	if result.Valid {
		t.Error("Expected validation to fail for Session-Expires value too small")
	}
	
	if result.ErrorCode != 422 {
		t.Errorf("Expected error code 422, got %d", result.ErrorCode)
	}
	
	if result.ErrorReason != "Session Interval Too Small" {
		t.Errorf("Expected 'Session Interval Too Small', got '%s'", result.ErrorReason)
	}
}

func TestSessionTimerValidator_InvalidSessionExpires(t *testing.T) {
	validator := NewSessionTimerValidator(90, 1800, false)
	
	req := parser.NewRequestMessage("INVITE", "sip:test@example.com")
	req.SetHeader("Session-Expires", "invalid")
	
	result := validator.Validate(req)
	
	if result.Valid {
		t.Error("Expected validation to fail for invalid Session-Expires format")
	}
	
	if result.ErrorCode != 400 {
		t.Errorf("Expected error code 400, got %d", result.ErrorCode)
	}
}

func TestSessionTimerValidator_ValidRefresher(t *testing.T) {
	validator := NewSessionTimerValidator(90, 1800, false)
	
	req := parser.NewRequestMessage("INVITE", "sip:test@example.com")
	req.SetHeader("Session-Expires", "1800;refresher=uac")
	
	result := validator.Validate(req)
	
	if !result.Valid {
		t.Errorf("Expected validation to pass for valid refresher parameter, got error: %s", result.Details)
	}
}

func TestSessionTimerValidator_InvalidRefresher(t *testing.T) {
	validator := NewSessionTimerValidator(90, 1800, false)
	
	req := parser.NewRequestMessage("INVITE", "sip:test@example.com")
	req.SetHeader("Session-Expires", "1800;refresher=invalid")
	
	result := validator.Validate(req)
	
	if result.Valid {
		t.Error("Expected validation to fail for invalid refresher parameter")
	}
	
	if result.ErrorCode != 400 {
		t.Errorf("Expected error code 400, got %d", result.ErrorCode)
	}
}

func TestSessionTimerValidator_ValidMinSE(t *testing.T) {
	validator := NewSessionTimerValidator(90, 1800, false)
	
	req := parser.NewRequestMessage("INVITE", "sip:test@example.com")
	req.SetHeader("Session-Expires", "1800")
	req.SetHeader("Min-SE", "90")
	
	result := validator.Validate(req)
	
	if !result.Valid {
		t.Errorf("Expected validation to pass for valid Min-SE, got error: %s", result.Details)
	}
}

func TestSessionTimerValidator_InvalidMinSE(t *testing.T) {
	validator := NewSessionTimerValidator(90, 1800, false)
	
	req := parser.NewRequestMessage("INVITE", "sip:test@example.com")
	req.SetHeader("Min-SE", "invalid")
	
	result := validator.Validate(req)
	
	if result.Valid {
		t.Error("Expected validation to fail for invalid Min-SE format")
	}
	
	if result.ErrorCode != 400 {
		t.Errorf("Expected error code 400, got %d", result.ErrorCode)
	}
}

func TestSessionTimerValidator_MinSEGreaterThanSE(t *testing.T) {
	validator := NewSessionTimerValidator(90, 1800, false)
	
	req := parser.NewRequestMessage("INVITE", "sip:test@example.com")
	req.SetHeader("Session-Expires", "1800")
	req.SetHeader("Min-SE", "3600")
	
	result := validator.Validate(req)
	
	if result.Valid {
		t.Error("Expected validation to fail when Min-SE is greater than Session-Expires")
	}
	
	if result.ErrorCode != 400 {
		t.Errorf("Expected error code 400, got %d", result.ErrorCode)
	}
}

func TestParseSessionExpires(t *testing.T) {
	// Test valid session expires
	expires, refresher, err := parseSessionExpires("1800")
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if expires != 1800 {
		t.Errorf("Expected expires 1800, got %d", expires)
	}
	if refresher != "" {
		t.Errorf("Expected empty refresher, got %s", refresher)
	}
	
	// Test with refresher parameter
	expires, refresher, err = parseSessionExpires("1800;refresher=uac")
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if expires != 1800 {
		t.Errorf("Expected expires 1800, got %d", expires)
	}
	if refresher != "uac" {
		t.Errorf("Expected refresher 'uac', got %s", refresher)
	}
	
	// Test invalid format
	_, _, err = parseSessionExpires("invalid")
	if err == nil {
		t.Error("Expected error for invalid format")
	}
}

func TestContainsTimer(t *testing.T) {
	// Test with timer support
	if !containsTimer("timer") {
		t.Error("Expected containsTimer to return true for 'timer'")
	}
	
	if !containsTimer("replaces, timer, 100rel") {
		t.Error("Expected containsTimer to return true for comma-separated list with timer")
	}
	
	// Test without timer support
	if containsTimer("replaces, 100rel") {
		t.Error("Expected containsTimer to return false for list without timer")
	}
	
	if containsTimer("") {
		t.Error("Expected containsTimer to return false for empty string")
	}
}