package sessiontimer

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/zurustar/xylitol2/internal/parser"
)

func TestSessionTimerEnforcement(t *testing.T) {
	logger := &MockLogger{}
	manager := NewManager(1800, 90, 7200, logger)

	// Test Session-Timer requirement for INVITE
	inviteMsg := parser.NewRequestMessage(parser.MethodINVITE, "sip:user@example.com")
	if !manager.IsSessionTimerRequired(inviteMsg) {
		t.Error("Session-Timer should be required for INVITE requests")
	}

	// Test Session-Timer not required for other methods
	registerMsg := parser.NewRequestMessage(parser.MethodREGISTER, "sip:user@example.com")
	if manager.IsSessionTimerRequired(registerMsg) {
		t.Error("Session-Timer should not be required for REGISTER requests")
	}

	byeMsg := parser.NewRequestMessage(parser.MethodBYE, "sip:user@example.com")
	if manager.IsSessionTimerRequired(byeMsg) {
		t.Error("Session-Timer should not be required for BYE requests")
	}

	// Test Session-Timer not required for responses
	responseMsg := parser.NewResponseMessage(parser.StatusOK, "OK")
	if manager.IsSessionTimerRequired(responseMsg) {
		t.Error("Session-Timer should not be required for response messages")
	}
}

func TestSessionTerminationCallback(t *testing.T) {
	logger := &MockLogger{}
	manager := NewManager(1800, 90, 7200, logger)

	// Track terminated sessions with synchronization
	terminatedSessions := make([]string, 0)
	var mu sync.Mutex
	done := make(chan bool, 1)
	
	manager.SetSessionTerminationCallback(func(callID string) {
		mu.Lock()
		terminatedSessions = append(terminatedSessions, callID)
		mu.Unlock()
		done <- true
	})

	// Create a session that will expire quickly
	callID := "test-termination-callback"
	session := manager.CreateSession(callID, 1)
	
	// Manually set expiry to past time to simulate expiration
	session.SessionExpires = time.Now().Add(-time.Hour)

	// Run cleanup with callback
	manager.CleanupExpiredSessions()

	// Wait for callback to be called
	select {
	case <-done:
		// Callback was called
	case <-time.After(1 * time.Second):
		t.Fatal("Callback was not called within timeout")
	}

	// Check that callback was called
	mu.Lock()
	if len(terminatedSessions) != 1 {
		t.Errorf("Expected 1 terminated session, got %d", len(terminatedSessions))
	}

	if len(terminatedSessions) > 0 && terminatedSessions[0] != callID {
		t.Errorf("Expected terminated session %s, got %s", callID, terminatedSessions[0])
	}
	mu.Unlock()

	// Verify session was removed
	if manager.GetSession(callID) != nil {
		t.Error("Session should have been removed after termination")
	}
}

func TestCleanupTimer(t *testing.T) {
	logger := &MockLogger{}
	manager := NewManager(1800, 90, 7200, logger)

	// Track terminated sessions with synchronization
	terminatedSessions := make([]string, 0)
	var mu sync.Mutex
	done := make(chan bool, 1)
	
	manager.SetSessionTerminationCallback(func(callID string) {
		mu.Lock()
		terminatedSessions = append(terminatedSessions, callID)
		mu.Unlock()
		done <- true
	})

	// Start cleanup timer
	manager.StartCleanupTimer()
	defer manager.StopCleanupTimer()

	// Create sessions with different expiry times
	expiredCallID := "expired-session"
	activeCallID := "active-session"

	expiredSession := manager.CreateSession(expiredCallID, 1)
	manager.CreateSession(activeCallID, 3600)

	// Set one session to be expired
	expiredSession.SessionExpires = time.Now().Add(-time.Hour)

	// Trigger cleanup manually
	manager.CleanupExpiredSessions()

	// Wait for callback to be called
	select {
	case <-done:
		// Callback was called
	case <-time.After(1 * time.Second):
		t.Fatal("Callback was not called within timeout")
	}

	// Check that expired session was terminated
	mu.Lock()
	if len(terminatedSessions) != 1 {
		t.Errorf("Expected 1 terminated session, got %d", len(terminatedSessions))
	}

	if len(terminatedSessions) > 0 && terminatedSessions[0] != expiredCallID {
		t.Errorf("Expected terminated session %s, got %s", expiredCallID, terminatedSessions[0])
	}
	mu.Unlock()

	// Check that active session remains
	if manager.GetSession(activeCallID) == nil {
		t.Error("Active session should not have been terminated")
	}

	// Check that expired session was removed
	if manager.GetSession(expiredCallID) != nil {
		t.Error("Expired session should have been removed")
	}
}

func TestSessionRefreshPreventsTermination(t *testing.T) {
	logger := &MockLogger{}
	manager := NewManager(1800, 90, 7200, logger)

	// Track terminated sessions
	terminatedSessions := make([]string, 0)
	manager.SetSessionTerminationCallback(func(callID string) {
		terminatedSessions = append(terminatedSessions, callID)
	})

	// Create a session
	callID := "refresh-test-session"
	manager.CreateSession(callID, 1800)

	// Wait a bit and then refresh
	time.Sleep(10 * time.Millisecond)
	err := manager.RefreshSession(callID)
	if err != nil {
		t.Fatalf("Failed to refresh session: %v", err)
	}

	// Run cleanup - session should not be terminated since it was refreshed
	manager.cleanupExpiredSessionsWithCallback()

	// Check that no sessions were terminated
	if len(terminatedSessions) != 0 {
		t.Errorf("Expected 0 terminated sessions, got %d", len(terminatedSessions))
	}

	// Check that session still exists
	if manager.GetSession(callID) == nil {
		t.Error("Session should still exist after refresh")
	}
}

func TestSessionLimitsEnforcement(t *testing.T) {
	logger := &MockLogger{}
	minSE := 90
	maxSE := 7200
	manager := NewManager(1800, minSE, maxSE, logger)

	tests := []struct {
		name            string
		requestedExpires int
		expectedExpires  int
	}{
		{
			name:            "Below minimum",
			requestedExpires: 30,
			expectedExpires:  minSE,
		},
		{
			name:            "Above maximum", 
			requestedExpires: 10000,
			expectedExpires:  maxSE,
		},
		{
			name:            "Within range",
			requestedExpires: 1800,
			expectedExpires:  1800,
		},
		{
			name:            "At minimum",
			requestedExpires: minSE,
			expectedExpires:  minSE,
		},
		{
			name:            "At maximum",
			requestedExpires: maxSE,
			expectedExpires:  maxSE,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			callID := "test-limits-" + tt.name
			session := manager.CreateSession(callID, tt.requestedExpires)

			// Calculate actual duration from session expiry
			actualDuration := time.Until(session.SessionExpires)
			expectedDuration := time.Duration(tt.expectedExpires) * time.Second

			// Allow for some tolerance (1 second) due to timing
			if actualDuration < expectedDuration-time.Second || actualDuration > expectedDuration+time.Second {
				t.Errorf("Expected session expires duration ~%v, got %v", expectedDuration, actualDuration)
			}
		})
	}
}

func TestConcurrentSessionTermination(t *testing.T) {
	logger := &MockLogger{}
	manager := NewManager(1800, 90, 7200, logger)

	// Track terminated sessions with thread-safe access
	terminatedSessions := make(map[string]bool)
	terminationMutex := sync.RWMutex{}
	callbackCount := make(chan bool, 10)
	
	manager.SetSessionTerminationCallback(func(callID string) {
		terminationMutex.Lock()
		terminatedSessions[callID] = true
		terminationMutex.Unlock()
		callbackCount <- true
	})

	// Create multiple sessions that will expire
	numSessions := 10
	callIDs := make([]string, numSessions)
	
	for i := 0; i < numSessions; i++ {
		callID := fmt.Sprintf("concurrent-session-%d", i)
		callIDs[i] = callID
		session := manager.CreateSession(callID, 1)
		// Set all sessions to be expired
		session.SessionExpires = time.Now().Add(-time.Hour)
	}

	// Run cleanup
	manager.CleanupExpiredSessions()

	// Wait for all callbacks to be called
	for i := 0; i < numSessions; i++ {
		select {
		case <-callbackCount:
			// Callback received
		case <-time.After(2 * time.Second):
			t.Fatalf("Timeout waiting for callback %d", i+1)
		}
	}

	// Check that all sessions were terminated
	terminationMutex.RLock()
	if len(terminatedSessions) != numSessions {
		t.Errorf("Expected %d terminated sessions, got %d", numSessions, len(terminatedSessions))
	}

	for _, callID := range callIDs {
		if !terminatedSessions[callID] {
			t.Errorf("Session %s was not terminated", callID)
		}
	}
	terminationMutex.RUnlock()

	// Check that all sessions were removed from manager
	if manager.GetSessionCount() != 0 {
		t.Errorf("Expected 0 sessions after cleanup, got %d", manager.GetSessionCount())
	}
}

func TestStopCleanupTimer(t *testing.T) {
	logger := &MockLogger{}
	manager := NewManager(1800, 90, 7200, logger)

	// Start and immediately stop cleanup timer
	manager.StartCleanupTimer()
	manager.StopCleanupTimer()

	// This test mainly ensures no panic occurs and cleanup can be stopped
	// In a real scenario, we'd need to test that the goroutine actually stops
	// but that's difficult to test reliably in a unit test
}