package handlers

import (
	"fmt"
	"net"
	"strings"

	"github.com/zurustar/xylitol2/internal/parser"
	"github.com/zurustar/xylitol2/internal/transaction"
	"github.com/zurustar/xylitol2/internal/transport"
)

// TransportAdapter bridges the validation chain with the transport layer
type TransportAdapter struct {
	handlerManager HandlerManager
	txnManager     transaction.TransactionManager
	parser         parser.MessageParser
	transport      transport.TransportManager
}

// NewTransportAdapter creates a new transport adapter
func NewTransportAdapter(
	handlerManager HandlerManager,
	txnManager transaction.TransactionManager,
	parser parser.MessageParser,
	transport transport.TransportManager,
) *TransportAdapter {
	return &TransportAdapter{
		handlerManager: handlerManager,
		txnManager:     txnManager,
		parser:         parser,
		transport:      transport,
	}
}

// HandleMessage implements the transport.MessageHandler interface
func (ta *TransportAdapter) HandleMessage(data []byte, transportType string, addr net.Addr) error {
	// Parse the incoming SIP message
	msg, err := ta.parser.Parse(data)
	if err != nil {
		return fmt.Errorf("failed to parse SIP message: %w", err)
	}

	// Set transport information on the message
	msg.Transport = transportType
	msg.Source = addr

	// Handle the parsed message
	return ta.handleParsedMessage(msg)
}

// handleParsedMessage processes a parsed SIP message
func (ta *TransportAdapter) handleParsedMessage(msg *parser.SIPMessage) error {
	if msg.IsRequest() {
		return ta.handleRequest(msg)
	} else {
		return ta.handleResponse(msg)
	}
}

// handleRequest processes SIP request messages
func (ta *TransportAdapter) handleRequest(req *parser.SIPMessage) error {
	// Find or create transaction for this request
	txn := ta.txnManager.FindTransaction(req)
	if txn == nil {
		txn = ta.txnManager.CreateTransaction(req)
		if txn == nil {
			return fmt.Errorf("failed to create transaction for request")
		}
	}

	// Process the request through the handler manager (which includes validation)
	err := ta.handlerManager.HandleRequest(req, txn)
	if err != nil {
		// If handler processing failed, try to send a 500 response
		if sendErr := ta.sendErrorResponse(req, txn, 500, "Internal Server Error"); sendErr != nil {
			return fmt.Errorf("handler error: %w, send error response failed: %v", err, sendErr)
		}
		return fmt.Errorf("handler processing failed: %w", err)
	}

	return nil
}

// handleResponse processes SIP response messages
func (ta *TransportAdapter) handleResponse(resp *parser.SIPMessage) error {
	// Find the transaction for this response
	txn := ta.txnManager.FindTransaction(resp)
	if txn == nil {
		return fmt.Errorf("no transaction found for response")
	}

	// Process the response through the transaction
	return txn.ProcessMessage(resp)
}

// sendErrorResponse sends an error response for a request
func (ta *TransportAdapter) sendErrorResponse(req *parser.SIPMessage, txn transaction.Transaction, code int, reason string) error {
	// Create error response
	resp := ta.createErrorResponse(req, code, reason)
	
	// Send response through transaction
	return txn.SendResponse(resp)
}

// createErrorResponse creates a basic error response
func (ta *TransportAdapter) createErrorResponse(req *parser.SIPMessage, code int, reason string) *parser.SIPMessage {
	// Create response message
	resp := parser.NewResponseMessage(code, reason)
	
	// Copy required headers from request
	if via := req.GetHeader("Via"); via != "" {
		resp.SetHeader("Via", via)
	}
	
	if from := req.GetHeader("From"); from != "" {
		resp.SetHeader("From", from)
	}
	
	if to := req.GetHeader("To"); to != "" {
		// Add tag to To header if not present
		if !adapterContainsTag(to) {
			to += ";tag=" + adapterGenerateTag()
		}
		resp.SetHeader("To", to)
	}
	
	if callID := req.GetHeader("Call-ID"); callID != "" {
		resp.SetHeader("Call-ID", callID)
	}
	
	if cseq := req.GetHeader("CSeq"); cseq != "" {
		resp.SetHeader("CSeq", cseq)
	}
	
	// Set Content-Length
	resp.SetHeader("Content-Length", "0")
	
	return resp
}

// RegisterMethodHandler registers a method handler with the handler manager
func (ta *TransportAdapter) RegisterMethodHandler(handler MethodHandler) {
	ta.handlerManager.RegisterHandler(handler)
}

// GetSupportedMethods returns the list of supported SIP methods
func (ta *TransportAdapter) GetSupportedMethods() []string {
	return ta.handlerManager.GetSupportedMethods()
}

// Start initializes the transport adapter and registers itself as the message handler
func (ta *TransportAdapter) Start() error {
	// Register this adapter as the message handler for the transport
	ta.transport.RegisterHandler(ta)
	return nil
}

// Helper functions

// adapterContainsTag checks if a header contains a tag parameter
func adapterContainsTag(header string) bool {
	return strings.Contains(header, "tag=")
}

// adapterGenerateTag generates a random tag for To header
func adapterGenerateTag() string {
	return fmt.Sprintf("tag-%d", adapterGenerateRandomNumber())
}

// adapterGenerateRandomNumber generates a random number (simplified implementation)
func adapterGenerateRandomNumber() int64 {
	// This is a simplified implementation
	// In production, use crypto/rand for better randomness
	return 123456789
}