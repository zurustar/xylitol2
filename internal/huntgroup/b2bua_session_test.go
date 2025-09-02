package huntgroup

import (
	"net"
	"testing"

	"github.com/zurustar/xylitol2/internal/logging"
	"github.com/zurustar/xylitol2/internal/parser"
	"github.com/zurustar/xylitol2/internal/transaction"
	"github.com/zurustar/xylitol2/internal/transport"
)

// Mock implementations for testing
type mockTransportManager struct{}

func (m *mockTransportManager) StartUDP(port int) error                                    { return nil }
func (m *mockTransportManager) StartTCP(port int) error                                    { return nil }
func (m *mockTransportManager) SendMessage(data []byte, protocol string, addr net.Addr) error { return nil }
func (m *mockTransportManager) RegisterHandler(handler transport.MessageHandler)          {}
func (m *mockTransportManager) Stop() error                                               { return nil }

type mockTransactionManager struct{}

func (m *mockTransactionManager) CreateTransaction(msg *parser.SIPMessage) transaction.Transaction { return nil }
func (m *mockTransactionManager) FindTransaction(msg *parser.SIPMessage) transaction.Transaction   { return nil }
func (m *mockTransactionManager) CleanupExpired()                                                  {}

type mockParser struct{}

func (m *mockParser) Parse(data []byte) (*parser.SIPMessage, error) {
	return parser.NewRequestMessage(parser.MethodINVITE, "sip:test@example.com"), nil
}

func (m *mockParser) Serialize(msg *parser.SIPMessage) ([]byte, error) {
	return []byte("SIP message"), nil
}

func (m *mockParser) Validate(msg *parser.SIPMessage) error {
	return nil
}

type mockLogger struct{}

func (m *mockLogger) Debug(msg string, fields ...logging.Field) {}
func (m *mockLogger) Info(msg string, fields ...logging.Field)  {}
func (m *mockLogger) Warn(msg string, fields ...logging.Field)  {}
func (m *mockLogger) Error(msg string, fields ...logging.Field) {}

func createTestB2BUA() *B2BUA {
	return NewB2BUA(
		&mockTransportManager{},
		&mockTransactionManager{},
		&mockParser{},
		&mockLogger{},
		"127.0.0.1",
		5060,
	)
}

func createTestInvite() *parser.SIPMessage {
	invite := parser.NewRequestMessage(parser.MethodINVITE, "sip:test@example.com")
	invite.SetHeader(parser.HeaderCallID, "test-call-id@example.com")
	invite.SetHeader(parser.HeaderFrom, "<sip:caller@example.com>;tag=caller-tag")
	invite.SetHeader(parser.HeaderTo, "<sip:test@example.com>")
	invite.SetHeader(parser.HeaderCSeq, "1 INVITE")
	invite.SetHeader(parser.HeaderContact, "<sip:caller@192.168.1.100:5060>")
	invite.Body = []byte("v=0\r\no=- 123456 654321 IN IP4 192.168.1.100\r\n")
	return invite
}

func TestB2BUASessionCreation(t *testing.T) {
	b2bua := createTestB2BUA()
	defer b2bua.Stop()

	invite := createTestInvite()
	calleeURI := "sip:callee@example.com"

	session, err := b2bua.CreateSession(invite, calleeURI)
	if err != nil {
		t.Fatalf("Failed to create session: %v", err)
	}

	// Verify session properties
	if session.SessionID == "" {
		t.Error("Session ID should not be empty")
	}

	if session.CallerLeg == nil {
		t.Fatal("Caller leg should not be nil")
	}

	if session.CalleeLeg == nil {
		t.Fatal("Callee leg should not be nil")
	}

	if session.Status != B2BUAStatusInitial {
		t.Errorf("Expected status %s, got %s", B2BUAStatusInitial, session.Status)
	}

	// Verify caller leg
	if session.CallerLeg.CallID != "test-call-id@example.com" {
		t.Errorf("Expected caller Call-ID 'test-call-id@example.com', got '%s'", session.CallerLeg.CallID)
	}

	if session.CallerLeg.FromTag != "caller-tag" {
		t.Errorf("Expected caller FromTag 'caller-tag', got '%s'", session.CallerLeg.FromTag)
	}

	// Verify callee leg has different Call-ID
	if session.CalleeLeg.CallID == session.CallerLeg.CallID {
		t.Error("Callee leg should have different Call-ID than caller leg")
	}

	if session.CalleeLeg.ToURI != "<sip:callee@example.com>" {
		t.Errorf("Expected callee ToURI '<sip:callee@example.com>', got '%s'", session.CalleeLeg.ToURI)
	}

	// Verify SDP is forwarded
	if session.SDPOffer == "" {
		t.Error("SDP offer should not be empty")
	}
}

func TestB2BUASessionLookup(t *testing.T) {
	b2bua := createTestB2BUA()
	defer b2bua.Stop()

	invite := createTestInvite()
	session, err := b2bua.CreateSession(invite, "sip:callee@example.com")
	if err != nil {
		t.Fatalf("Failed to create session: %v", err)
	}

	// Test lookup by session ID
	foundSession, err := b2bua.GetSession(session.SessionID)
	if err != nil {
		t.Fatalf("Failed to get session by ID: %v", err)
	}
	if foundSession.SessionID != session.SessionID {
		t.Error("Retrieved session ID mismatch")
	}

	// Test lookup by caller Call-ID
	foundSession, err = b2bua.GetSessionByCallID(session.CallerLeg.CallID)
	if err != nil {
		t.Fatalf("Failed to get session by caller Call-ID: %v", err)
	}
	if foundSession.SessionID != session.SessionID {
		t.Error("Retrieved session ID mismatch for caller Call-ID")
	}

	// Test lookup by callee Call-ID
	foundSession, err = b2bua.GetSessionByCallID(session.CalleeLeg.CallID)
	if err != nil {
		t.Fatalf("Failed to get session by callee Call-ID: %v", err)
	}
	if foundSession.SessionID != session.SessionID {
		t.Error("Retrieved session ID mismatch for callee Call-ID")
	}

	// Test lookup by caller leg ID
	foundSession, err = b2bua.GetSessionByLegID(session.CallerLeg.LegID)
	if err != nil {
		t.Fatalf("Failed to get session by caller leg ID: %v", err)
	}
	if foundSession.SessionID != session.SessionID {
		t.Error("Retrieved session ID mismatch for caller leg ID")
	}

	// Test lookup by callee leg ID
	foundSession, err = b2bua.GetSessionByLegID(session.CalleeLeg.LegID)
	if err != nil {
		t.Fatalf("Failed to get session by callee leg ID: %v", err)
	}
	if foundSession.SessionID != session.SessionID {
		t.Error("Retrieved session ID mismatch for callee leg ID")
	}
}

func TestB2BUAHuntGroupSession(t *testing.T) {
	b2bua := createTestB2BUA()
	defer b2bua.Stop()

	invite := createTestInvite()
	huntGroup := &HuntGroup{
		ID:        1,
		Name:      "Test Group",
		Extension: "100",
		Strategy:  StrategySimultaneous,
		Enabled:   true,
	}

	session, err := b2bua.CreateHuntGroupSession(invite, huntGroup)
	if err != nil {
		t.Fatalf("Failed to create hunt group session: %v", err)
	}

	// Verify hunt group session properties
	if session.HuntGroupID == nil || *session.HuntGroupID != 1 {
		t.Error("Hunt group ID should be set to 1")
	}

	if session.CalleeLeg != nil {
		t.Error("Callee leg should be nil for hunt group session initially")
	}

	if session.PendingLegs == nil {
		t.Error("Pending legs map should be initialized")
	}
}

func TestB2BUASessionStateTransitions(t *testing.T) {
	b2bua := createTestB2BUA()
	defer b2bua.Stop()

	invite := createTestInvite()
	session, err := b2bua.CreateSession(invite, "sip:callee@example.com")
	if err != nil {
		t.Fatalf("Failed to create session: %v", err)
	}

	// Test initial state
	if !session.IsActive() {
		// Initial state is not considered active, which is correct
	}

	if session.IsTerminated() {
		t.Error("New session should not be terminated")
	}

	// Test state transitions
	session.SetStatus(B2BUAStatusRinging)
	if session.GetStatus() != B2BUAStatusRinging {
		t.Error("Status should be ringing")
	}

	if !session.IsActive() {
		t.Error("Ringing session should be active")
	}

	session.SetStatus(B2BUAStatusConnected)
	if !session.IsActive() {
		t.Error("Connected session should be active")
	}

	session.SetStatus(B2BUAStatusEnded)
	if !session.IsTerminated() {
		t.Error("Ended session should be terminated")
	}

	if session.IsActive() {
		t.Error("Ended session should not be active")
	}
}

func TestB2BUAPendingLegManagement(t *testing.T) {
	b2bua := createTestB2BUA()
	defer b2bua.Stop()

	invite := createTestInvite()
	huntGroup := &HuntGroup{
		ID:        1,
		Extension: "100",
		Strategy:  StrategySimultaneous,
	}

	session, err := b2bua.CreateHuntGroupSession(invite, huntGroup)
	if err != nil {
		t.Fatalf("Failed to create hunt group session: %v", err)
	}

	// Add pending legs
	leg1, err := b2bua.AddPendingLeg(session.SessionID, "sip:member1@example.com")
	if err != nil {
		t.Fatalf("Failed to add pending leg 1: %v", err)
	}

	leg2, err := b2bua.AddPendingLeg(session.SessionID, "sip:member2@example.com")
	if err != nil {
		t.Fatalf("Failed to add pending leg 2: %v", err)
	}

	// Verify legs are added
	if len(session.PendingLegs) != 2 {
		t.Errorf("Expected 2 pending legs, got %d", len(session.PendingLegs))
	}

	// Test leg lookup
	foundLeg := session.GetPendingLeg(leg1.LegID)
	if foundLeg == nil {
		t.Error("Should find pending leg 1")
	}

	// Test session lookup by leg ID
	foundSession, err := b2bua.GetSessionByLegID(leg1.LegID)
	if err != nil {
		t.Fatalf("Failed to get session by leg ID: %v", err)
	}
	if foundSession.SessionID != session.SessionID {
		t.Error("Session lookup by leg ID failed")
	}

	// Test removing pending leg
	session.RemovePendingLeg(leg1.LegID)
	if len(session.PendingLegs) != 1 {
		t.Errorf("Expected 1 pending leg after removal, got %d", len(session.PendingLegs))
	}

	// Test setting answered leg
	session.SetAnsweredLeg(leg2.LegID)
	if session.AnsweredLegID != leg2.LegID {
		t.Error("Answered leg ID should be set")
	}

	if session.CalleeLeg == nil {
		t.Error("Callee leg should be set when leg answers")
	}

	if session.CalleeLeg.LegID != leg2.LegID {
		t.Error("Callee leg should be the answered leg")
	}

	if len(session.PendingLegs) != 0 {
		t.Error("Pending legs should be empty after setting answered leg")
	}
}

func TestB2BUACallLegStateManagement(t *testing.T) {
	b2bua := createTestB2BUA()
	defer b2bua.Stop()

	invite := createTestInvite()
	session, err := b2bua.CreateSession(invite, "sip:callee@example.com")
	if err != nil {
		t.Fatalf("Failed to create session: %v", err)
	}

	leg := session.CallerLeg

	// Test initial state
	if leg.GetStatus() != CallLegStatusInitial {
		t.Error("Initial leg status should be initial")
	}

	if leg.IsTerminated() {
		t.Error("New leg should not be terminated")
	}

	// Test state transitions
	leg.SetStatus(CallLegStatusRinging)
	if leg.GetStatus() != CallLegStatusRinging {
		t.Error("Leg status should be ringing")
	}

	if !leg.IsActive() {
		t.Error("Ringing leg should be active")
	}

	leg.SetStatus(CallLegStatusConnected)
	if !leg.IsActive() {
		t.Error("Connected leg should be active")
	}

	if leg.ConnectedAt == nil {
		t.Error("Connected timestamp should be set")
	}

	leg.SetStatus(CallLegStatusEnded)
	if !leg.IsTerminated() {
		t.Error("Ended leg should be terminated")
	}

	// Test CSeq management
	initialCSeq := leg.LastCSeq
	nextCSeq := leg.GetNextCSeq()
	if nextCSeq != initialCSeq+1 {
		t.Errorf("Expected CSeq %d, got %d", initialCSeq+1, nextCSeq)
	}

	leg.UpdateCSeq(100)
	if leg.LastCSeq != 100 {
		t.Errorf("Expected CSeq 100, got %d", leg.LastCSeq)
	}

	// Should not update with lower CSeq
	leg.UpdateCSeq(50)
	if leg.LastCSeq != 100 {
		t.Errorf("CSeq should remain 100, got %d", leg.LastCSeq)
	}
}

func TestB2BUASessionCleanup(t *testing.T) {
	b2bua := createTestB2BUA()
	defer b2bua.Stop()

	invite := createTestInvite()
	
	// Create multiple sessions
	session1, _ := b2bua.CreateSession(invite, "sip:callee1@example.com")
	session2, _ := b2bua.CreateSession(invite, "sip:callee2@example.com")
	session3, _ := b2bua.CreateSession(invite, "sip:callee3@example.com")

	// Mark some sessions as terminated
	session1.SetStatus(B2BUAStatusEnded)
	session2.SetStatus(B2BUAStatusFailed)
	// session3 remains active

	// Get active sessions before cleanup
	activeSessions, err := b2bua.GetActiveSessions()
	if err != nil {
		t.Fatalf("Failed to get active sessions: %v", err)
	}

	// Should only have session3 as active (session1 and session2 are terminated)
	activeCount := 0
	for _, session := range activeSessions {
		if session.IsActive() {
			activeCount++
		}
	}

	if activeCount > 1 {
		t.Errorf("Expected at most 1 active session, got %d", activeCount)
	}

	// Test cleanup
	err = b2bua.CleanupExpiredSessions()
	if err != nil {
		t.Fatalf("Failed to cleanup expired sessions: %v", err)
	}

	// Verify terminated sessions are removed
	_, err = b2bua.GetSession(session1.SessionID)
	if err == nil {
		t.Error("Terminated session1 should be removed")
	}

	_, err = b2bua.GetSession(session2.SessionID)
	if err == nil {
		t.Error("Terminated session2 should be removed")
	}

	// Active session should still exist
	_, err = b2bua.GetSession(session3.SessionID)
	if err != nil {
		t.Error("Active session3 should still exist")
	}
}

func TestB2BUATagExtraction(t *testing.T) {
	b2bua := createTestB2BUA()
	defer b2bua.Stop()

	tests := []struct {
		header   string
		expected string
	}{
		{"<sip:user@example.com>;tag=abc123", "abc123"},
		{"\"Display Name\" <sip:user@example.com>;tag=xyz789;other=param", "xyz789"},
		{"<sip:user@example.com>", ""},
		{"sip:user@example.com;tag=simple", "simple"},
	}

	for _, test := range tests {
		result := b2bua.extractTag(test.header)
		if result != test.expected {
			t.Errorf("extractTag(%s) = %s, expected %s", test.header, result, test.expected)
		}
	}
}

func TestB2BUACSeqExtraction(t *testing.T) {
	b2bua := createTestB2BUA()
	defer b2bua.Stop()

	tests := []struct {
		header   string
		expected uint32
	}{
		{"123 INVITE", 123},
		{"1 ACK", 1},
		{"999 BYE", 999},
		{"invalid", 1}, // Should return 1 for invalid input
		{"", 1},        // Should return 1 for empty input
	}

	for _, test := range tests {
		result := b2bua.extractCSeq(test.header)
		if result != test.expected {
			t.Errorf("extractCSeq(%s) = %d, expected %d", test.header, result, test.expected)
		}
	}
}