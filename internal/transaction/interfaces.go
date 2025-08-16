package transaction

import (
	"github.com/zurustar/xylitol2/internal/parser"
)

// TransactionState represents the state of a SIP transaction
type TransactionState int

const (
	StateTrying TransactionState = iota
	StateCalling
	StateProceeding
	StateCompleted
	StateConfirmed
	StateTerminated
)

// String returns the string representation of the transaction state
func (ts TransactionState) String() string {
	switch ts {
	case StateTrying:
		return "Trying"
	case StateCalling:
		return "Calling"
	case StateProceeding:
		return "Proceeding"
	case StateCompleted:
		return "Completed"
	case StateConfirmed:
		return "Confirmed"
	case StateTerminated:
		return "Terminated"
	default:
		return "Unknown"
	}
}

// Transaction defines the interface for SIP transactions
type Transaction interface {
	GetState() TransactionState
	ProcessMessage(msg *parser.SIPMessage) error
	SendResponse(response *parser.SIPMessage) error
	GetID() string
	IsClient() bool
}

// TransactionManager defines the interface for managing SIP transactions
type TransactionManager interface {
	CreateTransaction(msg *parser.SIPMessage) Transaction
	FindTransaction(msg *parser.SIPMessage) Transaction
	CleanupExpired()
}