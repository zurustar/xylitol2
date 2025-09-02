package huntgroup

import (
	"net"
	"sync"
	"time"

	"github.com/zurustar/xylitol2/internal/parser"
	"github.com/zurustar/xylitol2/internal/transaction"
)

// HuntGroupStrategy defines the strategy for calling group members
type HuntGroupStrategy string

const (
	// StrategySimultaneous calls all members simultaneously (ring group)
	StrategySimultaneous HuntGroupStrategy = "simultaneous"
	// StrategySequential calls members one by one in order
	StrategySequential HuntGroupStrategy = "sequential"
	// StrategyRoundRobin calls members in round-robin fashion
	StrategyRoundRobin HuntGroupStrategy = "round_robin"
	// StrategyLongestIdle calls the member who has been idle the longest
	StrategyLongestIdle HuntGroupStrategy = "longest_idle"
)

// HuntGroup represents a hunt group configuration
type HuntGroup struct {
	ID          int                `json:"id" db:"id"`
	Name        string             `json:"name" db:"name"`
	Extension   string             `json:"extension" db:"extension"`     // The extension that triggers this hunt group
	Strategy    HuntGroupStrategy  `json:"strategy" db:"strategy"`       // Call strategy
	RingTimeout int                `json:"ring_timeout" db:"ring_timeout"` // Timeout for each member in seconds
	Enabled     bool               `json:"enabled" db:"enabled"`
	Description string             `json:"description" db:"description"`
	CreatedAt   time.Time          `json:"created_at" db:"created_at"`
	UpdatedAt   time.Time          `json:"updated_at" db:"updated_at"`
	Members     []*HuntGroupMember `json:"members,omitempty"`
}

// HuntGroupMember represents a member of a hunt group
type HuntGroupMember struct {
	ID          int       `json:"id" db:"id"`
	GroupID     int       `json:"group_id" db:"group_id"`
	Extension   string    `json:"extension" db:"extension"`   // Member extension/URI
	Priority    int       `json:"priority" db:"priority"`     // Priority order (lower = higher priority)
	Enabled     bool      `json:"enabled" db:"enabled"`
	Timeout     int       `json:"timeout" db:"timeout"`       // Individual timeout override
	CreatedAt   time.Time `json:"created_at" db:"created_at"`
	UpdatedAt   time.Time `json:"updated_at" db:"updated_at"`
}

// CallSession represents an active call session in a hunt group
type CallSession struct {
	ID            string                 `json:"id"`
	GroupID       int                    `json:"group_id"`
	CallerURI     string                 `json:"caller_uri"`
	OriginalINVITE *parser.SIPMessage    `json:"-"`
	MemberCalls   map[string]*MemberCall `json:"member_calls"`
	StartTime     time.Time              `json:"start_time"`
	Status        CallSessionStatus      `json:"status"`
	AnsweredBy    string                 `json:"answered_by,omitempty"`
	AnsweredAt    *time.Time             `json:"answered_at,omitempty"`
}

// MemberCall represents a call to a hunt group member
type MemberCall struct {
	MemberExtension string            `json:"member_extension"`
	CallID          string            `json:"call_id"`
	Status          MemberCallStatus  `json:"status"`
	StartTime       time.Time         `json:"start_time"`
	AnswerTime      *time.Time        `json:"answer_time,omitempty"`
	EndTime         *time.Time        `json:"end_time,omitempty"`
	Transaction     interface{}       `json:"-"` // Transaction interface
}

// CallSessionStatus represents the status of a hunt group call session
type CallSessionStatus string

const (
	SessionStatusRinging   CallSessionStatus = "ringing"
	SessionStatusAnswered  CallSessionStatus = "answered"
	SessionStatusCancelled CallSessionStatus = "cancelled"
	SessionStatusFailed    CallSessionStatus = "failed"
	SessionStatusCompleted CallSessionStatus = "completed"
)

// MemberCallStatus represents the status of a call to a hunt group member
type MemberCallStatus string

const (
	MemberCallStatusRinging   MemberCallStatus = "ringing"
	MemberCallStatusAnswered  MemberCallStatus = "answered"
	MemberCallStatusBusy      MemberCallStatus = "busy"
	MemberCallStatusNoAnswer  MemberCallStatus = "no_answer"
	MemberCallStatusFailed    MemberCallStatus = "failed"
	MemberCallStatusCancelled MemberCallStatus = "cancelled"
)

// HuntGroupManager defines the interface for hunt group management
type HuntGroupManager interface {
	// Group management
	CreateGroup(group *HuntGroup) error
	GetGroup(id int) (*HuntGroup, error)
	GetGroupByExtension(extension string) (*HuntGroup, error)
	UpdateGroup(group *HuntGroup) error
	DeleteGroup(id int) error
	ListGroups() ([]*HuntGroup, error)
	EnableGroup(groupID int) error
	DisableGroup(groupID int) error

	// Member management
	AddMember(groupID int, member *HuntGroupMember) error
	RemoveMember(groupID int, memberID int) error
	UpdateMember(member *HuntGroupMember) error
	GetGroupMembers(groupID int) ([]*HuntGroupMember, error)
	EnableMember(groupID int, memberID int) error
	DisableMember(groupID int, memberID int) error

	// Call session management
	CreateSession(session *CallSession) error
	GetSession(sessionID string) (*CallSession, error)
	UpdateSession(session *CallSession) error
	EndSession(sessionID string) error
	GetActiveSessions() ([]*CallSession, error)
	
	// Statistics
	GetCallStatistics(groupID int) (*CallStatistics, error)
}

// HuntGroupEngine defines the interface for hunt group call processing
type HuntGroupEngine interface {
	// Process incoming call to hunt group
	ProcessIncomingCall(invite *parser.SIPMessage, group *HuntGroup) (*CallSession, error)
	
	// Handle member call responses
	HandleMemberResponse(sessionID string, memberExtension string, response *parser.SIPMessage) error
	
	// Cancel all pending calls in a session
	CancelSession(sessionID string) error
	
	// Get call statistics
	GetCallStatistics(groupID int) (*CallStatistics, error)
}

// CallStatistics represents call statistics for a hunt group
type CallStatistics struct {
	GroupID           int           `json:"group_id"`
	TotalCalls        int           `json:"total_calls"`
	AnsweredCalls     int           `json:"answered_calls"`
	MissedCalls       int           `json:"missed_calls"`
	AverageRingTime   time.Duration `json:"average_ring_time"`
	AverageCallLength time.Duration `json:"average_call_length"`
	BusiestMember     string        `json:"busiest_member"`
	LastCallTime      *time.Time    `json:"last_call_time"`
}

// B2BUASession represents a Back-to-Back User Agent session with enhanced state management
type B2BUASession struct {
	SessionID     string                 `json:"session_id"`
	CallerLeg     *CallLeg               `json:"caller_leg"`
	CalleeLeg     *CallLeg               `json:"callee_leg"`
	PendingLegs   map[string]*CallLeg    `json:"pending_legs,omitempty"`  // For hunt group parallel forking
	AnsweredLegID string                 `json:"answered_leg_id,omitempty"` // Which leg answered first
	Status        B2BUASessionStatus     `json:"status"`
	StartTime     time.Time              `json:"start_time"`
	ConnectTime   *time.Time             `json:"connect_time,omitempty"`
	EndTime       *time.Time             `json:"end_time,omitempty"`
	HuntGroupID   *int                   `json:"hunt_group_id,omitempty"`
	LastActivity  time.Time              `json:"last_activity"`
	SDPOffer      string                 `json:"sdp_offer,omitempty"`     // SDP from caller
	SDPAnswer     string                 `json:"sdp_answer,omitempty"`    // SDP from answering callee
	mutex         sync.RWMutex           `json:"-"`
}

// CallLeg represents one leg of a B2BUA session with enhanced dialog management
type CallLeg struct {
	LegID         string                 `json:"leg_id"`
	CallID        string                 `json:"call_id"`
	FromURI       string                 `json:"from_uri"`
	ToURI         string                 `json:"to_uri"`
	FromTag       string                 `json:"from_tag"`
	ToTag         string                 `json:"to_tag"`
	ContactURI    string                 `json:"contact_uri"`
	RemoteTarget  string                 `json:"remote_target,omitempty"` // Contact from remote party
	RouteSet      []string               `json:"route_set,omitempty"`     // Record-Route headers
	Status        CallLegStatus          `json:"status"`
	RemoteAddr    net.Addr               `json:"-"`
	LocalSDP      string                 `json:"local_sdp,omitempty"`
	RemoteSDP     string                 `json:"remote_sdp,omitempty"`
	LastCSeq      uint32                 `json:"last_cseq"`
	Transaction   transaction.Transaction `json:"-"`
	DialogID      string                 `json:"dialog_id,omitempty"`     // Associated SIP dialog ID
	CreatedAt     time.Time              `json:"created_at"`
	ConnectedAt   *time.Time             `json:"connected_at,omitempty"`
	mutex         sync.RWMutex           `json:"-"`
}

// B2BUASessionStatus represents the status of a B2BUA session
type B2BUASessionStatus string

const (
	B2BUAStatusInitial     B2BUASessionStatus = "initial"
	B2BUAStatusInitiating  B2BUASessionStatus = "initiating"
	B2BUAStatusProceeding  B2BUASessionStatus = "proceeding"  // Received 1xx responses
	B2BUAStatusRinging     B2BUASessionStatus = "ringing"
	B2BUAStatusConnected   B2BUASessionStatus = "connected"
	B2BUAStatusEnding      B2BUASessionStatus = "ending"
	B2BUAStatusEnded       B2BUASessionStatus = "ended"
	B2BUAStatusFailed      B2BUASessionStatus = "failed"
	B2BUAStatusCancelled   B2BUASessionStatus = "cancelled"
)

// CallLegStatus represents the status of a call leg
type CallLegStatus string

const (
	CallLegStatusInitial     CallLegStatus = "initial"
	CallLegStatusInitiating  CallLegStatus = "initiating"
	CallLegStatusProceeding  CallLegStatus = "proceeding"  // Received 1xx responses
	CallLegStatusRinging     CallLegStatus = "ringing"
	CallLegStatusConnected   CallLegStatus = "connected"
	CallLegStatusEnding      CallLegStatus = "ending"
	CallLegStatusEnded       CallLegStatus = "ended"
	CallLegStatusFailed      CallLegStatus = "failed"
	CallLegStatusCancelled   CallLegStatus = "cancelled"
)

// B2BUAManager defines the interface for B2BUA session management
type B2BUAManager interface {
	// Session management
	CreateSession(callerInvite *parser.SIPMessage, calleeURI string) (*B2BUASession, error)
	CreateHuntGroupSession(callerInvite *parser.SIPMessage, huntGroup *HuntGroup) (*B2BUASession, error)
	GetSession(sessionID string) (*B2BUASession, error)
	GetSessionByCallID(callID string) (*B2BUASession, error)
	GetSessionByLegID(legID string) (*B2BUASession, error)
	UpdateSession(session *B2BUASession) error
	EndSession(sessionID string) error
	GetActiveSessions() ([]*B2BUASession, error)
	CleanupExpiredSessions() error
	
	// Message handling
	HandleCallerMessage(sessionID string, message *parser.SIPMessage) error
	HandleCalleeMessage(sessionID string, message *parser.SIPMessage) error
	
	// Call control
	BridgeCalls(sessionID string) error
	TransferCall(sessionID string, targetURI string) error
	
	// Hunt group specific
	AddPendingLeg(sessionID string, memberURI string) (*CallLeg, error)
	HandleMemberAnswer(sessionID string, legID string, response *parser.SIPMessage) error
	CancelPendingLegs(sessionID string, exceptLegID string) error
}

// Session management methods for B2BUASession
func (s *B2BUASession) Lock() {
	s.mutex.Lock()
}

func (s *B2BUASession) Unlock() {
	s.mutex.Unlock()
}

func (s *B2BUASession) RLock() {
	s.mutex.RLock()
}

func (s *B2BUASession) RUnlock() {
	s.mutex.RUnlock()
}

func (s *B2BUASession) UpdateActivity() {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	s.LastActivity = time.Now().UTC()
}

func (s *B2BUASession) SetStatus(status B2BUASessionStatus) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	s.Status = status
	s.LastActivity = time.Now().UTC()
}

func (s *B2BUASession) GetStatus() B2BUASessionStatus {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	return s.Status
}

func (s *B2BUASession) IsActive() bool {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	return s.Status == B2BUAStatusConnected || s.Status == B2BUAStatusRinging || s.Status == B2BUAStatusProceeding
}

func (s *B2BUASession) IsTerminated() bool {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	return s.Status == B2BUAStatusEnded || s.Status == B2BUAStatusFailed || s.Status == B2BUAStatusCancelled
}

func (s *B2BUASession) AddPendingLeg(leg *CallLeg) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	if s.PendingLegs == nil {
		s.PendingLegs = make(map[string]*CallLeg)
	}
	s.PendingLegs[leg.LegID] = leg
	s.LastActivity = time.Now().UTC()
}

func (s *B2BUASession) RemovePendingLeg(legID string) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	if s.PendingLegs != nil {
		delete(s.PendingLegs, legID)
	}
	s.LastActivity = time.Now().UTC()
}

func (s *B2BUASession) GetPendingLeg(legID string) *CallLeg {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	if s.PendingLegs != nil {
		return s.PendingLegs[legID]
	}
	return nil
}

func (s *B2BUASession) GetAllPendingLegs() []*CallLeg {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	legs := make([]*CallLeg, 0, len(s.PendingLegs))
	for _, leg := range s.PendingLegs {
		legs = append(legs, leg)
	}
	return legs
}

func (s *B2BUASession) SetAnsweredLeg(legID string) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	s.AnsweredLegID = legID
	if leg, exists := s.PendingLegs[legID]; exists {
		s.CalleeLeg = leg
		delete(s.PendingLegs, legID)
	}
	now := time.Now().UTC()
	s.ConnectTime = &now
	s.LastActivity = now
}

// Call leg management methods
func (l *CallLeg) Lock() {
	l.mutex.Lock()
}

func (l *CallLeg) Unlock() {
	l.mutex.Unlock()
}

func (l *CallLeg) RLock() {
	l.mutex.RLock()
}

func (l *CallLeg) RUnlock() {
	l.mutex.RUnlock()
}

func (l *CallLeg) SetStatus(status CallLegStatus) {
	l.mutex.Lock()
	defer l.mutex.Unlock()
	l.Status = status
	if status == CallLegStatusConnected && l.ConnectedAt == nil {
		now := time.Now().UTC()
		l.ConnectedAt = &now
	}
}

func (l *CallLeg) GetStatus() CallLegStatus {
	l.mutex.RLock()
	defer l.mutex.RUnlock()
	return l.Status
}

func (l *CallLeg) IsActive() bool {
	l.mutex.RLock()
	defer l.mutex.RUnlock()
	return l.Status == CallLegStatusConnected || l.Status == CallLegStatusRinging || l.Status == CallLegStatusProceeding
}

func (l *CallLeg) IsTerminated() bool {
	l.mutex.RLock()
	defer l.mutex.RUnlock()
	return l.Status == CallLegStatusEnded || l.Status == CallLegStatusFailed || l.Status == CallLegStatusCancelled
}

func (l *CallLeg) UpdateCSeq(cseq uint32) {
	l.mutex.Lock()
	defer l.mutex.Unlock()
	if cseq > l.LastCSeq {
		l.LastCSeq = cseq
	}
}

func (l *CallLeg) GetNextCSeq() uint32 {
	l.mutex.Lock()
	defer l.mutex.Unlock()
	l.LastCSeq++
	return l.LastCSeq
}