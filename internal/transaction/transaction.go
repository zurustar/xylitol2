package transaction

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/zurustar/xylitol2/internal/parser"
)

// Timer constants as defined in RFC3261
const (
	TimerT1 = 500 * time.Millisecond  // RTT estimate
	TimerT2 = 4 * time.Second         // Maximum retransmit interval
	TimerT4 = 5 * time.Second         // Maximum duration a message will remain in the network
)

// Transaction timer types
type TimerType int

const (
	TimerA TimerType = iota // INVITE request retransmit
	TimerB                  // INVITE transaction timeout
	TimerD                  // Wait time for response retransmits
	TimerE                  // Non-INVITE request retransmit
	TimerF                  // Non-INVITE transaction timeout
	TimerG                  // INVITE response retransmit
	TimerH                  // Wait time for ACK receipt
	TimerI                  // Wait time for ACK retransmits
	TimerJ                  // Wait time for non-INVITE request retransmits
	TimerK                  // Wait time for response retransmits
)

// TransactionTimer represents a transaction timer
type TransactionTimer struct {
	Type     TimerType
	Duration time.Duration
	Timer    *time.Timer
	Callback func()
}

// BaseTransaction provides common functionality for all transactions
type BaseTransaction struct {
	id          string
	state       TransactionState
	isClient    bool
	method      string
	branch      string
	callID      string
	fromTag     string
	toTag        string
	cseq        uint32
	timers      map[TimerType]*TransactionTimer
	mutex       sync.RWMutex
	lastRequest *parser.SIPMessage
	lastResponse *parser.SIPMessage
	transport   string
	created     time.Time
}

// NewBaseTransaction creates a new base transaction
func NewBaseTransaction(msg *parser.SIPMessage, isClient bool) *BaseTransaction {
	bt := &BaseTransaction{
		id:       generateTransactionID(msg),
		isClient: isClient,
		timers:   make(map[TimerType]*TransactionTimer),
		created:  time.Now(),
	}

	if msg.IsRequest() {
		bt.method = msg.GetMethod()
		bt.callID = msg.GetHeader(parser.HeaderCallID)
		bt.fromTag = extractTag(msg.GetHeader(parser.HeaderFrom))
		bt.toTag = extractTag(msg.GetHeader(parser.HeaderTo))
		bt.branch = extractBranch(msg.GetHeader(parser.HeaderVia))
		bt.cseq = extractCSeq(msg.GetHeader(parser.HeaderCSeq))
		bt.lastRequest = msg.Clone()
	}

	bt.transport = msg.Transport

	return bt
}

// GetID returns the transaction ID
func (bt *BaseTransaction) GetID() string {
	bt.mutex.RLock()
	defer bt.mutex.RUnlock()
	return bt.id
}

// GetState returns the current transaction state
func (bt *BaseTransaction) GetState() TransactionState {
	bt.mutex.RLock()
	defer bt.mutex.RUnlock()
	return bt.state
}

// IsClient returns true if this is a client transaction
func (bt *BaseTransaction) IsClient() bool {
	return bt.isClient
}

// setState sets the transaction state
func (bt *BaseTransaction) setState(state TransactionState) {
	bt.mutex.Lock()
	defer bt.mutex.Unlock()
	bt.state = state
}

// SetTimer sets a timer for the transaction
func (bt *BaseTransaction) SetTimer(timerType TimerType, duration time.Duration, callback func()) {
	bt.mutex.Lock()
	defer bt.mutex.Unlock()

	// Cancel existing timer if it exists
	if existingTimer, exists := bt.timers[timerType]; exists {
		existingTimer.Timer.Stop()
	}

	// Create new timer
	timer := &TransactionTimer{
		Type:     timerType,
		Duration: duration,
		Callback: callback,
	}

	timer.Timer = time.AfterFunc(duration, func() {
		bt.mutex.Lock()
		delete(bt.timers, timerType)
		bt.mutex.Unlock()
		callback()
	})

	bt.timers[timerType] = timer
}

// CancelTimer cancels a specific timer
func (bt *BaseTransaction) CancelTimer(timerType TimerType) {
	bt.mutex.Lock()
	defer bt.mutex.Unlock()

	if timer, exists := bt.timers[timerType]; exists {
		timer.Timer.Stop()
		delete(bt.timers, timerType)
	}
}

// CancelAllTimers cancels all active timers
func (bt *BaseTransaction) CancelAllTimers() {
	bt.mutex.Lock()
	defer bt.mutex.Unlock()

	for _, timer := range bt.timers {
		timer.Timer.Stop()
	}
	bt.timers = make(map[TimerType]*TransactionTimer)
}

// IsExpired checks if the transaction has expired
func (bt *BaseTransaction) IsExpired() bool {
	bt.mutex.RLock()
	defer bt.mutex.RUnlock()
	
	// Transaction expires after 32 seconds for INVITE, 64*T1 for non-INVITE
	var expireTime time.Duration
	if bt.method == parser.MethodINVITE {
		expireTime = 32 * time.Second
	} else {
		expireTime = 64 * TimerT1
	}
	
	return time.Since(bt.created) > expireTime
}

// generateTransactionID generates a unique transaction ID
func generateTransactionID(msg *parser.SIPMessage) string {
	// Transaction ID is based on branch parameter, method, and Call-ID
	branch := extractBranch(msg.GetHeader(parser.HeaderVia))
	method := msg.GetMethod()
	callID := msg.GetHeader(parser.HeaderCallID)
	
	if branch != "" && strings.HasPrefix(branch, "z9hG4bK") {
		// RFC3261 compliant branch parameter
		if method == parser.MethodACK || method == parser.MethodCANCEL {
			// ACK and CANCEL use the same transaction as the original INVITE
			return fmt.Sprintf("%s-%s-%s", branch, parser.MethodINVITE, callID)
		}
		return fmt.Sprintf("%s-%s-%s", branch, method, callID)
	}
	
	// Fallback for non-compliant branch parameters
	fromTag := extractTag(msg.GetHeader(parser.HeaderFrom))
	cseq := msg.GetHeader(parser.HeaderCSeq)
	return fmt.Sprintf("%s-%s-%s-%s", callID, fromTag, cseq, method)
}

// extractBranch extracts the branch parameter from Via header
func extractBranch(via string) string {
	if via == "" {
		return ""
	}
	
	// Look for branch parameter
	parts := strings.Split(via, ";")
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if strings.HasPrefix(part, "branch=") {
			return strings.TrimPrefix(part, "branch=")
		}
	}
	
	return ""
}

// extractTag extracts the tag parameter from From/To header
func extractTag(header string) string {
	if header == "" {
		return ""
	}
	
	// Look for tag parameter
	parts := strings.Split(header, ";")
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if strings.HasPrefix(part, "tag=") {
			return strings.TrimPrefix(part, "tag=")
		}
	}
	
	return ""
}

// extractCSeq extracts the sequence number from CSeq header
func extractCSeq(cseq string) uint32 {
	if cseq == "" {
		return 0
	}
	
	parts := strings.Fields(cseq)
	if len(parts) < 1 {
		return 0
	}
	
	var seq uint32
	fmt.Sscanf(parts[0], "%d", &seq)
	return seq
}

// generateRandomString generates a random string for branch parameters
func generateRandomString(length int) string {
	bytes := make([]byte, length)
	rand.Read(bytes)
	return hex.EncodeToString(bytes)[:length]
}