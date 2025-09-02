package huntgroup

import (
	"testing"
	"time"
)

func TestHuntGroupTimeoutManagement(t *testing.T) {
	b2bua := createTestB2BUA()
	defer b2bua.Stop()

	invite := createTestInvite()
	huntGroup := &HuntGroup{
		ID:          1,
		Extension:   "100",
		Strategy:    StrategySimultaneous,
		RingTimeout: 2, // 2 seconds timeout for testing
		Enabled:     true,
	}

	// Create hunt group session
	session, err := b2bua.CreateHuntGroupSession(invite, huntGroup)
	if err != nil {
		t.Fatalf("Failed to create hunt group session: %v", err)
	}

	// Verify timeout timer is started
	b2bua.timeoutMutex.RLock()
	timer, exists := b2bua.huntGroupTimeouts[session.SessionID]
	b2bua.timeoutMutex.RUnlock()

	if !exists {
		t.Error("Hunt group timeout timer should be started")
	}

	if timer == nil {
		t.Error("Timeout timer should not be nil")
	}

	// Cancel timeout
	b2bua.CancelHuntGroupTimeout(session.SessionID)

	// Verify timeout timer is cancelled
	b2bua.timeoutMutex.RLock()
	_, exists = b2bua.huntGroupTimeouts[session.SessionID]
	b2bua.timeoutMutex.RUnlock()

	if exists {
		t.Error("Hunt group timeout timer should be cancelled")
	}
}

func TestHuntGroupTimeoutExecution(t *testing.T) {
	b2bua := createTestB2BUA()
	defer b2bua.Stop()

	invite := createTestInvite()
	huntGroup := &HuntGroup{
		ID:          1,
		Extension:   "100",
		Strategy:    StrategySimultaneous,
		RingTimeout: 1, // 1 second timeout for testing
		Enabled:     true,
	}

	// Create hunt group session
	session, err := b2bua.CreateHuntGroupSession(invite, huntGroup)
	if err != nil {
		t.Fatalf("Failed to create hunt group session: %v", err)
	}

	// Wait for timeout to occur
	time.Sleep(1500 * time.Millisecond)

	// Verify session status is failed
	updatedSession, err := b2bua.GetSession(session.SessionID)
	if err == nil {
		// Session might still exist briefly before cleanup
		if updatedSession.GetStatus() != B2BUAStatusFailed {
			t.Errorf("Expected session status to be failed after timeout, got %s", updatedSession.GetStatus())
		}
	}

	// Verify timeout timer is cleaned up
	b2bua.timeoutMutex.RLock()
	_, exists := b2bua.huntGroupTimeouts[session.SessionID]
	b2bua.timeoutMutex.RUnlock()

	if exists {
		t.Error("Hunt group timeout timer should be cleaned up after timeout")
	}
}

func TestHuntGroupTimeoutCancelOnConnect(t *testing.T) {
	b2bua := createTestB2BUA()
	defer b2bua.Stop()

	invite := createTestInvite()
	huntGroup := &HuntGroup{
		ID:          1,
		Extension:   "100",
		Strategy:    StrategySimultaneous,
		RingTimeout: 10, // Long timeout
		Enabled:     true,
	}

	// Create hunt group session
	session, err := b2bua.CreateHuntGroupSession(invite, huntGroup)
	if err != nil {
		t.Fatalf("Failed to create hunt group session: %v", err)
	}

	// Simulate call connection by setting up callee leg and bridging
	calleeLeg := &CallLeg{
		LegID:     b2bua.generateLegID("callee"),
		CallID:    b2bua.generateCallID(),
		Status:    CallLegStatusConnected,
		CreatedAt: time.Now().UTC(),
	}
	session.CalleeLeg = calleeLeg
	session.SetStatus(B2BUAStatusRinging)

	// Bridge calls (this should cancel the timeout)
	err = b2bua.BridgeCalls(session.SessionID)
	if err != nil {
		t.Fatalf("Failed to bridge calls: %v", err)
	}

	// Verify timeout timer is cancelled
	b2bua.timeoutMutex.RLock()
	_, exists := b2bua.huntGroupTimeouts[session.SessionID]
	b2bua.timeoutMutex.RUnlock()

	if exists {
		t.Error("Hunt group timeout timer should be cancelled when call is bridged")
	}

	// Verify session is connected
	if session.GetStatus() != B2BUAStatusConnected {
		t.Errorf("Expected session status to be connected, got %s", session.GetStatus())
	}
}

func TestHuntGroupNoTimeoutWhenDisabled(t *testing.T) {
	b2bua := createTestB2BUA()
	defer b2bua.Stop()

	invite := createTestInvite()
	huntGroup := &HuntGroup{
		ID:          1,
		Extension:   "100",
		Strategy:    StrategySimultaneous,
		RingTimeout: 0, // No timeout
		Enabled:     true,
	}

	// Create hunt group session
	session, err := b2bua.CreateHuntGroupSession(invite, huntGroup)
	if err != nil {
		t.Fatalf("Failed to create hunt group session: %v", err)
	}

	// Verify no timeout timer is started
	b2bua.timeoutMutex.RLock()
	_, exists := b2bua.huntGroupTimeouts[session.SessionID]
	b2bua.timeoutMutex.RUnlock()

	if exists {
		t.Error("Hunt group timeout timer should not be started when timeout is disabled")
	}
}

func TestTimeoutResponseGeneration(t *testing.T) {
	b2bua := createTestB2BUA()
	defer b2bua.Stop()

	invite := createTestInvite()
	session, err := b2bua.CreateSession(invite, "sip:callee@example.com")
	if err != nil {
		t.Fatalf("Failed to create session: %v", err)
	}

	// Test timeout response generation
	err = b2bua.sendTimeoutResponseToCaller(session)
	if err != nil {
		t.Errorf("Failed to send timeout response: %v", err)
	}

	// In a real implementation, we would verify the response was sent correctly
	// For now, we just verify the method doesn't error
}

func TestMultipleTimeoutManagement(t *testing.T) {
	b2bua := createTestB2BUA()
	defer b2bua.Stop()

	invite := createTestInvite()
	huntGroup := &HuntGroup{
		ID:          1,
		Extension:   "100",
		Strategy:    StrategySimultaneous,
		RingTimeout: 5,
		Enabled:     true,
	}

	// Create multiple sessions
	session1, err := b2bua.CreateHuntGroupSession(invite, huntGroup)
	if err != nil {
		t.Fatalf("Failed to create session 1: %v", err)
	}

	session2, err := b2bua.CreateHuntGroupSession(invite, huntGroup)
	if err != nil {
		t.Fatalf("Failed to create session 2: %v", err)
	}

	// Verify both timeouts are managed
	b2bua.timeoutMutex.RLock()
	timer1, exists1 := b2bua.huntGroupTimeouts[session1.SessionID]
	timer2, exists2 := b2bua.huntGroupTimeouts[session2.SessionID]
	b2bua.timeoutMutex.RUnlock()

	if !exists1 || timer1 == nil {
		t.Error("Session 1 timeout should be managed")
	}

	if !exists2 || timer2 == nil {
		t.Error("Session 2 timeout should be managed")
	}

	// Cancel one timeout
	b2bua.CancelHuntGroupTimeout(session1.SessionID)

	// Verify only session1 timeout is cancelled
	b2bua.timeoutMutex.RLock()
	_, exists1 = b2bua.huntGroupTimeouts[session1.SessionID]
	_, exists2 = b2bua.huntGroupTimeouts[session2.SessionID]
	b2bua.timeoutMutex.RUnlock()

	if exists1 {
		t.Error("Session 1 timeout should be cancelled")
	}

	if !exists2 {
		t.Error("Session 2 timeout should still exist")
	}
}