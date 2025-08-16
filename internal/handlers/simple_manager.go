package handlers

import (
	"net"

	"github.com/zurustar/xylitol2/internal/auth"
	"github.com/zurustar/xylitol2/internal/logging"
	"github.com/zurustar/xylitol2/internal/parser"
	"github.com/zurustar/xylitol2/internal/registrar"
	"github.com/zurustar/xylitol2/internal/sessiontimer"
	"github.com/zurustar/xylitol2/internal/transaction"
	"github.com/zurustar/xylitol2/internal/transport"
)

// SimpleHandlerManager implements the HandlerManager interface and MessageHandler
type SimpleHandlerManager struct {
	messageParser      parser.MessageParser
	transactionManager transaction.TransactionManager
	authProcessor      auth.MessageProcessor
	registrar          registrar.Registrar
	sessionTimerMgr    sessiontimer.SessionTimerManager
	transportManager   transport.TransportManager
	logger             logging.Logger
}

// NewHandlerManager creates a new simple handler manager
func NewHandlerManager(
	messageParser parser.MessageParser,
	transactionManager transaction.TransactionManager,
	authProcessor auth.MessageProcessor,
	registrar registrar.Registrar,
	sessionTimerMgr sessiontimer.SessionTimerManager,
	logger logging.Logger,
) transport.MessageHandler {
	return &SimpleHandlerManager{
		messageParser:      messageParser,
		transactionManager: transactionManager,
		authProcessor:      authProcessor,
		registrar:          registrar,
		sessionTimerMgr:    sessionTimerMgr,
		logger:             logger,
	}
}

// SetTransportManager sets the transport manager for sending responses
func (h *SimpleHandlerManager) SetTransportManager(tm transport.TransportManager) {
	h.transportManager = tm
}

// HandleMessage handles incoming SIP messages (implements MessageHandler interface)
func (h *SimpleHandlerManager) HandleMessage(data []byte, transport string, addr net.Addr) error {
	// Parse the message
	msg, err := h.messageParser.Parse(data)
	if err != nil {
		h.logger.Error("Failed to parse SIP message", 
			logging.Field{Key: "error", Value: err},
			logging.Field{Key: "transport", Value: transport},
			logging.Field{Key: "addr", Value: addr.String()})
		return err
	}

	// Set transport and address information
	msg.Transport = transport
	msg.Source = addr

	h.logger.Debug("Received SIP message", 
		logging.Field{Key: "method", Value: msg.GetMethod()},
		logging.Field{Key: "transport", Value: transport},
		logging.Field{Key: "addr", Value: addr.String()})

	h.logger.Info("SIP message received and parsed successfully", 
		logging.Field{Key: "method", Value: msg.GetMethod()},
		logging.Field{Key: "call_id", Value: msg.GetHeader("Call-ID")})

	// Process the message based on method
	return h.processMessage(msg, transport, addr)
}

// processMessage processes the parsed SIP message and sends appropriate response
func (h *SimpleHandlerManager) processMessage(msg *parser.SIPMessage, transport string, addr net.Addr) error {
	method := msg.GetMethod()
	
	h.logger.Debug("Processing SIP message", 
		logging.Field{Key: "method", Value: method},
		logging.Field{Key: "transport", Value: transport})

	switch method {
	case "OPTIONS":
		return h.handleOPTIONS(msg, transport, addr)
	case "REGISTER":
		return h.handleREGISTER(msg, transport, addr)
	case "INVITE":
		return h.handleINVITE(msg, transport, addr)
	default:
		return h.sendMethodNotAllowed(msg, transport, addr)
	}
}

// handleOPTIONS handles OPTIONS requests
func (h *SimpleHandlerManager) handleOPTIONS(msg *parser.SIPMessage, transport string, addr net.Addr) error {
	h.logger.Debug("Handling OPTIONS request")
	
	// Get Request-URI to determine if this is for the server itself or needs to be proxied
	requestURI := msg.GetRequestURI()
	h.logger.Debug("OPTIONS Request-URI", logging.Field{Key: "uri", Value: requestURI})
	
	// Check if this is an OPTIONS request to the server itself
	if h.isRequestForServer(requestURI) {
		// This is an OPTIONS request to the server itself - respond directly
		h.logger.Debug("OPTIONS request for server itself")
		return h.handleServerOPTIONS(msg, transport, addr)
	}
	
	// This is an OPTIONS request that needs to be proxied
	h.logger.Debug("OPTIONS request needs to be proxied")
	return h.handleProxyOPTIONS(msg, transport, addr)
}

// handleServerOPTIONS handles OPTIONS requests directed to the server itself
func (h *SimpleHandlerManager) handleServerOPTIONS(msg *parser.SIPMessage, transport string, addr net.Addr) error {
	// Server OPTIONS requests typically don't require authentication
	// They are used for capability discovery
	
	response := h.createResponse(msg, 200, "OK")
	response += "Allow: INVITE, ACK, CANCEL, BYE, REGISTER, OPTIONS\r\n"
	response += "Accept: application/sdp\r\n"
	response += "Supported: timer, replaces\r\n"
	response += "Content-Length: 0\r\n\r\n"
	
	return h.sendResponse(response, transport, addr)
}

// handleProxyOPTIONS handles OPTIONS requests that need to be proxied
func (h *SimpleHandlerManager) handleProxyOPTIONS(msg *parser.SIPMessage, transport string, addr net.Addr) error {
	// First check authentication for proxy requests
	authResponse, user, err := h.authProcessor.ProcessIncomingRequest(msg, nil)
	if err != nil {
		h.logger.Error("Authentication processing failed", logging.Field{Key: "error", Value: err})
		return h.sendServerError(msg, "Authentication error", transport, addr)
	}
	
	if authResponse != nil {
		// Authentication failed, send the auth response
		return h.sendAuthResponse(authResponse, transport, addr)
	}
	
	if user == nil {
		// Authentication required but not provided
		return h.sendUnauthorized(msg, transport, addr)
	}
	
	// Extract target user from Request-URI
	targetUser := h.extractUserFromURI(msg.GetRequestURI())
	if targetUser == "" {
		return h.sendBadRequest(msg, "Invalid Request-URI", transport, addr)
	}
	
	// Check if target user is registered
	contacts, err := h.registrar.FindContacts(targetUser)
	if err != nil || len(contacts) == 0 {
		h.logger.Debug("Target user not found or not registered", 
			logging.Field{Key: "user", Value: targetUser})
		return h.sendNotFound(msg, transport, addr)
	}
	
	// TODO: Implement actual proxying to the registered contact
	// For now, return 200 OK indicating the user is reachable
	response := h.createResponse(msg, 200, "OK")
	response += "Allow: INVITE, ACK, CANCEL, BYE, REGISTER, OPTIONS\r\n"
	response += "Accept: application/sdp\r\n"
	response += "Content-Length: 0\r\n\r\n"
	
	return h.sendResponse(response, transport, addr)
}

// handleREGISTER handles REGISTER requests
func (h *SimpleHandlerManager) handleREGISTER(msg *parser.SIPMessage, transport string, addr net.Addr) error {
	h.logger.Debug("Handling REGISTER request")
	
	// REGISTER requests always require authentication
	authResponse, user, err := h.authProcessor.ProcessIncomingRequest(msg, nil)
	if err != nil {
		h.logger.Error("Authentication processing failed", logging.Field{Key: "error", Value: err})
		return h.sendServerError(msg, "Authentication error", transport, addr)
	}
	
	if authResponse != nil {
		// Authentication failed, send the auth response
		return h.sendAuthResponse(authResponse, transport, addr)
	}
	
	if user == nil {
		// Authentication required but not provided
		return h.sendUnauthorized(msg, transport, addr)
	}
	
	// TODO: Process the registration with the registrar
	// For now, just send 200 OK
	h.logger.Info("Registration successful", logging.Field{Key: "user", Value: user.Username})
	
	// Send 200 OK response
	response := h.createResponse(msg, 200, "OK")
	response += "Content-Length: 0\r\n\r\n"
	
	return h.sendResponse(response, transport, addr)
}

// handleINVITE handles INVITE requests
func (h *SimpleHandlerManager) handleINVITE(msg *parser.SIPMessage, transport string, addr net.Addr) error {
	h.logger.Debug("Handling INVITE request")
	
	// INVITE requests require authentication
	authResponse, user, err := h.authProcessor.ProcessIncomingRequest(msg, nil)
	if err != nil {
		h.logger.Error("Authentication processing failed", logging.Field{Key: "error", Value: err})
		return h.sendServerError(msg, "Authentication error", transport, addr)
	}
	
	if authResponse != nil {
		// Authentication failed, send the auth response
		return h.sendAuthResponse(authResponse, transport, addr)
	}
	
	if user == nil {
		// Authentication required but not provided
		return h.sendUnauthorized(msg, transport, addr)
	}
	
	// Validate Session-Timer requirements
	if h.sessionTimerMgr.IsSessionTimerRequired(msg) {
		sessionExpires := msg.GetHeader("Session-Expires")
		if sessionExpires == "" {
			h.logger.Debug("Session-Timer required but not provided")
			return h.sendSessionTimerRequired(msg, transport, addr)
		}
	}
	
	// Extract target user from Request-URI
	targetUser := h.extractUserFromURI(msg.GetRequestURI())
	if targetUser == "" {
		return h.sendBadRequest(msg, "Invalid Request-URI", transport, addr)
	}
	
	// Check if target user is registered
	contacts, err := h.registrar.FindContacts(targetUser)
	if err != nil || len(contacts) == 0 {
		h.logger.Debug("Target user not found or not registered", 
			logging.Field{Key: "user", Value: targetUser})
		return h.sendNotFound(msg, transport, addr)
	}
	
	// TODO: Implement actual call proxying
	// For now, send 200 OK
	response := h.createResponse(msg, 200, "OK")
	response += "Content-Length: 0\r\n\r\n"
	
	return h.sendResponse(response, transport, addr)
}

// sendMethodNotAllowed sends 405 Method Not Allowed response
func (h *SimpleHandlerManager) sendMethodNotAllowed(msg *parser.SIPMessage, transport string, addr net.Addr) error {
	h.logger.Debug("Sending 405 Method Not Allowed")
	
	response := h.createResponse(msg, 405, "Method Not Allowed")
	response += "Allow: INVITE, ACK, CANCEL, BYE, REGISTER, OPTIONS\r\n"
	response += "Content-Length: 0\r\n\r\n"
	
	return h.sendResponse(response, transport, addr)
}

// createResponse creates a basic SIP response
func (h *SimpleHandlerManager) createResponse(msg *parser.SIPMessage, statusCode int, reasonPhrase string) string {
	response := "SIP/2.0 " + string(rune('0'+statusCode/100)) + string(rune('0'+(statusCode/10)%10)) + string(rune('0'+statusCode%10)) + " " + reasonPhrase + "\r\n"
	
	// Copy Via headers
	viaHeaders := msg.GetHeaders("Via")
	for _, via := range viaHeaders {
		response += "Via: " + via + "\r\n"
	}
	
	// Copy From header
	if from := msg.GetHeader("From"); from != "" {
		response += "From: " + from + "\r\n"
	}
	
	// Copy To header (add tag if not present)
	if to := msg.GetHeader("To"); to != "" {
		if !containsTag(to) {
			to += ";tag=server-tag-" + generateTag()
		}
		response += "To: " + to + "\r\n"
	}
	
	// Copy Call-ID header
	if callID := msg.GetHeader("Call-ID"); callID != "" {
		response += "Call-ID: " + callID + "\r\n"
	}
	
	// Copy CSeq header
	if cseq := msg.GetHeader("CSeq"); cseq != "" {
		response += "CSeq: " + cseq + "\r\n"
	}
	
	return response
}

// sendResponse sends the response back to the client
func (h *SimpleHandlerManager) sendResponse(response string, transport string, addr net.Addr) error {
	h.logger.Debug("Sending SIP response", 
		logging.Field{Key: "transport", Value: transport},
		logging.Field{Key: "addr", Value: addr.String()})
	
	if h.transportManager == nil {
		h.logger.Error("Transport manager not set, cannot send response")
		return nil
	}
	
	// Send response using transport manager
	err := h.transportManager.SendMessage([]byte(response), transport, addr)
	if err != nil {
		h.logger.Error("Failed to send SIP response", 
			logging.Field{Key: "error", Value: err},
			logging.Field{Key: "transport", Value: transport},
			logging.Field{Key: "addr", Value: addr.String()})
		return err
	}
	
	h.logger.Info("SIP response sent successfully", 
		logging.Field{Key: "response_length", Value: len(response)},
		logging.Field{Key: "transport", Value: transport},
		logging.Field{Key: "addr", Value: addr.String()})
	
	return nil
}

// isRequestForServer determines if the request is directed to the server itself
func (h *SimpleHandlerManager) isRequestForServer(requestURI string) bool {
	// Simple heuristic: if the Request-URI doesn't contain a specific user part,
	// or if it's addressed to the server's domain, treat it as a server request
	if requestURI == "" {
		return true
	}
	
	// Check if it's a domain-only URI (no user part)
	if !containsString(requestURI, "@") {
		return true
	}
	
	// Check if it's addressed to a generic server URI
	if containsString(requestURI, "sip:test.local") || containsString(requestURI, "sip:sipserver.local") {
		userPart := h.extractUserFromURI(requestURI)
		return userPart == "" || userPart == "server" || userPart == "proxy"
	}
	
	return false
}

// extractUserFromURI extracts the user part from a SIP URI
func (h *SimpleHandlerManager) extractUserFromURI(uri string) string {
	// Simple URI parsing - in production, use proper SIP URI parser
	if !containsString(uri, "sip:") {
		return ""
	}
	
	// Remove sip: prefix
	uri = uri[4:]
	
	// Find @ symbol
	atIndex := findSubstring(uri, "@")
	if atIndex == -1 {
		return ""
	}
	
	return uri[:atIndex]
}

// sendUnauthorized sends 401 Unauthorized response
func (h *SimpleHandlerManager) sendUnauthorized(msg *parser.SIPMessage, transport string, addr net.Addr) error {
	response := h.createResponse(msg, 401, "Unauthorized")
	response += "WWW-Authenticate: Digest realm=\"sipserver.local\", nonce=\"" + generateNonce() + "\"\r\n"
	response += "Content-Length: 0\r\n\r\n"
	
	return h.sendResponse(response, transport, addr)
}

// sendAuthResponse sends an authentication response
func (h *SimpleHandlerManager) sendAuthResponse(authResponse *parser.SIPMessage, transport string, addr net.Addr) error {
	// Convert auth response to string and send
	// TODO: Implement proper SIPMessage to string conversion
	// For now, create a simple 401 response
	response := h.createResponse(authResponse, 401, "Unauthorized")
	response += "WWW-Authenticate: Digest realm=\"sipserver.local\", nonce=\"" + generateNonce() + "\"\r\n"
	response += "Content-Length: 0\r\n\r\n"
	
	return h.sendResponse(response, transport, addr)
}

// sendNotFound sends 404 Not Found response
func (h *SimpleHandlerManager) sendNotFound(msg *parser.SIPMessage, transport string, addr net.Addr) error {
	response := h.createResponse(msg, 404, "Not Found")
	response += "Content-Length: 0\r\n\r\n"
	
	return h.sendResponse(response, transport, addr)
}

// sendBadRequest sends 400 Bad Request response
func (h *SimpleHandlerManager) sendBadRequest(msg *parser.SIPMessage, reason string, transport string, addr net.Addr) error {
	response := h.createResponse(msg, 400, "Bad Request")
	if reason != "" {
		response += "Reason: " + reason + "\r\n"
	}
	response += "Content-Length: 0\r\n\r\n"
	
	return h.sendResponse(response, transport, addr)
}

// sendServerError sends 500 Internal Server Error response
func (h *SimpleHandlerManager) sendServerError(msg *parser.SIPMessage, reason string, transport string, addr net.Addr) error {
	response := h.createResponse(msg, 500, "Internal Server Error")
	if reason != "" {
		response += "Reason: " + reason + "\r\n"
	}
	response += "Content-Length: 0\r\n\r\n"
	
	return h.sendResponse(response, transport, addr)
}

// sendSessionTimerRequired sends 421 Extension Required response for Session-Timer
func (h *SimpleHandlerManager) sendSessionTimerRequired(msg *parser.SIPMessage, transport string, addr net.Addr) error {
	response := h.createResponse(msg, 421, "Extension Required")
	response += "Require: timer\r\n"
	response += "Content-Length: 0\r\n\r\n"
	
	return h.sendResponse(response, transport, addr)
}

// Helper functions
func containsTag(header string) bool {
	return len(header) > 4 && (header[len(header)-4:] == ";tag" || 
		findSubstring(header, ";tag=") >= 0)
}

func containsString(s, substr string) bool {
	return findSubstring(s, substr) >= 0
}

func generateTag() string {
	// Simple tag generation - in production, use proper random generation
	return "12345"
}

func generateNonce() string {
	// Simple nonce generation - in production, use proper random generation
	return "abcdef123456"
}

func findSubstring(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}