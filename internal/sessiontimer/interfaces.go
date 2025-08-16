package sessiontimer

import (
	"time"

	"github.com/zurustar/xylitol2/internal/parser"
)

// Session represents an active SIP session with timer information
type Session struct {
	CallID         string
	SessionExpires time.Time
	Refresher      string
	MinSE          int
}

// SessionTimerManager defines the interface for managing session timers
type SessionTimerManager interface {
	CreateSession(callID string, sessionExpires int) *Session
	RefreshSession(callID string) error
	CleanupExpiredSessions()
	IsSessionTimerRequired(msg *parser.SIPMessage) bool
	StartCleanupTimer()
	StopCleanupTimer()
	SetSessionTerminationCallback(callback func(callID string))
	RemoveSession(callID string)
}