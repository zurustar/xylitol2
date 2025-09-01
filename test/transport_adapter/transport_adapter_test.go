package transport_adapter_test

import (
	"fmt"
	"net"
	"strings"
	"testing"

	"github.com/zurustar/xylitol2/internal/handlers"
	"github.com/zurustar/xylitol2/internal/parser"
	"github.com/zurustar/xylitol2/internal/transaction"
	"github.com/zurustar/xylitol2/internal/transport"
)

// Test-specific mocks

type mockHandlerManager struct {
	handlers        []handlers.MethodHandler
	handleRequestFn func(*parser.SIPMessage, transaction.Transaction) error
}

func (m *mockHandlerManager) RegisterHandler(handler handlers.MethodHandler) {
	m.handlers = append(m.handlers, handler)
}

func (m *mockHandlerManager) HandleRequest(req *parser.SIPMessage, txn transaction.Transaction) error {
	if m.handleRequestFn != nil {
		return m.handleRequestFn(req, txn)
	}
	return nil
}

func (m *mockHandlerManager) GetSupportedMethods() []string {
	return []string{"INVITE", "REGISTER"}
}

type mockTransactionManager struct {
	transactions map[string]transaction.Transaction
}

func newMockTransactionManager() *mockTransactionManager {
	return &mockTransactionManager{
		transactions: make(map[string]transaction.Transaction),
	}
}

func (m *mockTransactionManager) CreateTransaction(msg *parser.SIPMessage) transaction.Transaction {
	txn := &mockTransaction{id: "test-txn-" + msg.GetHeader("Call-ID")}
	m.transactions[txn.id] = txn
	return txn
}

func (m *mockTransactionManager) FindTransaction(msg *parser.SIPMessage) transaction.Transaction {
	for _, txn := range m.transactions {
		return txn // Return first transaction for simplicity
	}
	return nil
}

func (m *mockTransactionManager) CleanupExpired() {
	// No-op for testing
}

type mockTransaction struct {
	id          string
	state       transaction.TransactionState
	responses   []*parser.SIPMessage
	processErr  error
	sendRespErr error
}

func (m *mockTransaction) GetState() transaction.TransactionState {
	return m.state
}

func (m *mockTransaction) ProcessMessage(msg *parser.SIPMessage) error {
	return m.processErr
}

func (m *mockTransaction) SendResponse(response *parser.SIPMessage) error {
	if m.sendRespErr != nil {
		return m.sendRespErr
	}
	m.responses = append(m.responses, response)
	return nil
}

func (m *mockTransaction) GetID() string {
	return m.id
}

func (m *mockTransaction) IsClient() bool {
	return false
}

type mockMessageParser struct {
	parseErr error
	message  *parser.SIPMessage
}

func (m *mockMessageParser) Parse(data []byte) (*parser.SIPMessage, error) {
	if m.parseErr != nil {
		return nil, m.parseErr
	}
	if m.message != nil {
		return m.message, nil
	}
	
	// Create a basic test message
	msg := parser.NewRequestMessage("INVITE", "sip:test@example.com")
	msg.SetHeader("Via", "SIP/2.0/UDP 192.168.1.1:5060;branch=z9hG4bK123")
	msg.SetHeader("From", "sip:alice@example.com;tag=abc123")
	msg.SetHeader("To", "sip:bob@example.com")
	msg.SetHeader("Call-ID", "test-call-id")
	msg.SetHeader("CSeq", "1 INVITE")
	msg.SetHeader("Content-Length", "0")
	return msg, nil
}

func (m *mockMessageParser) Serialize(msg *parser.SIPMessage) ([]byte, error) {
	return []byte("SIP message"), nil
}

func (m *mockMessageParser) Validate(msg *parser.SIPMessage) error {
	return nil
}

type mockTransportManager struct {
	handler transport.MessageHandler
}

func (m *mockTransportManager) StartUDP(port int) error {
	return nil
}

func (m *mockTransportManager) StartTCP(port int) error {
	return nil
}

func (m *mockTransportManager) SendMessage(msg []byte, transport string, addr net.Addr) error {
	return nil
}

func (m *mockTransportManager) RegisterHandler(handler transport.MessageHandler) {
	m.handler = handler
}

func (m *mockTransportManager) Stop() error {
	return nil
}

type mockMethodHandler struct {
	methods   []string
	handleErr error
}

func (m *mockMethodHandler) HandleRequest(req *parser.SIPMessage, txn transaction.Transaction) error {
	return m.handleErr
}

func (m *mockMethodHandler) CanHandle(method string) bool {
	for _, supportedMethod := range m.methods {
		if supportedMethod == method {
			return true
		}
	}
	return false
}

// Test functions

func TestNewTransportAdapter(t *testing.T) {
	handlerMgr := &mockHandlerManager{}
	txnMgr := newMockTransactionManager()
	parser := &mockMessageParser{}
	transport := &mockTransportManager{}

	adapter := handlers.NewTransportAdapter(handlerMgr, txnMgr, parser, transport)

	if adapter == nil {
		t.Fatal("NewTransportAdapter returned nil")
	}
}

func TestTransportAdapterHandleMessageParseError(t *testing.T) {
	handlerMgr := &mockHandlerManager{}
	txnMgr := newMockTransactionManager()
	parser := &mockMessageParser{parseErr: fmt.Errorf("parse error")}
	transport := &mockTransportManager{}

	adapter := handlers.NewTransportAdapter(handlerMgr, txnMgr, parser, transport)

	addr, _ := net.ResolveUDPAddr("udp", "192.168.1.1:5060")
	err := adapter.HandleMessage([]byte("invalid sip message"), "UDP", addr)

	if err == nil {
		t.Error("Expected error for parse failure")
	}

	if !strings.Contains(err.Error(), "failed to parse SIP message") {
		t.Errorf("Expected parse error message, got: %v", err)
	}
}

func TestTransportAdapterHandleRequestSuccess(t *testing.T) {
	handlerMgr := &mockHandlerManager{}
	txnMgr := newMockTransactionManager()
	parser := &mockMessageParser{}
	transport := &mockTransportManager{}

	adapter := handlers.NewTransportAdapter(handlerMgr, txnMgr, parser, transport)

	addr, _ := net.ResolveUDPAddr("udp", "192.168.1.1:5060")
	err := adapter.HandleMessage([]byte("INVITE sip:test@example.com SIP/2.0\r\n\r\n"), "UDP", addr)

	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
}

func TestTransportAdapterHandleRequestHandlerError(t *testing.T) {
	handlerMgr := &mockHandlerManager{
		handleRequestFn: func(*parser.SIPMessage, transaction.Transaction) error {
			return fmt.Errorf("handler error")
		},
	}
	txnMgr := newMockTransactionManager()
	parser := &mockMessageParser{}
	transport := &mockTransportManager{}

	adapter := handlers.NewTransportAdapter(handlerMgr, txnMgr, parser, transport)

	addr, _ := net.ResolveUDPAddr("udp", "192.168.1.1:5060")
	err := adapter.HandleMessage([]byte("INVITE sip:test@example.com SIP/2.0\r\n\r\n"), "UDP", addr)

	if err == nil {
		t.Error("Expected error for handler failure")
	}

	if !strings.Contains(err.Error(), "handler processing failed") {
		t.Errorf("Expected handler error message, got: %v", err)
	}
}

func TestTransportAdapterRegisterMethodHandler(t *testing.T) {
	handlerMgr := &mockHandlerManager{}
	txnMgr := newMockTransactionManager()
	parser := &mockMessageParser{}
	transport := &mockTransportManager{}

	adapter := handlers.NewTransportAdapter(handlerMgr, txnMgr, parser, transport)

	handler := &mockMethodHandler{methods: []string{"INVITE"}}
	adapter.RegisterMethodHandler(handler)

	if len(handlerMgr.handlers) != 1 {
		t.Error("Handler not registered")
	}

	if handlerMgr.handlers[0] != handler {
		t.Error("Wrong handler registered")
	}
}

func TestTransportAdapterStart(t *testing.T) {
	handlerMgr := &mockHandlerManager{}
	txnMgr := newMockTransactionManager()
	parser := &mockMessageParser{}
	transport := &mockTransportManager{}

	adapter := handlers.NewTransportAdapter(handlerMgr, txnMgr, parser, transport)

	err := adapter.Start()
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if transport.handler != adapter {
		t.Error("Transport adapter not registered as handler")
	}
}

func TestTransportAdapterGetSupportedMethods(t *testing.T) {
	handlerMgr := &mockHandlerManager{}
	txnMgr := newMockTransactionManager()
	parser := &mockMessageParser{}
	transport := &mockTransportManager{}

	adapter := handlers.NewTransportAdapter(handlerMgr, txnMgr, parser, transport)

	methods := adapter.GetSupportedMethods()
	if len(methods) != 2 {
		t.Errorf("Expected 2 methods, got %d", len(methods))
	}
}