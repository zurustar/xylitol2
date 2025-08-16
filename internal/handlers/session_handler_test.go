package handlers

import (
	"errors"
	"testing"
	"time"

	"github.com/zurustar/xylitol2/internal/database"
	"github.com/zurustar/xylitol2/internal/parser"
	"github.com/zurustar/xylitol2/internal/sessiontimer"
	"github.com/zurustar/xylitol2/internal/transaction"
)

// Mock implementations for testing

type mockProxyEngine struct {
	forwardRequestFunc func(req *parser.SIPMessage, targets []*database.RegistrarContact) error
}

func (m *mockProxyEngine) ProcessRequest(req *parser.SIPMessage, txn transaction.Transaction) error {
	return nil
}

func (m *mockProxyEngine) ForwardRequest(req *parser.SIPMessage, targets []*database.RegistrarContact) error {
	if m.forwardRequestFunc != nil {
		return m.forwardRequestFunc(req, targets)
	}
	return nil
}

func (m *mockProxyEngine) ProcessResponse(resp *parser.SIPMessage, txn transaction.Transaction) error {
	return nil
}

type mockRegistrar struct {
	findContactsFunc func(aor string) ([]*database.RegistrarContact, error)
}

func (m *mockRegistrar) Register(contact *database.RegistrarContact, expires int) error {
	return nil
}

func (m *mockRegistrar) Unregister(aor string) error {
	return nil
}

func (m *mockRegistrar) FindContacts(aor string) ([]*database.RegistrarContact, error) {
	if m.findContactsFunc != nil {
		return m.findContactsFunc(aor)
	}
	return []*database.RegistrarContact{}, nil
}

func (m *mockRegistrar) CleanupExpired() {}

type mockSessionTimerManager struct {
	isSessionTimerRequiredFunc func(msg *parser.SIPMessage) bool
	createSessionFunc          func(callID string, sessionExpires int) *sessiontimer.Session
}

func (m *mockSessionTimerManager) CreateSession(callID string, sessionExpires int) *sessiontimer.Session {
	if m.createSessionFunc != nil {
		return m.createSessionFunc(callID, sessionExpires)
	}
	return &sessiontimer.Session{
		CallID:         callID,
		SessionExpires: time.Now().Add(time.Duration(sessionExpires) * time.Second),
		Refresher:      "uac",
		MinSE:          90,
	}
}

func (m *mockSessionTimerManager) RefreshSession(callID string) error {
	return nil
}

func (m *mockSessionTimerManager) CleanupExpiredSessions() {}

func (m *mockSessionTimerManager) IsSessionTimerRequired(msg *parser.SIPMessage) bool {
	if m.isSessionTimerRequiredFunc != nil {
		return m.isSessionTimerRequiredFunc(msg)
	}
	return true
}

func (m *mockSessionTimerManager) StartCleanupTimer() {}

func (m *mockSessionTimerManager) StopCleanupTimer() {}

func (m *mockSessionTimerManager) SetSessionTerminationCallback(callback func(callID string)) {}

func (m *mockSessionTimerManager) RemoveSession(callID string) {}

type mockTransaction struct {
	sendResponseFunc func(response *parser.SIPMessage) error
}

func (m *mockTransaction) GetState() transaction.TransactionState {
	return transaction.StateTrying
}

func (m *mockTransaction) ProcessMessage(msg *parser.SIPMessage) error {
	return nil
}

func (m *mockTransaction) SendResponse(response *parser.SIPMessage) error {
	if m.sendResponseFunc != nil {
		return m.sendResponseFunc(response)
	}
	return nil
}

func (m *mockTransaction) GetID() string {
	return "test-transaction-id"
}

func (m *mockTransaction) IsClient() bool {
	return false
}

func TestSessionHandler_CanHandle(t *testing.T) {
	mockSessionTimer := &mockSessionTimerManager{}
	handler := NewSessionHandler(nil, nil, mockSessionTimer)

	tests := []struct {
		method   string
		expected bool
	}{
		{parser.MethodINVITE, true},
		{parser.MethodACK, true},
		{parser.MethodBYE, true},
		{parser.MethodREGISTER, false},
		{parser.MethodOPTIONS, false},
		{parser.MethodINFO, false},
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

func TestSessionHandler_HandleInvite_Success(t *testing.T) {
	// Setup mocks
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

	mockSessionTimer := &mockSessionTimerManager{
		isSessionTimerRequiredFunc: func(msg *parser.SIPMessage) bool {
			return true
		},
		createSessionFunc: func(callID string, sessionExpires int) *sessiontimer.Session {
			return &sessiontimer.Session{
				CallID:         callID,
				SessionExpires: time.Now().Add(time.Duration(sessionExpires) * time.Second),
				Refresher:      "uac",
				MinSE:          90,
			}
		},
	}

	mockTxn := &mockTransaction{}

	handler := NewSessionHandler(mockProxy, mockReg, mockSessionTimer)

	// Create test INVITE request
	invite := parser.NewRequestMessage(parser.MethodINVITE, "sip:user@example.com")
	invite.SetHeader(parser.HeaderVia, "SIP/2.0/UDP client.example.com:5060;branch=z9hG4bK123")
	invite.SetHeader(parser.HeaderFrom, "Alice <sip:alice@example.com>;tag=abc123")
	invite.SetHeader(parser.HeaderTo, "Bob <sip:bob@example.com>")
	invite.SetHeader(parser.HeaderCallID, "test-call-id@example.com")
	invite.SetHeader(parser.HeaderCSeq, "1 INVITE")
	invite.SetHeader(parser.HeaderSessionExpires, "1800")
	invite.SetHeader(parser.HeaderMinSE, "90")

	err := handler.HandleRequest(invite, mockTxn)
	if err != nil {
		t.Errorf("HandleRequest failed: %v", err)
	}
}

func TestSessionHandler_HandleInvite_MissingSessionTimer(t *testing.T) {
	mockSessionTimer := &mockSessionTimerManager{
		isSessionTimerRequiredFunc: func(msg *parser.SIPMessage) bool {
			return false // Session timer not supported
		},
	}

	var sentResponse *parser.SIPMessage
	mockTxn := &mockTransaction{
		sendResponseFunc: func(response *parser.SIPMessage) error {
			sentResponse = response
			return nil
		},
	}

	handler := NewSessionHandler(nil, nil, mockSessionTimer)

	// Create test INVITE request without Session-Timer support
	invite := parser.NewRequestMessage(parser.MethodINVITE, "sip:user@example.com")
	invite.SetHeader(parser.HeaderVia, "SIP/2.0/UDP client.example.com:5060;branch=z9hG4bK123")
	invite.SetHeader(parser.HeaderFrom, "Alice <sip:alice@example.com>;tag=abc123")
	invite.SetHeader(parser.HeaderTo, "Bob <sip:bob@example.com>")
	invite.SetHeader(parser.HeaderCallID, "test-call-id@example.com")
	invite.SetHeader(parser.HeaderCSeq, "1 INVITE")

	err := handler.HandleRequest(invite, mockTxn)
	if err != nil {
		t.Errorf("HandleRequest failed: %v", err)
	}

	if sentResponse == nil {
		t.Fatal("Expected response to be sent")
	}

	if sentResponse.GetStatusCode() != parser.StatusExtensionRequired {
		t.Errorf("Expected status code %d, got %d", parser.StatusExtensionRequired, sentResponse.GetStatusCode())
	}

	if sentResponse.GetHeader(parser.HeaderRequire) != "timer" {
		t.Errorf("Expected Require header to be 'timer', got '%s'", sentResponse.GetHeader(parser.HeaderRequire))
	}
}

func TestSessionHandler_HandleInvite_MissingSessionExpires(t *testing.T) {
	mockSessionTimer := &mockSessionTimerManager{
		isSessionTimerRequiredFunc: func(msg *parser.SIPMessage) bool {
			return true
		},
	}

	var sentResponse *parser.SIPMessage
	mockTxn := &mockTransaction{
		sendResponseFunc: func(response *parser.SIPMessage) error {
			sentResponse = response
			return nil
		},
	}

	handler := NewSessionHandler(nil, nil, mockSessionTimer)

	// Create test INVITE request without Session-Expires header
	invite := parser.NewRequestMessage(parser.MethodINVITE, "sip:user@example.com")
	invite.SetHeader(parser.HeaderVia, "SIP/2.0/UDP client.example.com:5060;branch=z9hG4bK123")
	invite.SetHeader(parser.HeaderFrom, "Alice <sip:alice@example.com>;tag=abc123")
	invite.SetHeader(parser.HeaderTo, "Bob <sip:bob@example.com>")
	invite.SetHeader(parser.HeaderCallID, "test-call-id@example.com")
	invite.SetHeader(parser.HeaderCSeq, "1 INVITE")

	err := handler.HandleRequest(invite, mockTxn)
	if err != nil {
		t.Errorf("HandleRequest failed: %v", err)
	}

	if sentResponse == nil {
		t.Fatal("Expected response to be sent")
	}

	if sentResponse.GetStatusCode() != parser.StatusBadRequest {
		t.Errorf("Expected status code %d, got %d", parser.StatusBadRequest, sentResponse.GetStatusCode())
	}
}

func TestSessionHandler_HandleInvite_IntervalTooBrief(t *testing.T) {
	mockSessionTimer := &mockSessionTimerManager{
		isSessionTimerRequiredFunc: func(msg *parser.SIPMessage) bool {
			return true
		},
	}

	var sentResponse *parser.SIPMessage
	mockTxn := &mockTransaction{
		sendResponseFunc: func(response *parser.SIPMessage) error {
			sentResponse = response
			return nil
		},
	}

	handler := NewSessionHandler(nil, nil, mockSessionTimer)

	// Create test INVITE request with too short Session-Expires
	invite := parser.NewRequestMessage(parser.MethodINVITE, "sip:user@example.com")
	invite.SetHeader(parser.HeaderVia, "SIP/2.0/UDP client.example.com:5060;branch=z9hG4bK123")
	invite.SetHeader(parser.HeaderFrom, "Alice <sip:alice@example.com>;tag=abc123")
	invite.SetHeader(parser.HeaderTo, "Bob <sip:bob@example.com>")
	invite.SetHeader(parser.HeaderCallID, "test-call-id@example.com")
	invite.SetHeader(parser.HeaderCSeq, "1 INVITE")
	invite.SetHeader(parser.HeaderSessionExpires, "30") // Too short
	invite.SetHeader(parser.HeaderMinSE, "90")

	err := handler.HandleRequest(invite, mockTxn)
	if err != nil {
		t.Errorf("HandleRequest failed: %v", err)
	}

	if sentResponse == nil {
		t.Fatal("Expected response to be sent")
	}

	if sentResponse.GetStatusCode() != parser.StatusIntervalTooBrief {
		t.Errorf("Expected status code %d, got %d", parser.StatusIntervalTooBrief, sentResponse.GetStatusCode())
	}

	if sentResponse.GetHeader(parser.HeaderMinSE) != "90" {
		t.Errorf("Expected Min-SE header to be '90', got '%s'", sentResponse.GetHeader(parser.HeaderMinSE))
	}
}

func TestSessionHandler_HandleInvite_UserNotFound(t *testing.T) {
	mockProxy := &mockProxyEngine{}

	mockReg := &mockRegistrar{
		findContactsFunc: func(aor string) ([]*database.RegistrarContact, error) {
			return []*database.RegistrarContact{}, nil // No contacts found
		},
	}

	mockSessionTimer := &mockSessionTimerManager{
		isSessionTimerRequiredFunc: func(msg *parser.SIPMessage) bool {
			return true
		},
	}

	var sentResponse *parser.SIPMessage
	mockTxn := &mockTransaction{
		sendResponseFunc: func(response *parser.SIPMessage) error {
			sentResponse = response
			return nil
		},
	}

	handler := NewSessionHandler(mockProxy, mockReg, mockSessionTimer)

	// Create test INVITE request
	invite := parser.NewRequestMessage(parser.MethodINVITE, "sip:user@example.com")
	invite.SetHeader(parser.HeaderVia, "SIP/2.0/UDP client.example.com:5060;branch=z9hG4bK123")
	invite.SetHeader(parser.HeaderFrom, "Alice <sip:alice@example.com>;tag=abc123")
	invite.SetHeader(parser.HeaderTo, "Bob <sip:bob@example.com>")
	invite.SetHeader(parser.HeaderCallID, "test-call-id@example.com")
	invite.SetHeader(parser.HeaderCSeq, "1 INVITE")
	invite.SetHeader(parser.HeaderSessionExpires, "1800")

	err := handler.HandleRequest(invite, mockTxn)
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

func TestSessionHandler_HandleAck_Success(t *testing.T) {
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

	mockSessionTimer := &mockSessionTimerManager{}
	handler := NewSessionHandler(mockProxy, mockReg, mockSessionTimer)

	// Create test ACK request
	ack := parser.NewRequestMessage(parser.MethodACK, "sip:user@example.com")
	ack.SetHeader(parser.HeaderVia, "SIP/2.0/UDP client.example.com:5060;branch=z9hG4bK123")
	ack.SetHeader(parser.HeaderFrom, "Alice <sip:alice@example.com>;tag=abc123")
	ack.SetHeader(parser.HeaderTo, "Bob <sip:bob@example.com>;tag=def456")
	ack.SetHeader(parser.HeaderCallID, "test-call-id@example.com")
	ack.SetHeader(parser.HeaderCSeq, "1 ACK")

	err := handler.HandleRequest(ack, mockTxn)
	if err != nil {
		t.Errorf("HandleRequest failed: %v", err)
	}
}

func TestSessionHandler_HandleBye_Success(t *testing.T) {
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

	mockSessionTimer := &mockSessionTimerManager{}
	handler := NewSessionHandler(mockProxy, mockReg, mockSessionTimer)

	// Create test BYE request
	bye := parser.NewRequestMessage(parser.MethodBYE, "sip:user@example.com")
	bye.SetHeader(parser.HeaderVia, "SIP/2.0/UDP client.example.com:5060;branch=z9hG4bK123")
	bye.SetHeader(parser.HeaderFrom, "Alice <sip:alice@example.com>;tag=abc123")
	bye.SetHeader(parser.HeaderTo, "Bob <sip:bob@example.com>;tag=def456")
	bye.SetHeader(parser.HeaderCallID, "test-call-id@example.com")
	bye.SetHeader(parser.HeaderCSeq, "2 BYE")

	err := handler.HandleRequest(bye, mockTxn)
	if err != nil {
		t.Errorf("HandleRequest failed: %v", err)
	}
}

func TestSessionHandler_HandleBye_UserNotRegistered(t *testing.T) {
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

	mockSessionTimer := &mockSessionTimerManager{}
	handler := NewSessionHandler(nil, mockReg, mockSessionTimer)

	// Create test BYE request
	bye := parser.NewRequestMessage(parser.MethodBYE, "sip:user@example.com")
	bye.SetHeader(parser.HeaderVia, "SIP/2.0/UDP client.example.com:5060;branch=z9hG4bK123")
	bye.SetHeader(parser.HeaderFrom, "Alice <sip:alice@example.com>;tag=abc123")
	bye.SetHeader(parser.HeaderTo, "Bob <sip:bob@example.com>;tag=def456")
	bye.SetHeader(parser.HeaderCallID, "test-call-id@example.com")
	bye.SetHeader(parser.HeaderCSeq, "2 BYE")

	err := handler.HandleRequest(bye, mockTxn)
	if err != nil {
		t.Errorf("HandleRequest failed: %v", err)
	}

	if sentResponse == nil {
		t.Fatal("Expected response to be sent")
	}

	if sentResponse.GetStatusCode() != parser.StatusOK {
		t.Errorf("Expected status code %d, got %d", parser.StatusOK, sentResponse.GetStatusCode())
	}
}

func TestSessionHandler_ParseSessionExpires(t *testing.T) {
	mockSessionTimer := &mockSessionTimerManager{}
	handler := NewSessionHandler(nil, nil, mockSessionTimer)

	tests := []struct {
		header      string
		expected    int
		expectError bool
	}{
		{"1800", 1800, false},
		{"1800;refresher=uac", 1800, false},
		{"3600;refresher=uas", 3600, false},
		{"", 0, true},
		{"invalid", 0, true},
		{"-100", 0, true},
		{"0", 0, true},
	}

	for _, test := range tests {
		t.Run(test.header, func(t *testing.T) {
			result, err := handler.parseSessionExpires(test.header)
			if test.expectError {
				if err == nil {
					t.Errorf("Expected error for header '%s', but got none", test.header)
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error for header '%s': %v", test.header, err)
				}
				if result != test.expected {
					t.Errorf("Expected %d, got %d for header '%s'", test.expected, result, test.header)
				}
			}
		})
	}
}

func TestSessionHandler_ExtractAOR(t *testing.T) {
	mockSessionTimer := &mockSessionTimerManager{}
	handler := NewSessionHandler(nil, nil, mockSessionTimer)

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

func TestSessionHandler_RegistrarError(t *testing.T) {
	mockReg := &mockRegistrar{
		findContactsFunc: func(aor string) ([]*database.RegistrarContact, error) {
			return nil, errors.New("database error")
		},
	}

	mockSessionTimer := &mockSessionTimerManager{
		isSessionTimerRequiredFunc: func(msg *parser.SIPMessage) bool {
			return true
		},
	}

	var sentResponse *parser.SIPMessage
	mockTxn := &mockTransaction{
		sendResponseFunc: func(response *parser.SIPMessage) error {
			sentResponse = response
			return nil
		},
	}

	handler := NewSessionHandler(nil, mockReg, mockSessionTimer)

	// Create test INVITE request
	invite := parser.NewRequestMessage(parser.MethodINVITE, "sip:user@example.com")
	invite.SetHeader(parser.HeaderVia, "SIP/2.0/UDP client.example.com:5060;branch=z9hG4bK123")
	invite.SetHeader(parser.HeaderFrom, "Alice <sip:alice@example.com>;tag=abc123")
	invite.SetHeader(parser.HeaderTo, "Bob <sip:bob@example.com>")
	invite.SetHeader(parser.HeaderCallID, "test-call-id@example.com")
	invite.SetHeader(parser.HeaderCSeq, "1 INVITE")
	invite.SetHeader(parser.HeaderSessionExpires, "1800")

	err := handler.HandleRequest(invite, mockTxn)
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