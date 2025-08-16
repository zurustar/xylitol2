package handlers

import (
	"testing"

	"github.com/zurustar/xylitol2/internal/database"
	"github.com/zurustar/xylitol2/internal/parser"
	"github.com/zurustar/xylitol2/internal/sessiontimer"
	"github.com/zurustar/xylitol2/internal/transaction"
)

// Mock implementations for testing

type MockProxyEngine struct{}

func (m *MockProxyEngine) ProcessRequest(req *parser.SIPMessage, transaction transaction.Transaction) error {
	return nil
}

func (m *MockProxyEngine) ForwardRequest(req *parser.SIPMessage, targets []*database.RegistrarContact) error {
	return nil
}

func (m *MockProxyEngine) ProcessResponse(resp *parser.SIPMessage, transaction transaction.Transaction) error {
	return nil
}

type MockRegistrar struct {
	contacts map[string][]*database.RegistrarContact
}

func NewMockRegistrar() *MockRegistrar {
	return &MockRegistrar{
		contacts: make(map[string][]*database.RegistrarContact),
	}
}

func (m *MockRegistrar) Register(contact *database.RegistrarContact, expires int) error {
	return nil
}

func (m *MockRegistrar) Unregister(aor string) error {
	delete(m.contacts, aor)
	return nil
}

func (m *MockRegistrar) FindContacts(aor string) ([]*database.RegistrarContact, error) {
	if contacts, exists := m.contacts[aor]; exists {
		return contacts, nil
	}
	return nil, nil
}

func (m *MockRegistrar) CleanupExpired() {}

func (m *MockRegistrar) AddContact(aor string, contact *database.RegistrarContact) {
	m.contacts[aor] = append(m.contacts[aor], contact)
}

type MockSessionTimerManager struct {
	sessions map[string]*sessiontimer.Session
}

func NewMockSessionTimerManager() *MockSessionTimerManager {
	return &MockSessionTimerManager{
		sessions: make(map[string]*sessiontimer.Session),
	}
}

func (m *MockSessionTimerManager) CreateSession(callID string, sessionExpires int) *sessiontimer.Session {
	session := &sessiontimer.Session{
		CallID:    callID,
		Refresher: "uac",
		MinSE:     90,
	}
	m.sessions[callID] = session
	return session
}

func (m *MockSessionTimerManager) RefreshSession(callID string) error {
	return nil
}

func (m *MockSessionTimerManager) CleanupExpiredSessions() {}

func (m *MockSessionTimerManager) IsSessionTimerRequired(msg *parser.SIPMessage) bool {
	// For this test, we'll check if the message has Session-Timer support
	// If it doesn't have Session-Expires header, we consider it as not supporting Session-Timer
	if msg.IsRequest() && msg.GetMethod() == parser.MethodINVITE {
		return msg.GetHeader(parser.HeaderSessionExpires) != ""
	}
	return false
}

func (m *MockSessionTimerManager) StartCleanupTimer() {}

func (m *MockSessionTimerManager) StopCleanupTimer() {}

func (m *MockSessionTimerManager) SetSessionTerminationCallback(callback func(callID string)) {}

func (m *MockSessionTimerManager) RemoveSession(callID string) {
	delete(m.sessions, callID)
}

type MockTransaction struct {
	responses []*parser.SIPMessage
	id        string
}

func NewMockTransaction() *MockTransaction {
	return &MockTransaction{
		responses: make([]*parser.SIPMessage, 0),
		id:        "mock-transaction-id",
	}
}

func (m *MockTransaction) GetState() transaction.TransactionState {
	return transaction.StateTrying
}

func (m *MockTransaction) ProcessMessage(msg *parser.SIPMessage) error {
	return nil
}

func (m *MockTransaction) SendResponse(response *parser.SIPMessage) error {
	m.responses = append(m.responses, response)
	return nil
}

func (m *MockTransaction) GetID() string {
	return m.id
}

func (m *MockTransaction) IsClient() bool {
	return false
}

func (m *MockTransaction) GetLastResponse() *parser.SIPMessage {
	if len(m.responses) == 0 {
		return nil
	}
	return m.responses[len(m.responses)-1]
}

func TestSessionTimerEnforcementInINVITE(t *testing.T) {
	proxyEngine := &MockProxyEngine{}
	registrar := NewMockRegistrar()
	sessionTimerMgr := NewMockSessionTimerManager()
	
	handler := NewSessionHandler(proxyEngine, registrar, sessionTimerMgr)

	// Add a contact for the target user
	contact := &database.RegistrarContact{
		AOR: "user@example.com",
		URI: "sip:user@192.168.1.100:5060",
	}
	registrar.AddContact("user@example.com", contact)

	tests := []struct {
		name           string
		setupRequest   func() *parser.SIPMessage
		expectedStatus int
		description    string
	}{
		{
			name: "INVITE without Session-Expires header",
			setupRequest: func() *parser.SIPMessage {
				req := parser.NewRequestMessage(parser.MethodINVITE, "sip:user@example.com")
				req.SetHeader(parser.HeaderVia, "SIP/2.0/UDP 192.168.1.1:5060;branch=z9hG4bK123")
				req.SetHeader(parser.HeaderFrom, "Alice <sip:alice@example.com>;tag=123")
				req.SetHeader(parser.HeaderTo, "Bob <sip:user@example.com>")
				req.SetHeader(parser.HeaderCallID, "test-call-id-1")
				req.SetHeader(parser.HeaderCSeq, "1 INVITE")
				req.SetHeader(parser.HeaderMaxForwards, "70")
				return req
			},
			expectedStatus: parser.StatusExtensionRequired,
			description:    "Should reject INVITE without Session-Expires header with 421 Extension Required",
		},
		{
			name: "INVITE with valid Session-Expires header",
			setupRequest: func() *parser.SIPMessage {
				req := parser.NewRequestMessage(parser.MethodINVITE, "sip:user@example.com")
				req.SetHeader(parser.HeaderVia, "SIP/2.0/UDP 192.168.1.1:5060;branch=z9hG4bK123")
				req.SetHeader(parser.HeaderFrom, "Alice <sip:alice@example.com>;tag=123")
				req.SetHeader(parser.HeaderTo, "Bob <sip:user@example.com>")
				req.SetHeader(parser.HeaderCallID, "test-call-id-2")
				req.SetHeader(parser.HeaderCSeq, "1 INVITE")
				req.SetHeader(parser.HeaderMaxForwards, "70")
				req.SetHeader(parser.HeaderSessionExpires, "1800")
				return req
			},
			expectedStatus: 0, // Should be forwarded, no response generated
			description:    "Should accept INVITE with valid Session-Expires header",
		},
		{
			name: "INVITE with invalid Session-Expires header",
			setupRequest: func() *parser.SIPMessage {
				req := parser.NewRequestMessage(parser.MethodINVITE, "sip:user@example.com")
				req.SetHeader(parser.HeaderVia, "SIP/2.0/UDP 192.168.1.1:5060;branch=z9hG4bK123")
				req.SetHeader(parser.HeaderFrom, "Alice <sip:alice@example.com>;tag=123")
				req.SetHeader(parser.HeaderTo, "Bob <sip:user@example.com>")
				req.SetHeader(parser.HeaderCallID, "test-call-id-3")
				req.SetHeader(parser.HeaderCSeq, "1 INVITE")
				req.SetHeader(parser.HeaderMaxForwards, "70")
				req.SetHeader(parser.HeaderSessionExpires, "invalid")
				return req
			},
			expectedStatus: parser.StatusBadRequest,
			description:    "Should reject INVITE with invalid Session-Expires header with 400 Bad Request",
		},
		{
			name: "INVITE with Session-Expires below minimum",
			setupRequest: func() *parser.SIPMessage {
				req := parser.NewRequestMessage(parser.MethodINVITE, "sip:user@example.com")
				req.SetHeader(parser.HeaderVia, "SIP/2.0/UDP 192.168.1.1:5060;branch=z9hG4bK123")
				req.SetHeader(parser.HeaderFrom, "Alice <sip:alice@example.com>;tag=123")
				req.SetHeader(parser.HeaderTo, "Bob <sip:user@example.com>")
				req.SetHeader(parser.HeaderCallID, "test-call-id-4")
				req.SetHeader(parser.HeaderCSeq, "1 INVITE")
				req.SetHeader(parser.HeaderMaxForwards, "70")
				req.SetHeader(parser.HeaderSessionExpires, "30") // Below minimum of 90
				req.SetHeader(parser.HeaderMinSE, "90")
				return req
			},
			expectedStatus: parser.StatusIntervalTooBrief,
			description:    "Should reject INVITE with Session-Expires below minimum with 423 Interval Too Brief",
		},
		{
			name: "INVITE to non-existent user",
			setupRequest: func() *parser.SIPMessage {
				req := parser.NewRequestMessage(parser.MethodINVITE, "sip:nonexistent@example.com")
				req.SetHeader(parser.HeaderVia, "SIP/2.0/UDP 192.168.1.1:5060;branch=z9hG4bK123")
				req.SetHeader(parser.HeaderFrom, "Alice <sip:alice@example.com>;tag=123")
				req.SetHeader(parser.HeaderTo, "Bob <sip:nonexistent@example.com>")
				req.SetHeader(parser.HeaderCallID, "test-call-id-5")
				req.SetHeader(parser.HeaderCSeq, "1 INVITE")
				req.SetHeader(parser.HeaderMaxForwards, "70")
				req.SetHeader(parser.HeaderSessionExpires, "1800")
				return req
			},
			expectedStatus: parser.StatusNotFound,
			description:    "Should reject INVITE to non-existent user with 404 Not Found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := tt.setupRequest()
			txn := NewMockTransaction()

			err := handler.HandleRequest(req, txn)

			if tt.expectedStatus == 0 {
				// Should be forwarded without error
				if err != nil {
					t.Errorf("Expected no error for %s, got: %v", tt.description, err)
				}
				if len(txn.responses) != 0 {
					t.Errorf("Expected no response for %s, got %d responses", tt.description, len(txn.responses))
				}
			} else {
				// Should generate a response
				if err != nil {
					t.Errorf("Unexpected error for %s: %v", tt.description, err)
				}
				if len(txn.responses) != 1 {
					t.Errorf("Expected 1 response for %s, got %d", tt.description, len(txn.responses))
					return
				}

				response := txn.GetLastResponse()
				if response.GetStatusCode() != tt.expectedStatus {
					t.Errorf("Expected status %d for %s, got %d", 
						tt.expectedStatus, tt.description, response.GetStatusCode())
				}

				// Verify required headers are present in error responses
				if response.GetHeader(parser.HeaderVia) == "" {
					t.Errorf("Missing Via header in response for %s", tt.description)
				}
				if response.GetHeader(parser.HeaderFrom) == "" {
					t.Errorf("Missing From header in response for %s", tt.description)
				}
				if response.GetHeader(parser.HeaderTo) == "" {
					t.Errorf("Missing To header in response for %s", tt.description)
				}
				if response.GetHeader(parser.HeaderCallID) == "" {
					t.Errorf("Missing Call-ID header in response for %s", tt.description)
				}
				if response.GetHeader(parser.HeaderCSeq) == "" {
					t.Errorf("Missing CSeq header in response for %s", tt.description)
				}

				// Check specific headers for 421 Extension Required
				if tt.expectedStatus == parser.StatusExtensionRequired {
					if response.GetHeader(parser.HeaderRequire) != "timer" {
						t.Errorf("Expected Require: timer header for 421 response, got: %s", 
							response.GetHeader(parser.HeaderRequire))
					}
					if response.GetHeader(parser.HeaderSupported) != "timer" {
						t.Errorf("Expected Supported: timer header for 421 response, got: %s", 
							response.GetHeader(parser.HeaderSupported))
					}
				}

				// Check Min-SE header for 423 Interval Too Brief
				if tt.expectedStatus == parser.StatusIntervalTooBrief {
					if response.GetHeader(parser.HeaderMinSE) == "" {
						t.Errorf("Expected Min-SE header for 423 response")
					}
				}
			}
		})
	}
}

func TestBYESessionCleanup(t *testing.T) {
	proxyEngine := &MockProxyEngine{}
	registrar := NewMockRegistrar()
	sessionTimerMgr := NewMockSessionTimerManager()
	
	handler := NewSessionHandler(proxyEngine, registrar, sessionTimerMgr)

	// Add a contact for the target user
	contact := &database.RegistrarContact{
		AOR: "user@example.com",
		URI: "sip:user@192.168.1.100:5060",
	}
	registrar.AddContact("user@example.com", contact)

	// Create a session first
	callID := "test-bye-cleanup"
	sessionTimerMgr.CreateSession(callID, 1800)

	// Verify session exists
	if sessionTimerMgr.sessions[callID] == nil {
		t.Fatal("Session should exist before BYE")
	}

	// Create BYE request
	req := parser.NewRequestMessage(parser.MethodBYE, "sip:user@example.com")
	req.SetHeader(parser.HeaderVia, "SIP/2.0/UDP 192.168.1.1:5060;branch=z9hG4bK123")
	req.SetHeader(parser.HeaderFrom, "Alice <sip:alice@example.com>;tag=123")
	req.SetHeader(parser.HeaderTo, "Bob <sip:user@example.com>;tag=456")
	req.SetHeader(parser.HeaderCallID, callID)
	req.SetHeader(parser.HeaderCSeq, "2 BYE")
	req.SetHeader(parser.HeaderMaxForwards, "70")

	txn := NewMockTransaction()

	err := handler.HandleRequest(req, txn)
	if err != nil {
		t.Errorf("Unexpected error handling BYE: %v", err)
	}

	// Verify session was removed
	if sessionTimerMgr.sessions[callID] != nil {
		t.Error("Session should have been removed after BYE")
	}
}

func TestACKHandling(t *testing.T) {
	proxyEngine := &MockProxyEngine{}
	registrar := NewMockRegistrar()
	sessionTimerMgr := NewMockSessionTimerManager()
	
	handler := NewSessionHandler(proxyEngine, registrar, sessionTimerMgr)

	// Add a contact for the target user
	contact := &database.RegistrarContact{
		AOR: "user@example.com",
		URI: "sip:user@192.168.1.100:5060",
	}
	registrar.AddContact("user@example.com", contact)

	// Create ACK request
	req := parser.NewRequestMessage(parser.MethodACK, "sip:user@example.com")
	req.SetHeader(parser.HeaderVia, "SIP/2.0/UDP 192.168.1.1:5060;branch=z9hG4bK123")
	req.SetHeader(parser.HeaderFrom, "Alice <sip:alice@example.com>;tag=123")
	req.SetHeader(parser.HeaderTo, "Bob <sip:user@example.com>;tag=456")
	req.SetHeader(parser.HeaderCallID, "test-ack-call-id")
	req.SetHeader(parser.HeaderCSeq, "1 ACK")
	req.SetHeader(parser.HeaderMaxForwards, "70")

	txn := NewMockTransaction()

	err := handler.HandleRequest(req, txn)
	if err != nil {
		t.Errorf("Unexpected error handling ACK: %v", err)
	}

	// ACK should be forwarded without generating responses
	if len(txn.responses) != 0 {
		t.Errorf("Expected no responses for ACK, got %d", len(txn.responses))
	}
}

func TestCanHandle(t *testing.T) {
	proxyEngine := &MockProxyEngine{}
	registrar := NewMockRegistrar()
	sessionTimerMgr := NewMockSessionTimerManager()
	
	handler := NewSessionHandler(proxyEngine, registrar, sessionTimerMgr)

	tests := []struct {
		method   string
		expected bool
	}{
		{parser.MethodINVITE, true},
		{parser.MethodACK, true},
		{parser.MethodBYE, true},
		{parser.MethodREGISTER, false},
		{parser.MethodOPTIONS, false},
		{parser.MethodCANCEL, false},
		{parser.MethodINFO, false},
	}

	for _, tt := range tests {
		t.Run(tt.method, func(t *testing.T) {
			result := handler.CanHandle(tt.method)
			if result != tt.expected {
				t.Errorf("Expected CanHandle(%s) to return %v, got %v", 
					tt.method, tt.expected, result)
			}
		})
	}
}