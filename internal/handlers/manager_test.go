package handlers

import (
	"fmt"
	"strings"
	"testing"

	"github.com/zurustar/xylitol2/internal/parser"
	"github.com/zurustar/xylitol2/internal/transaction"
)

// Mock handler for testing
type mockHandler struct {
	supportedMethods []string
	handleFunc       func(req *parser.SIPMessage, txn transaction.Transaction) error
}

func (m *mockHandler) CanHandle(method string) bool {
	for _, supported := range m.supportedMethods {
		if supported == method {
			return true
		}
	}
	return false
}

func (m *mockHandler) HandleRequest(req *parser.SIPMessage, txn transaction.Transaction) error {
	if m.handleFunc != nil {
		return m.handleFunc(req, txn)
	}
	return nil
}

func TestManager_RegisterHandler(t *testing.T) {
	manager := NewManager()
	
	handler1 := &mockHandler{supportedMethods: []string{parser.MethodINVITE}}
	handler2 := &mockHandler{supportedMethods: []string{parser.MethodOPTIONS}}
	
	manager.RegisterHandler(handler1)
	manager.RegisterHandler(handler2)
	
	if len(manager.handlers) != 2 {
		t.Errorf("Expected 2 handlers, got %d", len(manager.handlers))
	}
}

func TestManager_HandleRequest_Success(t *testing.T) {
	manager := NewManager()
	
	var handledRequest *parser.SIPMessage
	handler := &mockHandler{
		supportedMethods: []string{parser.MethodINVITE},
		handleFunc: func(req *parser.SIPMessage, txn transaction.Transaction) error {
			handledRequest = req
			return nil
		},
	}
	
	manager.RegisterHandler(handler)
	
	mockTxn := &mockTransaction{}
	
	// Create test INVITE request
	invite := parser.NewRequestMessage(parser.MethodINVITE, "sip:user@example.com")
	invite.SetHeader(parser.HeaderVia, "SIP/2.0/UDP client.example.com:5060;branch=z9hG4bK123")
	invite.SetHeader(parser.HeaderFrom, "Alice <sip:alice@example.com>;tag=abc123")
	invite.SetHeader(parser.HeaderTo, "Bob <sip:bob@example.com>")
	invite.SetHeader(parser.HeaderCallID, "test-call-id@example.com")
	invite.SetHeader(parser.HeaderCSeq, "1 INVITE")
	
	err := manager.HandleRequest(invite, mockTxn)
	if err != nil {
		t.Errorf("HandleRequest failed: %v", err)
	}
	
	if handledRequest == nil {
		t.Error("Expected request to be handled")
	}
	
	if handledRequest.GetMethod() != parser.MethodINVITE {
		t.Errorf("Expected method %s, got %s", parser.MethodINVITE, handledRequest.GetMethod())
	}
}

func TestManager_HandleRequest_HandlerError(t *testing.T) {
	manager := NewManager()
	
	expectedError := fmt.Errorf("handler error")
	handler := &mockHandler{
		supportedMethods: []string{parser.MethodINVITE},
		handleFunc: func(req *parser.SIPMessage, txn transaction.Transaction) error {
			return expectedError
		},
	}
	
	manager.RegisterHandler(handler)
	
	mockTxn := &mockTransaction{}
	
	// Create test INVITE request
	invite := parser.NewRequestMessage(parser.MethodINVITE, "sip:user@example.com")
	
	err := manager.HandleRequest(invite, mockTxn)
	if err == nil {
		t.Error("Expected error from handler")
	}
	
	if err != expectedError {
		t.Errorf("Expected error %v, got %v", expectedError, err)
	}
}

func TestManager_HandleRequest_MethodNotAllowed(t *testing.T) {
	manager := NewManager()
	
	// Register handler that doesn't support the method we'll test
	handler := &mockHandler{supportedMethods: []string{parser.MethodINVITE}}
	manager.RegisterHandler(handler)
	
	var sentResponse *parser.SIPMessage
	mockTxn := &mockTransaction{
		sendResponseFunc: func(response *parser.SIPMessage) error {
			sentResponse = response
			return nil
		},
	}
	
	// Create test request with unsupported method
	req := parser.NewRequestMessage("UNSUPPORTED", "sip:user@example.com")
	req.SetHeader(parser.HeaderVia, "SIP/2.0/UDP client.example.com:5060;branch=z9hG4bK123")
	req.SetHeader(parser.HeaderFrom, "Alice <sip:alice@example.com>;tag=abc123")
	req.SetHeader(parser.HeaderTo, "Bob <sip:bob@example.com>")
	req.SetHeader(parser.HeaderCallID, "test-call-id@example.com")
	req.SetHeader(parser.HeaderCSeq, "1 UNSUPPORTED")
	
	err := manager.HandleRequest(req, mockTxn)
	if err != nil {
		t.Errorf("HandleRequest failed: %v", err)
	}
	
	if sentResponse == nil {
		t.Fatal("Expected response to be sent")
	}
	
	if sentResponse.GetStatusCode() != parser.StatusMethodNotAllowed {
		t.Errorf("Expected status code %d, got %d", parser.StatusMethodNotAllowed, sentResponse.GetStatusCode())
	}
	
	// Check Allow header
	allowHeader := sentResponse.GetHeader(parser.HeaderAllow)
	if allowHeader == "" {
		t.Error("Expected Allow header to be present")
	}
	
	if !strings.Contains(allowHeader, parser.MethodINVITE) {
		t.Errorf("Expected Allow header to contain %s, got: %s", parser.MethodINVITE, allowHeader)
	}
	
	// Verify mandatory response headers are copied
	if sentResponse.GetHeader(parser.HeaderVia) != req.GetHeader(parser.HeaderVia) {
		t.Error("Via header not copied correctly")
	}
	if sentResponse.GetHeader(parser.HeaderFrom) != req.GetHeader(parser.HeaderFrom) {
		t.Error("From header not copied correctly")
	}
	if sentResponse.GetHeader(parser.HeaderTo) != req.GetHeader(parser.HeaderTo) {
		t.Error("To header not copied correctly")
	}
	if sentResponse.GetHeader(parser.HeaderCallID) != req.GetHeader(parser.HeaderCallID) {
		t.Error("Call-ID header not copied correctly")
	}
	if sentResponse.GetHeader(parser.HeaderCSeq) != req.GetHeader(parser.HeaderCSeq) {
		t.Error("CSeq header not copied correctly")
	}
}

func TestManager_GetSupportedMethods(t *testing.T) {
	manager := NewManager()
	
	// Register handlers with different supported methods
	handler1 := &mockHandler{supportedMethods: []string{parser.MethodINVITE, parser.MethodACK}}
	handler2 := &mockHandler{supportedMethods: []string{parser.MethodOPTIONS, parser.MethodINFO}}
	handler3 := &mockHandler{supportedMethods: []string{parser.MethodBYE}}
	
	manager.RegisterHandler(handler1)
	manager.RegisterHandler(handler2)
	manager.RegisterHandler(handler3)
	
	supportedMethods := manager.GetSupportedMethods()
	
	expectedMethods := []string{
		parser.MethodINVITE,
		parser.MethodACK,
		parser.MethodOPTIONS,
		parser.MethodINFO,
		parser.MethodBYE,
	}
	
	if len(supportedMethods) != len(expectedMethods) {
		t.Errorf("Expected %d supported methods, got %d", len(expectedMethods), len(supportedMethods))
	}
	
	// Check that all expected methods are present
	methodSet := make(map[string]bool)
	for _, method := range supportedMethods {
		methodSet[method] = true
	}
	
	for _, expected := range expectedMethods {
		if !methodSet[expected] {
			t.Errorf("Expected method %s to be supported", expected)
		}
	}
}

func TestManager_GetSupportedMethods_NoDuplicates(t *testing.T) {
	manager := NewManager()
	
	// Register handlers with overlapping supported methods
	handler1 := &mockHandler{supportedMethods: []string{parser.MethodINVITE, parser.MethodOPTIONS}}
	handler2 := &mockHandler{supportedMethods: []string{parser.MethodOPTIONS, parser.MethodINFO}}
	
	manager.RegisterHandler(handler1)
	manager.RegisterHandler(handler2)
	
	supportedMethods := manager.GetSupportedMethods()
	
	// Count occurrences of each method
	methodCount := make(map[string]int)
	for _, method := range supportedMethods {
		methodCount[method]++
	}
	
	// Check that no method appears more than once
	for method, count := range methodCount {
		if count > 1 {
			t.Errorf("Method %s appears %d times in supported methods list", method, count)
		}
	}
}

func TestManager_HandleRequest_MultipleHandlers(t *testing.T) {
	manager := NewManager()
	
	var handler1Called, handler2Called bool
	
	handler1 := &mockHandler{
		supportedMethods: []string{parser.MethodINVITE},
		handleFunc: func(req *parser.SIPMessage, txn transaction.Transaction) error {
			handler1Called = true
			return nil
		},
	}
	
	handler2 := &mockHandler{
		supportedMethods: []string{parser.MethodOPTIONS},
		handleFunc: func(req *parser.SIPMessage, txn transaction.Transaction) error {
			handler2Called = true
			return nil
		},
	}
	
	manager.RegisterHandler(handler1)
	manager.RegisterHandler(handler2)
	
	mockTxn := &mockTransaction{}
	
	// Test INVITE request (should call handler1)
	invite := parser.NewRequestMessage(parser.MethodINVITE, "sip:user@example.com")
	err := manager.HandleRequest(invite, mockTxn)
	if err != nil {
		t.Errorf("HandleRequest failed: %v", err)
	}
	
	if !handler1Called {
		t.Error("Expected handler1 to be called for INVITE")
	}
	if handler2Called {
		t.Error("Expected handler2 not to be called for INVITE")
	}
	
	// Reset flags
	handler1Called = false
	handler2Called = false
	
	// Test OPTIONS request (should call handler2)
	options := parser.NewRequestMessage(parser.MethodOPTIONS, "sip:user@example.com")
	err = manager.HandleRequest(options, mockTxn)
	if err != nil {
		t.Errorf("HandleRequest failed: %v", err)
	}
	
	if handler1Called {
		t.Error("Expected handler1 not to be called for OPTIONS")
	}
	if !handler2Called {
		t.Error("Expected handler2 to be called for OPTIONS")
	}
}

func TestManager_CopyResponseHeaders(t *testing.T) {
	manager := NewManager()
	
	// Create test request
	req := parser.NewRequestMessage(parser.MethodOPTIONS, "sip:user@example.com")
	req.SetHeader(parser.HeaderVia, "SIP/2.0/UDP client.example.com:5060;branch=z9hG4bK123")
	req.SetHeader(parser.HeaderFrom, "Alice <sip:alice@example.com>;tag=abc123")
	req.SetHeader(parser.HeaderTo, "Bob <sip:bob@example.com>")
	req.SetHeader(parser.HeaderCallID, "test-call-id@example.com")
	req.SetHeader(parser.HeaderCSeq, "1 OPTIONS")
	
	// Create response
	resp := parser.NewResponseMessage(parser.StatusOK, parser.GetReasonPhraseForCode(parser.StatusOK))
	
	// Copy headers
	manager.copyResponseHeaders(req, resp)
	
	// Verify headers are copied correctly
	if resp.GetHeader(parser.HeaderVia) != req.GetHeader(parser.HeaderVia) {
		t.Error("Via header not copied correctly")
	}
	if resp.GetHeader(parser.HeaderFrom) != req.GetHeader(parser.HeaderFrom) {
		t.Error("From header not copied correctly")
	}
	if resp.GetHeader(parser.HeaderTo) != req.GetHeader(parser.HeaderTo) {
		t.Error("To header not copied correctly")
	}
	if resp.GetHeader(parser.HeaderCallID) != req.GetHeader(parser.HeaderCallID) {
		t.Error("Call-ID header not copied correctly")
	}
	if resp.GetHeader(parser.HeaderCSeq) != req.GetHeader(parser.HeaderCSeq) {
		t.Error("CSeq header not copied correctly")
	}
	if resp.GetHeader(parser.HeaderContentLength) != "0" {
		t.Error("Content-Length header not set to 0")
	}
}