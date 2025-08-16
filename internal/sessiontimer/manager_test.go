package sessiontimer

import (
	"fmt"
	"testing"
	"time"

	"github.com/zurustar/xylitol2/internal/logging"
	"github.com/zurustar/xylitol2/internal/parser"
)

// MockLogger implements the Logger interface for testing
type MockLogger struct {
	DebugMessages []string
	InfoMessages  []string
	WarnMessages  []string
	ErrorMessages []string
}

func (m *MockLogger) Debug(msg string, fields ...logging.Field) {
	m.DebugMessages = append(m.DebugMessages, msg)
}

func (m *MockLogger) Info(msg string, fields ...logging.Field) {
	m.InfoMessages = append(m.InfoMessages, msg)
}

func (m *MockLogger) Warn(msg string, fields ...logging.Field) {
	m.WarnMessages = append(m.WarnMessages, msg)
}

func (m *MockLogger) Error(msg string, fields ...logging.Field) {
	m.ErrorMessages = append(m.ErrorMessages, msg)
}

func TestNewManager(t *testing.T) {
	logger := &MockLogger{}
	defaultExpires := 1800
	minSE := 90
	maxSE := 7200

	manager := NewManager(defaultExpires, minSE, maxSE, logger)

	if manager == nil {
		t.Fatal("NewManager returned nil")
	}

	if manager.defaultExpires != defaultExpires {
		t.Errorf("Expected defaultExpires %d, got %d", defaultExpires, manager.defaultExpires)
	}

	if manager.minSE != minSE {
		t.Errorf("Expected minSE %d, got %d", minSE, manager.minSE)
	}

	if manager.maxSE != maxSE {
		t.Errorf("Expected maxSE %d, got %d", maxSE, manager.maxSE)
	}

	if len(manager.sessions) != 0 {
		t.Errorf("Expected empty sessions map, got %d sessions", len(manager.sessions))
	}
}

func TestCreateSession(t *testing.T) {
	logger := &MockLogger{}
	manager := NewManager(1800, 90, 7200, logger)

	callID := "test-call-id-123"
	sessionExpires := 1800

	session := manager.CreateSession(callID, sessionExpires)

	if session == nil {
		t.Fatal("CreateSession returned nil")
	}

	if session.CallID != callID {
		t.Errorf("Expected CallID %s, got %s", callID, session.CallID)
	}

	if session.Refresher != "uac" {
		t.Errorf("Expected Refresher 'uac', got %s", session.Refresher)
	}

	if session.MinSE != 90 {
		t.Errorf("Expected MinSE 90, got %d", session.MinSE)
	}

	// Check that session expires is set to approximately now + sessionExpires
	expectedExpiry := time.Now().Add(time.Duration(sessionExpires) * time.Second)
	if session.SessionExpires.Before(expectedExpiry.Add(-time.Second)) ||
		session.SessionExpires.After(expectedExpiry.Add(time.Second)) {
		t.Errorf("Session expiry time is not within expected range")
	}

	// Verify session is stored in manager
	if manager.GetSessionCount() != 1 {
		t.Errorf("Expected 1 session in manager, got %d", manager.GetSessionCount())
	}

	retrievedSession := manager.GetSession(callID)
	if retrievedSession == nil {
		t.Fatal("GetSession returned nil for existing session")
	}

	if retrievedSession.CallID != callID {
		t.Errorf("Retrieved session has wrong CallID: expected %s, got %s", callID, retrievedSession.CallID)
	}
}

func TestCreateSessionWithLimits(t *testing.T) {
	logger := &MockLogger{}
	manager := NewManager(1800, 90, 7200, logger)

	tests := []struct {
		name           string
		callID         string
		sessionExpires int
		expectedExpires int
	}{
		{
			name:           "Below minimum",
			callID:         "test-below-min",
			sessionExpires: 30,
			expectedExpires: 90, // Should be clamped to minSE
		},
		{
			name:           "Above maximum",
			callID:         "test-above-max",
			sessionExpires: 10000,
			expectedExpires: 7200, // Should be clamped to maxSE
		},
		{
			name:           "Within range",
			callID:         "test-within-range",
			sessionExpires: 1800,
			expectedExpires: 1800, // Should remain unchanged
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			session := manager.CreateSession(tt.callID, tt.sessionExpires)
			
			// Calculate the actual expires duration from the session
			actualDuration := time.Until(session.SessionExpires)
			expectedDuration := time.Duration(tt.expectedExpires) * time.Second
			
			// Allow for some tolerance (1 second) due to timing
			if actualDuration < expectedDuration-time.Second || actualDuration > expectedDuration+time.Second {
				t.Errorf("Expected session expires duration ~%v, got %v", expectedDuration, actualDuration)
			}
		})
	}
}

func TestRefreshSession(t *testing.T) {
	logger := &MockLogger{}
	manager := NewManager(1800, 90, 7200, logger)

	callID := "test-refresh-call-id"
	sessionExpires := 1800

	// Create initial session
	session := manager.CreateSession(callID, sessionExpires)
	originalExpiry := session.SessionExpires

	// Wait a small amount to ensure time difference
	time.Sleep(10 * time.Millisecond)

	// Refresh the session
	err := manager.RefreshSession(callID)
	if err != nil {
		t.Fatalf("RefreshSession failed: %v", err)
	}

	// Get the refreshed session
	refreshedSession := manager.GetSession(callID)
	if refreshedSession == nil {
		t.Fatal("Session not found after refresh")
	}

	// Check that expiry time was updated
	if !refreshedSession.SessionExpires.After(originalExpiry) {
		t.Error("Session expiry time was not updated after refresh")
	}
}

func TestRefreshNonExistentSession(t *testing.T) {
	logger := &MockLogger{}
	manager := NewManager(1800, 90, 7200, logger)

	err := manager.RefreshSession("non-existent-call-id")
	if err == nil {
		t.Error("Expected error when refreshing non-existent session")
	}

	expectedError := "session not found: non-existent-call-id"
	if err.Error() != expectedError {
		t.Errorf("Expected error message '%s', got '%s'", expectedError, err.Error())
	}
}

func TestCleanupExpiredSessions(t *testing.T) {
	logger := &MockLogger{}
	manager := NewManager(1800, 90, 7200, logger)

	// Create sessions with different expiry times
	callID1 := "expired-session-1"
	callID2 := "expired-session-2"
	callID3 := "active-session"

	// Create sessions
	session1 := manager.CreateSession(callID1, 1)  // Will expire quickly
	session2 := manager.CreateSession(callID2, 1)  // Will expire quickly
	manager.CreateSession(callID3, 3600) // Long expiry

	// Manually set expiry times to simulate expired sessions
	session1.SessionExpires = time.Now().Add(-time.Hour) // Expired 1 hour ago
	session2.SessionExpires = time.Now().Add(-time.Minute) // Expired 1 minute ago

	if manager.GetSessionCount() != 3 {
		t.Fatalf("Expected 3 sessions before cleanup, got %d", manager.GetSessionCount())
	}

	// Run cleanup
	manager.CleanupExpiredSessions()

	// Check that expired sessions were removed
	if manager.GetSessionCount() != 1 {
		t.Errorf("Expected 1 session after cleanup, got %d", manager.GetSessionCount())
	}

	// Check that the active session remains
	activeSession := manager.GetSession(callID3)
	if activeSession == nil {
		t.Error("Active session was incorrectly removed during cleanup")
	}

	// Check that expired sessions were removed
	if manager.GetSession(callID1) != nil {
		t.Error("Expired session 1 was not removed")
	}

	if manager.GetSession(callID2) != nil {
		t.Error("Expired session 2 was not removed")
	}
}

func TestIsSessionTimerRequired(t *testing.T) {
	logger := &MockLogger{}
	manager := NewManager(1800, 90, 7200, logger)

	tests := []struct {
		name     string
		message  *parser.SIPMessage
		expected bool
	}{
		{
			name:     "INVITE request",
			message:  parser.NewRequestMessage(parser.MethodINVITE, "sip:user@example.com"),
			expected: true,
		},
		{
			name:     "REGISTER request",
			message:  parser.NewRequestMessage(parser.MethodREGISTER, "sip:user@example.com"),
			expected: false,
		},
		{
			name:     "BYE request",
			message:  parser.NewRequestMessage(parser.MethodBYE, "sip:user@example.com"),
			expected: false,
		},
		{
			name:     "OPTIONS request",
			message:  parser.NewRequestMessage(parser.MethodOPTIONS, "sip:user@example.com"),
			expected: false,
		},
		{
			name:     "200 OK response",
			message:  parser.NewResponseMessage(parser.StatusOK, "OK"),
			expected: false,
		},
		{
			name:     "404 Not Found response",
			message:  parser.NewResponseMessage(parser.StatusNotFound, "Not Found"),
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := manager.IsSessionTimerRequired(tt.message)
			if result != tt.expected {
				t.Errorf("Expected IsSessionTimerRequired to return %v for %s, got %v", 
					tt.expected, tt.name, result)
			}
		})
	}
}

func TestRemoveSession(t *testing.T) {
	logger := &MockLogger{}
	manager := NewManager(1800, 90, 7200, logger)

	callID := "test-remove-session"
	manager.CreateSession(callID, 1800)

	if manager.GetSessionCount() != 1 {
		t.Fatalf("Expected 1 session before removal, got %d", manager.GetSessionCount())
	}

	// Remove the session
	manager.RemoveSession(callID)

	if manager.GetSessionCount() != 0 {
		t.Errorf("Expected 0 sessions after removal, got %d", manager.GetSessionCount())
	}

	if manager.GetSession(callID) != nil {
		t.Error("Session still exists after removal")
	}

	// Removing non-existent session should not cause error
	manager.RemoveSession("non-existent")
	if manager.GetSessionCount() != 0 {
		t.Errorf("Session count changed after removing non-existent session")
	}
}

func TestGetters(t *testing.T) {
	logger := &MockLogger{}
	defaultExpires := 1800
	minSE := 90
	maxSE := 7200

	manager := NewManager(defaultExpires, minSE, maxSE, logger)

	if manager.GetDefaultExpires() != defaultExpires {
		t.Errorf("Expected GetDefaultExpires() to return %d, got %d", 
			defaultExpires, manager.GetDefaultExpires())
	}

	if manager.GetMinSE() != minSE {
		t.Errorf("Expected GetMinSE() to return %d, got %d", 
			minSE, manager.GetMinSE())
	}

	if manager.GetMaxSE() != maxSE {
		t.Errorf("Expected GetMaxSE() to return %d, got %d", 
			maxSE, manager.GetMaxSE())
	}
}

func TestConcurrentAccess(t *testing.T) {
	logger := &MockLogger{}
	manager := NewManager(1800, 90, 7200, logger)

	// Test concurrent session creation and access
	done := make(chan bool, 10)

	// Create sessions concurrently
	for i := 0; i < 10; i++ {
		go func(id int) {
			callID := fmt.Sprintf("concurrent-session-%d", id)
			manager.CreateSession(callID, 1800)
			
			// Try to access the session
			session := manager.GetSession(callID)
			if session == nil {
				t.Errorf("Failed to retrieve session %s", callID)
			}
			
			// Try to refresh the session
			err := manager.RefreshSession(callID)
			if err != nil {
				t.Errorf("Failed to refresh session %s: %v", callID, err)
			}
			
			done <- true
		}(i)
	}

	// Wait for all goroutines to complete
	for i := 0; i < 10; i++ {
		<-done
	}

	if manager.GetSessionCount() != 10 {
		t.Errorf("Expected 10 sessions after concurrent creation, got %d", manager.GetSessionCount())
	}

	// Test concurrent cleanup
	manager.CleanupExpiredSessions()
	
	// All sessions should still be active since they were just created
	if manager.GetSessionCount() != 10 {
		t.Errorf("Expected 10 sessions after cleanup, got %d", manager.GetSessionCount())
	}
}