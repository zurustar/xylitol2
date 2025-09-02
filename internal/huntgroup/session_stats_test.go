package huntgroup

import (
	"fmt"
	"testing"
	"time"
)

func TestSessionStatsCollector_StartSession(t *testing.T) {
	logger := &TestLogger{}
	collector := NewSessionStatsCollector(logger)

	// Create a test session
	session := &B2BUASession{
		SessionID: "test-session-1",
		CallerLeg: &CallLeg{
			FromURI: "sip:alice@example.com",
		},
		CalleeLeg: &CallLeg{
			ToURI: "sip:bob@example.com",
		},
		StartTime: time.Now().UTC(),
	}

	// Start session statistics
	collector.StartSession(session)

	// Verify statistics were created
	stats := collector.GetSessionStats("test-session-1")
	if stats == nil {
		t.Fatal("Session statistics should not be nil")
	}

	if stats.SessionID != "test-session-1" {
		t.Errorf("Expected session ID 'test-session-1', got '%s'", stats.SessionID)
	}

	if stats.CallerURI != "sip:alice@example.com" {
		t.Errorf("Expected caller URI 'sip:alice@example.com', got '%s'", stats.CallerURI)
	}

	if stats.CalleeURI != "sip:bob@example.com" {
		t.Errorf("Expected callee URI 'sip:bob@example.com', got '%s'", stats.CalleeURI)
	}
}

func TestSessionStatsCollector_UpdateSessionConnect(t *testing.T) {
	logger := &TestLogger{}
	collector := NewSessionStatsCollector(logger)

	// Create and start session
	session := &B2BUASession{
		SessionID: "test-session-1",
		StartTime: time.Now().UTC().Add(-5 * time.Second),
	}
	collector.StartSession(session)

	// Update for connection
	connectTime := time.Now().UTC()
	collector.UpdateSessionConnect("test-session-1", connectTime, "sip:member1@example.com")

	// Verify statistics were updated
	stats := collector.GetSessionStats("test-session-1")
	if stats == nil {
		t.Fatal("Session statistics should not be nil")
	}

	if stats.ConnectTime == nil {
		t.Fatal("Connect time should not be nil")
	}

	if !stats.ConnectTime.Equal(connectTime) {
		t.Errorf("Expected connect time %v, got %v", connectTime, *stats.ConnectTime)
	}

	if stats.AnsweredMember != "sip:member1@example.com" {
		t.Errorf("Expected answered member 'sip:member1@example.com', got '%s'", stats.AnsweredMember)
	}

	if stats.SetupDuration <= 0 {
		t.Errorf("Setup duration should be positive, got %v", stats.SetupDuration)
	}
}

func TestSessionStatsCollector_EndSession(t *testing.T) {
	logger := &TestLogger{}
	collector := NewSessionStatsCollector(logger)

	// Create and start session
	startTime := time.Now().UTC().Add(-10 * time.Second)
	session := &B2BUASession{
		SessionID: "test-session-1",
		StartTime: startTime,
	}
	collector.StartSession(session)

	// Update for connection
	connectTime := startTime.Add(2 * time.Second)
	collector.UpdateSessionConnect("test-session-1", connectTime, "sip:member1@example.com")

	// End session
	endTime := time.Now().UTC()
	finalStats := collector.EndSession("test-session-1", endTime, "BYE", "caller")

	if finalStats == nil {
		t.Fatal("Final statistics should not be nil")
	}

	if finalStats.EndTime == nil {
		t.Fatal("End time should not be nil")
	}

	if !finalStats.EndTime.Equal(endTime) {
		t.Errorf("Expected end time %v, got %v", endTime, *finalStats.EndTime)
	}

	if finalStats.EndReason != "BYE" {
		t.Errorf("Expected end reason 'BYE', got '%s'", finalStats.EndReason)
	}

	if finalStats.EndedBy != "caller" {
		t.Errorf("Expected ended by 'caller', got '%s'", finalStats.EndedBy)
	}

	if finalStats.Duration <= 0 {
		t.Errorf("Duration should be positive, got %v", finalStats.Duration)
	}

	if finalStats.TalkDuration <= 0 {
		t.Errorf("Talk duration should be positive, got %v", finalStats.TalkDuration)
	}
}

func TestSessionStatsCollector_GetCounts(t *testing.T) {
	logger := &TestLogger{}
	collector := NewSessionStatsCollector(logger)

	// Initially should be empty
	if collector.GetStatsCount() != 0 {
		t.Errorf("Expected 0 stats, got %d", collector.GetStatsCount())
	}

	if collector.GetActiveSessionsCount() != 0 {
		t.Errorf("Expected 0 active sessions, got %d", collector.GetActiveSessionsCount())
	}

	// Add some sessions
	for i := 0; i < 3; i++ {
		session := &B2BUASession{
			SessionID: fmt.Sprintf("session-%d", i),
			StartTime: time.Now().UTC(),
		}
		collector.StartSession(session)
	}

	if collector.GetStatsCount() != 3 {
		t.Errorf("Expected 3 stats, got %d", collector.GetStatsCount())
	}

	if collector.GetActiveSessionsCount() != 3 {
		t.Errorf("Expected 3 active sessions, got %d", collector.GetActiveSessionsCount())
	}

	// End one session
	collector.EndSession("session-0", time.Now().UTC(), "BYE", "caller")

	if collector.GetStatsCount() != 3 {
		t.Errorf("Expected 3 total stats, got %d", collector.GetStatsCount())
	}

	if collector.GetActiveSessionsCount() != 2 {
		t.Errorf("Expected 2 active sessions, got %d", collector.GetActiveSessionsCount())
	}

	if collector.GetCompletedSessionsCount() != 1 {
		t.Errorf("Expected 1 completed session, got %d", collector.GetCompletedSessionsCount())
	}
}

func TestSessionStatsCollector_CleanupStats(t *testing.T) {
	logger := &TestLogger{}
	collector := NewSessionStatsCollector(logger)

	// Add and end some old sessions
	oldTime := time.Now().UTC().Add(-2 * time.Hour)
	for i := 0; i < 2; i++ {
		session := &B2BUASession{
			SessionID: fmt.Sprintf("old-session-%d", i),
			StartTime: oldTime,
		}
		collector.StartSession(session)
		collector.EndSession(session.SessionID, oldTime.Add(5*time.Minute), "BYE", "caller")
	}

	// Add some recent sessions
	recentTime := time.Now().UTC().Add(-10 * time.Minute)
	for i := 0; i < 2; i++ {
		session := &B2BUASession{
			SessionID: fmt.Sprintf("recent-session-%d", i),
			StartTime: recentTime,
		}
		collector.StartSession(session)
	}

	if collector.GetStatsCount() != 4 {
		t.Errorf("Expected 4 total stats, got %d", collector.GetStatsCount())
	}

	// Cleanup stats older than 1 hour
	cleaned := collector.CleanupStats(1 * time.Hour)

	if cleaned != 2 {
		t.Errorf("Expected 2 cleaned stats, got %d", cleaned)
	}

	if collector.GetStatsCount() != 2 {
		t.Errorf("Expected 2 remaining stats, got %d", collector.GetStatsCount())
	}
}

func TestSessionStatsCollector_AverageDurations(t *testing.T) {
	logger := &TestLogger{}
	collector := NewSessionStatsCollector(logger)

	// Add sessions with known durations
	baseTime := time.Now().UTC().Add(-1 * time.Hour)
	
	for i := 0; i < 3; i++ {
		startTime := baseTime.Add(time.Duration(i) * time.Minute)
		connectTime := startTime.Add(2 * time.Second) // 2 second setup
		endTime := connectTime.Add(30 * time.Second)  // 30 second talk
		
		session := &B2BUASession{
			SessionID: fmt.Sprintf("session-%d", i),
			StartTime: startTime,
		}
		
		collector.StartSession(session)
		collector.UpdateSessionConnect(session.SessionID, connectTime, "member")
		collector.EndSession(session.SessionID, endTime, "BYE", "caller")
	}

	avgSetup := collector.GetAverageSetupDuration()
	if avgSetup != 2*time.Second {
		t.Errorf("Expected average setup duration 2s, got %v", avgSetup)
	}

	avgTalk := collector.GetAverageTalkDuration()
	if avgTalk != 30*time.Second {
		t.Errorf("Expected average talk duration 30s, got %v", avgTalk)
	}
}