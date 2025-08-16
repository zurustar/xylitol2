package transaction

import (
	"fmt"

	"github.com/zurustar/xylitol2/internal/parser"
)

// ServerTransaction represents a server transaction
type ServerTransaction struct {
	*BaseTransaction
	sendMessage func(*parser.SIPMessage) error
}

// NewServerTransaction creates a new server transaction
func NewServerTransaction(msg *parser.SIPMessage, sendFunc func(*parser.SIPMessage) error) *ServerTransaction {
	st := &ServerTransaction{
		BaseTransaction: NewBaseTransaction(msg, false),
		sendMessage:     sendFunc,
	}

	// Set initial state based on method
	if msg.GetMethod() == parser.MethodINVITE {
		st.setState(StateProceeding)
	} else {
		st.setState(StateTrying)
	}

	return st
}

// ProcessMessage processes an incoming message for the server transaction
func (st *ServerTransaction) ProcessMessage(msg *parser.SIPMessage) error {
	if msg.IsResponse() {
		return fmt.Errorf("server transaction received response message")
	}

	// Handle retransmissions
	if st.method == parser.MethodINVITE {
		return st.processInviteRequest(msg)
	} else {
		return st.processNonInviteRequest(msg)
	}
}

// processInviteRequest processes requests for INVITE server transactions
func (st *ServerTransaction) processInviteRequest(msg *parser.SIPMessage) error {
	switch st.GetState() {
	case StateProceeding:
		// Retransmit last response if we have one
		if st.lastResponse != nil && st.sendMessage != nil {
			st.sendMessage(st.lastResponse)
		}

	case StateCompleted:
		// Retransmit final response
		if st.lastResponse != nil && st.sendMessage != nil {
			st.sendMessage(st.lastResponse)
		}

	case StateConfirmed:
		// Absorb retransmissions

	case StateTerminated:
		// Ignore all messages
	}

	return nil
}

// processNonInviteRequest processes requests for non-INVITE server transactions
func (st *ServerTransaction) processNonInviteRequest(msg *parser.SIPMessage) error {
	switch st.GetState() {
	case StateTrying:
		// Retransmit last response if we have one
		if st.lastResponse != nil && st.sendMessage != nil {
			st.sendMessage(st.lastResponse)
		}

	case StateProceeding:
		// Retransmit last response if we have one
		if st.lastResponse != nil && st.sendMessage != nil {
			st.sendMessage(st.lastResponse)
		}

	case StateCompleted:
		// Retransmit final response
		if st.lastResponse != nil && st.sendMessage != nil {
			st.sendMessage(st.lastResponse)
		}

	case StateTerminated:
		// Ignore all messages
	}

	return nil
}

// SendResponse sends a response for the server transaction
func (st *ServerTransaction) SendResponse(response *parser.SIPMessage) error {
	if !response.IsResponse() {
		return fmt.Errorf("cannot send request as response")
	}

	statusCode := response.GetStatusCode()
	st.lastResponse = response.Clone()

	if st.method == parser.MethodINVITE {
		return st.sendInviteResponse(statusCode, response)
	} else {
		return st.sendNonInviteResponse(statusCode, response)
	}
}

// sendInviteResponse sends a response for INVITE server transactions
func (st *ServerTransaction) sendInviteResponse(statusCode int, response *parser.SIPMessage) error {
	// Send the response
	if st.sendMessage != nil {
		if err := st.sendMessage(response); err != nil {
			return err
		}
	}

	switch st.GetState() {
	case StateProceeding:
		if statusCode >= 100 && statusCode < 200 {
			// Provisional response - stay in Proceeding state
			// No state change needed
		} else if statusCode >= 200 && statusCode < 300 {
			// 2xx response - terminate immediately
			st.setState(StateTerminated)
		} else if statusCode >= 300 {
			// 3xx-6xx response - go to Completed state
			st.setState(StateCompleted)
			st.startTimerG(response)
			st.startTimerH()
		}

	case StateCompleted:
		// Can only send the same final response
		if statusCode >= 300 {
			// Retransmit final response
			st.startTimerG(response)
		}
	}

	return nil
}

// sendNonInviteResponse sends a response for non-INVITE server transactions
func (st *ServerTransaction) sendNonInviteResponse(statusCode int, response *parser.SIPMessage) error {
	// Send the response
	if st.sendMessage != nil {
		if err := st.sendMessage(response); err != nil {
			return err
		}
	}

	switch st.GetState() {
	case StateTrying:
		if statusCode >= 100 && statusCode < 200 {
			// Provisional response
			st.setState(StateProceeding)
		} else if statusCode >= 200 {
			// Final response
			st.setState(StateCompleted)
			st.startTimerJ()
		}

	case StateProceeding:
		if statusCode >= 200 {
			// Final response
			st.setState(StateCompleted)
			st.startTimerJ()
		}
		// Ignore additional 1xx responses
	}

	return nil
}

// startTimerG starts Timer G for INVITE server transactions (response retransmission)
func (st *ServerTransaction) startTimerG(response *parser.SIPMessage) {
	st.SetTimer(TimerG, TimerT1, func() {
		if st.GetState() == StateCompleted {
			// Retransmit response
			if st.sendMessage != nil {
				st.sendMessage(response)
			}
			// Set next retransmission with doubled interval, up to T2
			interval := TimerT1 * 2
			if interval > TimerT2 {
				interval = TimerT2
			}
			st.SetTimer(TimerG, interval, func() {
				if st.GetState() == StateCompleted && st.sendMessage != nil {
					st.sendMessage(response)
				}
			})
		}
	})
}

// startTimerH starts Timer H for INVITE server transactions (wait for ACK)
func (st *ServerTransaction) startTimerH() {
	st.SetTimer(TimerH, 64*TimerT1, func() {
		if st.GetState() == StateCompleted {
			st.setState(StateTerminated)
		}
	})
}

// startTimerI starts Timer I for INVITE server transactions (wait for ACK retransmissions)
func (st *ServerTransaction) startTimerI() {
	duration := TimerT4
	if st.transport == "UDP" {
		duration = TimerT4
	} else {
		duration = 0 // Immediate transition for reliable transports
	}

	if duration > 0 {
		st.SetTimer(TimerI, duration, func() {
			st.setState(StateTerminated)
		})
	} else {
		st.setState(StateTerminated)
	}
}

// startTimerJ starts Timer J for non-INVITE server transactions
func (st *ServerTransaction) startTimerJ() {
	duration := TimerT4
	if st.transport == "UDP" {
		duration = 64 * TimerT1
	} else {
		duration = 0 // Immediate transition for reliable transports
	}

	if duration > 0 {
		st.SetTimer(TimerJ, duration, func() {
			st.setState(StateTerminated)
		})
	} else {
		st.setState(StateTerminated)
	}
}

// HandleACK handles ACK requests for INVITE server transactions
func (st *ServerTransaction) HandleACK(ack *parser.SIPMessage) error {
	if st.method != parser.MethodINVITE {
		return fmt.Errorf("ACK received for non-INVITE transaction")
	}

	switch st.GetState() {
	case StateCompleted:
		// ACK received, transition to Confirmed state
		st.CancelAllTimers()
		st.setState(StateConfirmed)
		st.startTimerI()

	case StateConfirmed:
		// Absorb ACK retransmissions

	default:
		// Ignore ACK in other states
	}

	return nil
}

// GetState returns the current transaction state
func (st *ServerTransaction) GetState() TransactionState {
	return st.BaseTransaction.GetState()
}

// GetID returns the transaction ID
func (st *ServerTransaction) GetID() string {
	return st.BaseTransaction.GetID()
}

// IsClient returns false for server transactions
func (st *ServerTransaction) IsClient() bool {
	return st.BaseTransaction.IsClient()
}