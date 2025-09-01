package huntgroup

import (
	"time"

	"github.com/zurustar/xylitol2/internal/parser"
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

	// Member management
	AddMember(groupID int, member *HuntGroupMember) error
	RemoveMember(groupID int, memberID int) error
	UpdateMember(member *HuntGroupMember) error
	GetGroupMembers(groupID int) ([]*HuntGroupMember, error)

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

// B2BUASession represents a Back-to-Back User Agent session
type B2BUASession struct {
	SessionID     string                 `json:"session_id"`
	CallerLeg     *CallLeg               `json:"caller_leg"`
	CalleeLeg     *CallLeg               `json:"callee_leg"`
	Status        B2BUASessionStatus     `json:"status"`
	StartTime     time.Time              `json:"start_time"`
	ConnectTime   *time.Time             `json:"connect_time,omitempty"`
	EndTime       *time.Time             `json:"end_time,omitempty"`
	HuntGroupID   *int                   `json:"hunt_group_id,omitempty"`
}

// CallLeg represents one leg of a B2BUA session
type CallLeg struct {
	CallID      string            `json:"call_id"`
	FromURI     string            `json:"from_uri"`
	ToURI       string            `json:"to_uri"`
	ContactURI  string            `json:"contact_uri"`
	Status      CallLegStatus     `json:"status"`
	Transaction interface{}       `json:"-"`
}

// B2BUASessionStatus represents the status of a B2BUA session
type B2BUASessionStatus string

const (
	B2BUAStatusInitiating B2BUASessionStatus = "initiating"
	B2BUAStatusRinging    B2BUASessionStatus = "ringing"
	B2BUAStatusConnected  B2BUASessionStatus = "connected"
	B2BUAStatusEnding     B2BUASessionStatus = "ending"
	B2BUAStatusEnded      B2BUASessionStatus = "ended"
)

// CallLegStatus represents the status of a call leg
type CallLegStatus string

const (
	CallLegStatusInitiating CallLegStatus = "initiating"
	CallLegStatusRinging    CallLegStatus = "ringing"
	CallLegStatusConnected  CallLegStatus = "connected"
	CallLegStatusEnding     CallLegStatus = "ending"
	CallLegStatusEnded      CallLegStatus = "ended"
)

// B2BUAManager defines the interface for B2BUA session management
type B2BUAManager interface {
	// Session management
	CreateSession(callerInvite *parser.SIPMessage, calleeURI string) (*B2BUASession, error)
	GetSession(sessionID string) (*B2BUASession, error)
	UpdateSession(session *B2BUASession) error
	EndSession(sessionID string) error
	
	// Message handling
	HandleCallerMessage(sessionID string, message *parser.SIPMessage) error
	HandleCalleeMessage(sessionID string, message *parser.SIPMessage) error
	
	// Call control
	BridgeCalls(sessionID string) error
	TransferCall(sessionID string, targetURI string) error
}