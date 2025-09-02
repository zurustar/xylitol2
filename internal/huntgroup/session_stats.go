package huntgroup

import (
	"sync"
	"time"

	"github.com/zurustar/xylitol2/internal/logging"
)

// SessionStatistics represents statistics for a B2BUA session
type SessionStatistics struct {
	SessionID       string        `json:"session_id"`
	CallerURI       string        `json:"caller_uri"`
	CalleeURI       string        `json:"callee_uri"`
	StartTime       time.Time     `json:"start_time"`
	ConnectTime     *time.Time    `json:"connect_time,omitempty"`
	EndTime         *time.Time    `json:"end_time,omitempty"`
	Duration        time.Duration `json:"duration"`
	SetupDuration   time.Duration `json:"setup_duration"`   // Time from start to connect
	TalkDuration    time.Duration `json:"talk_duration"`    // Time from connect to end
	EndReason       string        `json:"end_reason"`       // BYE, CANCEL, timeout, etc.
	EndedBy         string        `json:"ended_by"`         // caller, callee, system
	HuntGroupID     *int          `json:"hunt_group_id,omitempty"`
	AnsweredMember  string        `json:"answered_member,omitempty"`
	TotalMembers    int           `json:"total_members,omitempty"`
	CreatedAt       time.Time     `json:"created_at"`
}

// SessionStatsCollector manages session statistics collection and storage
type SessionStatsCollector struct {
	stats  map[string]*SessionStatistics // sessionID -> stats
	mutex  sync.RWMutex
	logger logging.Logger
}

// NewSessionStatsCollector creates a new session statistics collector
func NewSessionStatsCollector(logger logging.Logger) *SessionStatsCollector {
	return &SessionStatsCollector{
		stats:  make(map[string]*SessionStatistics),
		logger: logger,
	}
}

// StartSession creates initial statistics for a new session
func (ssc *SessionStatsCollector) StartSession(session *B2BUASession) {
	ssc.mutex.Lock()
	defer ssc.mutex.Unlock()

	callerURI := ""
	calleeURI := ""
	
	if session.CallerLeg != nil {
		callerURI = session.CallerLeg.FromURI
	}
	if session.CalleeLeg != nil {
		calleeURI = session.CalleeLeg.ToURI
	}

	stats := &SessionStatistics{
		SessionID:   session.SessionID,
		CallerURI:   callerURI,
		CalleeURI:   calleeURI,
		StartTime:   session.StartTime,
		HuntGroupID: session.HuntGroupID,
		CreatedAt:   time.Now().UTC(),
	}

	ssc.stats[session.SessionID] = stats

	ssc.logger.Debug("Session statistics started",
		logging.Field{Key: "session_id", Value: session.SessionID},
		logging.Field{Key: "caller", Value: callerURI},
		logging.Field{Key: "callee", Value: calleeURI})
}

// UpdateSessionConnect updates statistics when a session connects
func (ssc *SessionStatsCollector) UpdateSessionConnect(sessionID string, connectTime time.Time, answeredMember string) {
	ssc.mutex.Lock()
	defer ssc.mutex.Unlock()

	stats, exists := ssc.stats[sessionID]
	if !exists {
		ssc.logger.Warn("Session statistics not found for connect update",
			logging.Field{Key: "session_id", Value: sessionID})
		return
	}

	stats.ConnectTime = &connectTime
	stats.SetupDuration = connectTime.Sub(stats.StartTime)
	if answeredMember != "" {
		stats.AnsweredMember = answeredMember
	}

	ssc.logger.Debug("Session statistics updated for connect",
		logging.Field{Key: "session_id", Value: sessionID},
		logging.Field{Key: "setup_duration", Value: stats.SetupDuration},
		logging.Field{Key: "answered_member", Value: answeredMember})
}

// EndSession finalizes statistics when a session ends
func (ssc *SessionStatsCollector) EndSession(sessionID string, endTime time.Time, endReason string, endedBy string) *SessionStatistics {
	ssc.mutex.Lock()
	defer ssc.mutex.Unlock()

	stats, exists := ssc.stats[sessionID]
	if !exists {
		ssc.logger.Warn("Session statistics not found for end",
			logging.Field{Key: "session_id", Value: sessionID})
		return nil
	}

	stats.EndTime = &endTime
	stats.Duration = endTime.Sub(stats.StartTime)
	stats.EndReason = endReason
	stats.EndedBy = endedBy

	// Calculate talk duration if session was connected
	if stats.ConnectTime != nil {
		stats.TalkDuration = endTime.Sub(*stats.ConnectTime)
	}

	ssc.logger.Info("Session statistics finalized",
		logging.Field{Key: "session_id", Value: sessionID},
		logging.Field{Key: "duration", Value: stats.Duration},
		logging.Field{Key: "setup_duration", Value: stats.SetupDuration},
		logging.Field{Key: "talk_duration", Value: stats.TalkDuration},
		logging.Field{Key: "end_reason", Value: endReason},
		logging.Field{Key: "ended_by", Value: endedBy})

	// Return a copy of the statistics
	statsCopy := *stats
	return &statsCopy
}

// GetSessionStats retrieves statistics for a session
func (ssc *SessionStatsCollector) GetSessionStats(sessionID string) *SessionStatistics {
	ssc.mutex.RLock()
	defer ssc.mutex.RUnlock()

	stats, exists := ssc.stats[sessionID]
	if !exists {
		return nil
	}

	// Return a copy
	statsCopy := *stats
	return &statsCopy
}

// GetAllStats retrieves all session statistics
func (ssc *SessionStatsCollector) GetAllStats() []*SessionStatistics {
	ssc.mutex.RLock()
	defer ssc.mutex.RUnlock()

	allStats := make([]*SessionStatistics, 0, len(ssc.stats))
	for _, stats := range ssc.stats {
		statsCopy := *stats
		allStats = append(allStats, &statsCopy)
	}

	return allStats
}

// CleanupStats removes statistics for completed sessions older than the specified duration
func (ssc *SessionStatsCollector) CleanupStats(olderThan time.Duration) int {
	ssc.mutex.Lock()
	defer ssc.mutex.Unlock()

	cutoffTime := time.Now().UTC().Add(-olderThan)
	cleaned := 0

	for sessionID, stats := range ssc.stats {
		// Only cleanup completed sessions that are old enough
		if stats.EndTime != nil && stats.EndTime.Before(cutoffTime) {
			delete(ssc.stats, sessionID)
			cleaned++
		}
	}

	if cleaned > 0 {
		ssc.logger.Info("Session statistics cleaned up",
			logging.Field{Key: "cleaned_count", Value: cleaned},
			logging.Field{Key: "remaining_count", Value: len(ssc.stats)})
	}

	return cleaned
}

// GetStatsCount returns the number of statistics records
func (ssc *SessionStatsCollector) GetStatsCount() int {
	ssc.mutex.RLock()
	defer ssc.mutex.RUnlock()
	return len(ssc.stats)
}

// GetActiveSessionsCount returns the number of active sessions (not ended)
func (ssc *SessionStatsCollector) GetActiveSessionsCount() int {
	ssc.mutex.RLock()
	defer ssc.mutex.RUnlock()

	active := 0
	for _, stats := range ssc.stats {
		if stats.EndTime == nil {
			active++
		}
	}

	return active
}

// GetCompletedSessionsCount returns the number of completed sessions
func (ssc *SessionStatsCollector) GetCompletedSessionsCount() int {
	ssc.mutex.RLock()
	defer ssc.mutex.RUnlock()

	completed := 0
	for _, stats := range ssc.stats {
		if stats.EndTime != nil {
			completed++
		}
	}

	return completed
}

// GetAverageSetupDuration calculates the average setup duration for completed sessions
func (ssc *SessionStatsCollector) GetAverageSetupDuration() time.Duration {
	ssc.mutex.RLock()
	defer ssc.mutex.RUnlock()

	var totalSetup time.Duration
	count := 0

	for _, stats := range ssc.stats {
		if stats.ConnectTime != nil {
			totalSetup += stats.SetupDuration
			count++
		}
	}

	if count == 0 {
		return 0
	}

	return totalSetup / time.Duration(count)
}

// GetAverageTalkDuration calculates the average talk duration for completed sessions
func (ssc *SessionStatsCollector) GetAverageTalkDuration() time.Duration {
	ssc.mutex.RLock()
	defer ssc.mutex.RUnlock()

	var totalTalk time.Duration
	count := 0

	for _, stats := range ssc.stats {
		if stats.EndTime != nil && stats.ConnectTime != nil {
			totalTalk += stats.TalkDuration
			count++
		}
	}

	if count == 0 {
		return 0
	}

	return totalTalk / time.Duration(count)
}