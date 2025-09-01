package handlers

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/zurustar/xylitol2/internal/database"
	"github.com/zurustar/xylitol2/internal/logging"
	"github.com/zurustar/xylitol2/internal/parser"
	"github.com/zurustar/xylitol2/internal/registrar"
	"github.com/zurustar/xylitol2/internal/transaction"
)

// RegisterHandler handles REGISTER requests for user registration
type RegisterHandler struct {
	registrar registrar.Registrar
	logger    logging.Logger
}

// NewRegisterHandler creates a new register handler
func NewRegisterHandler(registrar registrar.Registrar, logger logging.Logger) *RegisterHandler {
	return &RegisterHandler{
		registrar: registrar,
		logger:    logger,
	}
}

// CanHandle returns true if this handler can process the given method
func (h *RegisterHandler) CanHandle(method string) bool {
	return method == parser.MethodREGISTER
}

// HandleRequest processes REGISTER requests
func (h *RegisterHandler) HandleRequest(req *parser.SIPMessage, txn transaction.Transaction) error {
	h.logger.Debug("Handling REGISTER request")

	// Extract AOR from To header
	toHeader := req.GetHeader(parser.HeaderTo)
	if toHeader == "" {
		response := parser.NewResponseMessage(parser.StatusBadRequest, parser.GetReasonPhraseForCode(parser.StatusBadRequest))
		h.copyResponseHeaders(req, response)
		return txn.SendResponse(response)
	}

	aor := h.extractAOR(toHeader)
	if aor == "" {
		response := parser.NewResponseMessage(parser.StatusBadRequest, parser.GetReasonPhraseForCode(parser.StatusBadRequest))
		h.copyResponseHeaders(req, response)
		return txn.SendResponse(response)
	}

	// Get Contact header(s)
	contactHeaders := req.GetHeaders(parser.HeaderContact)
	if len(contactHeaders) == 0 {
		response := parser.NewResponseMessage(parser.StatusBadRequest, parser.GetReasonPhraseForCode(parser.StatusBadRequest))
		h.copyResponseHeaders(req, response)
		return txn.SendResponse(response)
	}

	// Get Expires header or default
	expiresHeader := req.GetHeader(parser.HeaderExpires)
	defaultExpires := 3600 // Default 1 hour

	// Process each contact
	var registeredContacts []string
	for _, contactHeader := range contactHeaders {
		if err := h.processContact(req, aor, contactHeader, expiresHeader, defaultExpires); err != nil {
			h.logger.Error("Failed to process contact", 
				logging.Field{Key: "error", Value: err},
				logging.Field{Key: "contact", Value: contactHeader})
			response := parser.NewResponseMessage(parser.StatusServerInternalError, parser.GetReasonPhraseForCode(parser.StatusServerInternalError))
			h.copyResponseHeaders(req, response)
			return txn.SendResponse(response)
		}
		registeredContacts = append(registeredContacts, contactHeader)
	}

	// Create successful response
	response := parser.NewResponseMessage(parser.StatusOK, parser.GetReasonPhraseForCode(parser.StatusOK))
	h.copyResponseHeaders(req, response)

	// Add registered contacts to response
	for _, contact := range registeredContacts {
		response.AddHeader(parser.HeaderContact, contact)
	}

	// Add Date header
	response.SetHeader("Date", time.Now().UTC().Format(time.RFC1123))

	h.logger.Info("Registration successful", 
		logging.Field{Key: "aor", Value: aor},
		logging.Field{Key: "contacts", Value: len(registeredContacts)})

	return txn.SendResponse(response)
}

// processContact processes a single contact header for registration
func (h *RegisterHandler) processContact(req *parser.SIPMessage, aor, contactHeader, expiresHeader string, defaultExpires int) error {
	// Parse contact URI and parameters
	contactURI, params := h.parseContactHeader(contactHeader)
	if contactURI == "" {
		return fmt.Errorf("invalid contact URI")
	}

	// Check for wildcard contact (unregister all)
	if contactURI == "*" {
		return h.registrar.Unregister(aor)
	}

	// Get expires value (from contact parameter, Expires header, or default)
	expires := defaultExpires
	if expiresParam, ok := params["expires"]; ok {
		if exp, err := strconv.Atoi(expiresParam); err == nil {
			expires = exp
		}
	} else if expiresHeader != "" {
		if exp, err := strconv.Atoi(expiresHeader); err == nil {
			expires = exp
		}
	}

	// If expires is 0, this is an unregistration
	if expires == 0 {
		// Find and remove specific contact
		contacts, err := h.registrar.FindContacts(aor)
		if err != nil {
			return err
		}

		for _, contact := range contacts {
			if contact.URI == contactURI {
				// Remove this specific contact
				return h.registrar.Unregister(aor) // Simplified - in real implementation, remove specific contact
			}
		}
		return nil // Contact not found, but that's OK
	}

	// Create contact for registration
	contact := &database.RegistrarContact{
		AOR:     aor,
		URI:     contactURI,
		Expires: time.Now().Add(time.Duration(expires) * time.Second),
		CallID:  req.GetHeader(parser.HeaderCallID),
		CSeq:    h.parseCSeq(req.GetHeader(parser.HeaderCSeq)),
	}

	return h.registrar.Register(contact, expires)
}

// parseContactHeader parses a Contact header and returns URI and parameters
func (h *RegisterHandler) parseContactHeader(header string) (string, map[string]string) {
	params := make(map[string]string)
	
	// Remove angle brackets if present
	header = strings.TrimSpace(header)
	if strings.HasPrefix(header, "<") && strings.Contains(header, ">") {
		endIdx := strings.Index(header, ">")
		uri := header[1:endIdx]
		paramStr := strings.TrimSpace(header[endIdx+1:])
		
		// Parse parameters
		if strings.HasPrefix(paramStr, ";") {
			paramStr = paramStr[1:]
			paramPairs := strings.Split(paramStr, ";")
			for _, pair := range paramPairs {
				if idx := strings.Index(pair, "="); idx != -1 {
					key := strings.TrimSpace(pair[:idx])
					value := strings.TrimSpace(pair[idx+1:])
					params[key] = value
				}
			}
		}
		
		return uri, params
	}
	
	// No angle brackets, parse as simple URI with possible parameters
	if idx := strings.Index(header, ";"); idx != -1 {
		uri := strings.TrimSpace(header[:idx])
		paramStr := header[idx+1:]
		
		paramPairs := strings.Split(paramStr, ";")
		for _, pair := range paramPairs {
			if eqIdx := strings.Index(pair, "="); eqIdx != -1 {
				key := strings.TrimSpace(pair[:eqIdx])
				value := strings.TrimSpace(pair[eqIdx+1:])
				params[key] = value
			}
		}
		
		return uri, params
	}
	
	return strings.TrimSpace(header), params
}

// parseCSeq parses the CSeq header and returns the sequence number
func (h *RegisterHandler) parseCSeq(cseqHeader string) uint32 {
	if cseqHeader == "" {
		return 0
	}
	
	parts := strings.Fields(cseqHeader)
	if len(parts) < 1 {
		return 0
	}
	
	seq, err := strconv.ParseUint(parts[0], 10, 32)
	if err != nil {
		return 0
	}
	
	return uint32(seq)
}

// copyResponseHeaders copies necessary headers from request to response
func (h *RegisterHandler) copyResponseHeaders(req *parser.SIPMessage, resp *parser.SIPMessage) {
	// Copy mandatory headers for responses
	if via := req.GetHeader(parser.HeaderVia); via != "" {
		resp.SetHeader(parser.HeaderVia, via)
	}
	if from := req.GetHeader(parser.HeaderFrom); from != "" {
		resp.SetHeader(parser.HeaderFrom, from)
	}
	if to := req.GetHeader(parser.HeaderTo); to != "" {
		// Add tag to To header if not present
		if !h.containsTag(to) {
			to += ";tag=reg-" + h.generateTag()
		}
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
func (h *RegisterHandler) extractAOR(uri string) string {
	// Remove display name if present
	if idx := strings.Index(uri, "<"); idx != -1 {
		if endIdx := strings.Index(uri[idx:], ">"); endIdx != -1 {
			uri = uri[idx+1 : idx+endIdx]
		}
	}
	
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

// containsTag checks if a header contains a tag parameter
func (h *RegisterHandler) containsTag(header string) bool {
	return strings.Contains(header, "tag=")
}

// generateTag generates a simple tag for To header
func (h *RegisterHandler) generateTag() string {
	return fmt.Sprintf("%d", time.Now().UnixNano()%1000000)
}