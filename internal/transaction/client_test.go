package transaction

import (
	"testing"
	"time"

	"github.com/zurustar/xylitol2/internal/parser"
)

func TestClientTransactionINVITE(t *testing.T) {
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

	ct := NewClientTransaction(invite, sendFunc)

	// Should start in Calling state
	if ct.GetState() != StateCalling {
		t.Errorf("Expected state Calling, got %v", ct.GetState())
	}

	// Test 100 Trying response
	trying := parser.NewResponseMessage(100, "Trying")
	trying.SetHeader(parser.HeaderVia, invite.GetHeader(parser.HeaderVia))
	trying.SetHeader(parser.HeaderFrom, invite.GetHeader(parser.HeaderFrom))
	trying.SetHeader(parser.HeaderTo, invite.GetHeader(parser.HeaderTo))
	trying.SetHeader(parser.HeaderCallID, invite.GetHeader(parser.HeaderCallID))
	trying.SetHeader(parser.HeaderCSeq, invite.GetHeader(parser.HeaderCSeq))

	err := ct.ProcessMessage(trying)
	if err != nil {
		t.Errorf("ProcessMessage failed: %v", err)
	}

	// Should transition to Proceeding state
	if ct.GetState() != StateProceeding {
		t.Errorf("Expected state Proceeding, got %v", ct.GetState())
	}

	// Test 200 OK response
	ok := parser.NewResponseMessage(200, "OK")
	ok.SetHeader(parser.HeaderVia, invite.GetHeader(parser.HeaderVia))
	ok.SetHeader(parser.HeaderFrom, invite.GetHeader(parser.HeaderFrom))
	ok.SetHeader(parser.HeaderTo, invite.GetHeader(parser.HeaderTo)+";tag=totag")
	ok.SetHeader(parser.HeaderCallID, invite.GetHeader(parser.HeaderCallID))
	ok.SetHeader(parser.HeaderCSeq, invite.GetHeader(parser.HeaderCSeq))

	err = ct.ProcessMessage(ok)
	if err != nil {
		t.Errorf("ProcessMessage failed: %v", err)
	}

	// Should transition to Terminated state
	if ct.GetState() != StateTerminated {
		t.Errorf("Expected state Terminated, got %v", ct.GetState())
	}
}

func TestClientTransactionINVITEError(t *testing.T) {
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

	ct := NewClientTransaction(invite, sendFunc)

	// Test 486 Busy Here response
	busy := parser.NewResponseMessage(486, "Busy Here")
	busy.SetHeader(parser.HeaderVia, invite.GetHeader(parser.HeaderVia))
	busy.SetHeader(parser.HeaderFrom, invite.GetHeader(parser.HeaderFrom))
	busy.SetHeader(parser.HeaderTo, invite.GetHeader(parser.HeaderTo)+";tag=totag")
	busy.SetHeader(parser.HeaderCallID, invite.GetHeader(parser.HeaderCallID))
	busy.SetHeader(parser.HeaderCSeq, invite.GetHeader(parser.HeaderCSeq))

	err := ct.ProcessMessage(busy)
	if err != nil {
		t.Errorf("ProcessMessage failed: %v", err)
	}

	// Should transition to Completed state
	if ct.GetState() != StateCompleted {
		t.Errorf("Expected state Completed, got %v", ct.GetState())
	}

	// Should have sent ACK
	if len(sentMessages) == 0 {
		t.Error("Expected ACK to be sent")
	} else {
		ack := sentMessages[len(sentMessages)-1]
		if ack.GetMethod() != parser.MethodACK {
			t.Errorf("Expected ACK, got %v", ack.GetMethod())
		}
	}
}

func TestClientTransactionNonINVITE(t *testing.T) {
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

	ct := NewClientTransaction(register, sendFunc)

	// Should start in Trying state
	if ct.GetState() != StateTrying {
		t.Errorf("Expected state Trying, got %v", ct.GetState())
	}

	// Test 200 OK response
	ok := parser.NewResponseMessage(200, "OK")
	ok.SetHeader(parser.HeaderVia, register.GetHeader(parser.HeaderVia))
	ok.SetHeader(parser.HeaderFrom, register.GetHeader(parser.HeaderFrom))
	ok.SetHeader(parser.HeaderTo, register.GetHeader(parser.HeaderTo))
	ok.SetHeader(parser.HeaderCallID, register.GetHeader(parser.HeaderCallID))
	ok.SetHeader(parser.HeaderCSeq, register.GetHeader(parser.HeaderCSeq))

	err := ct.ProcessMessage(ok)
	if err != nil {
		t.Errorf("ProcessMessage failed: %v", err)
	}

	// Should transition to Completed state
	if ct.GetState() != StateCompleted {
		t.Errorf("Expected state Completed, got %v", ct.GetState())
	}
}

func TestClientTransactionRetransmission(t *testing.T) {
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

	ct := NewClientTransaction(invite, sendFunc)

	// Should start in Calling state
	if ct.GetState() != StateCalling {
		t.Errorf("Expected state Calling, got %v", ct.GetState())
	}

	// Wait for Timer A to fire (should retransmit)
	time.Sleep(TimerT1 + 50*time.Millisecond)

	// Should have retransmitted the request
	if len(sentMessages) == 0 {
		t.Error("Expected request retransmission")
	}
}

func TestClientTransactionTimeout(t *testing.T) {
	sendFunc := func(msg *parser.SIPMessage) error {
		return nil
	}

	// Create non-INVITE request
	register := createTestMessage(parser.MethodREGISTER, map[string]string{
		parser.HeaderVia:    "SIP/2.0/UDP 192.168.1.1:5060;branch=z9hG4bKtest456",
		parser.HeaderCallID: "test-call-id-2",
	})

	ct := NewClientTransaction(register, sendFunc)

	// Simulate timeout by setting a very short Timer F
	ct.SetTimer(TimerF, 10*time.Millisecond, func() {
		if ct.GetState() == StateTrying || ct.GetState() == StateProceeding {
			ct.setState(StateTerminated)
		}
	})

	// Wait for timeout
	time.Sleep(20 * time.Millisecond)

	// Should be terminated
	if ct.GetState() != StateTerminated {
		t.Errorf("Expected state Terminated, got %v", ct.GetState())
	}
}

func TestClientTransactionSendResponse(t *testing.T) {
	sendFunc := func(msg *parser.SIPMessage) error {
		return nil
	}

	invite := createTestMessage(parser.MethodINVITE, map[string]string{
		parser.HeaderVia:    "SIP/2.0/UDP 192.168.1.1:5060;branch=z9hG4bKtest123",
		parser.HeaderCallID: "test-call-id",
	})

	ct := NewClientTransaction(invite, sendFunc)

	// Should not be able to send response from client transaction
	response := parser.NewResponseMessage(200, "OK")
	err := ct.SendResponse(response)
	if err == nil {
		t.Error("Expected error when sending response from client transaction")
	}
}

func TestClientTransactionProcessRequest(t *testing.T) {
	sendFunc := func(msg *parser.SIPMessage) error {
		return nil
	}

	invite := createTestMessage(parser.MethodINVITE, map[string]string{
		parser.HeaderVia:    "SIP/2.0/UDP 192.168.1.1:5060;branch=z9hG4bKtest123",
		parser.HeaderCallID: "test-call-id",
	})

	ct := NewClientTransaction(invite, sendFunc)

	// Should not be able to process request in client transaction
	request := createTestMessage(parser.MethodBYE, nil)
	err := ct.ProcessMessage(request)
	if err == nil {
		t.Error("Expected error when processing request in client transaction")
	}
}

func TestCreateACK(t *testing.T) {
	sendFunc := func(msg *parser.SIPMessage) error {
		return nil
	}

	invite := createTestMessage(parser.MethodINVITE, map[string]string{
		parser.HeaderVia:    "SIP/2.0/UDP 192.168.1.1:5060;branch=z9hG4bKtest123",
		parser.HeaderCallID: "test-call-id",
		parser.HeaderCSeq:   "123 INVITE",
	})

	ct := NewClientTransaction(invite, sendFunc)

	// Create response with To tag
	response := parser.NewResponseMessage(486, "Busy Here")
	response.SetHeader(parser.HeaderTo, "Test <sip:test@example.com>;tag=totag")

	ack := ct.createACK(response)
	if ack == nil {
		t.Fatal("createACK returned nil")
	}

	if ack.GetMethod() != parser.MethodACK {
		t.Errorf("Expected ACK method, got %v", ack.GetMethod())
	}

	if ack.GetHeader(parser.HeaderCSeq) != "123 ACK" {
		t.Errorf("Expected CSeq '123 ACK', got %v", ack.GetHeader(parser.HeaderCSeq))
	}

	if ack.GetHeader(parser.HeaderTo) != "Test <sip:test@example.com>;tag=totag" {
		t.Errorf("Expected To header with tag, got %v", ack.GetHeader(parser.HeaderTo))
	}
}