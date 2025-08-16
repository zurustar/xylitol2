package transaction

import (
	"fmt"
	"strings"
	"time"

	"github.com/zurustar/xylitol2/internal/parser"
)

// ClientTransaction represents a client transaction
type ClientTransaction struct {
	*BaseTransaction
	retransmitCount int
	sendMessage     func(*parser.SIPMessage) error
}

// NewClientTransaction creates a new client transaction
func NewClientTransaction(msg *parser.SIPMessage, sendFunc func(*parser.SIPMessage) error) *ClientTransaction {
	ct := &ClientTransaction{
		BaseTransaction: NewBaseTransaction(msg, true),
		sendMessage:     sendFunc,
	}

	// Set initial state based on method
	if msg.GetMethod() == parser.MethodINVITE {
		ct.setState(StateCalling)
		ct.startInviteClientTimers()
	} else {
		ct.setState(StateTrying)
		ct.startNonInviteClientTimers()
	}

	return ct
}

// ProcessMessage processes an incoming message for the client transaction
func (ct *ClientTransaction) ProcessMessage(msg *parser.SIPMessage) error {
	if !msg.IsResponse() {
		return fmt.Errorf("client transaction received request message")
	}

	statusCode := msg.GetStatusCode()
	ct.lastResponse = msg.Clone()

	if ct.method == parser.MethodINVITE {
		return ct.processInviteResponse(statusCode, msg)
	} else {
		return ct.processNonInviteResponse(statusCode, msg)
	}
}

// processInviteResponse processes responses for INVITE client transactions
func (ct *ClientTransaction) processInviteResponse(statusCode int, msg *parser.SIPMessage) error {
	switch ct.GetState() {
	case StateCalling:
		if statusCode >= 100 && statusCode < 200 {
			// Provisional response
			ct.CancelTimer(TimerA)
			ct.CancelTimer(TimerB)
			ct.setState(StateProceeding)
		} else if statusCode >= 200 && statusCode < 300 {
			// 2xx response
			ct.CancelAllTimers()
			ct.setState(StateTerminated)
		} else if statusCode >= 300 {
			// 3xx-6xx response
			ct.CancelAllTimers()
			ct.setState(StateCompleted)
			ct.startTimerD()
			// Send ACK for error response
			if ct.lastRequest != nil {
				ack := ct.createACK(msg)
				if ct.sendMessage != nil {
					ct.sendMessage(ack)
				}
			}
		}

	case StateProceeding:
		if statusCode >= 200 && statusCode < 300 {
			// 2xx response
			ct.CancelAllTimers()
			ct.setState(StateTerminated)
		} else if statusCode >= 300 {
			// 3xx-6xx response
			ct.CancelAllTimers()
			ct.setState(StateCompleted)
			ct.startTimerD()
			// Send ACK for error response
			if ct.lastRequest != nil {
				ack := ct.createACK(msg)
				if ct.sendMessage != nil {
					ct.sendMessage(ack)
				}
			}
		}
		// Ignore additional 1xx responses

	case StateCompleted:
		// Retransmit ACK for any response received
		if ct.lastRequest != nil {
			ack := ct.createACK(msg)
			if ct.sendMessage != nil {
				ct.sendMessage(ack)
			}
		}

	case StateTerminated:
		// Ignore all messages
	}

	return nil
}

// processNonInviteResponse processes responses for non-INVITE client transactions
func (ct *ClientTransaction) processNonInviteResponse(statusCode int, msg *parser.SIPMessage) error {
	switch ct.GetState() {
	case StateTrying:
		if statusCode >= 100 && statusCode < 200 {
			// Provisional response
			ct.CancelTimer(TimerE)
			ct.CancelTimer(TimerF)
			ct.setState(StateProceeding)
		} else if statusCode >= 200 {
			// Final response
			ct.CancelAllTimers()
			ct.setState(StateCompleted)
			ct.startTimerK()
		}

	case StateProceeding:
		if statusCode >= 200 {
			// Final response
			ct.CancelAllTimers()
			ct.setState(StateCompleted)
			ct.startTimerK()
		}
		// Ignore additional 1xx responses

	case StateCompleted:
		// Absorb retransmissions

	case StateTerminated:
		// Ignore all messages
	}

	return nil
}

// SendResponse is not applicable for client transactions
func (ct *ClientTransaction) SendResponse(response *parser.SIPMessage) error {
	return fmt.Errorf("cannot send response from client transaction")
}

// GetState returns the current transaction state
func (ct *ClientTransaction) GetState() TransactionState {
	return ct.BaseTransaction.GetState()
}

// GetID returns the transaction ID
func (ct *ClientTransaction) GetID() string {
	return ct.BaseTransaction.GetID()
}

// IsClient returns true for client transactions
func (ct *ClientTransaction) IsClient() bool {
	return ct.BaseTransaction.IsClient()
}

// startInviteClientTimers starts timers for INVITE client transactions
func (ct *ClientTransaction) startInviteClientTimers() {
	// Timer A: INVITE retransmission
	ct.SetTimer(TimerA, TimerT1, func() {
		if ct.GetState() == StateCalling {
			ct.retransmitRequest()
			// Double the interval for next retransmission, up to T2
			interval := TimerT1 * time.Duration(1<<ct.retransmitCount)
			if interval > TimerT2 {
				interval = TimerT2
			}
			ct.retransmitCount++
			ct.SetTimer(TimerA, interval, func() {
				ct.retransmitRequest()
			})
		}
	})

	// Timer B: INVITE transaction timeout
	ct.SetTimer(TimerB, 64*TimerT1, func() {
		if ct.GetState() == StateCalling || ct.GetState() == StateProceeding {
			ct.setState(StateTerminated)
		}
	})
}

// startNonInviteClientTimers starts timers for non-INVITE client transactions
func (ct *ClientTransaction) startNonInviteClientTimers() {
	// Timer E: Non-INVITE request retransmission
	ct.SetTimer(TimerE, TimerT1, func() {
		if ct.GetState() == StateTrying {
			ct.retransmitRequest()
			// Double the interval for next retransmission, up to T2
			interval := TimerT1 * time.Duration(1<<ct.retransmitCount)
			if interval > TimerT2 {
				interval = TimerT2
			}
			ct.retransmitCount++
			ct.SetTimer(TimerE, interval, func() {
				ct.retransmitRequest()
			})
		}
	})

	// Timer F: Non-INVITE transaction timeout
	ct.SetTimer(TimerF, 64*TimerT1, func() {
		if ct.GetState() == StateTrying || ct.GetState() == StateProceeding {
			ct.setState(StateTerminated)
		}
	})
}

// startTimerD starts Timer D for INVITE client transactions
func (ct *ClientTransaction) startTimerD() {
	duration := TimerT4
	if ct.transport == "UDP" {
		duration = 32 * time.Second
	}
	
	ct.SetTimer(TimerD, duration, func() {
		ct.setState(StateTerminated)
	})
}

// startTimerK starts Timer K for non-INVITE client transactions
func (ct *ClientTransaction) startTimerK() {
	duration := TimerT4
	if ct.transport == "UDP" {
		duration = TimerT4
	} else {
		duration = 0 // Immediate transition for reliable transports
	}

	if duration > 0 {
		ct.SetTimer(TimerK, duration, func() {
			ct.setState(StateTerminated)
		})
	} else {
		ct.setState(StateTerminated)
	}
}

// retransmitRequest retransmits the original request
func (ct *ClientTransaction) retransmitRequest() {
	if ct.lastRequest != nil && ct.sendMessage != nil {
		ct.sendMessage(ct.lastRequest)
	}
}

// createACK creates an ACK request for INVITE transactions
func (ct *ClientTransaction) createACK(response *parser.SIPMessage) *parser.SIPMessage {
	if ct.lastRequest == nil {
		return nil
	}

	ack := parser.NewRequestMessage(parser.MethodACK, ct.lastRequest.GetRequestURI())
	
	// Copy required headers from original request
	ack.SetHeader(parser.HeaderVia, ct.lastRequest.GetHeader(parser.HeaderVia))
	ack.SetHeader(parser.HeaderFrom, ct.lastRequest.GetHeader(parser.HeaderFrom))
	ack.SetHeader(parser.HeaderCallID, ct.lastRequest.GetHeader(parser.HeaderCallID))
	ack.SetHeader(parser.HeaderMaxForwards, "70")
	
	// Set To header with tag from response
	toHeader := response.GetHeader(parser.HeaderTo)
	if toHeader != "" {
		ack.SetHeader(parser.HeaderTo, toHeader)
	} else {
		ack.SetHeader(parser.HeaderTo, ct.lastRequest.GetHeader(parser.HeaderTo))
	}
	
	// Set CSeq with same sequence number but ACK method
	cseqHeader := ct.lastRequest.GetHeader(parser.HeaderCSeq)
	if cseqHeader != "" {
		parts := strings.Fields(cseqHeader)
		if len(parts) >= 1 {
			ack.SetHeader(parser.HeaderCSeq, fmt.Sprintf("%s %s", parts[0], parser.MethodACK))
		}
	}
	
	ack.SetHeader(parser.HeaderContentLength, "0")
	ack.Transport = ct.transport
	
	return ack
}