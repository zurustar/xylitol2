package huntgroup

import (
	"fmt"
	"sync"
	"time"

	"github.com/zurustar/xylitol2/internal/database"
	"github.com/zurustar/xylitol2/internal/logging"
	"github.com/zurustar/xylitol2/internal/parser"
	"github.com/zurustar/xylitol2/internal/registrar"
	"github.com/zurustar/xylitol2/internal/transaction"
	"github.com/zurustar/xylitol2/internal/transport"
)

// Engine implements the HuntGroupEngine interface
type Engine struct {
	manager            HuntGroupManager
	registrar          registrar.Registrar
	transportManager   transport.TransportManager
	transactionManager transaction.TransactionManager
	parser             parser.MessageParser
	logger             logging.Logger
	
	// Active sessions
	activeSessions map[string]*CallSession
	sessionMutex   sync.RWMutex
	
	// Configuration
	maxConcurrent   int
	defaultTimeout  int
	callWaitingTime int
}

// NewEngine creates a new hunt group engine
func NewEngine(
	manager HuntGroupManager,
	registrar registrar.Registrar,
	transportManager transport.TransportManager,
	transactionManager transaction.TransactionManager,
	parser parser.MessageParser,
	logger logging.Logger,
) *Engine {
	return &Engine{
		manager:            manager,
		registrar:          registrar,
		transportManager:   transportManager,
		transactionManager: transactionManager,
		parser:             parser,
		logger:             logger,
		activeSessions:     make(map[string]*CallSession),
		maxConcurrent:      10,
		defaultTimeout:     30,
		callWaitingTime:    5,
	}
}

// SetConfiguration sets engine configuration
func (e *Engine) SetConfiguration(maxConcurrent, defaultTimeout, callWaitingTime int) {
	e.maxConcurrent = maxConcurrent
	e.defaultTimeout = defaultTimeout
	e.callWaitingTime = callWaitingTime
}

// ProcessIncomingCall processes an incoming call to a hunt group
func (e *Engine) ProcessIncomingCall(invite *parser.SIPMessage, group *HuntGroup) (*CallSession, error) {
	if invite == nil || group == nil {
		return nil, fmt.Errorf("invalid parameters")
	}

	if !group.Enabled {
		return nil, fmt.Errorf("hunt group is disabled")
	}

	if len(group.Members) == 0 {
		return nil, fmt.Errorf("hunt group has no members")
	}

	// Create call session
	sessionID := e.generateSessionID()
	callerURI := invite.GetHeader(parser.HeaderFrom)
	
	session := &CallSession{
		ID:             sessionID,
		GroupID:        group.ID,
		CallerURI:      callerURI,
		OriginalINVITE: invite,
		MemberCalls:    make(map[string]*MemberCall),
		StartTime:      time.Now().UTC(),
		Status:         SessionStatusRinging,
	}

	// Store session
	e.sessionMutex.Lock()
	e.activeSessions[sessionID] = session
	e.sessionMutex.Unlock()

	// Log session creation
	if err := e.manager.CreateSession(session); err != nil {
		e.logger.Warn("Failed to log call session", 
			logging.Field{Key: "session_id", Value: sessionID},
			logging.Field{Key: "error", Value: err})
	}

	e.logger.Info("Hunt group call session created",
		logging.Field{Key: "session_id", Value: sessionID},
		logging.Field{Key: "group_id", Value: group.ID},
		logging.Field{Key: "caller", Value: callerURI})

	// Start calling members based on strategy
	switch group.Strategy {
	case StrategySimultaneous:
		return session, e.callMembersSimultaneously(session, group)
	case StrategySequential:
		return session, e.callMembersSequentially(session, group)
	case StrategyRoundRobin:
		return session, e.callMembersRoundRobin(session, group)
	case StrategyLongestIdle:
		return session, e.callMembersLongestIdle(session, group)
	default:
		return session, e.callMembersSimultaneously(session, group)
	}
}

// callMembersSimultaneously calls all enabled members simultaneously
func (e *Engine) callMembersSimultaneously(session *CallSession, group *HuntGroup) error {
	enabledMembers := e.getEnabledMembers(group.Members)
	if len(enabledMembers) == 0 {
		return fmt.Errorf("no enabled members in hunt group")
	}

	e.logger.Info("Calling hunt group members simultaneously",
		logging.Field{Key: "session_id", Value: session.ID},
		logging.Field{Key: "member_count", Value: len(enabledMembers)})

	// Call all enabled members
	for _, member := range enabledMembers {
		if err := e.callMember(session, member, group); err != nil {
			e.logger.Warn("Failed to call hunt group member",
				logging.Field{Key: "session_id", Value: session.ID},
				logging.Field{Key: "member", Value: member.Extension},
				logging.Field{Key: "error", Value: err})
		}
	}

	// Start timeout timer for the session
	go e.startSessionTimeout(session, group)

	return nil
}

// callMembersSequentially calls members one by one in priority order
func (e *Engine) callMembersSequentially(session *CallSession, group *HuntGroup) error {
	enabledMembers := e.getEnabledMembers(group.Members)
	if len(enabledMembers) == 0 {
		return fmt.Errorf("no enabled members in hunt group")
	}

	e.logger.Info("Calling hunt group members sequentially",
		logging.Field{Key: "session_id", Value: session.ID},
		logging.Field{Key: "member_count", Value: len(enabledMembers)})

	// Call first member
	if err := e.callMember(session, enabledMembers[0], group); err != nil {
		return fmt.Errorf("failed to call first member: %w", err)
	}

	// Start sequential calling process
	go e.continueSequentialCalling(session, group, enabledMembers, 0)

	return nil
}

// callMembersRoundRobin calls members using round-robin strategy
func (e *Engine) callMembersRoundRobin(session *CallSession, group *HuntGroup) error {
	// For simplicity, implement as sequential for now
	// In a full implementation, this would track the last called member
	return e.callMembersSequentially(session, group)
}

// callMembersLongestIdle calls the member who has been idle the longest
func (e *Engine) callMembersLongestIdle(session *CallSession, group *HuntGroup) error {
	// For simplicity, implement as sequential for now
	// In a full implementation, this would track member idle times
	return e.callMembersSequentially(session, group)
}

// callMember initiates a call to a specific hunt group member
func (e *Engine) callMember(session *CallSession, member *HuntGroupMember, group *HuntGroup) error {
	// Resolve member extension to contact
	contacts, err := e.registrar.FindContacts(fmt.Sprintf("sip:%s@%s", member.Extension, "test.local"))
	if err != nil || len(contacts) == 0 {
		return fmt.Errorf("member %s not registered", member.Extension)
	}

	contact := contacts[0]
	if contact.Expires.Before(time.Now().UTC()) {
		return fmt.Errorf("member %s registration expired", member.Extension)
	}

	// Create member call
	memberCall := &MemberCall{
		MemberExtension: member.Extension,
		CallID:          e.generateCallID(),
		Status:          MemberCallStatusRinging,
		StartTime:       time.Now().UTC(),
	}

	// Store member call
	session.MemberCalls[member.Extension] = memberCall

	// Create INVITE for member
	memberInvite := e.createMemberInvite(session.OriginalINVITE, contact, memberCall.CallID)

	// Send INVITE to member
	if err := e.sendInviteToMember(memberInvite, contact); err != nil {
		memberCall.Status = MemberCallStatusFailed
		return fmt.Errorf("failed to send INVITE to member %s: %w", member.Extension, err)
	}

	e.logger.Info("INVITE sent to hunt group member",
		logging.Field{Key: "session_id", Value: session.ID},
		logging.Field{Key: "member", Value: member.Extension},
		logging.Field{Key: "call_id", Value: memberCall.CallID})

	// Start member timeout
	timeout := member.Timeout
	if timeout == 0 {
		timeout = group.RingTimeout
	}
	go e.startMemberTimeout(session, member.Extension, timeout)

	return nil
}

// HandleMemberResponse handles responses from hunt group members
func (e *Engine) HandleMemberResponse(sessionID string, memberExtension string, response *parser.SIPMessage) error {
	e.sessionMutex.RLock()
	session, exists := e.activeSessions[sessionID]
	e.sessionMutex.RUnlock()

	if !exists {
		return fmt.Errorf("session not found: %s", sessionID)
	}

	memberCall, exists := session.MemberCalls[memberExtension]
	if !exists {
		return fmt.Errorf("member call not found: %s", memberExtension)
	}

	statusCode := response.GetStatusCode()

	e.logger.Info("Received response from hunt group member",
		logging.Field{Key: "session_id", Value: sessionID},
		logging.Field{Key: "member", Value: memberExtension},
		logging.Field{Key: "status_code", Value: statusCode})

	switch {
	case statusCode >= 200 && statusCode < 300:
		// Success response - member answered
		return e.handleMemberAnswer(session, memberExtension, response)
	case statusCode == 486 || statusCode == 600:
		// Busy
		memberCall.Status = MemberCallStatusBusy
		memberCall.EndTime = &[]time.Time{time.Now().UTC()}[0]
		return e.checkSessionCompletion(session)
	case statusCode == 408 || statusCode == 480:
		// No answer / timeout
		memberCall.Status = MemberCallStatusNoAnswer
		memberCall.EndTime = &[]time.Time{time.Now().UTC()}[0]
		return e.checkSessionCompletion(session)
	case statusCode >= 400:
		// Error response
		memberCall.Status = MemberCallStatusFailed
		memberCall.EndTime = &[]time.Time{time.Now().UTC()}[0]
		return e.checkSessionCompletion(session)
	default:
		// Provisional response - continue ringing
		return nil
	}
}

// handleMemberAnswer handles when a member answers the call
func (e *Engine) handleMemberAnswer(session *CallSession, memberExtension string, response *parser.SIPMessage) error {
	memberCall := session.MemberCalls[memberExtension]
	now := time.Now().UTC()
	
	memberCall.Status = MemberCallStatusAnswered
	memberCall.AnswerTime = &now
	
	session.Status = SessionStatusAnswered
	session.AnsweredBy = memberExtension
	session.AnsweredAt = &now

	e.logger.Info("Hunt group member answered",
		logging.Field{Key: "session_id", Value: session.ID},
		logging.Field{Key: "member", Value: memberExtension})

	// Cancel all other member calls
	for ext, call := range session.MemberCalls {
		if ext != memberExtension && call.Status == MemberCallStatusRinging {
			call.Status = MemberCallStatusCancelled
			call.EndTime = &now
			// Send CANCEL to member (implementation would go here)
		}
	}

	// Update session log
	if err := e.manager.UpdateSession(session); err != nil {
		e.logger.Warn("Failed to update session log",
			logging.Field{Key: "session_id", Value: session.ID},
			logging.Field{Key: "error", Value: err})
	}

	// Forward response to original caller
	return e.forwardResponseToCaller(session, response)
}

// CancelSession cancels all pending calls in a session
func (e *Engine) CancelSession(sessionID string) error {
	e.sessionMutex.RLock()
	session, exists := e.activeSessions[sessionID]
	e.sessionMutex.RUnlock()

	if !exists {
		return fmt.Errorf("session not found: %s", sessionID)
	}

	now := time.Now().UTC()
	session.Status = SessionStatusCancelled

	// Cancel all member calls
	for _, call := range session.MemberCalls {
		if call.Status == MemberCallStatusRinging {
			call.Status = MemberCallStatusCancelled
			call.EndTime = &now
			// Send CANCEL to member (implementation would go here)
		}
	}

	// Remove from active sessions
	e.sessionMutex.Lock()
	delete(e.activeSessions, sessionID)
	e.sessionMutex.Unlock()

	// Update session log
	if err := e.manager.UpdateSession(session); err != nil {
		e.logger.Warn("Failed to update cancelled session log",
			logging.Field{Key: "session_id", Value: sessionID},
			logging.Field{Key: "error", Value: err})
	}

	e.logger.Info("Hunt group session cancelled",
		logging.Field{Key: "session_id", Value: sessionID})

	return nil
}

// GetCallStatistics retrieves call statistics for a hunt group
func (e *Engine) GetCallStatistics(groupID int) (*CallStatistics, error) {
	return e.manager.GetCallStatistics(groupID)
}

// Helper methods

func (e *Engine) getEnabledMembers(members []*HuntGroupMember) []*HuntGroupMember {
	var enabled []*HuntGroupMember
	for _, member := range members {
		if member.Enabled {
			enabled = append(enabled, member)
		}
	}
	return enabled
}

func (e *Engine) generateSessionID() string {
	return fmt.Sprintf("hg-session-%d", time.Now().UnixNano())
}

func (e *Engine) generateCallID() string {
	return fmt.Sprintf("hg-call-%d", time.Now().UnixNano())
}

func (e *Engine) createMemberInvite(originalInvite *parser.SIPMessage, contact *database.RegistrarContact, callID string) *parser.SIPMessage {
	// Create a new INVITE based on the original
	memberInvite := originalInvite.Clone()
	
	// Update Call-ID
	memberInvite.SetHeader(parser.HeaderCallID, callID)
	
	// Update Request-URI to member contact
	if reqLine, ok := memberInvite.StartLine.(*parser.RequestLine); ok {
		reqLine.RequestURI = contact.URI
	}
	
	// Update To header to member
	memberInvite.SetHeader(parser.HeaderTo, fmt.Sprintf("<%s>", contact.URI))
	
	// Add Via header for this proxy
	viaHeader := fmt.Sprintf("SIP/2.0/UDP 127.0.0.1:5060;branch=z9hG4bK-%d", time.Now().UnixNano())
	existingVias := memberInvite.GetHeaders(parser.HeaderVia)
	memberInvite.RemoveHeader(parser.HeaderVia)
	memberInvite.AddHeader(parser.HeaderVia, viaHeader)
	for _, via := range existingVias {
		memberInvite.AddHeader(parser.HeaderVia, via)
	}
	
	return memberInvite
}

func (e *Engine) sendInviteToMember(invite *parser.SIPMessage, contact *database.RegistrarContact) error {
	// Serialize the INVITE
	data, err := e.parser.Serialize(invite)
	if err != nil {
		return fmt.Errorf("failed to serialize INVITE: %w", err)
	}

	// Parse contact URI to get address
	// This is a simplified implementation
	// In a real implementation, you would properly parse the SIP URI
	return e.transportManager.SendMessage(data, "udp", nil)
}

func (e *Engine) forwardResponseToCaller(session *CallSession, response *parser.SIPMessage) error {
	// Forward the response to the original caller
	// This is a simplified implementation
	// In a real B2BUA, you would maintain the caller transaction and forward properly
	e.logger.Info("Forwarding response to caller",
		logging.Field{Key: "session_id", Value: session.ID},
		logging.Field{Key: "status_code", Value: response.GetStatusCode()})
	return nil
}

func (e *Engine) startSessionTimeout(session *CallSession, group *HuntGroup) {
	timeout := time.Duration(group.RingTimeout) * time.Second
	timer := time.NewTimer(timeout)
	defer timer.Stop()

	<-timer.C

	// Check if session is still ringing
	e.sessionMutex.RLock()
	currentSession, exists := e.activeSessions[session.ID]
	e.sessionMutex.RUnlock()

	if exists && currentSession.Status == SessionStatusRinging {
		e.logger.Info("Hunt group session timed out",
			logging.Field{Key: "session_id", Value: session.ID})
		
		// Cancel the session
		e.CancelSession(session.ID)
	}
}

func (e *Engine) startMemberTimeout(session *CallSession, memberExtension string, timeout int) {
	timer := time.NewTimer(time.Duration(timeout) * time.Second)
	defer timer.Stop()

	<-timer.C

	// Check if member call is still ringing
	e.sessionMutex.RLock()
	currentSession, exists := e.activeSessions[session.ID]
	e.sessionMutex.RUnlock()

	if exists {
		if memberCall, exists := currentSession.MemberCalls[memberExtension]; exists {
			if memberCall.Status == MemberCallStatusRinging {
				e.logger.Info("Hunt group member call timed out",
					logging.Field{Key: "session_id", Value: session.ID},
					logging.Field{Key: "member", Value: memberExtension})
				
				now := time.Now().UTC()
				memberCall.Status = MemberCallStatusNoAnswer
				memberCall.EndTime = &now
				
				// Check if session should be completed
				e.checkSessionCompletion(currentSession)
			}
		}
	}
}

func (e *Engine) continueSequentialCalling(session *CallSession, group *HuntGroup, members []*HuntGroupMember, currentIndex int) {
	// Wait for call waiting time
	time.Sleep(time.Duration(e.callWaitingTime) * time.Second)

	// Check if session is still active and no one has answered
	e.sessionMutex.RLock()
	currentSession, exists := e.activeSessions[session.ID]
	e.sessionMutex.RUnlock()

	if !exists || currentSession.Status != SessionStatusRinging {
		return
	}

	// Check if current member is still ringing or has failed
	currentMember := members[currentIndex]
	memberCall := currentSession.MemberCalls[currentMember.Extension]
	
	if memberCall.Status == MemberCallStatusRinging {
		// Still ringing, wait more
		go e.continueSequentialCalling(session, group, members, currentIndex)
		return
	}

	// Current member failed or didn't answer, try next
	nextIndex := currentIndex + 1
	if nextIndex < len(members) {
		if err := e.callMember(session, members[nextIndex], group); err != nil {
			e.logger.Warn("Failed to call next sequential member",
				logging.Field{Key: "session_id", Value: session.ID},
				logging.Field{Key: "member", Value: members[nextIndex].Extension},
				logging.Field{Key: "error", Value: err})
		}
		go e.continueSequentialCalling(session, group, members, nextIndex)
	} else {
		// No more members to try
		e.checkSessionCompletion(currentSession)
	}
}

func (e *Engine) checkSessionCompletion(session *CallSession) error {
	// Check if all member calls have completed
	allCompleted := true
	for _, call := range session.MemberCalls {
		if call.Status == MemberCallStatusRinging {
			allCompleted = false
			break
		}
	}

	if allCompleted && session.Status == SessionStatusRinging {
		// No one answered
		session.Status = SessionStatusFailed
		
		// Remove from active sessions
		e.sessionMutex.Lock()
		delete(e.activeSessions, session.ID)
		e.sessionMutex.Unlock()

		// Update session log
		if err := e.manager.UpdateSession(session); err != nil {
			e.logger.Warn("Failed to update failed session log",
				logging.Field{Key: "session_id", Value: session.ID},
				logging.Field{Key: "error", Value: err})
		}

		e.logger.Info("Hunt group session failed - no members answered",
			logging.Field{Key: "session_id", Value: session.ID})

		// Send appropriate response to caller (implementation would go here)
	}

	return nil
}