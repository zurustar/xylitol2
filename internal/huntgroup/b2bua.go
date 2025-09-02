package huntgroup

import (
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/zurustar/xylitol2/internal/logging"
	"github.com/zurustar/xylitol2/internal/parser"
	"github.com/zurustar/xylitol2/internal/transaction"
	"github.com/zurustar/xylitol2/internal/transport"
)

// B2BUA implements the B2BUAManager interface with enhanced session management
type B2BUA struct {
	transportManager   transport.TransportManager
	transactionManager transaction.TransactionManager
	parser             parser.MessageParser
	logger             logging.Logger
	
	// Active sessions with multiple lookup indices
	activeSessions     map[string]*B2BUASession  // sessionID -> session
	sessionsByCallID   map[string]*B2BUASession  // callID -> session
	sessionsByLegID    map[string]*B2BUASession  // legID -> session
	sessionMutex       sync.RWMutex
	
	// Dialog and transaction management
	dialogManager      *DialogManager
	
	// Configuration
	serverHost string
	serverPort int
	
	// Session cleanup
	sessionTimeout time.Duration
	cleanupTicker  *time.Ticker
	stopCleanup    chan struct{}
}

// NewB2BUA creates a new B2BUA instance with enhanced session management
func NewB2BUA(
	transportManager transport.TransportManager,
	transactionManager transaction.TransactionManager,
	parser parser.MessageParser,
	logger logging.Logger,
	serverHost string,
	serverPort int,
) *B2BUA {
	b2bua := &B2BUA{
		transportManager:   transportManager,
		transactionManager: transactionManager,
		parser:             parser,
		logger:             logger,
		activeSessions:     make(map[string]*B2BUASession),
		sessionsByCallID:   make(map[string]*B2BUASession),
		sessionsByLegID:    make(map[string]*B2BUASession),
		dialogManager:      NewDialogManager(logger),
		serverHost:         serverHost,
		serverPort:         serverPort,
		sessionTimeout:     30 * time.Minute, // Default session timeout
		stopCleanup:        make(chan struct{}),
	}
	
	// Start cleanup goroutine
	b2bua.startCleanupRoutine()
	
	return b2bua
}

// startCleanupRoutine starts the session cleanup routine
func (b *B2BUA) startCleanupRoutine() {
	b.cleanupTicker = time.NewTicker(5 * time.Minute) // Cleanup every 5 minutes
	go func() {
		for {
			select {
			case <-b.cleanupTicker.C:
				if err := b.CleanupExpiredSessions(); err != nil {
					b.logger.Error("Failed to cleanup expired sessions", 
						logging.Field{Key: "error", Value: err.Error()})
				}
			case <-b.stopCleanup:
				b.cleanupTicker.Stop()
				return
			}
		}
	}()
}

// Stop stops the B2BUA and cleanup routines
func (b *B2BUA) Stop() {
	close(b.stopCleanup)
}

// CreateSession creates a new B2BUA session for direct calls
func (b *B2BUA) CreateSession(callerInvite *parser.SIPMessage, calleeURI string) (*B2BUASession, error) {
	if callerInvite == nil || calleeURI == "" {
		return nil, fmt.Errorf("invalid parameters: callerInvite=%v, calleeURI=%s", callerInvite, calleeURI)
	}

	sessionID := b.generateSessionID()
	now := time.Now().UTC()

	// Extract SDP from caller INVITE
	var sdpOffer string
	if len(callerInvite.Body) > 0 {
		sdpOffer = string(callerInvite.Body)
	}

	// Extract dialog information from caller INVITE
	callerFromTag := ExtractTagFromHeader(callerInvite.GetHeader(parser.HeaderFrom))
	callerToTag := ExtractTagFromHeader(callerInvite.GetHeader(parser.HeaderTo))
	callerFromURI := ExtractURIFromHeader(callerInvite.GetHeader(parser.HeaderFrom))
	callerToURI := ExtractURIFromHeader(callerInvite.GetHeader(parser.HeaderTo))

	// Create caller dialog (we are the UAS for this leg)
	callerDialog := b.dialogManager.CreateDialog(
		callerInvite.GetHeader(parser.HeaderCallID),
		callerToURI,   // Local URI (we are the To party)
		callerFromURI, // Remote URI (caller is the From party)
		callerToTag,   // Local tag (will be generated if empty)
		callerFromTag, // Remote tag (from caller)
	)

	// Generate To tag if not present
	if callerToTag == "" {
		callerToTag = b.generateTag()
		callerDialog.Lock()
		callerDialog.LocalTag = callerToTag
		callerDialog.Unlock()
	}

	// Create caller leg with enhanced dialog information
	callerLeg := &CallLeg{
		LegID:       b.generateLegID("caller"),
		CallID:      callerInvite.GetHeader(parser.HeaderCallID),
		FromURI:     callerInvite.GetHeader(parser.HeaderFrom),
		ToURI:       callerInvite.GetHeader(parser.HeaderTo),
		FromTag:     callerFromTag,
		ToTag:       callerToTag,
		ContactURI:  callerInvite.GetHeader(parser.HeaderContact),
		Status:      CallLegStatusInitial,
		RemoteAddr:  callerInvite.Source,
		RemoteSDP:   sdpOffer,
		LastCSeq:    ExtractCSeqNumber(callerInvite.GetHeader(parser.HeaderCSeq)),
		DialogID:    callerDialog.DialogID,
		CreatedAt:   now,
	}

	// Create callee dialog (we are the UAC for this leg)
	calleeCallID := b.generateCallID()
	calleeFromTag := b.generateTag()
	calleeFromURI := fmt.Sprintf("sip:%s:%d", b.serverHost, b.serverPort)
	
	calleeDialog := b.dialogManager.CreateDialog(
		calleeCallID,
		calleeFromURI, // Local URI (we are the From party)
		calleeURI,     // Remote URI (callee is the To party)
		calleeFromTag, // Local tag
		"",            // Remote tag (will be set when callee responds)
	)

	// Create callee leg with new Call-ID and dialog information
	calleeLeg := &CallLeg{
		LegID:      b.generateLegID("callee"),
		CallID:     calleeCallID,
		FromURI:    BuildHeaderWithTag(calleeFromURI, "", calleeFromTag),
		ToURI:      fmt.Sprintf("<%s>", calleeURI),
		FromTag:    calleeFromTag,
		ToTag:      "", // Will be set when callee responds
		ContactURI: fmt.Sprintf("<sip:%s:%d>", b.serverHost, b.serverPort),
		Status:     CallLegStatusInitial,
		LocalSDP:   sdpOffer, // Forward caller's SDP
		LastCSeq:   1,        // Start with CSeq 1 for new dialog
		DialogID:   calleeDialog.DialogID,
		CreatedAt:  now,
	}

	session := &B2BUASession{
		SessionID:    sessionID,
		CallerLeg:    callerLeg,
		CalleeLeg:    calleeLeg,
		PendingLegs:  make(map[string]*CallLeg),
		Status:       B2BUAStatusInitial,
		StartTime:    now,
		LastActivity: now,
		SDPOffer:     sdpOffer,
	}

	// Store session with multiple indices
	b.sessionMutex.Lock()
	b.activeSessions[sessionID] = session
	b.sessionsByCallID[callerLeg.CallID] = session
	b.sessionsByCallID[calleeLeg.CallID] = session
	b.sessionsByLegID[callerLeg.LegID] = session
	b.sessionsByLegID[calleeLeg.LegID] = session
	b.sessionMutex.Unlock()

	b.logger.Info("B2BUA session created",
		logging.Field{Key: "session_id", Value: sessionID},
		logging.Field{Key: "caller_leg_id", Value: callerLeg.LegID},
		logging.Field{Key: "callee_leg_id", Value: calleeLeg.LegID},
		logging.Field{Key: "caller", Value: callerLeg.FromURI},
		logging.Field{Key: "callee", Value: calleeURI})

	return session, nil
}

// CreateHuntGroupSession creates a new B2BUA session for hunt group calls
func (b *B2BUA) CreateHuntGroupSession(callerInvite *parser.SIPMessage, huntGroup *HuntGroup) (*B2BUASession, error) {
	if callerInvite == nil || huntGroup == nil {
		return nil, fmt.Errorf("invalid parameters: callerInvite=%v, huntGroup=%v", callerInvite, huntGroup)
	}

	sessionID := b.generateSessionID()
	now := time.Now().UTC()

	// Extract SDP from caller INVITE
	var sdpOffer string
	if len(callerInvite.Body) > 0 {
		sdpOffer = string(callerInvite.Body)
	}

	// Create caller leg
	callerLeg := &CallLeg{
		LegID:       b.generateLegID("caller"),
		CallID:      callerInvite.GetHeader(parser.HeaderCallID),
		FromURI:     callerInvite.GetHeader(parser.HeaderFrom),
		ToURI:       callerInvite.GetHeader(parser.HeaderTo),
		FromTag:     b.extractTag(callerInvite.GetHeader(parser.HeaderFrom)),
		ToTag:       b.extractTag(callerInvite.GetHeader(parser.HeaderTo)),
		ContactURI:  callerInvite.GetHeader(parser.HeaderContact),
		Status:      CallLegStatusInitial,
		RemoteAddr:  callerInvite.Source,
		RemoteSDP:   sdpOffer,
		LastCSeq:    b.extractCSeq(callerInvite.GetHeader(parser.HeaderCSeq)),
		CreatedAt:   now,
	}

	session := &B2BUASession{
		SessionID:    sessionID,
		CallerLeg:    callerLeg,
		CalleeLeg:    nil, // Will be set when a member answers
		PendingLegs:  make(map[string]*CallLeg),
		Status:       B2BUAStatusInitial,
		StartTime:    now,
		LastActivity: now,
		SDPOffer:     sdpOffer,
		HuntGroupID:  &huntGroup.ID,
	}

	// Store session with indices
	b.sessionMutex.Lock()
	b.activeSessions[sessionID] = session
	b.sessionsByCallID[callerLeg.CallID] = session
	b.sessionsByLegID[callerLeg.LegID] = session
	b.sessionMutex.Unlock()

	b.logger.Info("B2BUA hunt group session created",
		logging.Field{Key: "session_id", Value: sessionID},
		logging.Field{Key: "hunt_group_id", Value: huntGroup.ID},
		logging.Field{Key: "hunt_group_extension", Value: huntGroup.Extension},
		logging.Field{Key: "caller", Value: callerLeg.FromURI})

	return session, nil
}

// GetSession retrieves a B2BUA session by session ID
func (b *B2BUA) GetSession(sessionID string) (*B2BUASession, error) {
	b.sessionMutex.RLock()
	defer b.sessionMutex.RUnlock()

	session, exists := b.activeSessions[sessionID]
	if !exists {
		return nil, fmt.Errorf("session not found: %s", sessionID)
	}

	return session, nil
}

// GetSessionByCallID retrieves a B2BUA session by Call-ID
func (b *B2BUA) GetSessionByCallID(callID string) (*B2BUASession, error) {
	b.sessionMutex.RLock()
	defer b.sessionMutex.RUnlock()

	session, exists := b.sessionsByCallID[callID]
	if !exists {
		return nil, fmt.Errorf("session not found for Call-ID: %s", callID)
	}

	return session, nil
}

// GetSessionByLegID retrieves a B2BUA session by leg ID
func (b *B2BUA) GetSessionByLegID(legID string) (*B2BUASession, error) {
	b.sessionMutex.RLock()
	defer b.sessionMutex.RUnlock()

	session, exists := b.sessionsByLegID[legID]
	if !exists {
		return nil, fmt.Errorf("session not found for leg ID: %s", legID)
	}

	return session, nil
}

// GetActiveSessions returns all active sessions
func (b *B2BUA) GetActiveSessions() ([]*B2BUASession, error) {
	b.sessionMutex.RLock()
	defer b.sessionMutex.RUnlock()

	sessions := make([]*B2BUASession, 0, len(b.activeSessions))
	for _, session := range b.activeSessions {
		if session.IsActive() {
			sessions = append(sessions, session)
		}
	}

	return sessions, nil
}

// CleanupExpiredSessions removes expired and terminated sessions
func (b *B2BUA) CleanupExpiredSessions() error {
	b.sessionMutex.Lock()
	defer b.sessionMutex.Unlock()

	now := time.Now().UTC()
	expiredSessions := make([]string, 0)

	for sessionID, session := range b.activeSessions {
		session.RLock()
		isExpired := session.IsTerminated() || 
			(session.IsActive() && now.Sub(session.LastActivity) > b.sessionTimeout)
		session.RUnlock()

		if isExpired {
			expiredSessions = append(expiredSessions, sessionID)
		}
	}

	// Remove expired sessions
	for _, sessionID := range expiredSessions {
		session := b.activeSessions[sessionID]
		b.removeSessionFromIndices(session)
		delete(b.activeSessions, sessionID)
		
		b.logger.Info("Cleaned up expired session",
			logging.Field{Key: "session_id", Value: sessionID})
	}

	if len(expiredSessions) > 0 {
		b.logger.Info("Session cleanup completed",
			logging.Field{Key: "cleaned_sessions", Value: len(expiredSessions)},
			logging.Field{Key: "active_sessions", Value: len(b.activeSessions)})
	}

	return nil
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
	// Create transaction for caller INVITE
	callerTxn := b.transactionManager.CreateTransaction(invite)
	session.CallerLeg.Transaction = callerTxn

	// Create INVITE for callee leg
	calleeInvite := b.createCalleeInvite(session, invite)
	
	// Create transaction for callee INVITE
	calleeTxn := b.transactionManager.CreateTransaction(calleeInvite)
	session.CalleeLeg.Transaction = calleeTxn

	// Create transaction correlation
	correlation := b.dialogManager.CreateCorrelation(
		callerTxn.GetID(),
		calleeTxn.GetID(),
		parser.MethodINVITE,
	)
	correlation.SetAlegTransaction(callerTxn)
	correlation.SetBlegTransaction(calleeTxn)

	// Send INVITE to callee
	if err := b.sendMessageToCallee(session, calleeInvite); err != nil {
		return fmt.Errorf("failed to send INVITE to callee: %w", err)
	}

	// Update session and leg states
	session.SetStatus(B2BUAStatusInitiating)
	session.CallerLeg.SetStatus(CallLegStatusProceeding)
	session.CalleeLeg.SetStatus(CallLegStatusInitiating)

	return b.UpdateSession(session)
}

func (b *B2BUA) handleCallerAck(session *B2BUASession, ack *parser.SIPMessage) error {
	// Create ACK for callee leg
	calleeAck := b.createCalleeAck(session, ack)
	
	// Send ACK to callee
	return b.sendMessageToCallee(session, calleeAck)
}

func (b *B2BUA) handleCallerBye(session *B2BUASession, bye *parser.SIPMessage) error {
	// Update caller dialog with BYE CSeq
	if callerDialog := b.dialogManager.GetDialog(session.CallerLeg.DialogID); callerDialog != nil {
		cseqNum := ExtractCSeqNumber(bye.GetHeader(parser.HeaderCSeq))
		callerDialog.UpdateRemoteCSeq(cseqNum)
	}

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

	// Terminate dialogs
	if callerDialog := b.dialogManager.GetDialog(session.CallerLeg.DialogID); callerDialog != nil {
		b.dialogManager.TerminateDialog(callerDialog.DialogID)
	}
	if calleeDialog := b.dialogManager.GetDialog(session.CalleeLeg.DialogID); calleeDialog != nil {
		b.dialogManager.TerminateDialog(calleeDialog.DialogID)
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

	// Update callee dialog with response information
	if calleeDialog := b.dialogManager.GetDialog(session.CalleeLeg.DialogID); calleeDialog != nil {
		// Extract To tag from response (this becomes the remote tag for callee dialog)
		responseToTag := ExtractTagFromHeader(response.GetHeader(parser.HeaderTo))
		if responseToTag != "" && calleeDialog.RemoteTag == "" {
			calleeDialog.Lock()
			calleeDialog.RemoteTag = responseToTag
			session.CalleeLeg.ToTag = responseToTag
			calleeDialog.Unlock()
		}

		// Update remote target from Contact header
		if contact := response.GetHeader(parser.HeaderContact); contact != "" {
			calleeDialog.SetRemoteTarget(ExtractURIFromHeader(contact))
			session.CalleeLeg.RemoteTarget = contact
		}

		// Handle dialog state based on response
		switch {
		case statusCode >= 200 && statusCode < 300:
			// Success response - confirm dialog
			calleeDialog.ConfirmDialog()
			session.CalleeLeg.SetStatus(CallLegStatusConnected)
		case statusCode >= 300:
			// Error response - terminate dialog
			b.dialogManager.TerminateDialog(calleeDialog.DialogID)
			session.CalleeLeg.SetStatus(CallLegStatusFailed)
		case statusCode >= 100 && statusCode < 200:
			// Provisional response
			session.CalleeLeg.SetStatus(CallLegStatusProceeding)
			if statusCode == 180 {
				session.CalleeLeg.SetStatus(CallLegStatusRinging)
			}
		}
	}

	// Create response for caller
	callerResponse := b.createCallerResponse(session, response)
	
	// Handle different response types for session state
	switch {
	case statusCode >= 200 && statusCode < 300:
		// Success response - bridge calls
		if session.GetStatus() == B2BUAStatusRinging || session.GetStatus() == B2BUAStatusInitiating {
			// Confirm caller dialog
			if callerDialog := b.dialogManager.GetDialog(session.CallerLeg.DialogID); callerDialog != nil {
				callerDialog.ConfirmDialog()
			}
			session.CallerLeg.SetStatus(CallLegStatusConnected)
			
			if err := b.BridgeCalls(session.SessionID); err != nil {
				return err
			}
		}
	case statusCode >= 300:
		// Error or redirect response
		session.SetStatus(B2BUAStatusFailed)
		session.CallerLeg.SetStatus(CallLegStatusFailed)
		
		// Terminate caller dialog
		if callerDialog := b.dialogManager.GetDialog(session.CallerLeg.DialogID); callerDialog != nil {
			b.dialogManager.TerminateDialog(callerDialog.DialogID)
		}
	case statusCode == 180:
		// Ringing response
		session.SetStatus(B2BUAStatusRinging)
		session.CallerLeg.SetStatus(CallLegStatusRinging)
	case statusCode >= 100 && statusCode < 200:
		// Other provisional responses
		session.SetStatus(B2BUAStatusProceeding)
		session.CallerLeg.SetStatus(CallLegStatusProceeding)
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
	
	// Get callee dialog
	calleeDialog := b.dialogManager.GetDialog(session.CalleeLeg.DialogID)
	if calleeDialog == nil {
		b.logger.Error("Callee dialog not found", 
			logging.Field{Key: "dialog_id", Value: session.CalleeLeg.DialogID})
		return invite
	}

	// Update headers for callee leg dialog
	invite.SetHeader(parser.HeaderCallID, session.CalleeLeg.CallID)
	invite.SetHeader(parser.HeaderFrom, session.CalleeLeg.FromURI)
	invite.SetHeader(parser.HeaderTo, session.CalleeLeg.ToURI)
	invite.SetHeader(parser.HeaderContact, session.CalleeLeg.ContactURI)
	
	// Update CSeq with callee leg's sequence number
	calleeDialog.RLock()
	cseqNum := calleeDialog.LocalCSeq
	calleeDialog.RUnlock()
	
	invite.SetHeader(parser.HeaderCSeq, fmt.Sprintf("%d %s", cseqNum, parser.MethodINVITE))
	
	// Clear existing Via headers and add our own
	invite.RemoveHeader(parser.HeaderVia)
	viaHeader := fmt.Sprintf("SIP/2.0/UDP %s:%d;branch=z9hG4bK-%d", 
		b.serverHost, b.serverPort, time.Now().UnixNano())
	invite.AddHeader(parser.HeaderVia, viaHeader)
	
	// Set Max-Forwards
	invite.SetHeader(parser.HeaderMaxForwards, "70")
	
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
	bye := parser.NewRequestMessage(parser.MethodBYE, session.CalleeLeg.ToURI)
	
	// Get callee dialog
	calleeDialog := b.dialogManager.GetDialog(session.CalleeLeg.DialogID)
	if calleeDialog == nil {
		b.logger.Error("Callee dialog not found for BYE", 
			logging.Field{Key: "dialog_id", Value: session.CalleeLeg.DialogID})
		return bye
	}

	// Set headers based on callee dialog
	bye.SetHeader(parser.HeaderCallID, session.CalleeLeg.CallID)
	bye.SetHeader(parser.HeaderFrom, session.CalleeLeg.FromURI)
	
	// Build To header with remote tag from dialog
	calleeDialog.RLock()
	toHeader := BuildHeaderWithTag(
		ExtractURIFromHeader(session.CalleeLeg.ToURI),
		"",
		calleeDialog.RemoteTag,
	)
	cseqNum := calleeDialog.GetNextLocalCSeq()
	calleeDialog.RUnlock()
	
	bye.SetHeader(parser.HeaderTo, toHeader)
	bye.SetHeader(parser.HeaderCSeq, fmt.Sprintf("%d %s", cseqNum, parser.MethodBYE))
	bye.SetHeader(parser.HeaderMaxForwards, "70")
	bye.SetHeader(parser.HeaderContentLength, "0")
	
	// Add Via header
	viaHeader := fmt.Sprintf("SIP/2.0/UDP %s:%d;branch=z9hG4bK-%d", 
		b.serverHost, b.serverPort, time.Now().UnixNano())
	bye.AddHeader(parser.HeaderVia, viaHeader)
	
	// Use remote target if available, otherwise use To URI
	if calleeDialog.RemoteTarget != "" {
		bye.StartLine.(*parser.RequestLine).RequestURI = calleeDialog.RemoteTarget
	}
	
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
	
	// Get caller dialog
	callerDialog := b.dialogManager.GetDialog(session.CallerLeg.DialogID)
	if callerDialog == nil {
		b.logger.Error("Caller dialog not found", 
			logging.Field{Key: "dialog_id", Value: session.CallerLeg.DialogID})
		return response
	}

	// Update headers for caller leg dialog
	response.SetHeader(parser.HeaderCallID, session.CallerLeg.CallID)
	response.SetHeader(parser.HeaderFrom, session.CallerLeg.FromURI)
	
	// Build To header with our local tag
	callerDialog.RLock()
	toHeader := BuildHeaderWithTag(
		ExtractURIFromHeader(session.CallerLeg.ToURI),
		"",
		callerDialog.LocalTag,
	)
	callerDialog.RUnlock()
	response.SetHeader(parser.HeaderTo, toHeader)
	
	// Update Contact header to point to us
	response.SetHeader(parser.HeaderContact, session.CallerLeg.ContactURI)
	
	// Preserve Via headers from original request (they should be in reverse order)
	// The response will traverse back through the same path
	
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

// AddPendingLeg adds a new pending leg for hunt group calls
func (b *B2BUA) AddPendingLeg(sessionID string, memberURI string) (*CallLeg, error) {
	session, err := b.GetSession(sessionID)
	if err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	
	// Create new leg for hunt group member
	leg := &CallLeg{
		LegID:      b.generateLegID("member"),
		CallID:     b.generateCallID(),
		FromURI:    fmt.Sprintf("<sip:%s:%d>", b.serverHost, b.serverPort), // B2BUA as From
		ToURI:      fmt.Sprintf("<%s>", memberURI),
		FromTag:    b.generateTag(),
		ToTag:      "", // Will be set when member responds
		ContactURI: fmt.Sprintf("<sip:%s:%d>", b.serverHost, b.serverPort),
		Status:     CallLegStatusInitial,
		LocalSDP:   session.SDPOffer, // Forward caller's SDP
		LastCSeq:   1,                // Start with CSeq 1 for new dialog
		CreatedAt:  now,
	}

	// Add to session's pending legs
	session.AddPendingLeg(leg)

	// Add to lookup indices
	b.sessionMutex.Lock()
	b.sessionsByCallID[leg.CallID] = session
	b.sessionsByLegID[leg.LegID] = session
	b.sessionMutex.Unlock()

	b.logger.Info("Added pending leg to B2BUA session",
		logging.Field{Key: "session_id", Value: sessionID},
		logging.Field{Key: "leg_id", Value: leg.LegID},
		logging.Field{Key: "member_uri", Value: memberURI})

	return leg, nil
}

// HandleMemberAnswer handles when a hunt group member answers
func (b *B2BUA) HandleMemberAnswer(sessionID string, legID string, response *parser.SIPMessage) error {
	session, err := b.GetSession(sessionID)
	if err != nil {
		return err
	}

	leg := session.GetPendingLeg(legID)
	if leg == nil {
		return fmt.Errorf("pending leg not found: %s", legID)
	}

	statusCode := response.GetStatusCode()
	
	b.logger.Info("Hunt group member answered",
		logging.Field{Key: "session_id", Value: sessionID},
		logging.Field{Key: "leg_id", Value: legID},
		logging.Field{Key: "status_code", Value: statusCode})

	if statusCode >= 200 && statusCode < 300 {
		// Success response - this member answered
		session.SetAnsweredLeg(legID)
		
		// Cancel all other pending legs
		if err := b.CancelPendingLegs(sessionID, legID); err != nil {
			b.logger.Error("Failed to cancel pending legs",
				logging.Field{Key: "session_id", Value: sessionID},
				logging.Field{Key: "error", Value: err.Error()})
		}

		// Extract SDP answer if present
		if len(response.Body) > 0 {
			session.SDPAnswer = string(response.Body)
		}

		// Bridge the calls
		return b.BridgeCalls(sessionID)
	}

	// Non-success response - remove this leg and continue with others
	session.RemovePendingLeg(legID)
	
	// Remove from lookup indices
	b.sessionMutex.Lock()
	delete(b.sessionsByCallID, leg.CallID)
	delete(b.sessionsByLegID, leg.LegID)
	b.sessionMutex.Unlock()

	return nil
}

// CancelPendingLegs cancels all pending legs except the specified one
func (b *B2BUA) CancelPendingLegs(sessionID string, exceptLegID string) error {
	session, err := b.GetSession(sessionID)
	if err != nil {
		return err
	}

	pendingLegs := session.GetAllPendingLegs()
	
	for _, leg := range pendingLegs {
		if leg.LegID != exceptLegID {
			// Send CANCEL to this leg
			cancelMsg := b.createCancelForLeg(leg)
			if err := b.sendMessageToLeg(leg, cancelMsg); err != nil {
				b.logger.Error("Failed to send CANCEL to pending leg",
					logging.Field{Key: "session_id", Value: sessionID},
					logging.Field{Key: "leg_id", Value: leg.LegID},
					logging.Field{Key: "error", Value: err.Error()})
			}

			// Remove from session and indices
			session.RemovePendingLeg(leg.LegID)
			
			b.sessionMutex.Lock()
			delete(b.sessionsByCallID, leg.CallID)
			delete(b.sessionsByLegID, leg.LegID)
			b.sessionMutex.Unlock()

			b.logger.Debug("Cancelled pending leg",
				logging.Field{Key: "session_id", Value: sessionID},
				logging.Field{Key: "leg_id", Value: leg.LegID})
		}
	}

	return nil
}

// Helper methods for session management

func (b *B2BUA) removeSessionFromIndices(session *B2BUASession) {
	// Remove from Call-ID indices
	if session.CallerLeg != nil {
		delete(b.sessionsByCallID, session.CallerLeg.CallID)
		delete(b.sessionsByLegID, session.CallerLeg.LegID)
	}
	if session.CalleeLeg != nil {
		delete(b.sessionsByCallID, session.CalleeLeg.CallID)
		delete(b.sessionsByLegID, session.CalleeLeg.LegID)
	}
	
	// Remove pending legs
	for _, leg := range session.PendingLegs {
		delete(b.sessionsByCallID, leg.CallID)
		delete(b.sessionsByLegID, leg.LegID)
	}
}

func (b *B2BUA) generateLegID(prefix string) string {
	return fmt.Sprintf("%s-leg-%d", prefix, time.Now().UnixNano())
}

func (b *B2BUA) generateTag() string {
	return fmt.Sprintf("tag-%d", time.Now().UnixNano())
}

func (b *B2BUA) extractTag(headerValue string) string {
	// Simple tag extraction from From/To header
	// Format: "Display Name" <sip:user@domain>;tag=value
	if tagStart := strings.Index(headerValue, "tag="); tagStart != -1 {
		tagStart += 4
		tagEnd := strings.Index(headerValue[tagStart:], ";")
		if tagEnd == -1 {
			return headerValue[tagStart:]
		}
		return headerValue[tagStart : tagStart+tagEnd]
	}
	return ""
}

func (b *B2BUA) extractCSeq(cseqHeader string) uint32 {
	// CSeq header format: "123 INVITE"
	parts := strings.Fields(cseqHeader)
	if len(parts) > 0 {
		if cseq, err := strconv.ParseUint(parts[0], 10, 32); err == nil {
			return uint32(cseq)
		}
	}
	return 1
}

func (b *B2BUA) createCancelForLeg(leg *CallLeg) *parser.SIPMessage {
	cancel := parser.NewRequestMessage(parser.MethodCANCEL, leg.ToURI)
	
	// Set required headers
	cancel.SetHeader(parser.HeaderCallID, leg.CallID)
	cancel.SetHeader(parser.HeaderFrom, leg.FromURI)
	cancel.SetHeader(parser.HeaderTo, leg.ToURI)
	cancel.SetHeader(parser.HeaderCSeq, fmt.Sprintf("%d %s", leg.LastCSeq, parser.MethodCANCEL))
	cancel.SetHeader(parser.HeaderMaxForwards, "70")
	cancel.SetHeader(parser.HeaderContentLength, "0")
	
	// Add Via header
	viaHeader := fmt.Sprintf("SIP/2.0/UDP %s:%d;branch=z9hG4bK-%d", 
		b.serverHost, b.serverPort, time.Now().UnixNano())
	cancel.AddHeader(parser.HeaderVia, viaHeader)
	
	return cancel
}

func (b *B2BUA) sendMessageToLeg(leg *CallLeg, message *parser.SIPMessage) error {
	data, err := b.parser.Serialize(message)
	if err != nil {
		return fmt.Errorf("failed to serialize message for leg: %w", err)
	}

	b.logger.Debug("Sending message to leg",
		logging.Field{Key: "leg_id", Value: leg.LegID},
		logging.Field{Key: "method", Value: message.GetMethod()})

	return b.transportManager.SendMessage(data, "udp", leg.RemoteAddr)
}