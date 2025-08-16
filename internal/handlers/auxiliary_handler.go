package handlers

import (
	"fmt"
	"strings"

	"github.com/zurustar/xylitol2/internal/parser"
	"github.com/zurustar/xylitol2/internal/proxy"
	"github.com/zurustar/xylitol2/internal/registrar"
	"github.com/zurustar/xylitol2/internal/transaction"
)

// AuxiliaryHandler handles OPTIONS and INFO requests
type AuxiliaryHandler struct {
	proxyEngine proxy.ProxyEngine
	registrar   registrar.Registrar
}

// NewAuxiliaryHandler creates a new auxiliary handler
func NewAuxiliaryHandler(proxyEngine proxy.ProxyEngine, registrar registrar.Registrar) *AuxiliaryHandler {
	return &AuxiliaryHandler{
		proxyEngine: proxyEngine,
		registrar:   registrar,
	}
}

// CanHandle returns true if this handler can process the given method
func (h *AuxiliaryHandler) CanHandle(method string) bool {
	switch method {
	case parser.MethodOPTIONS, parser.MethodINFO:
		return true
	default:
		return false
	}
}

// HandleRequest processes OPTIONS and INFO requests
func (h *AuxiliaryHandler) HandleRequest(req *parser.SIPMessage, txn transaction.Transaction) error {
	method := req.GetMethod()
	
	switch method {
	case parser.MethodOPTIONS:
		return h.handleOptions(req, txn)
	case parser.MethodINFO:
		return h.handleInfo(req, txn)
	default:
		return fmt.Errorf("unsupported method: %s", method)
	}
}

// handleOptions processes OPTIONS requests with capability advertisement
func (h *AuxiliaryHandler) handleOptions(req *parser.SIPMessage, txn transaction.Transaction) error {
	// Create 200 OK response
	response := parser.NewResponseMessage(parser.StatusOK, parser.GetReasonPhraseForCode(parser.StatusOK))
	h.copyResponseHeaders(req, response)
	
	// Add supported methods
	supportedMethods := []string{
		parser.MethodINVITE,
		parser.MethodACK,
		parser.MethodBYE,
		parser.MethodCANCEL,
		parser.MethodREGISTER,
		parser.MethodOPTIONS,
		parser.MethodINFO,
	}
	response.SetHeader(parser.HeaderAllow, strings.Join(supportedMethods, ", "))
	
	// Add supported extensions
	supportedExtensions := []string{
		"timer", // Session-Timer support (RFC4028)
	}
	response.SetHeader(parser.HeaderSupported, strings.Join(supportedExtensions, ", "))
	
	// Add Accept header for supported content types
	acceptedTypes := []string{
		"application/sdp",
		"text/plain",
	}
	response.SetHeader("Accept", strings.Join(acceptedTypes, ", "))
	
	// Add Accept-Encoding header
	response.SetHeader("Accept-Encoding", "gzip")
	
	// Add Accept-Language header
	response.SetHeader("Accept-Language", "en")
	
	// Add User-Agent or Server header
	response.SetHeader(parser.HeaderServer, "SIP-Server/1.0")
	
	return txn.SendResponse(response)
}

// handleInfo processes INFO requests within established dialogs
func (h *AuxiliaryHandler) handleInfo(req *parser.SIPMessage, txn transaction.Transaction) error {
	// INFO requests should be forwarded within established dialogs
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

	// Forward the INFO request through proxy engine
	return h.proxyEngine.ForwardRequest(req, contacts)
}

// copyResponseHeaders copies necessary headers from request to response
func (h *AuxiliaryHandler) copyResponseHeaders(req *parser.SIPMessage, resp *parser.SIPMessage) {
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

// extractAOR extracts the Address of Record from a SIP URI
func (h *AuxiliaryHandler) extractAOR(uri string) string {
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