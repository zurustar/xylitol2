package proxy

import (
	"strings"
	"testing"
	"time"

	"github.com/zurustar/xylitol2/internal/parser"
	"github.com/zurustar/xylitol2/internal/transaction"
)

// Test helper functions for stateful proxy

func createTestStatefulEngine() *StatefulProxyEngine {
	mockReg := newMockRegistrar()
	mockTM := newMockTransportManager()
	mockTxnMgr := &mockTransactionManager{}
	mockParser := &mockParser{}

	forwardingEngine := NewRequestForwardingEngine(mockReg, mockTM, mockTxnMgr, mockParser, "proxy.example.com", 5060)
	return NewStatefulProxyEngine(forwardingEngine)
}

func createTestInviteWithCallID(callID string) *parser.SIPMessage {
	req := parser.NewRequestMessage(parser.MethodINVITE, "sip:alice@example.com")
	req.SetHeader(parser.HeaderVia, "SIP/2.0/UDP client.example.com:5060;branch=z9hG4bK-test")
	req.SetHeader(parser.HeaderFrom, "Bob <sip:bob@example.com>;tag=12345")
	req.SetHeader(parser.HeaderTo, "Alice <sip:alice@example.com>")
	req.SetHeader(parser.HeaderCallID, callID)
	req.SetHeader(parser.HeaderCSeq, "1 INVITE")
	req.SetHeader(parser.HeaderMaxForwards, "70")
	req.SetHeader(parser.HeaderContentLength, "0")
	return req
}

func createTestCancelRequest(callID string) *parser.SIPMessage {
	req := parser.NewRequestMessage(parser.MethodCANCEL, "sip:alice@example.com")
	req.SetHeader(parser.HeaderVia, "SIP/2.0/UDP client.example.com:5060;branch=z9hG4bK-test")
	req.SetHeader(parser.HeaderFrom, "Bob <sip:bob@example.com>;tag=12345")
	req.SetHeader(parser.HeaderTo, "Alice <sip:alice@example.com>")
	req.SetHeader(parser.HeaderCallID, callID)
	req.SetHeader(parser.HeaderCSeq, "1 CANCEL")
	req.SetHeader(parser.HeaderMaxForwards, "70")
	req.SetHeader(parser.HeaderContentLength, "0")
	return req
}

func createTestAckRequest(callID string) *parser.SIPMessage {
	req := parser.NewRequestMessage(parser.MethodACK, "sip:alice@example.com")
	req.SetHeader(parser.HeaderVia, "SIP/2.0/UDP client.example.com:5060;branch=z9hG4bK-test")
	req.SetHeader(parser.HeaderFrom, "Bob <sip:bob@example.com>;tag=12345")
	req.SetHeader(parser.HeaderTo, "Alice <sip:alice@example.com>;tag=67890")
	req.SetHeader(parser.HeaderCallID, callID)
	req.SetHeader(parser.HeaderCSeq, "1 ACK")
	req.SetHeader(parser.HeaderContentLength, "0")
	return req
}

func createTestResponseWithCallID(statusCode int, callID string) *parser.SIPMessage {
	resp := parser.NewResponseMessage(statusCode, parser.GetReasonPhraseForCode(statusCode))
	resp.SetHeader(parser.HeaderVia, "SIP/2.0/UDP 127.0.0.1:5060;branch=z9hG4bK-proxy")
	resp.AddHeader(parser.HeaderVia, "SIP/2.0/UDP 127.0.0.1:5061;branch=z9hG4bK-test")
	resp.SetHeader(parser.HeaderFrom, "Bob <sip:bob@example.com>;tag=12345")
	resp.SetHeader(parser.HeaderTo, "Alice <sip:alice@example.com>;tag=67890")
	resp.SetHeader(parser.HeaderCallID, callID)
	resp.SetHeader(parser.HeaderCSeq, "1 INVITE")
	resp.SetHeader(parser.HeaderContentLength, "0")
	return resp
}

// Tests

func TestNewStatefulProxyEngine(t *testing.T) {
	forwardingEngine := &RequestForwardingEngine{}
	engine := NewStatefulProxyEngine(forwardingEngine)

	if engine == nil {
		t.Fatal("Expected non-nil engine")
	}

	if engine.RequestForwardingEngine != forwardingEngine {
		t.Error("Expected forwarding engine to be set")
	}

	if engine.proxyStates == nil {
		t.Error("Expected proxy states map to be initialized")
	}
}

func TestProcessRequest_INVITE_SingleTarget(t *testing.T) {
	engine := createTestStatefulEngine()
	mockTxn := &mockTransaction{}

	// Add a registered contact
	engine.registrar.(*mockRegistrar).addContact("sip:alice@example.com", "sip:alice@127.0.0.1:5060")

	req := createTestInviteWithCallID("test-call-id-1")

	err := engine.ProcessRequest(req, mockTxn)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	// Check that proxy state was created
	if engine.GetProxyStateCount() != 1 {
		t.Errorf("Expected 1 proxy state, got %d", engine.GetProxyStateCount())
	}

	// Check that message was sent to target
	transportMgr := engine.transportManager.(*mockTransportManager)
	if len(transportMgr.sentMessages) == 0 {
		t.Fatal("Expected message to be sent to target")
	}
}

func TestProcessRequest_INVITE_MultipleTargets(t *testing.T) {
	engine := createTestStatefulEngine()
	mockTxn := &mockTransaction{}

	// Add multiple registered contacts (forking scenario)
	mockReg := engine.registrar.(*mockRegistrar)
	mockReg.addContact("sip:alice@example.com", "sip:alice@127.0.0.1:5060")
	mockReg.addContact("sip:alice@example.com", "sip:alice@127.0.0.1:5061")
	mockReg.addContact("sip:alice@example.com", "sip:alice@127.0.0.1:5062")

	req := createTestInviteWithCallID("test-call-id-2")

	err := engine.ProcessRequest(req, mockTxn)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	// Check that proxy state was created
	if engine.GetProxyStateCount() != 1 {
		t.Errorf("Expected 1 proxy state, got %d", engine.GetProxyStateCount())
	}

	// Check that messages were sent to all targets
	transportMgr := engine.transportManager.(*mockTransportManager)
	if len(transportMgr.sentMessages) != 3 {
		t.Errorf("Expected 3 messages to be sent (forking), got %d", len(transportMgr.sentMessages))
	}

	// Check proxy state has correct number of client transactions
	stateID := engine.generateProxyStateID(req)
	engine.mutex.RLock()
	proxyState := engine.proxyStates[stateID]
	engine.mutex.RUnlock()

	if proxyState == nil {
		t.Fatal("Expected proxy state to exist")
	}

	if len(proxyState.ClientTransactions) != 3 {
		t.Errorf("Expected 3 client transactions, got %d", len(proxyState.ClientTransactions))
	}
}

func TestProcessRequest_INVITE_NoTargets(t *testing.T) {
	engine := createTestStatefulEngine()
	mockTxn := &mockTransaction{}

	req := createTestInviteWithCallID("test-call-id-3")

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

	// Check that no proxy state was created
	if engine.GetProxyStateCount() != 0 {
		t.Errorf("Expected 0 proxy states, got %d", engine.GetProxyStateCount())
	}
}

func TestProcessRequest_CANCEL(t *testing.T) {
	engine := createTestStatefulEngine()
	mockTxn := &mockTransaction{}

	// First, create an INVITE transaction
	engine.registrar.(*mockRegistrar).addContact("sip:alice@example.com", "sip:alice@127.0.0.1:5060")
	inviteReq := createTestInviteWithCallID("test-call-id-4")
	engine.ProcessRequest(inviteReq, &mockTransaction{})

	// Now send CANCEL - need to use same Call-ID and CSeq number but CANCEL method
	cancelReq := createTestCancelRequest("test-call-id-4")
	// Fix the CSeq to match INVITE for proper transaction matching
	cancelReq.SetHeader(parser.HeaderCSeq, "1 CANCEL")
	
	err := engine.ProcessRequest(cancelReq, mockTxn)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	// Check that 200 OK was sent for CANCEL
	resp := mockTxn.getLastResponse()
	if resp == nil {
		t.Fatal("Expected response to CANCEL")
	}

	if resp.GetStatusCode() != parser.StatusOK {
		t.Errorf("Expected status code %d for CANCEL, got %d", parser.StatusOK, resp.GetStatusCode())
	}

	// Check that CANCEL was sent to targets
	transportMgr := engine.transportManager.(*mockTransportManager)
	// Should have INVITE + CANCEL messages
	if len(transportMgr.sentMessages) < 2 {
		t.Errorf("Expected at least 2 messages (INVITE + CANCEL), got %d", len(transportMgr.sentMessages))
	}
}

func TestProcessRequest_CANCEL_NoMatchingTransaction(t *testing.T) {
	engine := createTestStatefulEngine()
	mockTxn := &mockTransaction{}

	// Send CANCEL without matching INVITE
	cancelReq := createTestCancelRequest("nonexistent-call-id")
	err := engine.ProcessRequest(cancelReq, mockTxn)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	// Check that 481 response was sent
	resp := mockTxn.getLastResponse()
	if resp == nil {
		t.Fatal("Expected response to CANCEL")
	}

	if resp.GetStatusCode() != parser.StatusCallTransactionDoesNotExist {
		t.Errorf("Expected status code %d, got %d", parser.StatusCallTransactionDoesNotExist, resp.GetStatusCode())
	}
}

func TestProcessRequest_ACK(t *testing.T) {
	engine := createTestStatefulEngine()

	// First, create an INVITE transaction
	engine.registrar.(*mockRegistrar).addContact("sip:alice@example.com", "sip:alice@127.0.0.1:5060")
	inviteReq := createTestInviteWithCallID("test-call-id-5")
	engine.ProcessRequest(inviteReq, &mockTransaction{})

	// Get initial message count
	transportMgr := engine.transportManager.(*mockTransportManager)
	initialMsgCount := len(transportMgr.sentMessages)

	// Simulate a 200 OK response
	stateID := engine.generateProxyStateID(inviteReq)
	engine.mutex.RLock()
	proxyState := engine.proxyStates[stateID]
	engine.mutex.RUnlock()

	if proxyState != nil {
		// Set a client transaction to have received 200 OK
		proxyState.mutex.Lock()
		for _, clientTxn := range proxyState.ClientTransactions {
			clientTxn.Response = createTestResponseWithCallID(parser.StatusOK, "test-call-id-5")
			break
		}
		proxyState.mutex.Unlock()
	}

	// Now send ACK
	ackReq := createTestAckRequest("test-call-id-5")
	err := engine.ProcessRequest(ackReq, &mockTransaction{})
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	// Check that ACK was forwarded to target
	finalMsgCount := len(transportMgr.sentMessages)
	if finalMsgCount <= initialMsgCount {
		t.Errorf("Expected ACK to be sent to target, message count did not increase from %d to %d", initialMsgCount, finalMsgCount)
	}
}

func TestProcessResponse_ProvisionalResponse(t *testing.T) {
	engine := createTestStatefulEngine()
	serverTxn := &mockTransaction{}

	// Create INVITE transaction
	engine.registrar.(*mockRegistrar).addContact("sip:alice@example.com", "sip:alice@127.0.0.1:5060")
	inviteReq := createTestInviteWithCallID("test-call-id-6")
	engine.ProcessRequest(inviteReq, serverTxn)

	// Get the client transaction
	stateID := engine.generateProxyStateID(inviteReq)
	engine.mutex.RLock()
	proxyState := engine.proxyStates[stateID]
	engine.mutex.RUnlock()

	if proxyState == nil {
		t.Fatal("Expected proxy state to exist")
	}

	// Update the server transaction in proxy state to match our mock
	proxyState.mutex.Lock()
	proxyState.ServerTransaction = serverTxn
	proxyState.mutex.Unlock()

	var clientTxn transaction.Transaction
	for _, ct := range proxyState.ClientTransactions {
		clientTxn = ct.Transaction
		break
	}

	// Send 180 Ringing response
	resp := createTestResponseWithCallID(parser.StatusRinging, "test-call-id-6")
	err := engine.ProcessResponse(resp, clientTxn)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	// Check that provisional response was forwarded
	forwardedResp := serverTxn.getLastResponse()
	if forwardedResp == nil {
		t.Fatal("Expected provisional response to be forwarded")
	}

	if forwardedResp.GetStatusCode() != parser.StatusRinging {
		t.Errorf("Expected status code %d, got %d", parser.StatusRinging, forwardedResp.GetStatusCode())
	}

	// Check that final response was not sent yet
	if proxyState.FinalResponseSent {
		t.Error("Expected final response not to be sent yet")
	}
}

func TestProcessResponse_SuccessResponse(t *testing.T) {
	engine := createTestStatefulEngine()
	serverTxn := &mockTransaction{}

	// Create INVITE transaction with multiple targets
	mockReg := engine.registrar.(*mockRegistrar)
	mockReg.addContact("sip:alice@example.com", "sip:alice@127.0.0.1:5060")
	mockReg.addContact("sip:alice@example.com", "sip:alice@127.0.0.1:5061")

	inviteReq := createTestInviteWithCallID("test-call-id-7")
	engine.ProcessRequest(inviteReq, serverTxn)

	// Get the proxy state and client transactions
	stateID := engine.generateProxyStateID(inviteReq)
	engine.mutex.RLock()
	proxyState := engine.proxyStates[stateID]
	engine.mutex.RUnlock()

	if proxyState == nil {
		t.Fatal("Expected proxy state to exist")
	}

	// Update the server transaction in proxy state to match our mock
	proxyState.mutex.Lock()
	proxyState.ServerTransaction = serverTxn
	proxyState.mutex.Unlock()

	var firstClientTxn transaction.Transaction
	for _, ct := range proxyState.ClientTransactions {
		firstClientTxn = ct.Transaction
		break
	}

	// Send 200 OK response from first target
	resp := createTestResponseWithCallID(parser.StatusOK, "test-call-id-7")
	err := engine.ProcessResponse(resp, firstClientTxn)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	// Check that success response was forwarded
	forwardedResp := serverTxn.getLastResponse()
	if forwardedResp == nil {
		t.Fatal("Expected success response to be forwarded")
	}

	if forwardedResp.GetStatusCode() != parser.StatusOK {
		t.Errorf("Expected status code %d, got %d", parser.StatusOK, forwardedResp.GetStatusCode())
	}

	// Check that final response was sent
	if !proxyState.FinalResponseSent {
		t.Error("Expected final response to be sent")
	}

	// Check that CANCEL was sent to other targets
	transportMgr := engine.transportManager.(*mockTransportManager)
	// Should have 2 INVITEs + CANCELs for other targets
	if len(transportMgr.sentMessages) < 3 {
		t.Errorf("Expected at least 3 messages (2 INVITEs + CANCEL), got %d", len(transportMgr.sentMessages))
	}
}

func TestProcessResponse_ErrorResponse_BestResponseSelection(t *testing.T) {
	engine := createTestStatefulEngine()
	serverTxn := &mockTransaction{}

	// Create INVITE transaction with multiple targets
	mockReg := engine.registrar.(*mockRegistrar)
	mockReg.addContact("sip:alice@example.com", "sip:alice@127.0.0.1:5060")
	mockReg.addContact("sip:alice@example.com", "sip:alice@127.0.0.1:5061")

	inviteReq := createTestInviteWithCallID("test-call-id-8")
	engine.ProcessRequest(inviteReq, serverTxn)

	// Get the proxy state and client transactions
	stateID := engine.generateProxyStateID(inviteReq)
	engine.mutex.RLock()
	proxyState := engine.proxyStates[stateID]
	engine.mutex.RUnlock()

	if proxyState == nil {
		t.Fatal("Expected proxy state to exist")
	}

	// Update the server transaction in proxy state to match our mock
	proxyState.mutex.Lock()
	proxyState.ServerTransaction = serverTxn
	proxyState.mutex.Unlock()

	clientTxns := make([]transaction.Transaction, 0)
	for _, ct := range proxyState.ClientTransactions {
		clientTxns = append(clientTxns, ct.Transaction)
	}

	// Send 486 Busy Here from first target
	resp1 := createTestResponseWithCallID(parser.StatusBusyHere, "test-call-id-8")
	err := engine.ProcessResponse(resp1, clientTxns[0])
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	// Send 404 Not Found from second target (better response)
	resp2 := createTestResponseWithCallID(parser.StatusNotFound, "test-call-id-8")
	err = engine.ProcessResponse(resp2, clientTxns[1])
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	// Check that the better response (404) was forwarded
	forwardedResp := serverTxn.getLastResponse()
	if forwardedResp == nil {
		t.Fatal("Expected error response to be forwarded")
	}

	if forwardedResp.GetStatusCode() != parser.StatusNotFound {
		t.Errorf("Expected best response status code %d, got %d", parser.StatusNotFound, forwardedResp.GetStatusCode())
	}

	// Check that final response was sent
	if !proxyState.FinalResponseSent {
		t.Error("Expected final response to be sent")
	}
}

func TestGenerateProxyStateID(t *testing.T) {
	engine := createTestStatefulEngine()

	req := createTestInviteWithCallID("test-call-id")
	stateID := engine.generateProxyStateID(req)

	expectedID := "test-call-id-1-INVITE"
	if stateID != expectedID {
		t.Errorf("Expected state ID '%s', got '%s'", expectedID, stateID)
	}
}

func TestIsBetterResponse(t *testing.T) {
	engine := createTestStatefulEngine()

	tests := []struct {
		newCode     int
		currentCode int
		expected    bool
		description string
	}{
		{200, 404, true, "2xx is better than 4xx"},
		{404, 200, false, "4xx is not better than 2xx"},
		{404, 486, true, "404 is better than 486 (same class, lower code)"},
		{486, 404, false, "486 is not better than 404"},
		{300, 400, true, "3xx is better than 4xx"},
		{400, 300, false, "4xx is not better than 3xx"},
		{500, 600, true, "5xx is better than 6xx"},
		{600, 500, false, "6xx is not better than 5xx"},
	}

	for _, test := range tests {
		result := engine.isBetterResponse(test.newCode, test.currentCode)
		if result != test.expected {
			t.Errorf("%s: expected %v, got %v", test.description, test.expected, result)
		}
	}
}

func TestCleanupExpiredStates(t *testing.T) {
	engine := createTestStatefulEngine()

	// Create some proxy states
	engine.registrar.(*mockRegistrar).addContact("sip:alice@example.com", "sip:alice@127.0.0.1:5060")

	req1 := createTestInviteWithCallID("call-1")
	req2 := createTestInviteWithCallID("call-2")

	engine.ProcessRequest(req1, &mockTransaction{})
	engine.ProcessRequest(req2, &mockTransaction{})

	if engine.GetProxyStateCount() != 2 {
		t.Errorf("Expected 2 proxy states, got %d", engine.GetProxyStateCount())
	}

	// Manually set one state as old
	stateID1 := engine.generateProxyStateID(req1)
	engine.mutex.Lock()
	if state, exists := engine.proxyStates[stateID1]; exists {
		state.CreatedAt = time.Now().Add(-10 * time.Minute) // 10 minutes ago
	}
	engine.mutex.Unlock()

	// Cleanup expired states
	engine.CleanupExpiredStates()

	// Should have only 1 state remaining
	if engine.GetProxyStateCount() != 1 {
		t.Errorf("Expected 1 proxy state after cleanup, got %d", engine.GetProxyStateCount())
	}

	// The remaining state should be the newer one
	stateID2 := engine.generateProxyStateID(req2)
	engine.mutex.RLock()
	_, exists := engine.proxyStates[stateID2]
	engine.mutex.RUnlock()

	if !exists {
		t.Error("Expected newer proxy state to remain after cleanup")
	}
}

func TestClientTransactionState_String(t *testing.T) {
	tests := []struct {
		state    ClientTransactionState
		expected string
	}{
		{ClientStateTrying, "Trying"},
		{ClientStateProceeding, "Proceeding"},
		{ClientStateCompleted, "Completed"},
		{ClientStateTerminated, "Terminated"},
		{ClientTransactionState(999), "Unknown"},
	}

	for _, test := range tests {
		result := test.state.String()
		if result != test.expected {
			t.Errorf("Expected '%s', got '%s'", test.expected, result)
		}
	}
}

func TestStatefulProcessRequest_UnsupportedMethod(t *testing.T) {
	engine := createTestStatefulEngine()
	mockTxn := &mockTransaction{}

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
}

func TestStatefulProcessRequest_REGISTER(t *testing.T) {
	engine := createTestStatefulEngine()
	mockTxn := &mockTransaction{}

	req := parser.NewRequestMessage(parser.MethodREGISTER, "sip:alice@example.com")

	err := engine.ProcessRequest(req, mockTxn)
	if err == nil {
		t.Fatal("Expected error for REGISTER request")
	}

	if !strings.Contains(err.Error(), "REGISTER requests should be handled by registrar") {
		t.Errorf("Expected specific error message, got: %v", err)
	}
}