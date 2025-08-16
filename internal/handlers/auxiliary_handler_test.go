package handlers

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/zurustar/xylitol2/internal/database"
	"github.com/zurustar/xylitol2/internal/parser"
)

func TestAuxiliaryHandler_CanHandle(t *testing.T) {
	handler := NewAuxiliaryHandler(nil, nil)

	tests := []struct {
		method   string
		expected bool
	}{
		{parser.MethodOPTIONS, true},
		{parser.MethodINFO, true},
		{parser.MethodINVITE, false},
		{parser.MethodACK, false},
		{parser.MethodBYE, false},
		{parser.MethodREGISTER, false},
		{"UNKNOWN", false},
	}

	for _, test := range tests {
		t.Run(test.method, func(t *testing.T) {
			result := handler.CanHandle(test.method)
			if result != test.expected {
				t.Errorf("CanHandle(%s) = %v, expected %v", test.method, result, test.expected)
			}
		})
	}
}

func TestAuxiliaryHandler_HandleOptions_Success(t *testing.T) {
	handler := NewAuxiliaryHandler(nil, nil)

	var sentResponse *parser.SIPMessage
	mockTxn := &mockTransaction{
		sendResponseFunc: func(response *parser.SIPMessage) error {
			sentResponse = response
			return nil
		},
	}

	// Create test OPTIONS request
	options := parser.NewRequestMessage(parser.MethodOPTIONS, "sip:user@example.com")
	options.SetHeader(parser.HeaderVia, "SIP/2.0/UDP client.example.com:5060;branch=z9hG4bK123")
	options.SetHeader(parser.HeaderFrom, "Alice <sip:alice@example.com>;tag=abc123")
	options.SetHeader(parser.HeaderTo, "Bob <sip:bob@example.com>")
	options.SetHeader(parser.HeaderCallID, "test-call-id@example.com")
	options.SetHeader(parser.HeaderCSeq, "1 OPTIONS")

	err := handler.HandleRequest(options, mockTxn)
	if err != nil {
		t.Errorf("HandleRequest failed: %v", err)
	}

	if sentResponse == nil {
		t.Fatal("Expected response to be sent")
	}

	if sentResponse.GetStatusCode() != parser.StatusOK {
		t.Errorf("Expected status code %d, got %d", parser.StatusOK, sentResponse.GetStatusCode())
	}

	// Check Allow header
	allowHeader := sentResponse.GetHeader(parser.HeaderAllow)
	if allowHeader == "" {
		t.Error("Expected Allow header to be present")
	}

	expectedMethods := []string{
		parser.MethodINVITE,
		parser.MethodACK,
		parser.MethodBYE,
		parser.MethodCANCEL,
		parser.MethodREGISTER,
		parser.MethodOPTIONS,
		parser.MethodINFO,
	}

	for _, method := range expectedMethods {
		if !strings.Contains(allowHeader, method) {
			t.Errorf("Expected Allow header to contain %s, got: %s", method, allowHeader)
		}
	}

	// Check Supported header
	supportedHeader := sentResponse.GetHeader(parser.HeaderSupported)
	if supportedHeader == "" {
		t.Error("Expected Supported header to be present")
	}

	if !strings.Contains(supportedHeader, "timer") {
		t.Errorf("Expected Supported header to contain 'timer', got: %s", supportedHeader)
	}

	// Check Accept header
	acceptHeader := sentResponse.GetHeader("Accept")
	if acceptHeader == "" {
		t.Error("Expected Accept header to be present")
	}

	if !strings.Contains(acceptHeader, "application/sdp") {
		t.Errorf("Expected Accept header to contain 'application/sdp', got: %s", acceptHeader)
	}

	// Check Server header
	serverHeader := sentResponse.GetHeader(parser.HeaderServer)
	if serverHeader == "" {
		t.Error("Expected Server header to be present")
	}

	// Check Content-Length header
	contentLengthHeader := sentResponse.GetHeader(parser.HeaderContentLength)
	if contentLengthHeader != "0" {
		t.Errorf("Expected Content-Length header to be '0', got: %s", contentLengthHeader)
	}

	// Verify mandatory response headers are copied
	if sentResponse.GetHeader(parser.HeaderVia) != options.GetHeader(parser.HeaderVia) {
		t.Error("Via header not copied correctly")
	}
	if sentResponse.GetHeader(parser.HeaderFrom) != options.GetHeader(parser.HeaderFrom) {
		t.Error("From header not copied correctly")
	}
	if sentResponse.GetHeader(parser.HeaderTo) != options.GetHeader(parser.HeaderTo) {
		t.Error("To header not copied correctly")
	}
	if sentResponse.GetHeader(parser.HeaderCallID) != options.GetHeader(parser.HeaderCallID) {
		t.Error("Call-ID header not copied correctly")
	}
	if sentResponse.GetHeader(parser.HeaderCSeq) != options.GetHeader(parser.HeaderCSeq) {
		t.Error("CSeq header not copied correctly")
	}
}

func TestAuxiliaryHandler_HandleInfo_Success(t *testing.T) {
	mockProxy := &mockProxyEngine{
		forwardRequestFunc: func(req *parser.SIPMessage, targets []*database.RegistrarContact) error {
			return nil
		},
	}

	mockReg := &mockRegistrar{
		findContactsFunc: func(aor string) ([]*database.RegistrarContact, error) {
			return []*database.RegistrarContact{
				{
					AOR:     aor,
					URI:     "sip:user@example.com:5060",
					Expires: time.Now().Add(time.Hour),
					CallID:  "test-call-id",
					CSeq:    1,
				},
			}, nil
		},
	}

	mockTxn := &mockTransaction{}

	handler := NewAuxiliaryHandler(mockProxy, mockReg)

	// Create test INFO request
	info := parser.NewRequestMessage(parser.MethodINFO, "sip:user@example.com")
	info.SetHeader(parser.HeaderVia, "SIP/2.0/UDP client.example.com:5060;branch=z9hG4bK123")
	info.SetHeader(parser.HeaderFrom, "Alice <sip:alice@example.com>;tag=abc123")
	info.SetHeader(parser.HeaderTo, "Bob <sip:bob@example.com>;tag=def456")
	info.SetHeader(parser.HeaderCallID, "test-call-id@example.com")
	info.SetHeader(parser.HeaderCSeq, "2 INFO")
	info.SetHeader(parser.HeaderContentType, "application/dtmf-relay")
	info.Body = []byte("Signal=1\r\nDuration=100\r\n")

	err := handler.HandleRequest(info, mockTxn)
	if err != nil {
		t.Errorf("HandleRequest failed: %v", err)
	}
}

func TestAuxiliaryHandler_HandleInfo_UserNotFound(t *testing.T) {
	mockReg := &mockRegistrar{
		findContactsFunc: func(aor string) ([]*database.RegistrarContact, error) {
			return []*database.RegistrarContact{}, nil // No contacts found
		},
	}

	var sentResponse *parser.SIPMessage
	mockTxn := &mockTransaction{
		sendResponseFunc: func(response *parser.SIPMessage) error {
			sentResponse = response
			return nil
		},
	}

	handler := NewAuxiliaryHandler(nil, mockReg)

	// Create test INFO request
	info := parser.NewRequestMessage(parser.MethodINFO, "sip:user@example.com")
	info.SetHeader(parser.HeaderVia, "SIP/2.0/UDP client.example.com:5060;branch=z9hG4bK123")
	info.SetHeader(parser.HeaderFrom, "Alice <sip:alice@example.com>;tag=abc123")
	info.SetHeader(parser.HeaderTo, "Bob <sip:bob@example.com>;tag=def456")
	info.SetHeader(parser.HeaderCallID, "test-call-id@example.com")
	info.SetHeader(parser.HeaderCSeq, "2 INFO")

	err := handler.HandleRequest(info, mockTxn)
	if err != nil {
		t.Errorf("HandleRequest failed: %v", err)
	}

	if sentResponse == nil {
		t.Fatal("Expected response to be sent")
	}

	if sentResponse.GetStatusCode() != parser.StatusNotFound {
		t.Errorf("Expected status code %d, got %d", parser.StatusNotFound, sentResponse.GetStatusCode())
	}
}

func TestAuxiliaryHandler_HandleInfo_RegistrarError(t *testing.T) {
	mockReg := &mockRegistrar{
		findContactsFunc: func(aor string) ([]*database.RegistrarContact, error) {
			return nil, errors.New("database error")
		},
	}

	var sentResponse *parser.SIPMessage
	mockTxn := &mockTransaction{
		sendResponseFunc: func(response *parser.SIPMessage) error {
			sentResponse = response
			return nil
		},
	}

	handler := NewAuxiliaryHandler(nil, mockReg)

	// Create test INFO request
	info := parser.NewRequestMessage(parser.MethodINFO, "sip:user@example.com")
	info.SetHeader(parser.HeaderVia, "SIP/2.0/UDP client.example.com:5060;branch=z9hG4bK123")
	info.SetHeader(parser.HeaderFrom, "Alice <sip:alice@example.com>;tag=abc123")
	info.SetHeader(parser.HeaderTo, "Bob <sip:bob@example.com>;tag=def456")
	info.SetHeader(parser.HeaderCallID, "test-call-id@example.com")
	info.SetHeader(parser.HeaderCSeq, "2 INFO")

	err := handler.HandleRequest(info, mockTxn)
	if err != nil {
		t.Errorf("HandleRequest failed: %v", err)
	}

	if sentResponse == nil {
		t.Fatal("Expected response to be sent")
	}

	if sentResponse.GetStatusCode() != parser.StatusServerInternalError {
		t.Errorf("Expected status code %d, got %d", parser.StatusServerInternalError, sentResponse.GetStatusCode())
	}
}

func TestAuxiliaryHandler_HandleUnsupportedMethod(t *testing.T) {
	handler := NewAuxiliaryHandler(nil, nil)
	mockTxn := &mockTransaction{}

	// Create test request with unsupported method
	req := parser.NewRequestMessage("UNSUPPORTED", "sip:user@example.com")

	err := handler.HandleRequest(req, mockTxn)
	if err == nil {
		t.Error("Expected error for unsupported method")
	}

	expectedError := "unsupported method: UNSUPPORTED"
	if err.Error() != expectedError {
		t.Errorf("Expected error '%s', got '%s'", expectedError, err.Error())
	}
}

func TestAuxiliaryHandler_ExtractAOR(t *testing.T) {
	handler := NewAuxiliaryHandler(nil, nil)

	tests := []struct {
		uri      string
		expected string
	}{
		{"sip:user@example.com", "user@example.com"},
		{"sips:user@example.com", "user@example.com"},
		{"sip:user@example.com:5060", "user@example.com:5060"},
		{"sip:user@example.com;transport=tcp", "user@example.com"},
		{"sip:user@example.com?header=value", "user@example.com"},
		{"sip:user@example.com;transport=tcp?header=value", "user@example.com"},
		{"user@example.com", "user@example.com"},
	}

	for _, test := range tests {
		t.Run(test.uri, func(t *testing.T) {
			result := handler.extractAOR(test.uri)
			if result != test.expected {
				t.Errorf("Expected '%s', got '%s' for URI '%s'", test.expected, result, test.uri)
			}
		})
	}
}

func TestAuxiliaryHandler_CopyResponseHeaders(t *testing.T) {
	handler := NewAuxiliaryHandler(nil, nil)

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
	handler.copyResponseHeaders(req, resp)

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