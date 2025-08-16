package proxy

import (
	"net"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/zurustar/xylitol2/internal/database"
	"github.com/zurustar/xylitol2/internal/parser"
	"github.com/zurustar/xylitol2/internal/transaction"
	"github.com/zurustar/xylitol2/internal/transport"
)

// Mock implementations for testing

type mockRegistrar struct {
	contacts map[string][]*database.RegistrarContact
}

func newMockRegistrar() *mockRegistrar {
	return &mockRegistrar{
		contacts: make(map[string][]*database.RegistrarContact),
	}
}

func (m *mockRegistrar) Register(contact *database.RegistrarContact, expires int) error {
	return nil
}

func (m *mockRegistrar) Unregister(aor string) error {
	delete(m.contacts, aor)
	return nil
}

func (m *mockRegistrar) FindContacts(aor string) ([]*database.RegistrarContact, error) {
	if contacts, exists := m.contacts[aor]; exists {
		return contacts, nil
	}
	return nil, nil
}

func (m *mockRegistrar) CleanupExpired() {}

func (m *mockRegistrar) addContact(aor, uri string) {
	contact := &database.RegistrarContact{
		AOR:     aor,
		URI:     uri,
		Expires: time.Now().Add(time.Hour),
		CallID:  "test-call-id",
		CSeq:    1,
	}
	m.contacts[aor] = append(m.contacts[aor], contact)
}

type mockTransportManager struct {
	sentMessages []sentMessage
}

type sentMessage struct {
	data      []byte
	transport string
	addr      net.Addr
}

func newMockTransportManager() *mockTransportManager {
	return &mockTransportManager{
		sentMessages: make([]sentMessage, 0),
	}
}

func (m *mockTransportManager) StartUDP(port int) error { return nil }
func (m *mockTransportManager) StartTCP(port int) error { return nil }
func (m *mockTransportManager) RegisterHandler(handler transport.MessageHandler) {}
func (m *mockTransportManager) Stop() error { return nil }

func (m *mockTransportManager) SendMessage(msg []byte, transport string, addr net.Addr) error {
	m.sentMessages = append(m.sentMessages, sentMessage{
		data:      msg,
		transport: transport,
		addr:      addr,
	})
	return nil
}

func (m *mockTransportManager) getLastSentMessage() *sentMessage {
	if len(m.sentMessages) == 0 {
		return nil
	}
	return &m.sentMessages[len(m.sentMessages)-1]
}

type mockTransactionManager struct{}

func (m *mockTransactionManager) CreateTransaction(msg *parser.SIPMessage) transaction.Transaction {
	return &mockTransaction{}
}

func (m *mockTransactionManager) FindTransaction(msg *parser.SIPMessage) transaction.Transaction {
	return &mockTransaction{}
}

func (m *mockTransactionManager) CleanupExpired() {}

type mockTransaction struct {
	responses []*parser.SIPMessage
}

func (m *mockTransaction) GetState() transaction.TransactionState {
	return transaction.StateTrying
}

func (m *mockTransaction) ProcessMessage(msg *parser.SIPMessage) error {
	return nil
}

func (m *mockTransaction) SendResponse(response *parser.SIPMessage) error {
	m.responses = append(m.responses, response)
	return nil
}

func (m *mockTransaction) GetID() string {
	return "test-transaction-id"
}

func (m *mockTransaction) IsClient() bool {
	return false
}

func (m *mockTransaction) getLastResponse() *parser.SIPMessage {
	if len(m.responses) == 0 {
		return nil
	}
	return m.responses[len(m.responses)-1]
}

type mockParser struct{}

func (m *mockParser) Parse(data []byte) (*parser.SIPMessage, error) {
	// Simple mock parser - just return a basic message
	msg := parser.NewRequestMessage("INVITE", "sip:test@example.com")
	return msg, nil
}

func (m *mockParser) Serialize(msg *parser.SIPMessage) ([]byte, error) {
	// Simple serialization for testing
	return []byte(msg.StartLine.String()), nil
}

func (m *mockParser) Validate(msg *parser.SIPMessage) error {
	return nil
}

// Test helper functions

func createTestInviteRequest() *parser.SIPMessage {
	req := parser.NewRequestMessage(parser.MethodINVITE, "sip:alice@example.com")
	req.SetHeader(parser.HeaderVia, "SIP/2.0/UDP client.example.com:5060;branch=z9hG4bK-test")
	req.SetHeader(parser.HeaderFrom, "Bob <sip:bob@example.com>;tag=12345")
	req.SetHeader(parser.HeaderTo, "Alice <sip:alice@example.com>")
	req.SetHeader(parser.HeaderCallID, "test-call-id@example.com")
	req.SetHeader(parser.HeaderCSeq, "1 INVITE")
	req.SetHeader(parser.HeaderMaxForwards, "70")
	req.SetHeader(parser.HeaderContentLength, "0")
	return req
}

func createTestResponse() *parser.SIPMessage {
	resp := parser.NewResponseMessage(parser.StatusOK, "OK")
	resp.SetHeader(parser.HeaderVia, "SIP/2.0/UDP 127.0.0.1:5060;branch=z9hG4bK-proxy")
	resp.AddHeader(parser.HeaderVia, "SIP/2.0/UDP 127.0.0.1:5061;branch=z9hG4bK-test")
	resp.SetHeader(parser.HeaderFrom, "Bob <sip:bob@example.com>;tag=12345")
	resp.SetHeader(parser.HeaderTo, "Alice <sip:alice@example.com>;tag=67890")
	resp.SetHeader(parser.HeaderCallID, "test-call-id@example.com")
	resp.SetHeader(parser.HeaderCSeq, "1 INVITE")
	resp.SetHeader(parser.HeaderContentLength, "0")
	return resp
}

// Tests

func TestNewRequestForwardingEngine(t *testing.T) {
	mockReg := newMockRegistrar()
	mockTM := newMockTransportManager()
	mockTxnMgr := &mockTransactionManager{}
	mockParser := &mockParser{}

	engine := NewRequestForwardingEngine(mockReg, mockTM, mockTxnMgr, mockParser, "proxy.example.com", 5060)

	if engine == nil {
		t.Fatal("Expected non-nil engine")
	}

	if engine.serverHost != "proxy.example.com" {
		t.Errorf("Expected serverHost 'proxy.example.com', got '%s'", engine.serverHost)
	}

	if engine.serverPort != 5060 {
		t.Errorf("Expected serverPort 5060, got %d", engine.serverPort)
	}

	if engine.maxForwards != 70 {
		t.Errorf("Expected maxForwards 70, got %d", engine.maxForwards)
	}
}

func TestProcessRequest_INVITE(t *testing.T) {
	mockReg := newMockRegistrar()
	mockTM := newMockTransportManager()
	mockTxnMgr := &mockTransactionManager{}
	mockParser := &mockParser{}
	mockTxn := &mockTransaction{}

	engine := NewRequestForwardingEngine(mockReg, mockTM, mockTxnMgr, mockParser, "proxy.example.com", 5060)

	// Add a registered contact
	mockReg.addContact("sip:alice@example.com", "sip:alice@127.0.0.1:5060")

	req := createTestInviteRequest()

	err := engine.ProcessRequest(req, mockTxn)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	// Check that message was sent
	sentMsg := mockTM.getLastSentMessage()
	if sentMsg == nil {
		t.Fatal("Expected message to be sent")
	}

	if sentMsg.transport != "udp" {
		t.Errorf("Expected transport 'udp', got '%s'", sentMsg.transport)
	}
}

func TestProcessRequest_UserNotRegistered(t *testing.T) {
	mockReg := newMockRegistrar()
	mockTM := newMockTransportManager()
	mockTxnMgr := &mockTransactionManager{}
	mockParser := &mockParser{}
	mockTxn := &mockTransaction{}

	engine := NewRequestForwardingEngine(mockReg, mockTM, mockTxnMgr, mockParser, "proxy.example.com", 5060)

	req := createTestInviteRequest()

	err := engine.ProcessRequest(req, mockTxn)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	// Check that 404 response was sent
	resp := mockTxn.getLastResponse()
	if resp == nil {
		t.Fatal("Expected response to be sent")
	}

	if resp.GetStatusCode() != parser.StatusNotFound {
		t.Errorf("Expected status code %d, got %d", parser.StatusNotFound, resp.GetStatusCode())
	}
}

func TestProcessRequest_MaxForwardsExceeded(t *testing.T) {
	mockReg := newMockRegistrar()
	mockTM := newMockTransportManager()
	mockTxnMgr := &mockTransactionManager{}
	mockParser := &mockParser{}
	mockTxn := &mockTransaction{}

	engine := NewRequestForwardingEngine(mockReg, mockTM, mockTxnMgr, mockParser, "proxy.example.com", 5060)

	req := createTestInviteRequest()
	req.SetHeader(parser.HeaderMaxForwards, "0")

	err := engine.ProcessRequest(req, mockTxn)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	// Check that 483 response was sent
	resp := mockTxn.getLastResponse()
	if resp == nil {
		t.Fatal("Expected response to be sent")
	}

	if resp.GetStatusCode() != parser.StatusTooManyHops {
		t.Errorf("Expected status code %d, got %d", parser.StatusTooManyHops, resp.GetStatusCode())
	}
}

func TestProcessRequest_REGISTER(t *testing.T) {
	mockReg := newMockRegistrar()
	mockTM := newMockTransportManager()
	mockTxnMgr := &mockTransactionManager{}
	mockParser := &mockParser{}
	mockTxn := &mockTransaction{}

	engine := NewRequestForwardingEngine(mockReg, mockTM, mockTxnMgr, mockParser, "proxy.example.com", 5060)

	req := parser.NewRequestMessage(parser.MethodREGISTER, "sip:alice@example.com")

	err := engine.ProcessRequest(req, mockTxn)
	if err == nil {
		t.Fatal("Expected error for REGISTER request")
	}

	if !strings.Contains(err.Error(), "REGISTER requests should be handled by registrar") {
		t.Errorf("Expected specific error message, got: %v", err)
	}
}

func TestProcessRequest_UnsupportedMethod(t *testing.T) {
	mockReg := newMockRegistrar()
	mockTM := newMockTransportManager()
	mockTxnMgr := &mockTransactionManager{}
	mockParser := &mockParser{}
	mockTxn := &mockTransaction{}

	engine := NewRequestForwardingEngine(mockReg, mockTM, mockTxnMgr, mockParser, "proxy.example.com", 5060)

	req := parser.NewRequestMessage("UNSUPPORTED", "sip:alice@example.com")

	err := engine.ProcessRequest(req, mockTxn)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	// Check that 405 response was sent
	resp := mockTxn.getLastResponse()
	if resp == nil {
		t.Fatal("Expected response to be sent")
	}

	if resp.GetStatusCode() != parser.StatusMethodNotAllowed {
		t.Errorf("Expected status code %d, got %d", parser.StatusMethodNotAllowed, resp.GetStatusCode())
	}

	// Check Allow header
	allow := resp.GetHeader(parser.HeaderAllow)
	if allow == "" {
		t.Error("Expected Allow header in 405 response")
	}
}

func TestForwardRequest(t *testing.T) {
	mockReg := newMockRegistrar()
	mockTM := newMockTransportManager()
	mockTxnMgr := &mockTransactionManager{}
	mockParser := &mockParser{}

	engine := NewRequestForwardingEngine(mockReg, mockTM, mockTxnMgr, mockParser, "proxy.example.com", 5060)

	req := createTestInviteRequest()
	targets := []*database.RegistrarContact{
		{
			AOR:     "sip:alice@example.com",
			URI:     "sip:alice@127.0.0.1:5060",
			Expires: time.Now().Add(time.Hour),
			CallID:  "test-call-id",
			CSeq:    1,
		},
	}

	err := engine.ForwardRequest(req, targets)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	// Check that message was sent
	sentMsg := mockTM.getLastSentMessage()
	if sentMsg == nil {
		t.Fatal("Expected message to be sent")
	}

	if sentMsg.transport != "udp" {
		t.Errorf("Expected transport 'udp', got '%s'", sentMsg.transport)
	}

	// Check that address was resolved correctly
	if !strings.Contains(sentMsg.addr.String(), "127.0.0.1") {
		t.Errorf("Expected address to contain '127.0.0.1', got '%s'", sentMsg.addr.String())
	}
}

func TestForwardRequest_NoTargets(t *testing.T) {
	mockReg := newMockRegistrar()
	mockTM := newMockTransportManager()
	mockTxnMgr := &mockTransactionManager{}
	mockParser := &mockParser{}

	engine := NewRequestForwardingEngine(mockReg, mockTM, mockTxnMgr, mockParser, "proxy.example.com", 5060)

	req := createTestInviteRequest()
	targets := []*database.RegistrarContact{}

	err := engine.ForwardRequest(req, targets)
	if err == nil {
		t.Fatal("Expected error for no targets")
	}

	if !strings.Contains(err.Error(), "no targets to forward to") {
		t.Errorf("Expected specific error message, got: %v", err)
	}
}

func TestProcessResponse(t *testing.T) {
	mockReg := newMockRegistrar()
	mockTM := newMockTransportManager()
	mockTxnMgr := &mockTransactionManager{}
	mockParser := &mockParser{}
	mockTxn := &mockTransaction{}

	engine := NewRequestForwardingEngine(mockReg, mockTM, mockTxnMgr, mockParser, "proxy.example.com", 5060)

	resp := createTestResponse()

	err := engine.ProcessResponse(resp, mockTxn)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	// Check that message was sent
	sentMsg := mockTM.getLastSentMessage()
	if sentMsg == nil {
		t.Fatal("Expected message to be sent")
	}

	// Check that Via header was removed
	viaHeaders := resp.GetHeaders(parser.HeaderVia)
	if len(viaHeaders) != 1 {
		t.Errorf("Expected 1 Via header after processing, got %d", len(viaHeaders))
	}

	if viaHeaders[0] != "SIP/2.0/UDP 127.0.0.1:5061;branch=z9hG4bK-test" {
		t.Errorf("Expected client Via header to remain, got '%s'", viaHeaders[0])
	}
}

func TestProcessResponse_NoViaHeaders(t *testing.T) {
	mockReg := newMockRegistrar()
	mockTM := newMockTransportManager()
	mockTxnMgr := &mockTransactionManager{}
	mockParser := &mockParser{}
	mockTxn := &mockTransaction{}

	engine := NewRequestForwardingEngine(mockReg, mockTM, mockTxnMgr, mockParser, "proxy.example.com", 5060)

	resp := parser.NewResponseMessage(parser.StatusOK, "OK")

	err := engine.ProcessResponse(resp, mockTxn)
	if err == nil {
		t.Fatal("Expected error for response without Via headers")
	}

	if !strings.Contains(err.Error(), "response missing Via headers") {
		t.Errorf("Expected specific error message, got: %v", err)
	}
}

func TestExtractAOR(t *testing.T) {
	mockReg := newMockRegistrar()
	mockTM := newMockTransportManager()
	mockTxnMgr := &mockTransactionManager{}
	mockParser := &mockParser{}

	engine := NewRequestForwardingEngine(mockReg, mockTM, mockTxnMgr, mockParser, "proxy.example.com", 5060)

	tests := []struct {
		input    string
		expected string
	}{
		{"sip:alice@example.com", "sip:alice@example.com"},
		{"<sip:alice@example.com>", "sip:alice@example.com"},
		{"sip:alice@example.com;transport=tcp", "sip:alice@example.com"},
		{"sip:alice@example.com?header=value", "sip:alice@example.com"},
		{"<sip:alice@example.com;transport=tcp>", "sip:alice@example.com"},
	}

	for _, test := range tests {
		result, err := engine.extractAOR(test.input)
		if err != nil {
			t.Errorf("Unexpected error for input '%s': %v", test.input, err)
			continue
		}

		if result != test.expected {
			t.Errorf("For input '%s', expected '%s', got '%s'", test.input, test.expected, result)
		}
	}
}

func TestParseTargetURI(t *testing.T) {
	mockReg := newMockRegistrar()
	mockTM := newMockTransportManager()
	mockTxnMgr := &mockTransactionManager{}
	mockParser := &mockParser{}

	engine := NewRequestForwardingEngine(mockReg, mockTM, mockTxnMgr, mockParser, "proxy.example.com", 5060)

	tests := []struct {
		input             string
		expectedTransport string
		expectedHost      string
		expectedPort      int
	}{
		{"sip:alice@127.0.0.1", "udp", "127.0.0.1", 5060},
		{"sip:alice@127.0.0.1:5080", "udp", "127.0.0.1", 5080},
		{"sip:alice@127.0.0.1;transport=tcp", "tcp", "127.0.0.1", 5060},
		{"sips:alice@127.0.0.1", "tcp", "127.0.0.1", 5060},
		{"<sip:alice@127.0.0.1:5080;transport=tcp>", "tcp", "127.0.0.1", 5080},
	}

	for _, test := range tests {
		addr, transport, err := engine.parseTargetURI(test.input)
		if err != nil {
			t.Errorf("Unexpected error for input '%s': %v", test.input, err)
			continue
		}

		if transport != test.expectedTransport {
			t.Errorf("For input '%s', expected transport '%s', got '%s'", test.input, test.expectedTransport, transport)
		}

		addrStr := addr.String()
		if !strings.Contains(addrStr, test.expectedHost) {
			t.Errorf("For input '%s', expected host '%s' in address '%s'", test.input, test.expectedHost, addrStr)
		}

		if !strings.Contains(addrStr, strconv.Itoa(test.expectedPort)) {
			t.Errorf("For input '%s', expected port %d in address '%s'", test.input, test.expectedPort, addrStr)
		}
	}
}

func TestCheckMaxForwards(t *testing.T) {
	mockReg := newMockRegistrar()
	mockTM := newMockTransportManager()
	mockTxnMgr := &mockTransactionManager{}
	mockParser := &mockParser{}

	engine := NewRequestForwardingEngine(mockReg, mockTM, mockTxnMgr, mockParser, "proxy.example.com", 5060)

	// Test with valid Max-Forwards
	req := createTestInviteRequest()
	req.SetHeader(parser.HeaderMaxForwards, "10")

	err := engine.checkMaxForwards(req)
	if err != nil {
		t.Errorf("Expected no error for valid Max-Forwards, got: %v", err)
	}

	// Test with Max-Forwards = 0
	req.SetHeader(parser.HeaderMaxForwards, "0")
	err = engine.checkMaxForwards(req)
	if err == nil {
		t.Error("Expected error for Max-Forwards = 0")
	}

	// Test with missing Max-Forwards
	req.RemoveHeader(parser.HeaderMaxForwards)
	err = engine.checkMaxForwards(req)
	if err != nil {
		t.Errorf("Expected no error for missing Max-Forwards, got: %v", err)
	}

	// Check that Max-Forwards was added
	maxForwards := req.GetHeader(parser.HeaderMaxForwards)
	if maxForwards != "70" {
		t.Errorf("Expected Max-Forwards '70', got '%s'", maxForwards)
	}
}

func TestDecrementMaxForwards(t *testing.T) {
	mockReg := newMockRegistrar()
	mockTM := newMockTransportManager()
	mockTxnMgr := &mockTransactionManager{}
	mockParser := &mockParser{}

	engine := NewRequestForwardingEngine(mockReg, mockTM, mockTxnMgr, mockParser, "proxy.example.com", 5060)

	req := createTestInviteRequest()
	req.SetHeader(parser.HeaderMaxForwards, "10")

	engine.decrementMaxForwards(req)

	maxForwards := req.GetHeader(parser.HeaderMaxForwards)
	if maxForwards != "9" {
		t.Errorf("Expected Max-Forwards '9', got '%s'", maxForwards)
	}
}

func TestCreateViaHeader(t *testing.T) {
	mockReg := newMockRegistrar()
	mockTM := newMockTransportManager()
	mockTxnMgr := &mockTransactionManager{}
	mockParser := &mockParser{}

	engine := NewRequestForwardingEngine(mockReg, mockTM, mockTxnMgr, mockParser, "proxy.example.com", 5060)

	viaHeader := engine.createViaHeader("udp")

	expectedPrefix := "SIP/2.0/UDP proxy.example.com:5060;branch=z9hG4bK-"
	if !strings.HasPrefix(viaHeader, expectedPrefix) {
		t.Errorf("Expected Via header to start with '%s', got '%s'", expectedPrefix, viaHeader)
	}

	// Check that branch parameter is present and starts with z9hG4bK
	if !strings.Contains(viaHeader, "branch=z9hG4bK-") {
		t.Errorf("Expected branch parameter in Via header, got '%s'", viaHeader)
	}
}

func TestParseViaHeader(t *testing.T) {
	mockReg := newMockRegistrar()
	mockTM := newMockTransportManager()
	mockTxnMgr := &mockTransactionManager{}
	mockParser := &mockParser{}

	engine := NewRequestForwardingEngine(mockReg, mockTM, mockTxnMgr, mockParser, "proxy.example.com", 5060)

	tests := []struct {
		input             string
		expectedTransport string
		expectedHost      string
		expectedPort      int
	}{
		{"SIP/2.0/UDP 127.0.0.1:5060;branch=z9hG4bK-test", "udp", "127.0.0.1", 5060},
		{"SIP/2.0/TCP 127.0.0.1:5080;branch=z9hG4bK-test", "tcp", "127.0.0.1", 5080},
		{"SIP/2.0/UDP 127.0.0.1;branch=z9hG4bK-test", "udp", "127.0.0.1", 5060},
	}

	for _, test := range tests {
		addr, transport, err := engine.parseViaHeader(test.input)
		if err != nil {
			t.Errorf("Unexpected error for input '%s': %v", test.input, err)
			continue
		}

		if transport != test.expectedTransport {
			t.Errorf("For input '%s', expected transport '%s', got '%s'", test.input, test.expectedTransport, transport)
		}

		addrStr := addr.String()
		if !strings.Contains(addrStr, test.expectedHost) {
			t.Errorf("For input '%s', expected host '%s' in address '%s'", test.input, test.expectedHost, addrStr)
		}

		if !strings.Contains(addrStr, strconv.Itoa(test.expectedPort)) {
			t.Errorf("For input '%s', expected port %d in address '%s'", test.input, test.expectedPort, addrStr)
		}
	}
}