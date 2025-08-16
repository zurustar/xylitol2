package handlers

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/zurustar/xylitol2/internal/parser"
	"github.com/zurustar/xylitol2/internal/proxy"
	"github.com/zurustar/xylitol2/internal/registrar"
	"github.com/zurustar/xylitol2/internal/sessiontimer"
	"github.com/zurustar/xylitol2/internal/transaction"
)

// SessionHandler handles INVITE, ACK, and BYE requests for session management
type SessionHandler struct {
	proxyEngine     proxy.ProxyEngine
	registrar       registrar.Registrar
	sessionTimerMgr sessiontimer.SessionTimerManager
}

// NewSessionHandler creates a new session handler
func NewSessionHandler(proxyEngine proxy.ProxyEngine, registrar registrar.Registrar, sessionTimerMgr sessiontimer.SessionTimerManager) *SessionHandler {
	return &SessionHandler{
		proxyEngine:     proxyEngine,
		registrar:       registrar,
		sessionTimerMgr: sessionTimerMgr,
	}
}

// CanHandle returns true if this handler can process the given method
func (h *SessionHandler) CanHandle(method string) bool {
	switch method {
	case parser.MethodINVITE, parser.MethodACK, parser.MethodBYE:
		return true
	default:
		return false
	}
}

// HandleRequest processes INVITE, ACK, and BYE requests
func (h *SessionHandler) HandleRequest(req *parser.SIPMessage, txn transaction.Transaction) error {
	method := req.GetMethod()
	
	switch method {
	case parser.MethodINVITE:
		return h.handleInvite(req, txn)
	case parser.MethodACK:
		return h.handleAck(req, txn)
	case parser.MethodBYE:
		return h.handleBye(req, txn)
	default:
		return fmt.Errorf("unsupported method: %s", method)
	}
}

// handleInvite processes INVITE requests with Session-Timer validation
func (h *SessionHandler) handleInvite(req *parser.SIPMessage, txn transaction.Transaction) error {
	// Check if Session-Timer is required for this request
	sessionTimerRequired := h.sessionTimerMgr.IsSessionTimerRequired(req)
	sessionExpiresHeader := req.GetHeader(parser.HeaderSessionExpires)
	
	if sessionTimerRequired {
		if sessionExpiresHeader == "" {
			// Session-Timer is required but Session-Expires header is missing
			response := parser.NewResponseMessage(parser.StatusBadRequest, parser.GetReasonPhraseForCode(parser.StatusBadRequest))
			h.copyResponseHeaders(req, response)
			return txn.SendResponse(response)
		}
	} else {
		// Session-Timer is not required by the client, but server mandates it
		if sessionExpiresHeader == "" {
			response := parser.NewResponseMessage(parser.StatusExtensionRequired, parser.GetReasonPhraseForCode(parser.StatusExtensionRequired))
			h.copyResponseHeaders(req, response)
			response.AddHeader(parser.HeaderRequire, "timer")
			response.AddHeader(parser.HeaderSupported, "timer")
			return txn.SendResponse(response)
		}
	}

	// Parse Session-Expires value (we already checked it exists above)
	sessionExpires, err := h.parseSessionExpires(sessionExpiresHeader)
	if err != nil {
		response := parser.NewResponseMessage(parser.StatusBadRequest, parser.GetReasonPhraseForCode(parser.StatusBadRequest))
		h.copyResponseHeaders(req, response)
		return txn.SendResponse(response)
	}

	// Validate minimum session expires (Min-SE)
	minSE := h.getMinSE(req)
	if sessionExpires < minSE {
		response := parser.NewResponseMessage(parser.StatusIntervalTooBrief, parser.GetReasonPhraseForCode(parser.StatusIntervalTooBrief))
		h.copyResponseHeaders(req, response)
		response.SetHeader(parser.HeaderMinSE, strconv.Itoa(minSE))
		return txn.SendResponse(response)
	}

	// Extract target URI from Request-URI
	targetURI := req.GetRequestURI()
	aor := h.extractAOR(targetURI)

	// Find registered contacts for the target
	contacts, err := h.registrar.FindContacts(aor)
	if err != nil {
		response := parser.NewResponseMessage(parser.StatusServerInternalError, parser.GetReasonPhraseForCode(parser.StatusServerInternalError))
		h.copyResponseHeaders(req, response)
		return txn.SendResponse(response)
	}

	if len(contacts) == 0 {
		// User not found
		response := parser.NewResponseMessage(parser.StatusNotFound, parser.GetReasonPhraseForCode(parser.StatusNotFound))
		h.copyResponseHeaders(req, response)
		return txn.SendResponse(response)
	}

	// Create session with timer
	callID := req.GetHeader(parser.HeaderCallID)
	session := h.sessionTimerMgr.CreateSession(callID, sessionExpires)
	if session == nil {
		response := parser.NewResponseMessage(parser.StatusServerInternalError, parser.GetReasonPhraseForCode(parser.StatusServerInternalError))
		h.copyResponseHeaders(req, response)
		return txn.SendResponse(response)
	}

	// Forward the INVITE request through proxy engine
	return h.proxyEngine.ForwardRequest(req, contacts)
}

// handleAck processes ACK requests within established dialogs
func (h *SessionHandler) handleAck(req *parser.SIPMessage, txn transaction.Transaction) error {
	// ACK requests are handled differently based on whether they're for 2xx or non-2xx responses
	// For 2xx responses, ACK is end-to-end and should be forwarded
	// For non-2xx responses, ACK is hop-by-hop and terminates the transaction
	
	// Extract target URI from Request-URI
	targetURI := req.GetRequestURI()
	aor := h.extractAOR(targetURI)

	// Find registered contacts for the target
	contacts, err := h.registrar.FindContacts(aor)
	if err != nil {
		// ACK doesn't generate error responses, just log and return
		return fmt.Errorf("failed to find contacts for ACK: %w", err)
	}

	if len(contacts) == 0 {
		// No contacts found, but ACK doesn't generate responses
		return fmt.Errorf("no contacts found for ACK to %s", aor)
	}

	// Forward the ACK request
	return h.proxyEngine.ForwardRequest(req, contacts)
}

// handleBye processes BYE requests for session termination
func (h *SessionHandler) handleBye(req *parser.SIPMessage, txn transaction.Transaction) error {
	// Extract Call-ID to clean up session timer
	callID := req.GetHeader(parser.HeaderCallID)
	if callID != "" {
		// Remove session from session timer manager
		h.sessionTimerMgr.RemoveSession(callID)
	}

	// Extract target URI from Request-URI
	targetURI := req.GetRequestURI()
	aor := h.extractAOR(targetURI)

	// Find registered contacts for the target
	contacts, err := h.registrar.FindContacts(aor)
	if err != nil {
		response := parser.NewResponseMessage(parser.StatusServerInternalError, parser.GetReasonPhraseForCode(parser.StatusServerInternalError))
		h.copyResponseHeaders(req, response)
		return txn.SendResponse(response)
	}

	if len(contacts) == 0 {
		// For BYE, we might still want to respond with 200 OK even if user is not registered
		// as the session might be ending normally
		response := parser.NewResponseMessage(parser.StatusOK, parser.GetReasonPhraseForCode(parser.StatusOK))
		h.copyResponseHeaders(req, response)
		return txn.SendResponse(response)
	}

	// Forward the BYE request
	return h.proxyEngine.ForwardRequest(req, contacts)
}

// copyResponseHeaders copies necessary headers from request to response
func (h *SessionHandler) copyResponseHeaders(req *parser.SIPMessage, resp *parser.SIPMessage) {
	// Copy mandatory headers for responses
	if via := req.GetHeader(parser.HeaderVia); via != "" {
		resp.SetHeader(parser.HeaderVia, via)
	}
	if from := req.GetHeader(parser.HeaderFrom); from != "" {
		resp.SetHeader(parser.HeaderFrom, from)
	}
	if to := req.GetHeader(parser.HeaderTo); to != "" {
		resp.SetHeader(parser.HeaderTo, to)
	}
	if callID := req.GetHeader(parser.HeaderCallID); callID != "" {
		resp.SetHeader(parser.HeaderCallID, callID)
	}
	if cseq := req.GetHeader(parser.HeaderCSeq); cseq != "" {
		resp.SetHeader(parser.HeaderCSeq, cseq)
	}
	
	// Set Content-Length to 0 for responses without body
	resp.SetHeader(parser.HeaderContentLength, "0")
}

// parseSessionExpires parses the Session-Expires header value
func (h *SessionHandler) parseSessionExpires(header string) (int, error) {
	// Session-Expires header format: "1800" or "1800;refresher=uac"
	parts := strings.Split(header, ";")
	if len(parts) == 0 {
		return 0, fmt.Errorf("empty Session-Expires header")
	}
	
	expires, err := strconv.Atoi(strings.TrimSpace(parts[0]))
	if err != nil {
		return 0, fmt.Errorf("invalid Session-Expires value: %w", err)
	}
	
	if expires <= 0 {
		return 0, fmt.Errorf("Session-Expires must be positive")
	}
	
	return expires, nil
}

// getMinSE returns the minimum session expires value
func (h *SessionHandler) getMinSE(req *parser.SIPMessage) int {
	minSEHeader := req.GetHeader(parser.HeaderMinSE)
	if minSEHeader == "" {
		return 90 // Default Min-SE value (90 seconds)
	}
	
	minSE, err := strconv.Atoi(strings.TrimSpace(minSEHeader))
	if err != nil || minSE <= 0 {
		return 90 // Default on parse error
	}
	
	return minSE
}

// extractAOR extracts the Address of Record from a SIP URI
func (h *SessionHandler) extractAOR(uri string) string {
	// Simple AOR extraction - in a real implementation, this would be more sophisticated
	// Remove sip: or sips: prefix
	if strings.HasPrefix(uri, "sip:") {
		uri = uri[4:]
	} else if strings.HasPrefix(uri, "sips:") {
		uri = uri[5:]
	}
	
	// Remove parameters and headers
	if idx := strings.Index(uri, ";"); idx != -1 {
		uri = uri[:idx]
	}
	if idx := strings.Index(uri, "?"); idx != -1 {
		uri = uri[:idx]
	}
	
	return uri
}