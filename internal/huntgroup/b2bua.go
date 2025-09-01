package huntgroup

import (
	"fmt"
	"sync"
	"time"

	"github.com/zurustar/xylitol2/internal/logging"
	"github.com/zurustar/xylitol2/internal/parser"
	"github.com/zurustar/xylitol2/internal/transaction"
	"github.com/zurustar/xylitol2/internal/transport"
)

// B2BUA implements the B2BUAManager interface
type B2BUA struct {
	transportManager   transport.TransportManager
	transactionManager transaction.TransactionManager
	parser             parser.MessageParser
	logger             logging.Logger
	
	// Active sessions
	activeSessions map[string]*B2BUASession
	sessionMutex   sync.RWMutex
	
	// Configuration
	serverHost string
	serverPort int
}

// NewB2BUA creates a new B2BUA instance
func NewB2BUA(
	transportManager transport.TransportManager,
	transactionManager transaction.TransactionManager,
	parser parser.MessageParser,
	logger logging.Logger,
	serverHost string,
	serverPort int,
) *B2BUA {
	return &B2BUA{
		transportManager:   transportManager,
		transactionManager: transactionManager,
		parser:             parser,
		logger:             logger,
		activeSessions:     make(map[string]*B2BUASession),
		serverHost:         serverHost,
		serverPort:         serverPort,
	}
}

// CreateSession creates a new B2BUA session
func (b *B2BUA) CreateSession(callerInvite *parser.SIPMessage, calleeURI string) (*B2BUASession, error) {
	if callerInvite == nil || calleeURI == "" {
		return nil, fmt.Errorf("invalid parameters")
	}

	sessionID := b.generateSessionID()
	now := time.Now().UTC()

	// Create caller leg
	callerLeg := &CallLeg{
		CallID:     callerInvite.GetHeader(parser.HeaderCallID),
		FromURI:    callerInvite.GetHeader(parser.HeaderFrom),
		ToURI:      callerInvite.GetHeader(parser.HeaderTo),
		ContactURI: callerInvite.GetHeader(parser.HeaderContact),
		Status:     CallLegStatusInitiating,
	}

	// Create callee leg
	calleeLeg := &CallLeg{
		CallID:     b.generateCallID(),
		FromURI:    callerInvite.GetHeader(parser.HeaderFrom), // Forward caller's From
		ToURI:      fmt.Sprintf("<%s>", calleeURI),
		ContactURI: fmt.Sprintf("<sip:%s:%d>", b.serverHost, b.serverPort),
		Status:     CallLegStatusInitiating,
	}

	session := &B2BUASession{
		SessionID:   sessionID,
		CallerLeg:   callerLeg,
		CalleeLeg:   calleeLeg,
		Status:      B2BUAStatusInitiating,
		StartTime:   now,
	}

	// Store session
	b.sessionMutex.Lock()
	b.activeSessions[sessionID] = session
	b.sessionMutex.Unlock()

	b.logger.Info("B2BUA session created",
		logging.Field{Key: "session_id", Value: sessionID},
		logging.Field{Key: "caller", Value: callerLeg.FromURI},
		logging.Field{Key: "callee", Value: calleeURI})

	return session, nil
}

// GetSession retrieves a B2BUA session
func (b *B2BUA) GetSession(sessionID string) (*B2BUASession, error) {
	b.sessionMutex.RLock()
	defer b.sessionMutex.RUnlock()

	session, exists := b.activeSessions[sessionID]
	if !exists {
		return nil, fmt.Errorf("session not found: %s", sessionID)
	}

	return session, nil
}

// UpdateSession updates a B2BUA session
func (b *B2BUA) UpdateSession(session *B2BUASession) error {
	if session == nil {
		return fmt.Errorf("session cannot be nil")
	}

	b.sessionMutex.Lock()
	defer b.sessionMutex.Unlock()

	b.activeSessions[session.SessionID] = session
	return nil
}

// EndSession ends a B2BUA session
func (b *B2BUA) EndSession(sessionID string) error {
	b.sessionMutex.Lock()
	session, exists := b.activeSessions[sessionID]
	if exists {
		delete(b.activeSessions, sessionID)
	}
	b.sessionMutex.Unlock()

	if !exists {
		return fmt.Errorf("session not found: %s", sessionID)
	}

	now := time.Now().UTC()
	session.Status = B2BUAStatusEnded
	session.EndTime = &now
	session.CallerLeg.Status = CallLegStatusEnded
	session.CalleeLeg.Status = CallLegStatusEnded

	b.logger.Info("B2BUA session ended",
		logging.Field{Key: "session_id", Value: sessionID})

	return nil
}

// HandleCallerMessage handles messages from the caller leg
func (b *B2BUA) HandleCallerMessage(sessionID string, message *parser.SIPMessage) error {
	session, err := b.GetSession(sessionID)
	if err != nil {
		return err
	}

	method := message.GetMethod()
	
	b.logger.Debug("Handling caller message",
		logging.Field{Key: "session_id", Value: sessionID},
		logging.Field{Key: "method", Value: method})

	switch method {
	case parser.MethodINVITE:
		return b.handleCallerInvite(session, message)
	case parser.MethodACK:
		return b.handleCallerAck(session, message)
	case parser.MethodBYE:
		return b.handleCallerBye(session, message)
	case parser.MethodCANCEL:
		return b.handleCallerCancel(session, message)
	default:
		// Forward other methods to callee
		return b.forwardMessageToCallee(session, message)
	}
}

// HandleCalleeMessage handles messages from the callee leg
func (b *B2BUA) HandleCalleeMessage(sessionID string, message *parser.SIPMessage) error {
	session, err := b.GetSession(sessionID)
	if err != nil {
		return err
	}

	if message.IsResponse() {
		return b.handleCalleeResponse(session, message)
	}

	method := message.GetMethod()
	
	b.logger.Debug("Handling callee message",
		logging.Field{Key: "session_id", Value: sessionID},
		logging.Field{Key: "method", Value: method})

	switch method {
	case parser.MethodBYE:
		return b.handleCalleeBye(session, message)
	default:
		// Forward other methods to caller
		return b.forwardMessageToCaller(session, message)
	}
}

// BridgeCalls bridges the caller and callee legs
func (b *B2BUA) BridgeCalls(sessionID string) error {
	session, err := b.GetSession(sessionID)
	if err != nil {
		return err
	}

	if session.Status != B2BUAStatusRinging {
		return fmt.Errorf("session not in ringing state")
	}

	now := time.Now().UTC()
	session.Status = B2BUAStatusConnected
	session.ConnectTime = &now
	session.CallerLeg.Status = CallLegStatusConnected
	session.CalleeLeg.Status = CallLegStatusConnected

	b.logger.Info("B2BUA calls bridged",
		logging.Field{Key: "session_id", Value: sessionID})

	return b.UpdateSession(session)
}

// TransferCall transfers the call to a new target
func (b *B2BUA) TransferCall(sessionID string, targetURI string) error {
	_, err := b.GetSession(sessionID)
	if err != nil {
		return err
	}

	b.logger.Info("B2BUA call transfer initiated",
		logging.Field{Key: "session_id", Value: sessionID},
		logging.Field{Key: "target", Value: targetURI})

	// Implementation would create a new callee leg and transfer the call
	// This is a complex operation that would involve REFER or re-INVITE
	return fmt.Errorf("call transfer not implemented")
}

// Private helper methods

func (b *B2BUA) handleCallerInvite(session *B2BUASession, invite *parser.SIPMessage) error {
	// Create INVITE for callee leg
	calleeInvite := b.createCalleeInvite(session, invite)
	
	// Send INVITE to callee
	if err := b.sendMessageToCallee(session, calleeInvite); err != nil {
		return fmt.Errorf("failed to send INVITE to callee: %w", err)
	}

	session.Status = B2BUAStatusRinging
	session.CallerLeg.Status = CallLegStatusRinging
	session.CalleeLeg.Status = CallLegStatusRinging

	return b.UpdateSession(session)
}

func (b *B2BUA) handleCallerAck(session *B2BUASession, ack *parser.SIPMessage) error {
	// Create ACK for callee leg
	calleeAck := b.createCalleeAck(session, ack)
	
	// Send ACK to callee
	return b.sendMessageToCallee(session, calleeAck)
}

func (b *B2BUA) handleCallerBye(session *B2BUASession, bye *parser.SIPMessage) error {
	// Create BYE for callee leg
	calleeBye := b.createCalleeBye(session, bye)
	
	// Send BYE to callee
	if err := b.sendMessageToCallee(session, calleeBye); err != nil {
		return fmt.Errorf("failed to send BYE to callee: %w", err)
	}

	// Send 200 OK to caller
	response := b.createByeResponse(session, bye)
	if err := b.sendMessageToCaller(session, response); err != nil {
		return fmt.Errorf("failed to send BYE response to caller: %w", err)
	}

	// End session
	return b.EndSession(session.SessionID)
}

func (b *B2BUA) handleCallerCancel(session *B2BUASession, cancel *parser.SIPMessage) error {
	// Create CANCEL for callee leg
	calleeCancel := b.createCalleeCancel(session, cancel)
	
	// Send CANCEL to callee
	if err := b.sendMessageToCallee(session, calleeCancel); err != nil {
		return fmt.Errorf("failed to send CANCEL to callee: %w", err)
	}

	// Send 200 OK to caller for CANCEL
	response := b.createCancelResponse(session, cancel)
	if err := b.sendMessageToCaller(session, response); err != nil {
		return fmt.Errorf("failed to send CANCEL response to caller: %w", err)
	}

	return nil
}

func (b *B2BUA) handleCalleeResponse(session *B2BUASession, response *parser.SIPMessage) error {
	statusCode := response.GetStatusCode()
	
	b.logger.Debug("Handling callee response",
		logging.Field{Key: "session_id", Value: session.SessionID},
		logging.Field{Key: "status_code", Value: statusCode})

	// Create response for caller
	callerResponse := b.createCallerResponse(session, response)
	
	// Handle different response types
	switch {
	case statusCode >= 200 && statusCode < 300:
		// Success response
		if session.Status == B2BUAStatusRinging {
			if err := b.BridgeCalls(session.SessionID); err != nil {
				return err
			}
		}
	case statusCode >= 300:
		// Error or redirect response
		session.Status = B2BUAStatusEnded
		session.CallerLeg.Status = CallLegStatusEnded
		session.CalleeLeg.Status = CallLegStatusEnded
	}

	// Forward response to caller
	return b.sendMessageToCaller(session, callerResponse)
}

func (b *B2BUA) handleCalleeBye(session *B2BUASession, bye *parser.SIPMessage) error {
	// Create BYE for caller leg
	callerBye := b.createCallerBye(session, bye)
	
	// Send BYE to caller
	if err := b.sendMessageToCaller(session, callerBye); err != nil {
		return fmt.Errorf("failed to send BYE to caller: %w", err)
	}

	// Send 200 OK to callee
	response := b.createByeResponse(session, bye)
	if err := b.sendMessageToCallee(session, response); err != nil {
		return fmt.Errorf("failed to send BYE response to callee: %w", err)
	}

	// End session
	return b.EndSession(session.SessionID)
}

func (b *B2BUA) forwardMessageToCallee(session *B2BUASession, message *parser.SIPMessage) error {
	// Create message for callee leg
	calleeMessage := b.adaptMessageForCallee(session, message)
	return b.sendMessageToCallee(session, calleeMessage)
}

func (b *B2BUA) forwardMessageToCaller(session *B2BUASession, message *parser.SIPMessage) error {
	// Create message for caller leg
	callerMessage := b.adaptMessageForCaller(session, message)
	return b.sendMessageToCaller(session, callerMessage)
}

// Message creation helpers

func (b *B2BUA) createCalleeInvite(session *B2BUASession, callerInvite *parser.SIPMessage) *parser.SIPMessage {
	invite := callerInvite.Clone()
	
	// Update Call-ID for callee leg
	invite.SetHeader(parser.HeaderCallID, session.CalleeLeg.CallID)
	
	// Update To header
	invite.SetHeader(parser.HeaderTo, session.CalleeLeg.ToURI)
	
	// Update Contact header
	invite.SetHeader(parser.HeaderContact, session.CalleeLeg.ContactURI)
	
	// Add Via header for this B2BUA
	viaHeader := fmt.Sprintf("SIP/2.0/UDP %s:%d;branch=z9hG4bK-%d", 
		b.serverHost, b.serverPort, time.Now().UnixNano())
	b.addViaHeader(invite, viaHeader)
	
	return invite
}

func (b *B2BUA) createCalleeAck(session *B2BUASession, callerAck *parser.SIPMessage) *parser.SIPMessage {
	ack := callerAck.Clone()
	
	// Update Call-ID for callee leg
	ack.SetHeader(parser.HeaderCallID, session.CalleeLeg.CallID)
	
	// Update To header
	ack.SetHeader(parser.HeaderTo, session.CalleeLeg.ToURI)
	
	return ack
}

func (b *B2BUA) createCalleeBye(session *B2BUASession, callerBye *parser.SIPMessage) *parser.SIPMessage {
	bye := callerBye.Clone()
	
	// Update Call-ID for callee leg
	bye.SetHeader(parser.HeaderCallID, session.CalleeLeg.CallID)
	
	// Update To header
	bye.SetHeader(parser.HeaderTo, session.CalleeLeg.ToURI)
	
	return bye
}

func (b *B2BUA) createCalleeCancel(session *B2BUASession, callerCancel *parser.SIPMessage) *parser.SIPMessage {
	cancel := callerCancel.Clone()
	
	// Update Call-ID for callee leg
	cancel.SetHeader(parser.HeaderCallID, session.CalleeLeg.CallID)
	
	// Update To header
	cancel.SetHeader(parser.HeaderTo, session.CalleeLeg.ToURI)
	
	return cancel
}

func (b *B2BUA) createCallerResponse(session *B2BUASession, calleeResponse *parser.SIPMessage) *parser.SIPMessage {
	response := calleeResponse.Clone()
	
	// Update Call-ID for caller leg
	response.SetHeader(parser.HeaderCallID, session.CallerLeg.CallID)
	
	// Update To header
	response.SetHeader(parser.HeaderTo, session.CallerLeg.ToURI)
	
	// Update Contact header
	response.SetHeader(parser.HeaderContact, session.CallerLeg.ContactURI)
	
	return response
}

func (b *B2BUA) createCallerBye(session *B2BUASession, calleeBye *parser.SIPMessage) *parser.SIPMessage {
	bye := calleeBye.Clone()
	
	// Update Call-ID for caller leg
	bye.SetHeader(parser.HeaderCallID, session.CallerLeg.CallID)
	
	// Update To header
	bye.SetHeader(parser.HeaderTo, session.CallerLeg.ToURI)
	
	return bye
}

func (b *B2BUA) createByeResponse(session *B2BUASession, bye *parser.SIPMessage) *parser.SIPMessage {
	response := parser.NewResponseMessage(parser.StatusOK, "OK")
	
	// Copy required headers from BYE
	response.SetHeader(parser.HeaderCallID, bye.GetHeader(parser.HeaderCallID))
	response.SetHeader(parser.HeaderFrom, bye.GetHeader(parser.HeaderFrom))
	response.SetHeader(parser.HeaderTo, bye.GetHeader(parser.HeaderTo))
	response.SetHeader(parser.HeaderCSeq, bye.GetHeader(parser.HeaderCSeq))
	
	// Copy Via headers
	viaHeaders := bye.GetHeaders(parser.HeaderVia)
	for _, via := range viaHeaders {
		response.AddHeader(parser.HeaderVia, via)
	}
	
	response.SetHeader(parser.HeaderContentLength, "0")
	
	return response
}

func (b *B2BUA) createCancelResponse(session *B2BUASession, cancel *parser.SIPMessage) *parser.SIPMessage {
	response := parser.NewResponseMessage(parser.StatusOK, "OK")
	
	// Copy required headers from CANCEL
	response.SetHeader(parser.HeaderCallID, cancel.GetHeader(parser.HeaderCallID))
	response.SetHeader(parser.HeaderFrom, cancel.GetHeader(parser.HeaderFrom))
	response.SetHeader(parser.HeaderTo, cancel.GetHeader(parser.HeaderTo))
	response.SetHeader(parser.HeaderCSeq, cancel.GetHeader(parser.HeaderCSeq))
	
	// Copy Via headers
	viaHeaders := cancel.GetHeaders(parser.HeaderVia)
	for _, via := range viaHeaders {
		response.AddHeader(parser.HeaderVia, via)
	}
	
	response.SetHeader(parser.HeaderContentLength, "0")
	
	return response
}

func (b *B2BUA) adaptMessageForCallee(session *B2BUASession, message *parser.SIPMessage) *parser.SIPMessage {
	adapted := message.Clone()
	
	// Update Call-ID for callee leg
	adapted.SetHeader(parser.HeaderCallID, session.CalleeLeg.CallID)
	
	// Update To header
	adapted.SetHeader(parser.HeaderTo, session.CalleeLeg.ToURI)
	
	return adapted
}

func (b *B2BUA) adaptMessageForCaller(session *B2BUASession, message *parser.SIPMessage) *parser.SIPMessage {
	adapted := message.Clone()
	
	// Update Call-ID for caller leg
	adapted.SetHeader(parser.HeaderCallID, session.CallerLeg.CallID)
	
	// Update To header
	adapted.SetHeader(parser.HeaderTo, session.CallerLeg.ToURI)
	
	return adapted
}

func (b *B2BUA) addViaHeader(message *parser.SIPMessage, viaHeader string) {
	existingVias := message.GetHeaders(parser.HeaderVia)
	message.RemoveHeader(parser.HeaderVia)
	message.AddHeader(parser.HeaderVia, viaHeader)
	for _, via := range existingVias {
		message.AddHeader(parser.HeaderVia, via)
	}
}

func (b *B2BUA) sendMessageToCaller(session *B2BUASession, message *parser.SIPMessage) error {
	data, err := b.parser.Serialize(message)
	if err != nil {
		return fmt.Errorf("failed to serialize message for caller: %w", err)
	}

	// In a real implementation, this would send to the caller's address
	// For now, just log the action
	b.logger.Debug("Sending message to caller",
		logging.Field{Key: "session_id", Value: session.SessionID},
		logging.Field{Key: "method", Value: message.GetMethod()})

	return b.transportManager.SendMessage(data, "udp", nil)
}

func (b *B2BUA) sendMessageToCallee(session *B2BUASession, message *parser.SIPMessage) error {
	data, err := b.parser.Serialize(message)
	if err != nil {
		return fmt.Errorf("failed to serialize message for callee: %w", err)
	}

	// In a real implementation, this would send to the callee's address
	// For now, just log the action
	b.logger.Debug("Sending message to callee",
		logging.Field{Key: "session_id", Value: session.SessionID},
		logging.Field{Key: "method", Value: message.GetMethod()})

	return b.transportManager.SendMessage(data, "udp", nil)
}

func (b *B2BUA) generateSessionID() string {
	return fmt.Sprintf("b2bua-session-%d", time.Now().UnixNano())
}

func (b *B2BUA) generateCallID() string {
	return fmt.Sprintf("b2bua-call-%d@%s", time.Now().UnixNano(), b.serverHost)
}