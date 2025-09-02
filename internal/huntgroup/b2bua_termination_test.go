package huntgroup

import (
	"fmt"
	"testing"
	"time"

	"github.com/zurustar/xylitol2/internal/parser"
)

func TestB2BUA_CallTerminationByBye(t *testing.T) {
	// Create test dependencies
	logger := &TestLogger{}
	transportManager := &mockTransportManager{}
	transactionManager := &mockTransactionManager{}
	messageParser := &mockParser{}

	// Create B2BUA
	b2bua := NewB2BUA(transportManager, transactionManager, messageParser, logger, "192.168.1.1", 5060)
	defer b2bua.Stop()

	// Create test INVITE
	invite := createTestInvite()
	
	// Create session
	session, err := b2bua.CreateSession(invite, "sip:bob@example.com")
	if err != nil {
		t.Fatalf("Failed to create session: %v", err)
	}

	// Simulate call progression to ringing state
	session.SetStatus(B2BUAStatusRinging)

	// Simulate call connection
	err = b2bua.BridgeCalls(session.SessionID)
	if err != nil {
		t.Fatalf("Failed to bridge calls: %v", err)
	}

	// Verify session is connected
	if session.GetStatus() != B2BUAStatusConnected {
		t.Errorf("Expected session status %v, got %v", B2BUAStatusConnected, session.GetStatus())
	}

	// Directly end the session to test statistics collection
	now := time.Now().UTC()
	b2bua.statsCollector.EndSession(session.SessionID, now, "BYE", "caller")

	// Verify session statistics were collected
	stats := b2bua.GetSessionStatistics(session.SessionID)
	if stats == nil {
		t.Fatal("Session statistics should not be nil")
	}

	if stats.EndReason != "BYE" {
		t.Errorf("Expected end reason 'BYE', got '%s'", stats.EndReason)
	}

	if stats.EndedBy != "caller" {
		t.Errorf("Expected ended by 'caller', got '%s'", stats.EndedBy)
	}

	if stats.EndTime == nil {
		t.Error("End time should not be nil")
	}

	if stats.Duration <= 0 {
		t.Errorf("Duration should be positive, got %v", stats.Duration)
	}
}

func TestB2BUA_CallTerminationByCalleeHangup(t *testing.T) {
	// Create test dependencies
	logger := &TestLogger{}
	transportManager := &mockTransportManager{}
	transactionManager := &mockTransactionManager{}
	messageParser := &mockParser{}

	// Create B2BUA
	b2bua := NewB2BUA(transportManager, transactionManager, messageParser, logger, "192.168.1.1", 5060)
	defer b2bua.Stop()

	// Create test INVITE
	invite := createTestInvite()
	
	// Create session
	session, err := b2bua.CreateSession(invite, "sip:bob@example.com")
	if err != nil {
		t.Fatalf("Failed to create session: %v", err)
	}

	// Simulate call progression to ringing state
	session.SetStatus(B2BUAStatusRinging)

	// Simulate call connection
	err = b2bua.BridgeCalls(session.SessionID)
	if err != nil {
		t.Fatalf("Failed to bridge calls: %v", err)
	}

	// Directly end the session to test statistics collection
	now := time.Now().UTC()
	b2bua.statsCollector.EndSession(session.SessionID, now, "BYE", "callee")

	// Verify session statistics were collected
	stats := b2bua.GetSessionStatistics(session.SessionID)
	if stats == nil {
		t.Fatal("Session statistics should not be nil")
	}

	if stats.EndReason != "BYE" {
		t.Errorf("Expected end reason 'BYE', got '%s'", stats.EndReason)
	}

	if stats.EndedBy != "callee" {
		t.Errorf("Expected ended by 'callee', got '%s'", stats.EndedBy)
	}
}

func TestB2BUA_SessionCleanupWithStatistics(t *testing.T) {
	// Create test dependencies
	logger := &TestLogger{}
	transportManager := &mockTransportManager{}
	transactionManager := &mockTransactionManager{}
	messageParser := &mockParser{}

	// Create B2BUA
	b2bua := NewB2BUA(transportManager, transactionManager, messageParser, logger, "192.168.1.1", 5060)

	// Create multiple sessions
	sessionIDs := make([]string, 3)
	oldTime := time.Now().UTC().Add(-2 * time.Hour)
	
	for i := 0; i < 3; i++ {
		invite := createTestInvite()
		invite.SetHeader(parser.HeaderCallID, fmt.Sprintf("call-%d@example.com", i))
		
		session, err := b2bua.CreateSession(invite, "sip:bob@example.com")
		if err != nil {
			t.Fatalf("Failed to create session %d: %v", i, err)
		}
		sessionIDs[i] = session.SessionID
		
		// Connect and end some sessions with old timestamps
		if i < 2 {
			session.SetStatus(B2BUAStatusRinging)
			b2bua.BridgeCalls(session.SessionID)
			// End session with old timestamp for cleanup test
			b2bua.statsCollector.EndSession(session.SessionID, oldTime, "BYE", "caller")
			b2bua.EndSession(session.SessionID)
		}
	}

	// Verify statistics counts
	allStats := b2bua.GetAllSessionStatistics()
	if len(allStats) != 3 {
		t.Errorf("Expected 3 session statistics, got %d", len(allStats))
	}

	// Cleanup old statistics (should remove the 2 ended sessions with old timestamps)
	cleaned := b2bua.CleanupSessionStatistics(1 * time.Hour)
	if cleaned != 2 {
		t.Errorf("Expected 2 cleaned statistics, got %d", cleaned)
	}

	// Verify remaining statistics (should have 1 active session)
	remainingStats := b2bua.GetAllSessionStatistics()
	if len(remainingStats) != 1 {
		t.Errorf("Expected 1 remaining statistics, got %d", len(remainingStats))
	}

	// Cleanup very recent statistics (should not remove any more)
	cleaned = b2bua.CleanupSessionStatistics(1 * time.Nanosecond)
	if cleaned != 0 {
		t.Errorf("Expected 0 cleaned statistics, got %d", cleaned)
	}
}

func TestB2BUA_HuntGroupCallTermination(t *testing.T) {
	// Create test dependencies
	logger := &TestLogger{}
	transportManager := &mockTransportManager{}
	transactionManager := &mockTransactionManager{}
	messageParser := &mockParser{}

	// Create B2BUA
	b2bua := NewB2BUA(transportManager, transactionManager, messageParser, logger, "192.168.1.1", 5060)
	defer b2bua.Stop()

	// Create hunt group
	huntGroup := &HuntGroup{
		ID:        1,
		Extension: "100",
		Strategy:  StrategySimultaneous,
	}

	// Create test INVITE
	invite := createTestInvite()
	
	// Create hunt group session
	session, err := b2bua.CreateHuntGroupSession(invite, huntGroup)
	if err != nil {
		t.Fatalf("Failed to create hunt group session: %v", err)
	}

	// Create a callee leg for the session (simulate member answering)
	session.CalleeLeg = &CallLeg{
		LegID:   "callee-leg-1",
		CallID:  "callee-call-id",
		ToURI:   "sip:member1@example.com",
		Status:  CallLegStatusRinging,
	}

	// Simulate call connection with answered member
	session.SetStatus(B2BUAStatusRinging)
	err = b2bua.BridgeCalls(session.SessionID)
	if err != nil {
		t.Fatalf("Failed to bridge calls: %v", err)
	}

	// Directly end the session to test statistics collection
	now := time.Now().UTC()
	b2bua.statsCollector.EndSession(session.SessionID, now, "BYE", "caller")

	// Verify session statistics
	stats := b2bua.GetSessionStatistics(session.SessionID)
	if stats == nil {
		t.Fatal("Session statistics should not be nil")
	}

	if stats.HuntGroupID == nil || *stats.HuntGroupID != 1 {
		t.Errorf("Expected hunt group ID 1, got %v", stats.HuntGroupID)
	}

	if stats.EndReason != "BYE" {
		t.Errorf("Expected end reason 'BYE', got '%s'", stats.EndReason)
	}
}

