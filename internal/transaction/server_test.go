package transaction

import (
	"testing"
	"time"

	"github.com/zurustar/xylitol2/internal/parser"
)

func TestServerTransactionINVITE(t *testing.T) {
	sentMessages := []*parser.SIPMessage{}
	sendFunc := func(msg *parser.SIPMessage) error {
		sentMessages = append(sentMessages, msg)
		return nil
	}

	// Create INVITE request
	invite := createTestMessage(parser.MethodINVITE, map[string]string{
		parser.HeaderVia:    "SIP/2.0/UDP 192.168.1.1:5060;branch=z9hG4bKtest123",
		parser.HeaderCallID: "test-call-id",
	})

	st := NewServerTransaction(invite, sendFunc)

	// Should start in Proceeding state for INVITE
	if st.GetState() != StateProceeding {
		t.Errorf("Expected state Proceeding, got %v", st.GetState())
	}

	// Send 100 Trying response
	trying := parser.NewResponseMessage(100, "Trying")
	err := st.SendResponse(trying)
	if err != nil {
		t.Errorf("SendResponse failed: %v", err)
	}

	// Should stay in Proceeding state
	if st.GetState() != StateProceeding {
		t.Errorf("Expected state Proceeding, got %v", st.GetState())
	}

	// Send 200 OK response
	ok := parser.NewResponseMessage(200, "OK")
	err = st.SendResponse(ok)
	if err != nil {
		t.Errorf("SendResponse failed: %v", err)
	}

	// Should transition to Terminated state for 2xx
	if st.GetState() != StateTerminated {
		t.Errorf("Expected state Terminated, got %v", st.GetState())
	}

	// Should have sent both responses
	if len(sentMessages) != 2 {
		t.Errorf("Expected 2 sent messages, got %d", len(sentMessages))
	}
}

func TestServerTransactionINVITEError(t *testing.T) {
	sentMessages := []*parser.SIPMessage{}
	sendFunc := func(msg *parser.SIPMessage) error {
		sentMessages = append(sentMessages, msg)
		return nil
	}

	// Create INVITE request
	invite := createTestMessage(parser.MethodINVITE, map[string]string{
		parser.HeaderVia:    "SIP/2.0/UDP 192.168.1.1:5060;branch=z9hG4bKtest123",
		parser.HeaderCallID: "test-call-id",
	})

	st := NewServerTransaction(invite, sendFunc)

	// Send 486 Busy Here response
	busy := parser.NewResponseMessage(486, "Busy Here")
	err := st.SendResponse(busy)
	if err != nil {
		t.Errorf("SendResponse failed: %v", err)
	}

	// Should transition to Completed state
	if st.GetState() != StateCompleted {
		t.Errorf("Expected state Completed, got %v", st.GetState())
	}

	// Test ACK handling
	ack := createTestMessage(parser.MethodACK, map[string]string{
		parser.HeaderVia:    invite.GetHeader(parser.HeaderVia),
		parser.HeaderCallID: invite.GetHeader(parser.HeaderCallID),
	})

	err = st.HandleACK(ack)
	if err != nil {
		t.Errorf("HandleACK failed: %v", err)
	}

	// Should transition to Confirmed state
	if st.GetState() != StateConfirmed {
		t.Errorf("Expected state Confirmed, got %v", st.GetState())
	}
}

func TestServerTransactionNonINVITE(t *testing.T) {
	sentMessages := []*parser.SIPMessage{}
	sendFunc := func(msg *parser.SIPMessage) error {
		sentMessages = append(sentMessages, msg)
		return nil
	}

	// Create REGISTER request
	register := createTestMessage(parser.MethodREGISTER, map[string]string{
		parser.HeaderVia:    "SIP/2.0/UDP 192.168.1.1:5060;branch=z9hG4bKtest456",
		parser.HeaderCallID: "test-call-id-2",
	})

	st := NewServerTransaction(register, sendFunc)

	// Should start in Trying state for non-INVITE
	if st.GetState() != StateTrying {
		t.Errorf("Expected state Trying, got %v", st.GetState())
	}

	// Send 100 Trying response
	trying := parser.NewResponseMessage(100, "Trying")
	err := st.SendResponse(trying)
	if err != nil {
		t.Errorf("SendResponse failed: %v", err)
	}

	// Should transition to Proceeding state
	if st.GetState() != StateProceeding {
		t.Errorf("Expected state Proceeding, got %v", st.GetState())
	}

	// Send 200 OK response
	ok := parser.NewResponseMessage(200, "OK")
	err = st.SendResponse(ok)
	if err != nil {
		t.Errorf("SendResponse failed: %v", err)
	}

	// Should transition to Completed state
	if st.GetState() != StateCompleted {
		t.Errorf("Expected state Completed, got %v", st.GetState())
	}
}

func TestServerTransactionRetransmission(t *testing.T) {
	sentMessages := []*parser.SIPMessage{}
	sendFunc := func(msg *parser.SIPMessage) error {
		sentMessages = append(sentMessages, msg)
		return nil
	}

	// Create INVITE request
	invite := createTestMessage(parser.MethodINVITE, map[string]string{
		parser.HeaderVia:    "SIP/2.0/UDP 192.168.1.1:5060;branch=z9hG4bKtest123",
		parser.HeaderCallID: "test-call-id",
	})

	st := NewServerTransaction(invite, sendFunc)

	// Send 100 Trying response
	trying := parser.NewResponseMessage(100, "Trying")
	err := st.SendResponse(trying)
	if err != nil {
		t.Errorf("SendResponse failed: %v", err)
	}

	initialCount := len(sentMessages)

	// Process retransmitted request
	err = st.ProcessMessage(invite)
	if err != nil {
		t.Errorf("ProcessMessage failed: %v", err)
	}

	// Should have retransmitted the response
	if len(sentMessages) <= initialCount {
		t.Error("Expected response retransmission")
	}
}

func TestServerTransactionTimers(t *testing.T) {
	sendFunc := func(msg *parser.SIPMessage) error {
		return nil
	}

	// Create INVITE request
	invite := createTestMessage(parser.MethodINVITE, map[string]string{
		parser.HeaderVia:    "SIP/2.0/UDP 192.168.1.1:5060;branch=z9hG4bKtest123",
		parser.HeaderCallID: "test-call-id",
	})

	st := NewServerTransaction(invite, sendFunc)

	// Send error response to start Timer H
	busy := parser.NewResponseMessage(486, "Busy Here")
	err := st.SendResponse(busy)
	if err != nil {
		t.Errorf("SendResponse failed: %v", err)
	}

	// Should be in Completed state
	if st.GetState() != StateCompleted {
		t.Errorf("Expected state Completed, got %v", st.GetState())
	}

	// Simulate Timer H timeout
	st.SetTimer(TimerH, 10*time.Millisecond, func() {
		if st.GetState() == StateCompleted {
			st.setState(StateTerminated)
		}
	})

	// Wait for timeout
	time.Sleep(20 * time.Millisecond)

	// Should be terminated
	if st.GetState() != StateTerminated {
		t.Errorf("Expected state Terminated, got %v", st.GetState())
	}
}

func TestServerTransactionSendRequest(t *testing.T) {
	sendFunc := func(msg *parser.SIPMessage) error {
		return nil
	}

	invite := createTestMessage(parser.MethodINVITE, map[string]string{
		parser.HeaderVia:    "SIP/2.0/UDP 192.168.1.1:5060;branch=z9hG4bKtest123",
		parser.HeaderCallID: "test-call-id",
	})

	st := NewServerTransaction(invite, sendFunc)

	// Should not be able to send request as response
	request := createTestMessage(parser.MethodBYE, nil)
	err := st.SendResponse(request)
	if err == nil {
		t.Error("Expected error when sending request as response")
	}
}

func TestServerTransactionProcessResponse(t *testing.T) {
	sendFunc := func(msg *parser.SIPMessage) error {
		return nil
	}

	invite := createTestMessage(parser.MethodINVITE, map[string]string{
		parser.HeaderVia:    "SIP/2.0/UDP 192.168.1.1:5060;branch=z9hG4bKtest123",
		parser.HeaderCallID: "test-call-id",
	})

	st := NewServerTransaction(invite, sendFunc)

	// Should not be able to process response in server transaction
	response := parser.NewResponseMessage(200, "OK")
	err := st.ProcessMessage(response)
	if err == nil {
		t.Error("Expected error when processing response in server transaction")
	}
}

func TestServerTransactionACKNonINVITE(t *testing.T) {
	sendFunc := func(msg *parser.SIPMessage) error {
		return nil
	}

	register := createTestMessage(parser.MethodREGISTER, map[string]string{
		parser.HeaderVia:    "SIP/2.0/UDP 192.168.1.1:5060;branch=z9hG4bKtest456",
		parser.HeaderCallID: "test-call-id-2",
	})

	st := NewServerTransaction(register, sendFunc)

	// Should not be able to handle ACK for non-INVITE transaction
	ack := createTestMessage(parser.MethodACK, nil)
	err := st.HandleACK(ack)
	if err == nil {
		t.Error("Expected error when handling ACK for non-INVITE transaction")
	}
}

func TestServerTransactionTimerTransitions(t *testing.T) {
	sendFunc := func(msg *parser.SIPMessage) error {
		return nil
	}

	// Test Timer I transition
	invite := createTestMessage(parser.MethodINVITE, map[string]string{
		parser.HeaderVia:    "SIP/2.0/UDP 192.168.1.1:5060;branch=z9hG4bKtest123",
		parser.HeaderCallID: "test-call-id",
	})

	st := NewServerTransaction(invite, sendFunc)
	st.setState(StateConfirmed)

	// Start Timer I with short duration
	st.SetTimer(TimerI, 10*time.Millisecond, func() {
		st.setState(StateTerminated)
	})

	// Wait for timer
	time.Sleep(20 * time.Millisecond)

	if st.GetState() != StateTerminated {
		t.Errorf("Expected state Terminated after Timer I, got %v", st.GetState())
	}

	// Test Timer J transition for non-INVITE
	register := createTestMessage(parser.MethodREGISTER, map[string]string{
		parser.HeaderVia:    "SIP/2.0/UDP 192.168.1.1:5060;branch=z9hG4bKtest456",
		parser.HeaderCallID: "test-call-id-2",
	})

	st2 := NewServerTransaction(register, sendFunc)
	st2.setState(StateCompleted)

	// Start Timer J with short duration
	st2.SetTimer(TimerJ, 10*time.Millisecond, func() {
		st2.setState(StateTerminated)
	})

	// Wait for timer
	time.Sleep(20 * time.Millisecond)

	if st2.GetState() != StateTerminated {
		t.Errorf("Expected state Terminated after Timer J, got %v", st2.GetState())
	}
}