package transaction

import (
	"testing"
	"time"

	"github.com/zurustar/xylitol2/internal/parser"
)

func TestTransactionID(t *testing.T) {
	tests := []struct {
		name     string
		message  *parser.SIPMessage
		expected string
	}{
		{
			name: "INVITE with RFC3261 branch",
			message: createTestMessage(parser.MethodINVITE, map[string]string{
				parser.HeaderVia:    "SIP/2.0/UDP 192.168.1.1:5060;branch=z9hG4bKtest123",
				parser.HeaderCallID: "test-call-id",
			}),
			expected: "z9hG4bKtest123-INVITE-test-call-id",
		},
		{
			name: "ACK with RFC3261 branch",
			message: createTestMessage(parser.MethodACK, map[string]string{
				parser.HeaderVia:    "SIP/2.0/UDP 192.168.1.1:5060;branch=z9hG4bKtest123",
				parser.HeaderCallID: "test-call-id",
			}),
			expected: "z9hG4bKtest123-INVITE-test-call-id",
		},
		{
			name: "CANCEL with RFC3261 branch",
			message: createTestMessage(parser.MethodCANCEL, map[string]string{
				parser.HeaderVia:    "SIP/2.0/UDP 192.168.1.1:5060;branch=z9hG4bKtest123",
				parser.HeaderCallID: "test-call-id",
			}),
			expected: "z9hG4bKtest123-INVITE-test-call-id",
		},
		{
			name: "REGISTER with RFC3261 branch",
			message: createTestMessage(parser.MethodREGISTER, map[string]string{
				parser.HeaderVia:    "SIP/2.0/UDP 192.168.1.1:5060;branch=z9hG4bKtest123",
				parser.HeaderCallID: "test-call-id",
			}),
			expected: "z9hG4bKtest123-REGISTER-test-call-id",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			id := generateTransactionID(tt.message)
			if id != tt.expected {
				t.Errorf("generateTransactionID() = %v, want %v", id, tt.expected)
			}
		})
	}
}

func TestExtractBranch(t *testing.T) {
	tests := []struct {
		name     string
		via      string
		expected string
	}{
		{
			name:     "Valid branch parameter",
			via:      "SIP/2.0/UDP 192.168.1.1:5060;branch=z9hG4bKtest123",
			expected: "z9hG4bKtest123",
		},
		{
			name:     "Branch with additional parameters",
			via:      "SIP/2.0/UDP 192.168.1.1:5060;branch=z9hG4bKtest123;rport",
			expected: "z9hG4bKtest123",
		},
		{
			name:     "No branch parameter",
			via:      "SIP/2.0/UDP 192.168.1.1:5060;rport",
			expected: "",
		},
		{
			name:     "Empty via header",
			via:      "",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			branch := extractBranch(tt.via)
			if branch != tt.expected {
				t.Errorf("extractBranch() = %v, want %v", branch, tt.expected)
			}
		})
	}
}

func TestExtractTag(t *testing.T) {
	tests := []struct {
		name     string
		header   string
		expected string
	}{
		{
			name:     "Valid tag parameter",
			header:   "Alice <sip:alice@example.com>;tag=abc123",
			expected: "abc123",
		},
		{
			name:     "Tag with additional parameters",
			header:   "Alice <sip:alice@example.com>;tag=abc123;expires=3600",
			expected: "abc123",
		},
		{
			name:     "No tag parameter",
			header:   "Alice <sip:alice@example.com>",
			expected: "",
		},
		{
			name:     "Empty header",
			header:   "",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tag := extractTag(tt.header)
			if tag != tt.expected {
				t.Errorf("extractTag() = %v, want %v", tag, tt.expected)
			}
		})
	}
}

func TestExtractCSeq(t *testing.T) {
	tests := []struct {
		name     string
		cseq     string
		expected uint32
	}{
		{
			name:     "Valid CSeq",
			cseq:     "123 INVITE",
			expected: 123,
		},
		{
			name:     "Large sequence number",
			cseq:     "4294967295 REGISTER",
			expected: 4294967295,
		},
		{
			name:     "Invalid CSeq",
			cseq:     "INVITE",
			expected: 0,
		},
		{
			name:     "Empty CSeq",
			cseq:     "",
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			seq := extractCSeq(tt.cseq)
			if seq != tt.expected {
				t.Errorf("extractCSeq() = %v, want %v", seq, tt.expected)
			}
		})
	}
}

func TestBaseTransactionTimers(t *testing.T) {
	msg := createTestMessage(parser.MethodINVITE, map[string]string{
		parser.HeaderVia:    "SIP/2.0/UDP 192.168.1.1:5060;branch=z9hG4bKtest123",
		parser.HeaderCallID: "test-call-id",
	})

	bt := NewBaseTransaction(msg, true)

	// Test timer setting and cancellation
	timerFired := false
	bt.SetTimer(TimerA, 10*time.Millisecond, func() {
		timerFired = true
	})

	// Wait for timer to fire
	time.Sleep(20 * time.Millisecond)
	if !timerFired {
		t.Error("Timer should have fired")
	}

	// Test timer cancellation
	timerFired = false
	bt.SetTimer(TimerB, 10*time.Millisecond, func() {
		timerFired = true
	})

	bt.CancelTimer(TimerB)
	time.Sleep(20 * time.Millisecond)
	if timerFired {
		t.Error("Timer should have been cancelled")
	}

	// Test cancel all timers
	timer1Fired := false
	timer2Fired := false
	bt.SetTimer(TimerA, 10*time.Millisecond, func() {
		timer1Fired = true
	})
	bt.SetTimer(TimerB, 10*time.Millisecond, func() {
		timer2Fired = true
	})

	bt.CancelAllTimers()
	time.Sleep(20 * time.Millisecond)
	if timer1Fired || timer2Fired {
		t.Error("All timers should have been cancelled")
	}
}

func TestBaseTransactionExpiration(t *testing.T) {
	msg := createTestMessage(parser.MethodINVITE, map[string]string{
		parser.HeaderVia:    "SIP/2.0/UDP 192.168.1.1:5060;branch=z9hG4bKtest123",
		parser.HeaderCallID: "test-call-id",
	})

	bt := NewBaseTransaction(msg, true)

	// Should not be expired immediately
	if bt.IsExpired() {
		t.Error("Transaction should not be expired immediately")
	}

	// Test with old creation time
	bt.created = time.Now().Add(-35 * time.Second)
	if !bt.IsExpired() {
		t.Error("INVITE transaction should be expired after 35 seconds")
	}

	// Test non-INVITE transaction
	msg2 := createTestMessage(parser.MethodREGISTER, map[string]string{
		parser.HeaderVia:    "SIP/2.0/UDP 192.168.1.1:5060;branch=z9hG4bKtest456",
		parser.HeaderCallID: "test-call-id-2",
	})

	bt2 := NewBaseTransaction(msg2, true)
	bt2.created = time.Now().Add(-65 * TimerT1)
	if !bt2.IsExpired() {
		t.Error("Non-INVITE transaction should be expired after 64*T1")
	}
}

func TestTransactionStateString(t *testing.T) {
	tests := []struct {
		state    TransactionState
		expected string
	}{
		{StateTrying, "Trying"},
		{StateCalling, "Calling"},
		{StateProceeding, "Proceeding"},
		{StateCompleted, "Completed"},
		{StateConfirmed, "Confirmed"},
		{StateTerminated, "Terminated"},
		{TransactionState(999), "Unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			if tt.state.String() != tt.expected {
				t.Errorf("TransactionState.String() = %v, want %v", tt.state.String(), tt.expected)
			}
		})
	}
}

// Helper function to create test messages
func createTestMessage(method string, headers map[string]string) *parser.SIPMessage {
	msg := parser.NewRequestMessage(method, "sip:test@example.com")
	
	for name, value := range headers {
		msg.SetHeader(name, value)
	}
	
	// Set required headers if not provided
	if !msg.HasHeader(parser.HeaderVia) {
		msg.SetHeader(parser.HeaderVia, "SIP/2.0/UDP 192.168.1.1:5060;branch=z9hG4bKdefault")
	}
	if !msg.HasHeader(parser.HeaderFrom) {
		msg.SetHeader(parser.HeaderFrom, "Test <sip:test@example.com>;tag=fromtag")
	}
	if !msg.HasHeader(parser.HeaderTo) {
		msg.SetHeader(parser.HeaderTo, "Test <sip:test@example.com>")
	}
	if !msg.HasHeader(parser.HeaderCallID) {
		msg.SetHeader(parser.HeaderCallID, "default-call-id")
	}
	if !msg.HasHeader(parser.HeaderCSeq) {
		msg.SetHeader(parser.HeaderCSeq, "1 "+method)
	}
	if !msg.HasHeader(parser.HeaderMaxForwards) {
		msg.SetHeader(parser.HeaderMaxForwards, "70")
	}
	if !msg.HasHeader(parser.HeaderContentLength) {
		msg.SetHeader(parser.HeaderContentLength, "0")
	}
	
	msg.Transport = "UDP"
	return msg
}