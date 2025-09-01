package transport_adapter_test

import (
	"net"
	"testing"

	"github.com/zurustar/xylitol2/internal/handlers"
	"github.com/zurustar/xylitol2/internal/parser"
)

// Integration test to verify TransportAdapter works with ValidatedManager

func TestTransportAdapterWithValidatedManager(t *testing.T) {
	// Create a ValidatedManager (which includes validation chain)
	validatedManager := handlers.NewValidatedManager()
	
	// Create other components
	txnMgr := newMockTransactionManager()
	parser := &mockMessageParser{}
	transport := &mockTransportManager{}

	// Create TransportAdapter with ValidatedManager
	adapter := handlers.NewTransportAdapter(validatedManager, txnMgr, parser, transport)

	// Start the adapter
	err := adapter.Start()
	if err != nil {
		t.Fatalf("Failed to start adapter: %v", err)
	}

	// Verify that the transport manager has the adapter registered as handler
	if transport.handler != adapter {
		t.Error("Transport adapter not registered as handler")
	}

	// Test handling a message through the validation chain
	addr, _ := net.ResolveUDPAddr("udp", "192.168.1.1:5060")
	err = adapter.HandleMessage([]byte("INVITE sip:test@example.com SIP/2.0\r\n\r\n"), "UDP", addr)

	if err != nil {
		t.Errorf("Unexpected error handling message: %v", err)
	}
}

func TestTransportAdapterMethodHandlerRegistration(t *testing.T) {
	// Create a ValidatedManager
	validatedManager := handlers.NewValidatedManager()
	
	// Create other components
	txnMgr := newMockTransactionManager()
	parser := &mockMessageParser{}
	transport := &mockTransportManager{}

	// Create TransportAdapter
	adapter := handlers.NewTransportAdapter(validatedManager, txnMgr, parser, transport)

	// Register a method handler through the adapter
	handler := &mockMethodHandler{methods: []string{"INVITE", "BYE"}}
	adapter.RegisterMethodHandler(handler)

	// Verify that supported methods include the registered handler's methods
	methods := adapter.GetSupportedMethods()
	
	// The ValidatedManager should now include the registered handler
	if len(methods) == 0 {
		t.Error("No supported methods returned")
	}
}

func TestTransportAdapterResponseHandling(t *testing.T) {
	// Create components
	validatedManager := handlers.NewValidatedManager()
	txnMgr := newMockTransactionManager()
	transport := &mockTransportManager{}

	// Create a response message
	respMsg := parser.NewResponseMessage(200, "OK")
	respMsg.SetHeader("Via", "SIP/2.0/UDP 192.168.1.1:5060;branch=z9hG4bK123")
	respMsg.SetHeader("From", "sip:alice@example.com;tag=abc123")
	respMsg.SetHeader("To", "sip:bob@example.com;tag=def456")
	respMsg.SetHeader("Call-ID", "test-call-id")
	respMsg.SetHeader("CSeq", "1 INVITE")

	parser := &mockMessageParser{message: respMsg}

	// Create TransportAdapter
	adapter := handlers.NewTransportAdapter(validatedManager, txnMgr, parser, transport)

	// Create a transaction to be found
	txn := &mockTransaction{id: "test-txn"}
	txnMgr.transactions["test"] = txn

	// Test handling a response message
	addr, _ := net.ResolveUDPAddr("udp", "192.168.1.1:5060")
	err := adapter.HandleMessage([]byte("SIP/2.0 200 OK\r\n\r\n"), "UDP", addr)

	if err != nil {
		t.Errorf("Unexpected error handling response: %v", err)
	}
}

func TestTransportAdapterResponseNoTransaction(t *testing.T) {
	// Create components
	validatedManager := handlers.NewValidatedManager()
	txnMgr := newMockTransactionManager()
	transport := &mockTransportManager{}

	// Create a response message
	respMsg := parser.NewResponseMessage(200, "OK")
	respMsg.SetHeader("Via", "SIP/2.0/UDP 192.168.1.1:5060;branch=z9hG4bK123")
	respMsg.SetHeader("From", "sip:alice@example.com;tag=abc123")
	respMsg.SetHeader("To", "sip:bob@example.com;tag=def456")
	respMsg.SetHeader("Call-ID", "test-call-id")
	respMsg.SetHeader("CSeq", "1 INVITE")

	parser := &mockMessageParser{message: respMsg}

	// Create TransportAdapter
	adapter := handlers.NewTransportAdapter(validatedManager, txnMgr, parser, transport)

	// Don't create any transactions - should fail to find one

	// Test handling a response message without transaction
	addr, _ := net.ResolveUDPAddr("udp", "192.168.1.1:5060")
	err := adapter.HandleMessage([]byte("SIP/2.0 200 OK\r\n\r\n"), "UDP", addr)

	if err == nil {
		t.Error("Expected error for missing transaction")
	}

	if err.Error() != "no transaction found for response" {
		t.Errorf("Expected transaction error message, got: %v", err)
	}
}