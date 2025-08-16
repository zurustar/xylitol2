package sessiontimer

import (
	"fmt"
	"sync"
	"time"

	"github.com/zurustar/xylitol2/internal/logging"
	"github.com/zurustar/xylitol2/internal/parser"
)

// Manager implements the SessionTimerManager interface
type Manager struct {
	sessions              map[string]*Session
	defaultExpires        int
	minSE                 int
	maxSE                 int
	logger                logging.Logger
	mu                    sync.RWMutex
	cleanupTicker         *time.Ticker
	stopCleanup           chan bool
	terminationCallback   func(callID string)
}

// NewManager creates a new session timer manager
func NewManager(defaultExpires, minSE, maxSE int, logger logging.Logger) *Manager {
	return &Manager{
		sessions:       make(map[string]*Session),
		defaultExpires: defaultExpires,
		minSE:          minSE,
		maxSE:          maxSE,
		logger:         logger,
		stopCleanup:    make(chan bool),
	}
}

// CreateSession creates a new session with timer information
func (m *Manager) CreateSession(callID string, sessionExpires int) *Session {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Apply limits
	if sessionExpires < m.minSE {
		sessionExpires = m.minSE
	}
	if sessionExpires > m.maxSE {
		sessionExpires = m.maxSE
	}

	session := &Session{
		CallID:         callID,
		SessionExpires: time.Now().Add(time.Duration(sessionExpires) * time.Second),
		Refresher:      "uac", // Default to user agent client
		MinSE:          m.minSE,
	}

	m.sessions[callID] = session
	m.logger.Info("Session created", logging.Field{Key: "call_id", Value: callID}, logging.Field{Key: "expires", Value: sessionExpires})

	return session
}

// RefreshSession refreshes an existing session timer
func (m *Manager) RefreshSession(callID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	session, exists := m.sessions[callID]
	if !exists {
		return fmt.Errorf("session not found: %s", callID)
	}

	// Refresh the session timer
	session.SessionExpires = time.Now().Add(time.Duration(m.defaultExpires) * time.Second)
	m.logger.Info("Session refreshed", logging.Field{Key: "call_id", Value: callID})

	return nil
}

// CleanupExpiredSessions removes expired sessions
func (m *Manager) CleanupExpiredSessions() {
	m.cleanupExpiredSessionsWithCallback()
}

// IsSessionTimerRequired checks if Session-Timer is required for the message
func (m *Manager) IsSessionTimerRequired(msg *parser.SIPMessage) bool {
	if !msg.IsRequest() {
		return false
	}

	method := msg.GetMethod()
	// Session-Timer is required for INVITE requests
	return method == parser.MethodINVITE
}

// GetSession retrieves a session by Call-ID
func (m *Manager) GetSession(callID string) *Session {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.sessions[callID]
}

// RemoveSession removes a session
func (m *Manager) RemoveSession(callID string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.sessions[callID]; exists {
		delete(m.sessions, callID)
		m.logger.Info("Session removed", logging.Field{Key: "call_id", Value: callID})
	}
}

// GetSessionCount returns the number of active sessions
func (m *Manager) GetSessionCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return len(m.sessions)
}

// GetDefaultExpires returns the default session expires value
func (m *Manager) GetDefaultExpires() int {
	return m.defaultExpires
}

// GetMinSE returns the minimum session expires value
func (m *Manager) GetMinSE() int {
	return m.minSE
}

// GetMaxSE returns the maximum session expires value
func (m *Manager) GetMaxSE() int {
	return m.maxSE
}

// StartCleanupTimer starts the background cleanup timer
func (m *Manager) StartCleanupTimer() {
	// Run cleanup every 30 seconds
	m.cleanupTicker = time.NewTicker(30 * time.Second)
	
	go func() {
		for {
			select {
			case <-m.cleanupTicker.C:
				m.cleanupExpiredSessionsWithCallback()
			case <-m.stopCleanup:
				return
			}
		}
	}()
	
	m.logger.Info("Session timer cleanup started")
}

// StopCleanupTimer stops the background cleanup timer
func (m *Manager) StopCleanupTimer() {
	if m.cleanupTicker != nil {
		m.cleanupTicker.Stop()
		m.stopCleanup <- true
		m.logger.Info("Session timer cleanup stopped")
	}
}

// SetSessionTerminationCallback sets the callback function for session termination
func (m *Manager) SetSessionTerminationCallback(callback func(callID string)) {
	m.terminationCallback = callback
}

// cleanupExpiredSessionsWithCallback removes expired sessions and calls termination callback
func (m *Manager) cleanupExpiredSessionsWithCallback() {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	expiredSessions := make([]string, 0)

	for callID, session := range m.sessions {
		if now.After(session.SessionExpires) {
			expiredSessions = append(expiredSessions, callID)
		}
	}

	for _, callID := range expiredSessions {
		delete(m.sessions, callID)
		m.logger.Info("Session expired and removed", logging.Field{Key: "call_id", Value: callID})
		
		// Call termination callback if set
		if m.terminationCallback != nil {
			go m.terminationCallback(callID)
		}
	}

	if len(expiredSessions) > 0 {
		m.logger.Info("Cleaned up expired sessions", logging.Field{Key: "count", Value: len(expiredSessions)})
	}
}