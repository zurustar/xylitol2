package proxy

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/zurustar/xylitol2/internal/database"
	"github.com/zurustar/xylitol2/internal/parser"
	"github.com/zurustar/xylitol2/internal/transaction"
)

// ProxyState represents the state of a proxy transaction
type ProxyState struct {
	ID                string
	OriginalRequest   *parser.SIPMessage
	ServerTransaction transaction.Transaction
	ClientTransactions map[string]*ClientTransaction
	Targets           []*database.RegistrarContact
	BestResponse      *parser.SIPMessage
	BestResponseCode  int
	FinalResponseSent bool
	CreatedAt         time.Time
	mutex             sync.RWMutex
}

// ClientTransaction represents a client transaction for forking
type ClientTransaction struct {
	ID          string
	Transaction transaction.Transaction
	Target      *database.RegistrarContact
	Request     *parser.SIPMessage
	Response    *parser.SIPMessage
	State       ClientTransactionState
	CreatedAt   time.Time
}

// ClientTransactionState represents the state of a client transaction
type ClientTransactionState int

const (
	ClientStateTrying ClientTransactionState = iota
	ClientStateProceeding
	ClientStateCompleted
	ClientStateTerminated
)

// String returns the string representation of the client transaction state
func (cts ClientTransactionState) String() string {
	switch cts {
	case ClientStateTrying:
		return "Trying"
	case ClientStateProceeding:
		return "Proceeding"
	case ClientStateCompleted:
		return "Completed"
	case ClientStateTerminated:
		return "Terminated"
	default:
		return "Unknown"
	}
}

// StatefulProxyEngine implements stateful proxy functionality with forking
type StatefulProxyEngine struct {
	*RequestForwardingEngine
	proxyStates map[string]*ProxyState
	mutex       sync.RWMutex
}

// NewStatefulProxyEngine creates a new stateful proxy engine
func NewStatefulProxyEngine(
	forwardingEngine *RequestForwardingEngine,
) *StatefulProxyEngine {
	return &StatefulProxyEngine{
		RequestForwardingEngine: forwardingEngine,
		proxyStates:            make(map[string]*ProxyState),
	}
}

// ProcessRequest processes an incoming SIP request with stateful proxy logic
func (e *StatefulProxyEngine) ProcessRequest(req *parser.SIPMessage, transaction transaction.Transaction) error {
	if req == nil || !req.IsRequest() {
		return fmt.Errorf("invalid request message")
	}

	method := req.GetMethod()
	
	// Handle different request methods
	switch method {
	case parser.MethodINVITE:
		return e.processInviteRequest(req, transaction)
	case parser.MethodCANCEL:
		return e.processCancelRequest(req, transaction)
	case parser.MethodACK:
		return e.processAckRequest(req, transaction)
	case parser.MethodBYE, parser.MethodINFO:
		return e.processInDialogRequest(req, transaction)
	case parser.MethodREGISTER:
		// REGISTER requests are handled by the registrar, not proxied
		return fmt.Errorf("REGISTER requests should be handled by registrar")
	case parser.MethodOPTIONS:
		// OPTIONS can be handled locally or proxied depending on Request-URI
		return e.RequestForwardingEngine.processOptionsRequest(req, transaction)
	default:
		// Send 405 Method Not Allowed for unsupported methods
		return e.RequestForwardingEngine.sendMethodNotAllowed(req, transaction)
	}
}

// processInviteRequest processes INVITE requests with forking
func (e *StatefulProxyEngine) processInviteRequest(req *parser.SIPMessage, serverTxn transaction.Transaction) error {
	// Check Max-Forwards header to prevent loops
	if err := e.checkMaxForwards(req); err != nil {
		return e.sendTooManyHops(req, serverTxn)
	}

	// Decrement Max-Forwards
	e.decrementMaxForwards(req)

	// Extract target URI from Request-URI
	requestURI := req.GetRequestURI()
	if requestURI == "" {
		return e.sendBadRequest(req, serverTxn, "Missing Request-URI")
	}

	// Resolve targets using registrar database
	targets, err := e.resolveTarget(requestURI)
	if err != nil {
		return e.sendNotFound(req, serverTxn, "User not registered")
	}

	if len(targets) == 0 {
		return e.sendNotFound(req, serverTxn, "No registered contacts")
	}

	// Create proxy state for this transaction
	proxyState := &ProxyState{
		ID:                 e.generateProxyStateID(req),
		OriginalRequest:    req.Clone(),
		ServerTransaction:  serverTxn,
		ClientTransactions: make(map[string]*ClientTransaction),
		Targets:           targets,
		BestResponseCode:  600, // Initialize with worst possible response
		CreatedAt:         time.Now(),
	}

	// Store proxy state
	e.mutex.Lock()
	e.proxyStates[proxyState.ID] = proxyState
	e.mutex.Unlock()

	// Fork the request to all targets
	return e.forkRequest(proxyState)
}

// processCancelRequest processes CANCEL requests
func (e *StatefulProxyEngine) processCancelRequest(req *parser.SIPMessage, serverTxn transaction.Transaction) error {
	// Find the corresponding INVITE transaction
	inviteStateID := e.generateProxyStateID(req)
	
	e.mutex.RLock()
	proxyState, exists := e.proxyStates[inviteStateID]
	e.mutex.RUnlock()

	if !exists {
		// No matching INVITE transaction found
		return e.sendCallTransactionDoesNotExist(req, serverTxn)
	}

	proxyState.mutex.Lock()
	defer proxyState.mutex.Unlock()

	// Send CANCEL to all active client transactions
	for _, clientTxn := range proxyState.ClientTransactions {
		if clientTxn.State == ClientStateTrying || clientTxn.State == ClientStateProceeding {
			e.sendCancelToTarget(clientTxn, req)
		}
	}

	// Send 200 OK to CANCEL request
	response := parser.NewResponseMessage(parser.StatusOK, "OK")
	e.copyRequiredHeaders(req, response)
	return serverTxn.SendResponse(response)
}

// processAckRequest processes ACK requests
func (e *StatefulProxyEngine) processAckRequest(req *parser.SIPMessage, serverTxn transaction.Transaction) error {
	// ACK requests are forwarded to the target that sent the 2xx response
	stateID := e.generateProxyStateID(req)
	
	e.mutex.RLock()
	proxyState, exists := e.proxyStates[stateID]
	e.mutex.RUnlock()

	if !exists {
		// No matching transaction found - this is normal for ACK to 2xx responses
		return nil
	}

	proxyState.mutex.RLock()
	defer proxyState.mutex.RUnlock()

	// Find the client transaction that received the 2xx response
	for _, clientTxn := range proxyState.ClientTransactions {
		if clientTxn.Response != nil && clientTxn.Response.GetStatusCode() >= 200 && clientTxn.Response.GetStatusCode() < 300 {
			// Forward ACK to this target
			return e.forwardAckToTarget(clientTxn, req)
		}
	}

	return nil
}

// processInDialogRequest processes in-dialog requests (BYE, INFO, etc.)
func (e *StatefulProxyEngine) processInDialogRequest(req *parser.SIPMessage, serverTxn transaction.Transaction) error {
	// For in-dialog requests, we need to route based on the dialog state
	// For simplicity, we'll use the basic forwarding engine
	return e.RequestForwardingEngine.processProxyableRequest(req, serverTxn)
}

// forkRequest forks a request to multiple targets
func (e *StatefulProxyEngine) forkRequest(proxyState *ProxyState) error {
	proxyState.mutex.Lock()
	defer proxyState.mutex.Unlock()

	// Create client transactions for each target
	for i, target := range proxyState.Targets {
		clientTxn := &ClientTransaction{
			ID:        fmt.Sprintf("%s-client-%d", proxyState.ID, i),
			Target:    target,
			State:     ClientStateTrying,
			CreatedAt: time.Now(),
		}

		// Create a copy of the request for this target
		forwardedReq := proxyState.OriginalRequest.Clone()

		// Add Via header for this proxy
		viaHeader := e.createViaHeader(forwardedReq.Transport)
		e.addViaHeader(forwardedReq, viaHeader)

		// Update Request-URI to target contact
		if reqLine, ok := forwardedReq.StartLine.(*parser.RequestLine); ok {
			reqLine.RequestURI = target.URI
		}

		clientTxn.Request = forwardedReq

		// Create client transaction
		clientTxn.Transaction = e.transactionManager.CreateTransaction(forwardedReq)

		// Store client transaction
		proxyState.ClientTransactions[clientTxn.ID] = clientTxn

		// Send the request
		if err := e.sendRequestToTarget(clientTxn); err != nil {
			// Mark this client transaction as failed
			clientTxn.State = ClientStateTerminated
			continue
		}
	}

	// Check if all client transactions failed
	allFailed := true
	for _, clientTxn := range proxyState.ClientTransactions {
		if clientTxn.State != ClientStateTerminated {
			allFailed = false
			break
		}
	}

	if allFailed {
		// Send 500 Server Internal Error
		response := parser.NewResponseMessage(parser.StatusServerInternalError, "All targets failed")
		e.copyRequiredHeaders(proxyState.OriginalRequest, response)
		return proxyState.ServerTransaction.SendResponse(response)
	}

	return nil
}

// ProcessResponse processes an incoming SIP response with stateful proxy logic
func (e *StatefulProxyEngine) ProcessResponse(resp *parser.SIPMessage, transaction transaction.Transaction) error {
	if resp == nil || !resp.IsResponse() {
		return fmt.Errorf("invalid response message")
	}

	// Find the proxy state for this response
	proxyState := e.findProxyStateForResponse(resp)
	if proxyState == nil {
		// No matching proxy state - forward using basic engine
		return e.RequestForwardingEngine.ProcessResponse(resp, transaction)
	}

	proxyState.mutex.Lock()
	defer proxyState.mutex.Unlock()

	// Find the client transaction that sent this response
	var clientTxn *ClientTransaction
	for _, ct := range proxyState.ClientTransactions {
		if ct.Transaction == transaction {
			clientTxn = ct
			break
		}
	}

	if clientTxn == nil {
		return fmt.Errorf("no matching client transaction found for response")
	}

	// Update client transaction state
	clientTxn.Response = resp
	statusCode := resp.GetStatusCode()

	if statusCode >= 100 && statusCode < 200 {
		// Provisional response
		clientTxn.State = ClientStateProceeding
		return e.handleProvisionalResponse(proxyState, clientTxn, resp)
	} else if statusCode >= 200 && statusCode < 300 {
		// Success response
		clientTxn.State = ClientStateCompleted
		return e.handleSuccessResponse(proxyState, clientTxn, resp)
	} else if statusCode >= 300 {
		// Error response
		clientTxn.State = ClientStateCompleted
		return e.handleErrorResponse(proxyState, clientTxn, resp)
	}

	return nil
}

// handleProvisionalResponse handles provisional responses (1xx)
func (e *StatefulProxyEngine) handleProvisionalResponse(proxyState *ProxyState, clientTxn *ClientTransaction, resp *parser.SIPMessage) error {
	if proxyState.FinalResponseSent {
		return nil // Don't forward provisional responses after final response
	}

	// Forward the provisional response to the client
	forwardedResp := resp.Clone()
	e.removeTopViaHeader(forwardedResp)

	return proxyState.ServerTransaction.SendResponse(forwardedResp)
}

// handleSuccessResponse handles success responses (2xx)
func (e *StatefulProxyEngine) handleSuccessResponse(proxyState *ProxyState, clientTxn *ClientTransaction, resp *parser.SIPMessage) error {
	if proxyState.FinalResponseSent {
		return nil // Already sent final response
	}

	// Cancel all other client transactions
	e.cancelOtherClientTransactions(proxyState, clientTxn.ID)

	// Forward the success response
	forwardedResp := resp.Clone()
	e.removeTopViaHeader(forwardedResp)

	proxyState.BestResponse = forwardedResp
	proxyState.BestResponseCode = resp.GetStatusCode()
	proxyState.FinalResponseSent = true

	return proxyState.ServerTransaction.SendResponse(forwardedResp)
}

// handleErrorResponse handles error responses (3xx, 4xx, 5xx, 6xx)
func (e *StatefulProxyEngine) handleErrorResponse(proxyState *ProxyState, clientTxn *ClientTransaction, resp *parser.SIPMessage) error {
	if proxyState.FinalResponseSent {
		return nil // Already sent final response
	}

	statusCode := resp.GetStatusCode()

	// Update best response if this is better
	if e.isBetterResponse(statusCode, proxyState.BestResponseCode) {
		proxyState.BestResponse = resp.Clone()
		proxyState.BestResponseCode = statusCode
	}

	// Check if all client transactions are completed
	allCompleted := true
	for _, ct := range proxyState.ClientTransactions {
		if ct.State != ClientStateCompleted && ct.State != ClientStateTerminated {
			allCompleted = false
			break
		}
	}

	if allCompleted {
		// Send the best response
		if proxyState.BestResponse != nil {
			forwardedResp := proxyState.BestResponse.Clone()
			e.removeTopViaHeader(forwardedResp)
			proxyState.FinalResponseSent = true
			return proxyState.ServerTransaction.SendResponse(forwardedResp)
		}
	}

	return nil
}

// Helper methods

func (e *StatefulProxyEngine) generateProxyStateID(req *parser.SIPMessage) string {
	// Generate ID based on Call-ID and CSeq number (not method) for transaction matching
	callID := req.GetHeader(parser.HeaderCallID)
	cseqHeader := req.GetHeader(parser.HeaderCSeq)
	
	// Extract just the sequence number from CSeq header
	cseqParts := strings.Fields(cseqHeader)
	if len(cseqParts) >= 1 {
		return fmt.Sprintf("%s-%s-INVITE", callID, cseqParts[0])
	}
	
	return fmt.Sprintf("%s-%s", callID, cseqHeader)
}

func (e *StatefulProxyEngine) findProxyStateForResponse(resp *parser.SIPMessage) *ProxyState {
	// Find proxy state based on Call-ID and CSeq number (matching INVITE transaction)
	callID := resp.GetHeader(parser.HeaderCallID)
	cseqHeader := resp.GetHeader(parser.HeaderCSeq)
	
	// Extract just the sequence number from CSeq header
	cseqParts := strings.Fields(cseqHeader)
	if len(cseqParts) >= 1 {
		stateID := fmt.Sprintf("%s-%s-INVITE", callID, cseqParts[0])
		
		e.mutex.RLock()
		defer e.mutex.RUnlock()
		
		return e.proxyStates[stateID]
	}

	return nil
}

func (e *StatefulProxyEngine) sendRequestToTarget(clientTxn *ClientTransaction) error {
	// Parse target address
	targetAddr, transport, err := e.parseTargetURI(clientTxn.Target.URI)
	if err != nil {
		return fmt.Errorf("failed to parse target URI %s: %w", clientTxn.Target.URI, err)
	}

	// Serialize the message
	data, err := e.parser.Serialize(clientTxn.Request)
	if err != nil {
		return fmt.Errorf("failed to serialize request: %w", err)
	}

	// Send the request
	return e.transportManager.SendMessage(data, transport, targetAddr)
}

func (e *StatefulProxyEngine) sendCancelToTarget(clientTxn *ClientTransaction, originalCancel *parser.SIPMessage) error {
	// Create CANCEL request for this target
	cancelReq := parser.NewRequestMessage(parser.MethodCANCEL, clientTxn.Target.URI)

	// Copy required headers from original CANCEL
	e.copyRequiredHeaders(originalCancel, cancelReq)

	// Update Request-URI
	if reqLine, ok := cancelReq.StartLine.(*parser.RequestLine); ok {
		reqLine.RequestURI = clientTxn.Target.URI
	}

	// Parse target address
	targetAddr, transport, err := e.parseTargetURI(clientTxn.Target.URI)
	if err != nil {
		return fmt.Errorf("failed to parse target URI for CANCEL: %w", err)
	}

	// Serialize and send
	data, err := e.parser.Serialize(cancelReq)
	if err != nil {
		return fmt.Errorf("failed to serialize CANCEL: %w", err)
	}

	return e.transportManager.SendMessage(data, transport, targetAddr)
}

func (e *StatefulProxyEngine) forwardAckToTarget(clientTxn *ClientTransaction, ack *parser.SIPMessage) error {
	// Create ACK request for this target
	ackReq := ack.Clone()

	// Update Request-URI
	if reqLine, ok := ackReq.StartLine.(*parser.RequestLine); ok {
		reqLine.RequestURI = clientTxn.Target.URI
	}

	// Parse target address
	targetAddr, transport, err := e.parseTargetURI(clientTxn.Target.URI)
	if err != nil {
		return fmt.Errorf("failed to parse target URI for ACK: %w", err)
	}

	// Serialize and send
	data, err := e.parser.Serialize(ackReq)
	if err != nil {
		return fmt.Errorf("failed to serialize ACK: %w", err)
	}

	return e.transportManager.SendMessage(data, transport, targetAddr)
}

func (e *StatefulProxyEngine) cancelOtherClientTransactions(proxyState *ProxyState, excludeID string) {
	for id, clientTxn := range proxyState.ClientTransactions {
		if id != excludeID && (clientTxn.State == ClientStateTrying || clientTxn.State == ClientStateProceeding) {
			// Send CANCEL to this target
			cancelReq := parser.NewRequestMessage(parser.MethodCANCEL, clientTxn.Target.URI)
			e.copyRequiredHeaders(proxyState.OriginalRequest, cancelReq)
			
			// Update Request-URI
			if reqLine, ok := cancelReq.StartLine.(*parser.RequestLine); ok {
				reqLine.RequestURI = clientTxn.Target.URI
			}

			// Send CANCEL (ignore errors for cleanup)
			if targetAddr, transport, err := e.parseTargetURI(clientTxn.Target.URI); err == nil {
				if data, err := e.parser.Serialize(cancelReq); err == nil {
					e.transportManager.SendMessage(data, transport, targetAddr)
				}
			}

			clientTxn.State = ClientStateTerminated
		}
	}
}

func (e *StatefulProxyEngine) removeTopViaHeader(msg *parser.SIPMessage) {
	viaHeaders := msg.GetHeaders(parser.HeaderVia)
	if len(viaHeaders) > 0 {
		msg.RemoveHeader(parser.HeaderVia)
		for i := 1; i < len(viaHeaders); i++ {
			msg.AddHeader(parser.HeaderVia, viaHeaders[i])
		}
	}
}

func (e *StatefulProxyEngine) isBetterResponse(newCode, currentBestCode int) bool {
	// Response preference order (lower is better):
	// 2xx > 3xx > 4xx > 5xx > 6xx
	// Within same class, lower code is better
	
	newClass := newCode / 100
	currentClass := currentBestCode / 100
	
	if newClass != currentClass {
		// Different classes - prefer lower class number (except 2xx is best)
		if newClass == 2 {
			return true
		}
		if currentClass == 2 {
			return false
		}
		return newClass < currentClass
	}
	
	// Same class - prefer lower code
	return newCode < currentBestCode
}

// Error response methods

func (e *StatefulProxyEngine) sendCallTransactionDoesNotExist(req *parser.SIPMessage, transaction transaction.Transaction) error {
	response := parser.NewResponseMessage(parser.StatusCallTransactionDoesNotExist, "Call/Transaction Does Not Exist")
	e.copyRequiredHeaders(req, response)
	return transaction.SendResponse(response)
}

// CleanupExpiredStates removes expired proxy states
func (e *StatefulProxyEngine) CleanupExpiredStates() {
	e.mutex.Lock()
	defer e.mutex.Unlock()

	now := time.Now()
	expireTime := 5 * time.Minute // Expire states after 5 minutes

	for id, state := range e.proxyStates {
		if now.Sub(state.CreatedAt) > expireTime {
			delete(e.proxyStates, id)
		}
	}
}

// GetProxyStateCount returns the number of active proxy states
func (e *StatefulProxyEngine) GetProxyStateCount() int {
	e.mutex.RLock()
	defer e.mutex.RUnlock()
	return len(e.proxyStates)
}