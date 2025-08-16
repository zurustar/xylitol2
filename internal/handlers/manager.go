package handlers

import (
	"github.com/zurustar/xylitol2/internal/parser"
	"github.com/zurustar/xylitol2/internal/transaction"
)

// Manager implements the HandlerManager interface
type Manager struct {
	handlers []MethodHandler
}

// NewManager creates a new handler manager
func NewManager() *Manager {
	return &Manager{
		handlers: make([]MethodHandler, 0),
	}
}

// RegisterHandler registers a method handler
func (m *Manager) RegisterHandler(handler MethodHandler) {
	m.handlers = append(m.handlers, handler)
}

// HandleRequest routes the request to the appropriate handler
func (m *Manager) HandleRequest(req *parser.SIPMessage, txn transaction.Transaction) error {
	method := req.GetMethod()
	
	// Find a handler that can process this method
	for _, handler := range m.handlers {
		if handler.CanHandle(method) {
			return handler.HandleRequest(req, txn)
		}
	}
	
	// No handler found, send 405 Method Not Allowed
	return m.sendMethodNotAllowed(req, txn)
}

// GetSupportedMethods returns a list of all supported methods
func (m *Manager) GetSupportedMethods() []string {
	methodSet := make(map[string]bool)
	
	// Collect all methods supported by registered handlers
	allMethods := []string{
		parser.MethodINVITE,
		parser.MethodACK,
		parser.MethodBYE,
		parser.MethodCANCEL,
		parser.MethodREGISTER,
		parser.MethodOPTIONS,
		parser.MethodINFO,
		parser.MethodPRACK,
		parser.MethodUPDATE,
		parser.MethodSUBSCRIBE,
		parser.MethodNOTIFY,
		parser.MethodREFER,
		parser.MethodMESSAGE,
	}
	
	for _, method := range allMethods {
		for _, handler := range m.handlers {
			if handler.CanHandle(method) {
				methodSet[method] = true
				break
			}
		}
	}
	
	// Convert set to slice
	supportedMethods := make([]string, 0, len(methodSet))
	for method := range methodSet {
		supportedMethods = append(supportedMethods, method)
	}
	
	return supportedMethods
}

// sendMethodNotAllowed sends a 405 Method Not Allowed response
func (m *Manager) sendMethodNotAllowed(req *parser.SIPMessage, txn transaction.Transaction) error {
	response := parser.NewResponseMessage(parser.StatusMethodNotAllowed, parser.GetReasonPhraseForCode(parser.StatusMethodNotAllowed))
	
	// Copy mandatory headers from request
	m.copyResponseHeaders(req, response)
	
	// Add Allow header with supported methods
	supportedMethods := m.GetSupportedMethods()
	if len(supportedMethods) > 0 {
		allowHeader := ""
		for i, method := range supportedMethods {
			if i > 0 {
				allowHeader += ", "
			}
			allowHeader += method
		}
		response.SetHeader(parser.HeaderAllow, allowHeader)
	}
	
	return txn.SendResponse(response)
}

// copyResponseHeaders copies necessary headers from request to response
func (m *Manager) copyResponseHeaders(req *parser.SIPMessage, resp *parser.SIPMessage) {
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