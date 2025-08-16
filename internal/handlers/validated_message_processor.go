package handlers

import (
	"fmt"
	"net"

	"github.com/zurustar/xylitol2/internal/auth"
	"github.com/zurustar/xylitol2/internal/database"
	"github.com/zurustar/xylitol2/internal/logging"
	"github.com/zurustar/xylitol2/internal/parser"
	"github.com/zurustar/xylitol2/internal/registrar"
	"github.com/zurustar/xylitol2/internal/sessiontimer"
	"github.com/zurustar/xylitol2/internal/transaction"
	"github.com/zurustar/xylitol2/internal/transport"
)

// ValidatedMessageProcessor processes SIP messages using a validation chain
// This ensures proper priority ordering of validations (Session-Timer before Authentication)
type ValidatedMessageProcessor struct {
	messageParser      parser.MessageParser
	transactionManager transaction.TransactionManager
	authProcessor      auth.MessageProcessor
	registrar          registrar.Registrar
	sessionTimerMgr    sessiontimer.SessionTimerManager
	transportManager   transport.TransportManager
	logger             logging.Logger
	validationChain    *ValidationChain
	userManager        database.UserManager
	realm              string
}

// NewValidatedMessageProcessor creates a new validated message processor
func NewValidatedMessageProcessor(
	messageParser parser.MessageParser,
	transactionManager transaction.TransactionManager,
	authProcessor auth.MessageProcessor,
	registrar registrar.Registrar,
	sessionTimerMgr sessiontimer.SessionTimerManager,
	userManager database.UserManager,
	realm string,
	logger logging.Logger,
) *ValidatedMessageProcessor {
	processor := &ValidatedMessageProcessor{
		messageParser:      messageParser,
		transactionManager: transactionManager,
		authProcessor:      authProcessor,
		registrar:          registrar,
		sessionTimerMgr:    sessionTimerMgr,
		userManager:        userManager,
		realm:              realm,
		logger:             logger,
		validationChain:    NewValidationChain(),
	}
	
	// Initialize validation chain with proper priority ordering
	processor.initializeValidationChain()
	
	return processor
}

// initializeValidationChain sets up the validation chain with proper priority ordering
func (vmp *ValidatedMessageProcessor) initializeValidationChain() {
	// Add Session-Timer validator with high priority (10)
	sessionTimerValidator := NewSessionTimerValidator(
		vmp.sessionTimerMgr,
		90,  // Default Min-SE
		7200, // Default Max-SE
	)
	vmp.validationChain.AddValidator(sessionTimerValidator)
	
	// Add Authentication validator with lower priority (20)
	authValidator := NewAuthenticationValidator(
		vmp.authProcessor,
		vmp.userManager,
		vmp.realm,
	)
	vmp.validationChain.AddValidator(authValidator)
	
	vmp.logger.Info("Validation chain initialized with Session-Timer priority",
		logging.Field{Key: "validators", Value: len(vmp.validationChain.GetValidators())})
}

// SetTransportManager sets the transport manager for sending responses
func (vmp *ValidatedMessageProcessor) SetTransportManager(tm transport.TransportManager) {
	vmp.transportManager = tm
}

// HandleMessage handles incoming SIP messages with validation chain processing
func (vmp *ValidatedMessageProcessor) HandleMessage(data []byte, transport string, addr net.Addr) error {
	// Parse the message
	msg, err := vmp.messageParser.Parse(data)
	if err != nil {
		vmp.logger.Error("Failed to parse SIP message", 
			logging.Field{Key: "error", Value: err},
			logging.Field{Key: "transport", Value: transport},
			logging.Field{Key: "addr", Value: addr.String()})
		return err
	}

	// Set transport and address information
	msg.Transport = transport
	msg.Source = addr

	vmp.logger.Debug("Received SIP message", 
		logging.Field{Key: "method", Value: msg.GetMethod()},
		logging.Field{Key: "transport", Value: transport},
		logging.Field{Key: "addr", Value: addr.String()})

	// Process the message with validation chain
	return vmp.processMessageWithValidation(msg, transport, addr)
}

// processMessageWithValidation processes a SIP message using the validation chain
func (vmp *ValidatedMessageProcessor) processMessageWithValidation(msg *parser.SIPMessage, transport string, addr net.Addr) error {
	method := msg.GetMethod()
	
	vmp.logger.Debug("Processing SIP message with validation chain", 
		logging.Field{Key: "method", Value: method},
		logging.Field{Key: "transport", Value: transport})

	// Run validation chain
	validationResult := vmp.validationChain.Validate(msg)
	if !validationResult.Valid {
		// Validation failed, send error response
		vmp.logger.Info("Message validation failed", 
			logging.Field{Key: "method", Value: method},
			logging.Field{Key: "error", Value: validationResult.Error})
		
		if validationResult.Response != nil {
			return vmp.sendSIPResponse(validationResult.Response, transport, addr)
		}
		
		// If no response provided, create a generic error response
		return vmp.sendGenericError(msg, transport, addr)
	}
	
	vmp.logger.Debug("Message validation passed", 
		logging.Field{Key: "method", Value: method})

	// Validation passed, process the message based on method
	switch method {
	case parser.MethodOPTIONS:
		return vmp.handleOPTIONS(msg, transport, addr)
	case parser.MethodREGISTER:
		return vmp.handleREGISTER(msg, transport, addr)
	case parser.MethodINVITE:
		return vmp.handleINVITE(msg, transport, addr)
	case parser.MethodBYE:
		return vmp.handleBYE(msg, transport, addr)
	case parser.MethodACK:
		return vmp.handleACK(msg, transport, addr)
	default:
		return vmp.sendMethodNotAllowed(msg, transport, addr)
	}
}

// handleOPTIONS handles OPTIONS requests (validation already passed)
func (vmp *ValidatedMessageProcessor) handleOPTIONS(msg *parser.SIPMessage, transport string, addr net.Addr) error {
	vmp.logger.Debug("Handling validated OPTIONS request")
	
	// Get Request-URI to determine if this is for the server itself or needs to be proxied
	requestURI := msg.GetRequestURI()
	
	if vmp.isRequestForServer(requestURI) {
		// Server OPTIONS request
		response := vmp.createServerOPTIONSResponse(msg)
		return vmp.sendSIPResponse(response, transport, addr)
	}
	
	// Proxy OPTIONS request - authentication already validated
	return vmp.handleProxyOPTIONS(msg, transport, addr)
}

// handleREGISTER handles REGISTER requests (validation already passed)
func (vmp *ValidatedMessageProcessor) handleREGISTER(msg *parser.SIPMessage, transport string, addr net.Addr) error {
	vmp.logger.Debug("Handling validated REGISTER request")
	
	// Authentication already validated, process registration
	// TODO: Implement actual registration logic
	response := vmp.createSuccessResponse(msg)
	return vmp.sendSIPResponse(response, transport, addr)
}

// handleINVITE handles INVITE requests (validation already passed)
func (vmp *ValidatedMessageProcessor) handleINVITE(msg *parser.SIPMessage, transport string, addr net.Addr) error {
	vmp.logger.Debug("Handling validated INVITE request")
	
	// Both Session-Timer and Authentication validation already passed
	// Extract target user from Request-URI
	targetUser := vmp.extractUserFromURI(msg.GetRequestURI())
	if targetUser == "" {
		return vmp.sendBadRequest(msg, "Invalid Request-URI", transport, addr)
	}
	
	// Check if target user is registered
	contacts, err := vmp.registrar.FindContacts(targetUser)
	if err != nil || len(contacts) == 0 {
		vmp.logger.Debug("Target user not found or not registered", 
			logging.Field{Key: "user", Value: targetUser})
		return vmp.sendNotFound(msg, transport, addr)
	}
	
	// Create session with timer (Session-Timer validation already passed)
	callID := msg.GetHeader(parser.HeaderCallID)
	sessionExpiresHeader := msg.GetHeader(parser.HeaderSessionExpires)
	if sessionExpiresHeader != "" {
		sessionExpires, err := vmp.parseSessionExpires(sessionExpiresHeader)
		if err == nil {
			session := vmp.sessionTimerMgr.CreateSession(callID, sessionExpires)
			if session != nil {
				vmp.logger.Debug("Session timer created", 
					logging.Field{Key: "call_id", Value: callID},
					logging.Field{Key: "expires", Value: sessionExpires})
			}
		}
	}
	
	// TODO: Implement actual call proxying
	response := vmp.createSuccessResponse(msg)
	return vmp.sendSIPResponse(response, transport, addr)
}

// handleBYE handles BYE requests (validation already passed)
func (vmp *ValidatedMessageProcessor) handleBYE(msg *parser.SIPMessage, transport string, addr net.Addr) error {
	vmp.logger.Debug("Handling validated BYE request")
	
	// Clean up session timer
	callID := msg.GetHeader(parser.HeaderCallID)
	if callID != "" {
		vmp.sessionTimerMgr.RemoveSession(callID)
		vmp.logger.Debug("Session timer removed", logging.Field{Key: "call_id", Value: callID})
	}
	
	// TODO: Implement actual BYE processing
	response := vmp.createSuccessResponse(msg)
	return vmp.sendSIPResponse(response, transport, addr)
}

// handleACK handles ACK requests (validation already passed)
func (vmp *ValidatedMessageProcessor) handleACK(msg *parser.SIPMessage, transport string, addr net.Addr) error {
	vmp.logger.Debug("Handling validated ACK request")
	
	// TODO: Implement actual ACK processing
	// ACK doesn't generate responses, just log and return
	return nil
}

// handleProxyOPTIONS handles OPTIONS requests that need to be proxied
func (vmp *ValidatedMessageProcessor) handleProxyOPTIONS(msg *parser.SIPMessage, transport string, addr net.Addr) error {
	// Extract target user from Request-URI
	targetUser := vmp.extractUserFromURI(msg.GetRequestURI())
	if targetUser == "" {
		return vmp.sendBadRequest(msg, "Invalid Request-URI", transport, addr)
	}
	
	// Check if target user is registered
	contacts, err := vmp.registrar.FindContacts(targetUser)
	if err != nil || len(contacts) == 0 {
		vmp.logger.Debug("Target user not found or not registered", 
			logging.Field{Key: "user", Value: targetUser})
		return vmp.sendNotFound(msg, transport, addr)
	}
	
	// TODO: Implement actual proxying to the registered contact
	// For now, return 200 OK indicating the user is reachable
	response := vmp.createServerOPTIONSResponse(msg)
	return vmp.sendSIPResponse(response, transport, addr)
}

// Helper methods for response creation and sending

// createServerOPTIONSResponse creates a response for server OPTIONS requests
func (vmp *ValidatedMessageProcessor) createServerOPTIONSResponse(msg *parser.SIPMessage) *parser.SIPMessage {
	response := parser.NewResponseMessage(parser.StatusOK, "OK")
	vmp.copyResponseHeaders(msg, response)
	
	response.SetHeader(parser.HeaderAllow, "INVITE, ACK, CANCEL, BYE, REGISTER, OPTIONS")
	response.SetHeader(parser.HeaderAccept, "application/sdp")
	response.SetHeader(parser.HeaderSupported, "timer, replaces")
	
	return response
}

// createSuccessResponse creates a generic 200 OK response
func (vmp *ValidatedMessageProcessor) createSuccessResponse(msg *parser.SIPMessage) *parser.SIPMessage {
	response := parser.NewResponseMessage(parser.StatusOK, "OK")
	vmp.copyResponseHeaders(msg, response)
	return response
}

// sendMethodNotAllowed sends 405 Method Not Allowed response
func (vmp *ValidatedMessageProcessor) sendMethodNotAllowed(msg *parser.SIPMessage, transport string, addr net.Addr) error {
	response := parser.NewResponseMessage(parser.StatusMethodNotAllowed, "Method Not Allowed")
	vmp.copyResponseHeaders(msg, response)
	response.SetHeader(parser.HeaderAllow, "INVITE, ACK, CANCEL, BYE, REGISTER, OPTIONS")
	
	return vmp.sendSIPResponse(response, transport, addr)
}

// sendNotFound sends 404 Not Found response
func (vmp *ValidatedMessageProcessor) sendNotFound(msg *parser.SIPMessage, transport string, addr net.Addr) error {
	response := parser.NewResponseMessage(parser.StatusNotFound, "Not Found")
	vmp.copyResponseHeaders(msg, response)
	
	return vmp.sendSIPResponse(response, transport, addr)
}

// sendBadRequest sends 400 Bad Request response
func (vmp *ValidatedMessageProcessor) sendBadRequest(msg *parser.SIPMessage, reason string, transport string, addr net.Addr) error {
	response := parser.NewResponseMessage(parser.StatusBadRequest, "Bad Request")
	vmp.copyResponseHeaders(msg, response)
	
	if reason != "" {
		response.SetHeader("Reason", reason)
	}
	
	return vmp.sendSIPResponse(response, transport, addr)
}

// sendGenericError sends a generic 500 Internal Server Error response
func (vmp *ValidatedMessageProcessor) sendGenericError(msg *parser.SIPMessage, transport string, addr net.Addr) error {
	response := parser.NewResponseMessage(parser.StatusServerInternalError, "Internal Server Error")
	vmp.copyResponseHeaders(msg, response)
	
	return vmp.sendSIPResponse(response, transport, addr)
}

// sendSIPResponse sends a SIP response message
func (vmp *ValidatedMessageProcessor) sendSIPResponse(response *parser.SIPMessage, transport string, addr net.Addr) error {
	if vmp.transportManager == nil {
		vmp.logger.Error("Transport manager not set, cannot send response")
		return fmt.Errorf("transport manager not available")
	}
	
	// Serialize the response
	responseData, err := vmp.messageParser.Serialize(response)
	if err != nil {
		vmp.logger.Error("Failed to serialize SIP response", logging.Field{Key: "error", Value: err})
		return fmt.Errorf("failed to serialize response: %w", err)
	}
	
	// Send response using transport manager
	err = vmp.transportManager.SendMessage(responseData, transport, addr)
	if err != nil {
		vmp.logger.Error("Failed to send SIP response", 
			logging.Field{Key: "error", Value: err},
			logging.Field{Key: "transport", Value: transport},
			logging.Field{Key: "addr", Value: addr.String()})
		return err
	}
	
	vmp.logger.Info("SIP response sent successfully", 
		logging.Field{Key: "status_code", Value: response.GetStatusCode()},
		logging.Field{Key: "transport", Value: transport},
		logging.Field{Key: "addr", Value: addr.String()})
	
	return nil
}

// copyResponseHeaders copies necessary headers from request to response
func (vmp *ValidatedMessageProcessor) copyResponseHeaders(req *parser.SIPMessage, resp *parser.SIPMessage) {
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

// Utility methods

// isRequestForServer determines if the request is directed to the server itself
func (vmp *ValidatedMessageProcessor) isRequestForServer(requestURI string) bool {
	if requestURI == "" {
		return true
	}
	
	// Check if it's a domain-only URI (no user part)
	if !vmp.containsString(requestURI, "@") {
		return true
	}
	
	// Check if it's addressed to a generic server URI
	if vmp.containsString(requestURI, "sip:test.local") || vmp.containsString(requestURI, "sip:sipserver.local") {
		userPart := vmp.extractUserFromURI(requestURI)
		return userPart == "" || userPart == "server" || userPart == "proxy"
	}
	
	return false
}

// extractUserFromURI extracts the user part from a SIP URI
func (vmp *ValidatedMessageProcessor) extractUserFromURI(uri string) string {
	if !vmp.containsString(uri, "sip:") {
		return ""
	}
	
	// Remove sip: prefix
	uri = uri[4:]
	
	// Find @ symbol
	atIndex := vmp.findSubstring(uri, "@")
	if atIndex == -1 {
		return ""
	}
	
	return uri[:atIndex]
}

// parseSessionExpires parses the Session-Expires header value
func (vmp *ValidatedMessageProcessor) parseSessionExpires(header string) (int, error) {
	// Use the same parsing logic as the SessionTimerValidator
	validator := &SessionTimerValidator{}
	return validator.parseSessionExpires(header)
}

// containsString checks if a string contains a substring
func (vmp *ValidatedMessageProcessor) containsString(s, substr string) bool {
	return vmp.findSubstring(s, substr) >= 0
}

// findSubstring finds the index of a substring in a string
func (vmp *ValidatedMessageProcessor) findSubstring(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}