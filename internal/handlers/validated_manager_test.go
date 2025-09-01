package handlers

import (
	"testing"

	"github.com/zurustar/xylitol2/internal/parser"
	"github.com/zurustar/xylitol2/internal/transaction"
	"github.com/zurustar/xylitol2/internal/validation"
)

// testTransaction implements transaction.Transaction for testing
type testTransaction struct {
	sentResponse *parser.SIPMessage
	sendError    error
}

func (tt *testTransaction) SendResponse(resp *parser.SIPMessage) error {
	tt.sentResponse = resp
	return tt.sendError
}

func (tt *testTransaction) GetState() transaction.TransactionState {
	return transaction.StateTrying
}

func (tt *testTransaction) ProcessMessage(msg *parser.SIPMessage) error {
	return nil
}

func (tt *testTransaction) GetID() string {
	return "test-transaction-id"
}

func (tt *testTransaction) IsClient() bool {
	return false
}

// testMethodHandler implements MethodHandler for testing
type testMethodHandler struct {
	canHandleMethods []string
	handleError      error
	handleCalled     bool
}

func (mh *testMethodHandler) HandleRequest(req *parser.SIPMessage, txn transaction.Transaction) error {
	mh.handleCalled = true
	return mh.handleError
}

func (mh *testMethodHandler) CanHandle(method string) bool {
	for _, m := range mh.canHandleMethods {
		if m == method {
			return true
		}
	}
	return false
}

func TestValidatedManager_ValidationSuccess(t *testing.T) {
	manager := NewValidatedManager()
	
	// Setup default validators
	config := DefaultValidationConfig()
	config.SessionTimerConfig.RequireSupport = false // Don't require Session-Timer for this test
	config.AuthConfig.RequireAuth = false           // Don't require auth for this test
	manager.SetupDefaultValidators(config)
	
	// Register a mock handler
	mockHandler := &testMethodHandler{
		canHandleMethods: []string{"INVITE"},
	}
	manager.RegisterHandler(mockHandler)
	
	// Create valid INVITE request
	req := parser.NewRequestMessage("INVITE", "sip:test@example.com")
	req.SetHeader("Via", "SIP/2.0/UDP 192.168.1.1:5060;branch=z9hG4bK123")
	req.SetHeader("From", "sip:alice@example.com;tag=abc123")
	req.SetHeader("To", "sip:bob@example.com")
	req.SetHeader("Call-ID", "call123@example.com")
	req.SetHeader("CSeq", "1 INVITE")
	
	txn := &testTransaction{}
	
	err := manager.HandleRequest(req, txn)
	
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	
	// Should not have sent error response
	if txn.sentResponse != nil {
		t.Errorf("Expected no response for successful validation, got status %d", txn.sentResponse.GetStatusCode())
	}
	
	// Should have called the handler
	if !mockHandler.handleCalled {
		t.Error("Expected handler to be called after successful validation")
	}
}

func TestValidatedManager_ValidationFailure_SessionTimer(t *testing.T) {
	manager := NewValidatedManager()
	
	// Setup validators with Session-Timer required
	config := DefaultValidationConfig()
	config.SessionTimerConfig.RequireSupport = true
	config.AuthConfig.RequireAuth = false
	manager.SetupDefaultValidators(config)
	
	// Register a mock handler
	mockHandler := &testMethodHandler{
		canHandleMethods: []string{"INVITE"},
	}
	manager.RegisterHandler(mockHandler)
	
	// Create INVITE request without Session-Timer support
	req := parser.NewRequestMessage("INVITE", "sip:test@example.com")
	req.SetHeader("Via", "SIP/2.0/UDP 192.168.1.1:5060;branch=z9hG4bK123")
	req.SetHeader("From", "sip:alice@example.com;tag=abc123")
	req.SetHeader("To", "sip:bob@example.com")
	req.SetHeader("Call-ID", "call123@example.com")
	req.SetHeader("CSeq", "1 INVITE")
	// No Session-Timer support
	
	txn := &testTransaction{}
	
	err := manager.HandleRequest(req, txn)
	
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	
	// Should have sent 421 Extension Required response
	if txn.sentResponse == nil {
		t.Fatal("Expected error response for validation failure")
	}
	
	if txn.sentResponse.GetStatusCode() != 421 {
		t.Errorf("Expected status code 421, got %d", txn.sentResponse.GetStatusCode())
	}
	
	// Should NOT have called the handler
	if mockHandler.handleCalled {
		t.Error("Expected handler not to be called after validation failure")
	}
}

func TestValidatedManager_ValidationFailure_Auth(t *testing.T) {
	manager := NewValidatedManager()
	
	// Setup validators with auth required but Session-Timer not required
	config := DefaultValidationConfig()
	config.SessionTimerConfig.RequireSupport = false
	config.AuthConfig.RequireAuth = true
	manager.SetupDefaultValidators(config)
	
	// Register a mock handler
	mockHandler := &testMethodHandler{
		canHandleMethods: []string{"INVITE"},
	}
	manager.RegisterHandler(mockHandler)
	
	// Create INVITE request without authorization
	req := parser.NewRequestMessage("INVITE", "sip:test@example.com")
	req.SetHeader("Via", "SIP/2.0/UDP 192.168.1.1:5060;branch=z9hG4bK123")
	req.SetHeader("From", "sip:alice@example.com;tag=abc123")
	req.SetHeader("To", "sip:bob@example.com")
	req.SetHeader("Call-ID", "call123@example.com")
	req.SetHeader("CSeq", "1 INVITE")
	// No Authorization header
	
	txn := &testTransaction{}
	
	err := manager.HandleRequest(req, txn)
	
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	
	// Should have sent 401 Unauthorized response
	if txn.sentResponse == nil {
		t.Fatal("Expected error response for validation failure")
	}
	
	if txn.sentResponse.GetStatusCode() != 401 {
		t.Errorf("Expected status code 401, got %d", txn.sentResponse.GetStatusCode())
	}
	
	// Should NOT have called the handler
	if mockHandler.handleCalled {
		t.Error("Expected handler not to be called after validation failure")
	}
}

func TestValidatedManager_ValidationPriority(t *testing.T) {
	manager := NewValidatedManager()
	
	// Setup validators with both Session-Timer and auth required
	config := DefaultValidationConfig()
	config.SessionTimerConfig.RequireSupport = true
	config.AuthConfig.RequireAuth = true
	manager.SetupDefaultValidators(config)
	
	// Create INVITE request missing both Session-Timer and auth
	req := parser.NewRequestMessage("INVITE", "sip:test@example.com")
	req.SetHeader("Via", "SIP/2.0/UDP 192.168.1.1:5060;branch=z9hG4bK123")
	req.SetHeader("From", "sip:alice@example.com;tag=abc123")
	req.SetHeader("To", "sip:bob@example.com")
	req.SetHeader("Call-ID", "call123@example.com")
	req.SetHeader("CSeq", "1 INVITE")
	// Missing both Session-Timer and Authorization
	
	txn := &testTransaction{}
	
	err := manager.HandleRequest(req, txn)
	
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	
	// Should fail on Session-Timer first (priority 10) before auth (priority 20)
	if txn.sentResponse == nil {
		t.Fatal("Expected error response")
	}
	
	if txn.sentResponse.GetStatusCode() != 421 {
		t.Errorf("Expected status code 421 (Session-Timer failure first), got %d", txn.sentResponse.GetStatusCode())
	}
}

func TestValidatedManager_NonRequestMessage(t *testing.T) {
	manager := NewValidatedManager()
	
	// Create response message
	resp := parser.NewResponseMessage(200, "OK")
	
	txn := &testTransaction{}
	
	err := manager.HandleRequest(resp, txn)
	
	if err == nil {
		t.Error("Expected error for non-request message")
	}
}

func TestValidatedManager_AddRemoveValidators(t *testing.T) {
	manager := NewValidatedManager()
	
	// Add custom validator
	customValidator := validation.NewSyntaxValidator()
	manager.AddValidator(customValidator)
	
	validators := manager.GetValidators()
	if len(validators) != 1 {
		t.Errorf("Expected 1 validator, got %d", len(validators))
	}
	
	// Remove validator
	if !manager.RemoveValidator("SyntaxValidator") {
		t.Error("Expected RemoveValidator to return true")
	}
	
	validators = manager.GetValidators()
	if len(validators) != 0 {
		t.Errorf("Expected 0 validators after removal, got %d", len(validators))
	}
}

func TestValidatedManager_SetupDefaultValidators(t *testing.T) {
	manager := NewValidatedManager()
	
	config := DefaultValidationConfig()
	manager.SetupDefaultValidators(config)
	
	validators := manager.GetValidators()
	
	// Should have 3 validators: Syntax, SessionTimer, Auth
	if len(validators) != 3 {
		t.Errorf("Expected 3 validators, got %d", len(validators))
	}
	
	// Check priority order
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

func TestValidatedManager_DisabledValidators(t *testing.T) {
	manager := NewValidatedManager()
	
	// Disable Session-Timer and Auth validators
	config := ValidationConfig{
		SessionTimerConfig: SessionTimerConfig{
			Enabled: false,
		},
		AuthConfig: AuthConfig{
			Enabled: false,
		},
	}
	manager.SetupDefaultValidators(config)
	
	validators := manager.GetValidators()
	
	// Should only have Syntax validator
	if len(validators) != 1 {
		t.Errorf("Expected 1 validator when others disabled, got %d", len(validators))
	}
	
	if validators[0].Name() != "SyntaxValidator" {
		t.Errorf("Expected only SyntaxValidator, got %s", validators[0].Name())
	}
}

func TestDefaultValidationConfig(t *testing.T) {
	config := DefaultValidationConfig()
	
	// Check Session-Timer config
	if !config.SessionTimerConfig.Enabled {
		t.Error("Expected Session-Timer to be enabled by default")
	}
	if config.SessionTimerConfig.MinSE != 90 {
		t.Errorf("Expected MinSE 90, got %d", config.SessionTimerConfig.MinSE)
	}
	if config.SessionTimerConfig.DefaultSE != 1800 {
		t.Errorf("Expected DefaultSE 1800, got %d", config.SessionTimerConfig.DefaultSE)
	}
	if !config.SessionTimerConfig.RequireSupport {
		t.Error("Expected Session-Timer support to be required by default")
	}
	
	// Check Auth config
	if !config.AuthConfig.Enabled {
		t.Error("Expected Auth to be enabled by default")
	}
	if !config.AuthConfig.RequireAuth {
		t.Error("Expected Auth to be required by default")
	}
	if config.AuthConfig.Realm != "sip-server" {
		t.Errorf("Expected realm 'sip-server', got '%s'", config.AuthConfig.Realm)
	}
}