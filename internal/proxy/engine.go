package proxy

import (
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/zurustar/xylitol2/internal/database"
	"github.com/zurustar/xylitol2/internal/huntgroup"
	"github.com/zurustar/xylitol2/internal/parser"
	"github.com/zurustar/xylitol2/internal/registrar"
	"github.com/zurustar/xylitol2/internal/transaction"
	"github.com/zurustar/xylitol2/internal/transport"
)

// RequestForwardingEngine implements the ProxyEngine interface for request forwarding
type RequestForwardingEngine struct {
	registrar         registrar.Registrar
	transportManager  transport.TransportManager
	transactionManager transaction.TransactionManager
	parser            parser.MessageParser
	huntGroupManager  huntgroup.HuntGroupManager
	huntGroupEngine   huntgroup.HuntGroupEngine
	serverHost        string
	serverPort        int
	maxForwards       int
}

// NewRequestForwardingEngine creates a new request forwarding engine
func NewRequestForwardingEngine(
	registrar registrar.Registrar,
	transportManager transport.TransportManager,
	transactionManager transaction.TransactionManager,
	parser parser.MessageParser,
	huntGroupManager huntgroup.HuntGroupManager,
	huntGroupEngine huntgroup.HuntGroupEngine,
	serverHost string,
	serverPort int,
) *RequestForwardingEngine {
	return &RequestForwardingEngine{
		registrar:          registrar,
		transportManager:   transportManager,
		transactionManager: transactionManager,
		parser:             parser,
		huntGroupManager:   huntGroupManager,
		huntGroupEngine:    huntGroupEngine,
		serverHost:         serverHost,
		serverPort:         serverPort,
		maxForwards:        70, // RFC3261 default
	}
}

// ProcessRequest processes an incoming SIP request for proxy forwarding
func (e *RequestForwardingEngine) ProcessRequest(req *parser.SIPMessage, transaction transaction.Transaction) error {
	if req == nil || !req.IsRequest() {
		return fmt.Errorf("invalid request message")
	}

	method := req.GetMethod()
	
	// Handle different request methods
	switch method {
	case parser.MethodINVITE, parser.MethodBYE, parser.MethodCANCEL, parser.MethodACK, parser.MethodINFO:
		return e.processProxyableRequest(req, transaction)
	case parser.MethodREGISTER:
		// REGISTER requests are handled by the registrar, not proxied
		return fmt.Errorf("REGISTER requests should be handled by registrar")
	case parser.MethodOPTIONS:
		// OPTIONS can be handled locally or proxied depending on Request-URI
		return e.processOptionsRequest(req, transaction)
	default:
		// Send 405 Method Not Allowed for unsupported methods
		return e.sendMethodNotAllowed(req, transaction)
	}
}

// processProxyableRequest processes requests that should be proxied
func (e *RequestForwardingEngine) processProxyableRequest(req *parser.SIPMessage, transaction transaction.Transaction) error {
	// Check Max-Forwards header to prevent loops
	if err := e.checkMaxForwards(req); err != nil {
		return e.sendTooManyHops(req, transaction)
	}

	// Decrement Max-Forwards
	e.decrementMaxForwards(req)

	// Extract target URI from Request-URI
	requestURI := req.GetRequestURI()
	if requestURI == "" {
		return e.sendBadRequest(req, transaction, "Missing Request-URI")
	}

	// Resolve target using registrar database or hunt groups
	targets, err := e.resolveTarget(requestURI)
	if err != nil {
		// Check if this is a hunt group error
		if strings.HasPrefix(err.Error(), "hunt_group:") {
			return e.handleHuntGroupCall(req, transaction, err.Error())
		}
		return e.sendNotFound(req, transaction, "User not registered")
	}

	if len(targets) == 0 {
		return e.sendNotFound(req, transaction, "No registered contacts")
	}

	// Forward request to targets
	return e.ForwardRequest(req, targets)
}

// processOptionsRequest processes OPTIONS requests
func (e *RequestForwardingEngine) processOptionsRequest(req *parser.SIPMessage, transaction transaction.Transaction) error {
	requestURI := req.GetRequestURI()
	
	// If Request-URI is for this server, handle locally
	if e.isLocalURI(requestURI) {
		return e.sendOptionsResponse(req, transaction)
	}

	// Otherwise, proxy the request
	return e.processProxyableRequest(req, transaction)
}

// ForwardRequest forwards a SIP request to the specified targets
func (e *RequestForwardingEngine) ForwardRequest(req *parser.SIPMessage, targets []*database.RegistrarContact) error {
	if len(targets) == 0 {
		return fmt.Errorf("no targets to forward to")
	}

	// For now, implement simple serial forking (forward to first target)
	// TODO: Implement parallel forking in task 10.2
	target := targets[0]

	// Create a copy of the request for forwarding
	forwardedReq := req.Clone()

	// Add Via header for this proxy
	viaHeader := e.createViaHeader(req.Transport)
	e.addViaHeader(forwardedReq, viaHeader)

	// Update Request-URI to target contact
	if reqLine, ok := forwardedReq.StartLine.(*parser.RequestLine); ok {
		reqLine.RequestURI = target.URI
	}

	// Parse target address
	targetAddr, transport, err := e.parseTargetURI(target.URI)
	if err != nil {
		return fmt.Errorf("failed to parse target URI %s: %w", target.URI, err)
	}

	// Serialize the message
	data, err := e.parser.Serialize(forwardedReq)
	if err != nil {
		return fmt.Errorf("failed to serialize forwarded request: %w", err)
	}

	// Send the request
	return e.transportManager.SendMessage(data, transport, targetAddr)
}

// ProcessResponse processes an incoming SIP response for proxy routing
func (e *RequestForwardingEngine) ProcessResponse(resp *parser.SIPMessage, transaction transaction.Transaction) error {
	if resp == nil || !resp.IsResponse() {
		return fmt.Errorf("invalid response message")
	}

	// Remove the top Via header (this proxy's Via)
	viaHeaders := resp.GetHeaders(parser.HeaderVia)
	if len(viaHeaders) == 0 {
		return fmt.Errorf("response missing Via headers")
	}

	// Remove the first Via header
	resp.RemoveHeader(parser.HeaderVia)
	for i := 1; i < len(viaHeaders); i++ {
		resp.AddHeader(parser.HeaderVia, viaHeaders[i])
	}

	// Get the next Via header to determine where to route the response
	remainingVias := resp.GetHeaders(parser.HeaderVia)
	if len(remainingVias) == 0 {
		return fmt.Errorf("no remaining Via headers for response routing")
	}

	// Parse the next Via header to get routing information
	nextVia := remainingVias[0]
	targetAddr, transport, err := e.parseViaHeader(nextVia)
	if err != nil {
		return fmt.Errorf("failed to parse Via header for response routing: %w", err)
	}

	// Serialize the response
	data, err := e.parser.Serialize(resp)
	if err != nil {
		return fmt.Errorf("failed to serialize response: %w", err)
	}

	// Send the response back
	return e.transportManager.SendMessage(data, transport, targetAddr)
}

// resolveTarget resolves a target URI using the registrar database or hunt groups
func (e *RequestForwardingEngine) resolveTarget(requestURI string) ([]*database.RegistrarContact, error) {
	// Extract AOR from Request-URI
	aor, err := e.extractAOR(requestURI)
	if err != nil {
		return nil, fmt.Errorf("failed to extract AOR from Request-URI: %w", err)
	}

	// Extract extension from AOR (simplified - assumes sip:extension@domain format)
	extension := e.extractExtensionFromAOR(aor)
	
	// Check if this is a hunt group extension
	if e.huntGroupManager != nil && extension != "" {
		if group, err := e.huntGroupManager.GetGroupByExtension(extension); err == nil && group != nil {
			// This is a hunt group - return empty contacts to indicate special handling needed
			return nil, fmt.Errorf("hunt_group:%d", group.ID)
		}
	}

	// Find registered contacts
	contacts, err := e.registrar.FindContacts(aor)
	if err != nil {
		return nil, fmt.Errorf("failed to find contacts for AOR %s: %w", aor, err)
	}

	// Filter out expired contacts
	var validContacts []*database.RegistrarContact
	now := time.Now().UTC()
	for _, contact := range contacts {
		if contact.Expires.After(now) {
			validContacts = append(validContacts, contact)
		}
	}

	return validContacts, nil
}

// extractAOR extracts the Address of Record from a URI
func (e *RequestForwardingEngine) extractAOR(uri string) (string, error) {
	// Simple AOR extraction - remove parameters and headers
	uri = strings.TrimSpace(uri)
	
	// Remove angle brackets if present
	if strings.HasPrefix(uri, "<") && strings.HasSuffix(uri, ">") {
		uri = uri[1 : len(uri)-1]
	}
	
	// Remove parameters (everything after ';')
	if idx := strings.Index(uri, ";"); idx >= 0 {
		uri = uri[:idx]
	}
	
	// Remove headers (everything after '?')
	if idx := strings.Index(uri, "?"); idx >= 0 {
		uri = uri[:idx]
	}
	
	return uri, nil
}

// parseTargetURI parses a target URI and returns address and transport
func (e *RequestForwardingEngine) parseTargetURI(uri string) (net.Addr, string, error) {
	// Simple URI parsing for sip: and sips: URIs
	uri = strings.TrimSpace(uri)
	
	// Remove angle brackets if present
	if strings.HasPrefix(uri, "<") && strings.HasSuffix(uri, ">") {
		uri = uri[1 : len(uri)-1]
	}
	
	// Check scheme
	var transport string
	if strings.HasPrefix(uri, "sip:") {
		transport = "udp" // Default for sip:
		uri = uri[4:]
	} else if strings.HasPrefix(uri, "sips:") {
		transport = "tcp" // Default for sips: (should be TLS, but using TCP for now)
		uri = uri[5:]
	} else {
		return nil, "", fmt.Errorf("unsupported URI scheme")
	}
	
	// Extract host and port
	var host string
	var port int = 5060 // Default SIP port
	
	// Remove user part if present
	if idx := strings.Index(uri, "@"); idx >= 0 {
		uri = uri[idx+1:]
	}
	
	// Remove parameters and headers
	if idx := strings.Index(uri, ";"); idx >= 0 {
		// Check for transport parameter
		params := uri[idx+1:]
		uri = uri[:idx]
		
		// Parse transport parameter
		paramPairs := strings.Split(params, ";")
		for _, param := range paramPairs {
			if strings.HasPrefix(param, "transport=") {
				transport = param[10:]
				break
			}
		}
	}
	
	if idx := strings.Index(uri, "?"); idx >= 0 {
		uri = uri[:idx]
	}
	
	// Parse host:port
	if strings.Contains(uri, ":") {
		parts := strings.Split(uri, ":")
		if len(parts) != 2 {
			return nil, "", fmt.Errorf("invalid host:port format")
		}
		host = parts[0]
		var err error
		port, err = strconv.Atoi(parts[1])
		if err != nil {
			return nil, "", fmt.Errorf("invalid port number: %w", err)
		}
	} else {
		host = uri
	}
	
	// Resolve address
	addr, err := net.ResolveUDPAddr("udp", fmt.Sprintf("%s:%d", host, port))
	if err != nil {
		return nil, "", fmt.Errorf("failed to resolve address: %w", err)
	}
	
	return addr, transport, nil
}

// createViaHeader creates a Via header for this proxy
func (e *RequestForwardingEngine) createViaHeader(transport string) string {
	// Generate a unique branch parameter
	branch := e.generateBranch()
	return fmt.Sprintf("SIP/2.0/%s %s:%d;branch=%s", 
		strings.ToUpper(transport), e.serverHost, e.serverPort, branch)
}

// addViaHeader adds a Via header to the top of the Via header list
func (e *RequestForwardingEngine) addViaHeader(msg *parser.SIPMessage, viaHeader string) {
	// Get existing Via headers
	existingVias := msg.GetHeaders(parser.HeaderVia)
	
	// Remove all Via headers
	msg.RemoveHeader(parser.HeaderVia)
	
	// Add new Via header first
	msg.AddHeader(parser.HeaderVia, viaHeader)
	
	// Add back existing Via headers
	for _, via := range existingVias {
		msg.AddHeader(parser.HeaderVia, via)
	}
}

// parseViaHeader parses a Via header and returns routing information
func (e *RequestForwardingEngine) parseViaHeader(viaHeader string) (net.Addr, string, error) {
	// Parse Via header format: SIP/2.0/transport host:port;parameters
	parts := strings.Fields(viaHeader)
	if len(parts) < 2 {
		return nil, "", fmt.Errorf("invalid Via header format")
	}
	
	// Parse protocol/version/transport
	protocolPart := parts[0] // SIP/2.0/UDP or SIP/2.0/TCP
	protocolParts := strings.Split(protocolPart, "/")
	if len(protocolParts) != 3 {
		return nil, "", fmt.Errorf("invalid protocol part in Via header")
	}
	transport := strings.ToLower(protocolParts[2])
	
	// Parse host:port part (may include parameters)
	hostPart := parts[1]
	if idx := strings.Index(hostPart, ";"); idx >= 0 {
		hostPart = hostPart[:idx]
	}
	
	// Default port
	port := 5060
	host := hostPart
	
	// Parse host:port
	if strings.Contains(hostPart, ":") {
		hostPortParts := strings.Split(hostPart, ":")
		if len(hostPortParts) == 2 {
			host = hostPortParts[0]
			var err error
			port, err = strconv.Atoi(hostPortParts[1])
			if err != nil {
				return nil, "", fmt.Errorf("invalid port in Via header: %w", err)
			}
		}
	}
	
	// Resolve address based on transport
	var addr net.Addr
	var err error
	if transport == "tcp" {
		addr, err = net.ResolveTCPAddr("tcp", fmt.Sprintf("%s:%d", host, port))
	} else {
		addr, err = net.ResolveUDPAddr("udp", fmt.Sprintf("%s:%d", host, port))
	}
	
	if err != nil {
		return nil, "", fmt.Errorf("failed to resolve address from Via header: %w", err)
	}
	
	return addr, transport, nil
}

// generateBranch generates a unique branch parameter for Via headers
func (e *RequestForwardingEngine) generateBranch() string {
	// RFC3261 requires branch parameters to start with "z9hG4bK"
	timestamp := time.Now().UnixNano()
	return fmt.Sprintf("z9hG4bK-%d", timestamp)
}

// checkMaxForwards checks the Max-Forwards header to prevent loops
func (e *RequestForwardingEngine) checkMaxForwards(req *parser.SIPMessage) error {
	maxForwardsHeader := req.GetHeader(parser.HeaderMaxForwards)
	if maxForwardsHeader == "" {
		// Add Max-Forwards if missing
		req.SetHeader(parser.HeaderMaxForwards, strconv.Itoa(e.maxForwards))
		return nil
	}
	
	maxForwards, err := strconv.Atoi(maxForwardsHeader)
	if err != nil {
		return fmt.Errorf("invalid Max-Forwards header: %w", err)
	}
	
	if maxForwards <= 0 {
		return fmt.Errorf("Max-Forwards exceeded")
	}
	
	return nil
}

// decrementMaxForwards decrements the Max-Forwards header
func (e *RequestForwardingEngine) decrementMaxForwards(req *parser.SIPMessage) {
	maxForwardsHeader := req.GetHeader(parser.HeaderMaxForwards)
	if maxForwardsHeader != "" {
		if maxForwards, err := strconv.Atoi(maxForwardsHeader); err == nil {
			req.SetHeader(parser.HeaderMaxForwards, strconv.Itoa(maxForwards-1))
		}
	}
}

// isLocalURI checks if a URI is for this server
func (e *RequestForwardingEngine) isLocalURI(uri string) bool {
	// Simple check - see if the URI contains this server's host
	return strings.Contains(uri, e.serverHost)
}

// Error response methods

func (e *RequestForwardingEngine) sendBadRequest(req *parser.SIPMessage, transaction transaction.Transaction, reason string) error {
	response := parser.NewResponseMessage(parser.StatusBadRequest, reason)
	e.copyRequiredHeaders(req, response)
	return transaction.SendResponse(response)
}

func (e *RequestForwardingEngine) sendNotFound(req *parser.SIPMessage, transaction transaction.Transaction, reason string) error {
	response := parser.NewResponseMessage(parser.StatusNotFound, reason)
	e.copyRequiredHeaders(req, response)
	return transaction.SendResponse(response)
}

func (e *RequestForwardingEngine) sendTooManyHops(req *parser.SIPMessage, transaction transaction.Transaction) error {
	response := parser.NewResponseMessage(parser.StatusTooManyHops, "Too Many Hops")
	e.copyRequiredHeaders(req, response)
	return transaction.SendResponse(response)
}

func (e *RequestForwardingEngine) sendMethodNotAllowed(req *parser.SIPMessage, transaction transaction.Transaction) error {
	response := parser.NewResponseMessage(parser.StatusMethodNotAllowed, "Method Not Allowed")
	e.copyRequiredHeaders(req, response)
	response.SetHeader(parser.HeaderAllow, "INVITE, ACK, BYE, CANCEL, OPTIONS, INFO, REGISTER")
	return transaction.SendResponse(response)
}

func (e *RequestForwardingEngine) sendOptionsResponse(req *parser.SIPMessage, transaction transaction.Transaction) error {
	response := parser.NewResponseMessage(parser.StatusOK, "OK")
	e.copyRequiredHeaders(req, response)
	response.SetHeader(parser.HeaderAllow, "INVITE, ACK, BYE, CANCEL, OPTIONS, INFO, REGISTER")
	response.SetHeader(parser.HeaderSupported, "timer")
	return transaction.SendResponse(response)
}

// copyRequiredHeaders copies required headers from request to response
func (e *RequestForwardingEngine) copyRequiredHeaders(request, response *parser.SIPMessage) {
	// Copy Via headers
	viaHeaders := request.GetHeaders(parser.HeaderVia)
	for _, via := range viaHeaders {
		response.AddHeader(parser.HeaderVia, via)
	}
	
	// Copy From header
	if from := request.GetHeader(parser.HeaderFrom); from != "" {
		response.SetHeader(parser.HeaderFrom, from)
	}
	
	// Copy To header
	if to := request.GetHeader(parser.HeaderTo); to != "" {
		response.SetHeader(parser.HeaderTo, to)
	}
	
	// Copy Call-ID header
	if callID := request.GetHeader(parser.HeaderCallID); callID != "" {
		response.SetHeader(parser.HeaderCallID, callID)
	}
	
	// Copy CSeq header
	if cseq := request.GetHeader(parser.HeaderCSeq); cseq != "" {
		response.SetHeader(parser.HeaderCSeq, cseq)
	}
	
	// Set Content-Length to 0 for responses without body
	response.SetHeader(parser.HeaderContentLength, "0")
}

// extractExtensionFromAOR extracts the extension part from an AOR
func (e *RequestForwardingEngine) extractExtensionFromAOR(aor string) string {
	// Remove sip: or sips: scheme
	if strings.HasPrefix(aor, "sip:") {
		aor = aor[4:]
	} else if strings.HasPrefix(aor, "sips:") {
		aor = aor[5:]
	}
	
	// Extract user part (before @)
	if idx := strings.Index(aor, "@"); idx >= 0 {
		return aor[:idx]
	}
	
	return aor
}

// handleHuntGroupCall handles incoming calls to hunt groups
func (e *RequestForwardingEngine) handleHuntGroupCall(req *parser.SIPMessage, transaction transaction.Transaction, huntGroupError string) error {
	if e.huntGroupEngine == nil {
		return e.sendNotFound(req, transaction, "Hunt group service not available")
	}

	// Extract hunt group ID from error message
	parts := strings.Split(huntGroupError, ":")
	if len(parts) != 2 {
		return e.sendNotFound(req, transaction, "Invalid hunt group reference")
	}

	groupID, err := strconv.Atoi(parts[1])
	if err != nil {
		return e.sendNotFound(req, transaction, "Invalid hunt group ID")
	}

	// Get hunt group
	group, err := e.huntGroupManager.GetGroup(groupID)
	if err != nil {
		return e.sendNotFound(req, transaction, "Hunt group not found")
	}

	// Only handle INVITE requests for hunt groups
	if req.GetMethod() != parser.MethodINVITE {
		return e.sendMethodNotAllowed(req, transaction)
	}

	// Process the hunt group call
	_, err = e.huntGroupEngine.ProcessIncomingCall(req, group)
	if err != nil {
		return e.sendServerError(req, transaction, "Hunt group processing failed")
	}

	// For now, send 100 Trying response
	// In a full implementation, this would be handled by the B2BUA
	response := parser.NewResponseMessage(100, "Trying")
	e.copyRequiredHeaders(req, response)
	return transaction.SendResponse(response)
}

// sendServerError sends a 500 Internal Server Error response
func (e *RequestForwardingEngine) sendServerError(req *parser.SIPMessage, transaction transaction.Transaction, reason string) error {
	response := parser.NewResponseMessage(500, reason)
	e.copyRequiredHeaders(req, response)
	return transaction.SendResponse(response)
}