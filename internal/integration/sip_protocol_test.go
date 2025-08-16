package integration

import (
	"bytes"
	"net"
	"strings"
	"testing"
	"time"
)

// TestSIPProtocolCompliance tests RFC3261 compliance scenarios
func TestSIPProtocolCompliance(t *testing.T) {
	suite := SetupTestSuite(t)
	defer suite.Cleanup(t)

	t.Run("Malformed_SIP_Message", func(t *testing.T) {
		malformedMsg := `INVALID MESSAGE FORMAT
This is not a valid SIP message
`
		
		response, err := suite.SendUDPMessage(t, malformedMsg)
		if err != nil {
			t.Fatalf("Failed to send malformed message: %v", err)
		}

		// Should get 400 Bad Request for malformed message
		if !bytes.Contains([]byte(response), []byte("400 Bad Request")) {
			t.Errorf("Expected 400 Bad Request for malformed message, got: %s", response)
		}
	})

	t.Run("Missing_Required_Headers", func(t *testing.T) {
		// REGISTER message missing required headers
		incompleteMsg := `REGISTER sip:test.local SIP/2.0
Via: SIP/2.0/UDP 192.168.1.100:5060;branch=z9hG4bK-incomplete
From: <sip:alice@test.local>;tag=test-tag
Content-Length: 0

`
		
		response, err := suite.SendUDPMessage(t, incompleteMsg)
		if err != nil {
			t.Fatalf("Failed to send incomplete message: %v", err)
		}

		// Should get 400 Bad Request for missing required headers
		if !bytes.Contains([]byte(response), []byte("400 Bad Request")) {
			t.Errorf("Expected 400 Bad Request for missing headers, got: %s", response)
		}
	})

	t.Run("Unsupported_SIP_Method", func(t *testing.T) {
		unsupportedMsg := `SUBSCRIBE sip:test.local SIP/2.0
Via: SIP/2.0/UDP 192.168.1.100:5060;branch=z9hG4bK-subscribe
From: <sip:alice@test.local>;tag=test-tag
To: <sip:test.local>
Call-ID: test-subscribe-call-id
CSeq: 1 SUBSCRIBE
Content-Length: 0

`
		
		response, err := suite.SendUDPMessage(t, unsupportedMsg)
		if err != nil {
			t.Fatalf("Failed to send SUBSCRIBE message: %v", err)
		}

		// Should get 405 Method Not Allowed for unsupported method
		if !bytes.Contains([]byte(response), []byte("405 Method Not Allowed")) {
			t.Errorf("Expected 405 Method Not Allowed for SUBSCRIBE, got: %s", response)
		}

		// Should include Allow header
		if !bytes.Contains([]byte(response), []byte("Allow:")) {
			t.Errorf("Expected Allow header in 405 response")
		}
	})

	t.Run("Invalid_SIP_Version", func(t *testing.T) {
		invalidVersionMsg := `OPTIONS sip:test.local SIP/1.0
Via: SIP/1.0/UDP 192.168.1.100:5060;branch=z9hG4bK-invalid-version
From: <sip:test@test.local>;tag=test-tag
To: <sip:test.local>
Call-ID: test-invalid-version-call-id
CSeq: 1 OPTIONS
Content-Length: 0

`
		
		response, err := suite.SendUDPMessage(t, invalidVersionMsg)
		if err != nil {
			t.Fatalf("Failed to send invalid version message: %v", err)
		}

		// Should get 505 Version Not Supported or 400 Bad Request
		if !bytes.Contains([]byte(response), []byte("505 Version Not Supported")) && 
		   !bytes.Contains([]byte(response), []byte("400 Bad Request")) {
			t.Errorf("Expected 505 or 400 response for invalid SIP version, got: %s", response)
		}
	})

	t.Run("Content_Length_Mismatch", func(t *testing.T) {
		mismatchMsg := `INVITE sip:bob@test.local SIP/2.0
Via: SIP/2.0/UDP 192.168.1.100:5060;branch=z9hG4bK-content-mismatch
From: <sip:alice@test.local>;tag=test-tag
To: <sip:bob@test.local>
Call-ID: test-content-mismatch-call-id
CSeq: 1 INVITE
Contact: <sip:alice@192.168.1.100:5060>
Session-Expires: 1800;refresher=uac
Content-Type: application/sdp
Content-Length: 1000

v=0
o=alice 2890844526 2890844526 IN IP4 192.168.1.100
s=-
c=IN IP4 192.168.1.100
t=0 0
m=audio 5004 RTP/AVP 0
a=rtpmap:0 PCMU/8000
`
		
		response, err := suite.SendUDPMessage(t, mismatchMsg)
		if err != nil {
			t.Fatalf("Failed to send content length mismatch message: %v", err)
		}

		// Should get 400 Bad Request for content length mismatch
		if !bytes.Contains([]byte(response), []byte("400 Bad Request")) {
			t.Errorf("Expected 400 Bad Request for content length mismatch, got: %s", response)
		}
	})
}

// TestSIPTransactionHandling tests SIP transaction state management
func TestSIPTransactionHandling(t *testing.T) {
	suite := SetupTestSuite(t)
	defer suite.Cleanup(t)

	t.Run("Duplicate_Request_Handling", func(t *testing.T) {
		// Send the same REGISTER request twice
		registerMsg := suite.CreateREGISTERMessage("alice", "sip:alice@192.168.1.100:5060")
		
		// First request
		response1, err := suite.SendUDPMessage(t, registerMsg)
		if err != nil {
			t.Fatalf("Failed to send first REGISTER: %v", err)
		}

		// Second identical request (should be handled as retransmission)
		response2, err := suite.SendUDPMessage(t, registerMsg)
		if err != nil {
			t.Fatalf("Failed to send second REGISTER: %v", err)
		}

		// Both responses should be identical (retransmission handling)
		if response1 != response2 {
			t.Errorf("Duplicate request responses differ:\nFirst: %s\nSecond: %s", response1, response2)
		}

		// Should get 401 Unauthorized for both
		if !bytes.Contains([]byte(response1), []byte("401 Unauthorized")) {
			t.Errorf("Expected 401 Unauthorized, got: %s", response1)
		}
	})

	t.Run("Transaction_Branch_Handling", func(t *testing.T) {
		// Send requests with different branch parameters
		registerMsg1 := `REGISTER sip:test.local SIP/2.0
Via: SIP/2.0/UDP 192.168.1.100:5060;branch=z9hG4bK-branch1
From: <sip:alice@test.local>;tag=test-tag
To: <sip:alice@test.local>
Call-ID: test-branch-call-id
CSeq: 1 REGISTER
Contact: <sip:alice@192.168.1.100:5060>
Expires: 3600
Content-Length: 0

`

		registerMsg2 := `REGISTER sip:test.local SIP/2.0
Via: SIP/2.0/UDP 192.168.1.100:5060;branch=z9hG4bK-branch2
From: <sip:alice@test.local>;tag=test-tag
To: <sip:alice@test.local>
Call-ID: test-branch-call-id
CSeq: 2 REGISTER
Contact: <sip:alice@192.168.1.100:5060>
Expires: 3600
Content-Length: 0

`
		
		response1, err := suite.SendUDPMessage(t, registerMsg1)
		if err != nil {
			t.Fatalf("Failed to send first branch request: %v", err)
		}

		response2, err := suite.SendUDPMessage(t, registerMsg2)
		if err != nil {
			t.Fatalf("Failed to send second branch request: %v", err)
		}

		// Both should get responses (different transactions)
		if !bytes.Contains([]byte(response1), []byte("SIP/2.0")) {
			t.Errorf("Expected SIP response for first branch, got: %s", response1)
		}
		if !bytes.Contains([]byte(response2), []byte("SIP/2.0")) {
			t.Errorf("Expected SIP response for second branch, got: %s", response2)
		}
	})

	t.Run("CSeq_Validation", func(t *testing.T) {
		// Send request with invalid CSeq
		invalidCSeqMsg := `REGISTER sip:test.local SIP/2.0
Via: SIP/2.0/UDP 192.168.1.100:5060;branch=z9hG4bK-invalid-cseq
From: <sip:alice@test.local>;tag=test-tag
To: <sip:alice@test.local>
Call-ID: test-invalid-cseq-call-id
CSeq: invalid REGISTER
Contact: <sip:alice@192.168.1.100:5060>
Content-Length: 0

`
		
		response, err := suite.SendUDPMessage(t, invalidCSeqMsg)
		if err != nil {
			t.Fatalf("Failed to send invalid CSeq message: %v", err)
		}

		// Should get 400 Bad Request for invalid CSeq
		if !bytes.Contains([]byte(response), []byte("400 Bad Request")) {
			t.Errorf("Expected 400 Bad Request for invalid CSeq, got: %s", response)
		}
	})
}

// TestSIPDialogHandling tests SIP dialog state management
func TestSIPDialogHandling(t *testing.T) {
	suite := SetupTestSuite(t)
	defer suite.Cleanup(t)

	t.Run("INVITE_Dialog_Establishment", func(t *testing.T) {
		inviteMsg := suite.CreateINVITEMessage("alice", "bob", "sip:alice@192.168.1.100:5060")
		
		response, err := suite.SendUDPMessage(t, inviteMsg)
		if err != nil {
			t.Fatalf("Failed to send INVITE: %v", err)
		}

		// Should get some response (likely 404 since bob isn't registered)
		if !bytes.Contains([]byte(response), []byte("SIP/2.0")) {
			t.Errorf("Expected SIP response for INVITE, got: %s", response)
		}

		// Response should have To tag if dialog is established
		if bytes.Contains([]byte(response), []byte("200 OK")) {
			if !strings.Contains(response, "To:") || !strings.Contains(response, "tag=") {
				t.Errorf("Expected To tag in successful INVITE response")
			}
		}
	})

	t.Run("ACK_Handling", func(t *testing.T) {
		// Send ACK message
		ackMsg := `ACK sip:bob@test.local SIP/2.0
Via: SIP/2.0/UDP 192.168.1.100:5060;branch=z9hG4bK-ack-test
From: <sip:alice@test.local>;tag=alice-tag
To: <sip:bob@test.local>;tag=bob-tag
Call-ID: test-ack-call-id
CSeq: 1 ACK
Content-Length: 0

`
		
		// ACK should not generate a response (it's a request within a dialog)
		conn, err := net.Dial("udp", "127.0.0.1:5060")
		if err != nil {
			t.Fatalf("Failed to create UDP connection: %v", err)
		}
		defer conn.Close()

		_, err = conn.Write([]byte(ackMsg))
		if err != nil {
			t.Fatalf("Failed to send ACK: %v", err)
		}

		// Set short timeout since ACK shouldn't generate response
		conn.SetReadDeadline(time.Now().Add(1 * time.Second))
		buffer := make([]byte, 4096)
		n, err := conn.Read(buffer)
		
		// Should timeout (no response expected for ACK)
		if err == nil {
			response := string(buffer[:n])
			t.Errorf("Unexpected response to ACK: %s", response)
		}
	})

	t.Run("BYE_Handling", func(t *testing.T) {
		byeMsg := `BYE sip:bob@test.local SIP/2.0
Via: SIP/2.0/UDP 192.168.1.100:5060;branch=z9hG4bK-bye-test
From: <sip:alice@test.local>;tag=alice-tag
To: <sip:bob@test.local>;tag=bob-tag
Call-ID: test-bye-call-id
CSeq: 2 BYE
Content-Length: 0

`
		
		response, err := suite.SendUDPMessage(t, byeMsg)
		if err != nil {
			t.Fatalf("Failed to send BYE: %v", err)
		}

		// Should get some response for BYE (likely 481 Call/Transaction Does Not Exist)
		if !bytes.Contains([]byte(response), []byte("SIP/2.0")) {
			t.Errorf("Expected SIP response for BYE, got: %s", response)
		}
	})
}

// TestSIPHeaderHandling tests SIP header parsing and validation
func TestSIPHeaderHandling(t *testing.T) {
	suite := SetupTestSuite(t)
	defer suite.Cleanup(t)

	t.Run("Multi_Value_Headers", func(t *testing.T) {
		// Test message with multiple Via headers
		multiViaMsg := `OPTIONS sip:test.local SIP/2.0
Via: SIP/2.0/UDP proxy1.example.com:5060;branch=z9hG4bK-proxy1
Via: SIP/2.0/UDP 192.168.1.100:5060;branch=z9hG4bK-client
From: <sip:test@test.local>;tag=test-tag
To: <sip:test.local>
Call-ID: test-multi-via-call-id
CSeq: 1 OPTIONS
Content-Length: 0

`
		
		response, err := suite.SendUDPMessage(t, multiViaMsg)
		if err != nil {
			t.Fatalf("Failed to send multi-Via message: %v", err)
		}

		// Should handle multiple Via headers correctly
		if !bytes.Contains([]byte(response), []byte("200 OK")) {
			t.Errorf("Expected 200 OK for multi-Via message, got: %s", response)
		}

		// Response should have Via headers in reverse order
		if !strings.Contains(response, "Via:") {
			t.Errorf("Expected Via header in response")
		}
	})

	t.Run("Header_Folding", func(t *testing.T) {
		// Test message with folded headers (continuation lines)
		foldedHeaderMsg := `OPTIONS sip:test.local SIP/2.0
Via: SIP/2.0/UDP 192.168.1.100:5060;branch=z9hG4bK-folded
From: <sip:test@test.local>;tag=test-tag
To: <sip:test.local>
Call-ID: test-folded-call-id
CSeq: 1 OPTIONS
Contact: <sip:test@192.168.1.100:5060>;
 expires=3600;
 q=0.8
Content-Length: 0

`
		
		response, err := suite.SendUDPMessage(t, foldedHeaderMsg)
		if err != nil {
			t.Fatalf("Failed to send folded header message: %v", err)
		}

		// Should handle folded headers correctly
		if !bytes.Contains([]byte(response), []byte("SIP/2.0")) {
			t.Errorf("Expected SIP response for folded headers, got: %s", response)
		}
	})

	t.Run("Compact_Header_Forms", func(t *testing.T) {
		// Test message with compact header forms
		compactMsg := `OPTIONS sip:test.local SIP/2.0
v: SIP/2.0/UDP 192.168.1.100:5060;branch=z9hG4bK-compact
f: <sip:test@test.local>;tag=test-tag
t: <sip:test.local>
i: test-compact-call-id
CSeq: 1 OPTIONS
l: 0

`
		
		response, err := suite.SendUDPMessage(t, compactMsg)
		if err != nil {
			t.Fatalf("Failed to send compact header message: %v", err)
		}

		// Should handle compact headers correctly
		if !bytes.Contains([]byte(response), []byte("SIP/2.0")) {
			t.Errorf("Expected SIP response for compact headers, got: %s", response)
		}
	})

	t.Run("Case_Insensitive_Headers", func(t *testing.T) {
		// Test message with mixed case headers
		mixedCaseMsg := `OPTIONS sip:test.local SIP/2.0
via: SIP/2.0/UDP 192.168.1.100:5060;branch=z9hG4bK-mixed-case
FROM: <sip:test@test.local>;tag=test-tag
To: <sip:test.local>
call-id: test-mixed-case-call-id
cseq: 1 OPTIONS
content-length: 0

`
		
		response, err := suite.SendUDPMessage(t, mixedCaseMsg)
		if err != nil {
			t.Fatalf("Failed to send mixed case header message: %v", err)
		}

		// Should handle case-insensitive headers correctly
		if !bytes.Contains([]byte(response), []byte("SIP/2.0")) {
			t.Errorf("Expected SIP response for mixed case headers, got: %s", response)
		}
	})
}

// TestSIPErrorHandling tests various error conditions
func TestSIPErrorHandling(t *testing.T) {
	suite := SetupTestSuite(t)
	defer suite.Cleanup(t)

	t.Run("Request_URI_Validation", func(t *testing.T) {
		// Test with invalid Request-URI
		invalidURIMsg := `INVITE invalid-uri SIP/2.0
Via: SIP/2.0/UDP 192.168.1.100:5060;branch=z9hG4bK-invalid-uri
From: <sip:alice@test.local>;tag=test-tag
To: <sip:bob@test.local>
Call-ID: test-invalid-uri-call-id
CSeq: 1 INVITE
Contact: <sip:alice@192.168.1.100:5060>
Session-Expires: 1800;refresher=uac
Content-Length: 0

`
		
		response, err := suite.SendUDPMessage(t, invalidURIMsg)
		if err != nil {
			t.Fatalf("Failed to send invalid URI message: %v", err)
		}

		// Should get 400 Bad Request for invalid Request-URI
		if !bytes.Contains([]byte(response), []byte("400 Bad Request")) {
			t.Errorf("Expected 400 Bad Request for invalid URI, got: %s", response)
		}
	})

	t.Run("Max_Forwards_Handling", func(t *testing.T) {
		// Test with Max-Forwards: 0
		maxForwardsMsg := `OPTIONS sip:test.local SIP/2.0
Via: SIP/2.0/UDP 192.168.1.100:5060;branch=z9hG4bK-max-forwards
From: <sip:test@test.local>;tag=test-tag
To: <sip:test.local>
Call-ID: test-max-forwards-call-id
CSeq: 1 OPTIONS
Max-Forwards: 0
Content-Length: 0

`
		
		response, err := suite.SendUDPMessage(t, maxForwardsMsg)
		if err != nil {
			t.Fatalf("Failed to send Max-Forwards message: %v", err)
		}

		// Should get 483 Too Many Hops for Max-Forwards: 0
		if !bytes.Contains([]byte(response), []byte("483 Too Many Hops")) {
			t.Errorf("Expected 483 Too Many Hops for Max-Forwards: 0, got: %s", response)
		}
	})

	t.Run("Loop_Detection", func(t *testing.T) {
		// Test with Via header containing server's own address (loop detection)
		loopMsg := `OPTIONS sip:test.local SIP/2.0
Via: SIP/2.0/UDP 127.0.0.1:5060;branch=z9hG4bK-loop-test
Via: SIP/2.0/UDP 192.168.1.100:5060;branch=z9hG4bK-client
From: <sip:test@test.local>;tag=test-tag
To: <sip:test.local>
Call-ID: test-loop-call-id
CSeq: 1 OPTIONS
Content-Length: 0

`
		
		response, err := suite.SendUDPMessage(t, loopMsg)
		if err != nil {
			t.Fatalf("Failed to send loop detection message: %v", err)
		}

		// Should detect loop and respond appropriately
		if !bytes.Contains([]byte(response), []byte("SIP/2.0")) {
			t.Errorf("Expected SIP response for loop detection, got: %s", response)
		}
	})
}

// TestSIPTimerHandling tests SIP timer behavior
func TestSIPTimerHandling(t *testing.T) {
	suite := SetupTestSuite(t)
	defer suite.Cleanup(t)

	t.Run("Transaction_Timeout", func(t *testing.T) {
		// This test would require more complex setup to test actual timer behavior
		// For now, we'll test that the server handles requests without hanging
		
		inviteMsg := suite.CreateINVITEMessage("alice", "nonexistent", "sip:alice@192.168.1.100:5060")
		
		start := time.Now()
		response, err := suite.SendUDPMessage(t, inviteMsg)
		duration := time.Since(start)
		
		if err != nil {
			t.Fatalf("Failed to send INVITE: %v", err)
		}

		// Should get response quickly (not hang waiting for timers)
		if duration > 5*time.Second {
			t.Errorf("Request took too long: %v", duration)
		}

		// Should get some response
		if !bytes.Contains([]byte(response), []byte("SIP/2.0")) {
			t.Errorf("Expected SIP response, got: %s", response)
		}
	})
}

// TestSIPContentHandling tests SIP message body handling
func TestSIPContentHandling(t *testing.T) {
	suite := SetupTestSuite(t)
	defer suite.Cleanup(t)

	t.Run("SDP_Content_Handling", func(t *testing.T) {
		inviteMsg := suite.CreateINVITEMessage("alice", "bob", "sip:alice@192.168.1.100:5060")
		
		response, err := suite.SendUDPMessage(t, inviteMsg)
		if err != nil {
			t.Fatalf("Failed to send INVITE with SDP: %v", err)
		}

		// Should handle SDP content correctly
		if !bytes.Contains([]byte(response), []byte("SIP/2.0")) {
			t.Errorf("Expected SIP response for INVITE with SDP, got: %s", response)
		}
	})

	t.Run("Empty_Content_Handling", func(t *testing.T) {
		optionsMsg := suite.CreateOPTIONSMessage()
		
		response, err := suite.SendUDPMessage(t, optionsMsg)
		if err != nil {
			t.Fatalf("Failed to send OPTIONS with empty content: %v", err)
		}

		// Should handle empty content correctly
		if !bytes.Contains([]byte(response), []byte("200 OK")) {
			t.Errorf("Expected 200 OK for OPTIONS with empty content, got: %s", response)
		}
	})

	t.Run("Unknown_Content_Type", func(t *testing.T) {
		unknownContentMsg := `INVITE sip:bob@test.local SIP/2.0
Via: SIP/2.0/UDP 192.168.1.100:5060;branch=z9hG4bK-unknown-content
From: <sip:alice@test.local>;tag=test-tag
To: <sip:bob@test.local>
Call-ID: test-unknown-content-call-id
CSeq: 1 INVITE
Contact: <sip:alice@192.168.1.100:5060>
Session-Expires: 1800;refresher=uac
Content-Type: application/unknown
Content-Length: 13

Unknown content
`
		
		response, err := suite.SendUDPMessage(t, unknownContentMsg)
		if err != nil {
			t.Fatalf("Failed to send INVITE with unknown content type: %v", err)
		}

		// Should handle unknown content type (may accept or reject)
		if !bytes.Contains([]byte(response), []byte("SIP/2.0")) {
			t.Errorf("Expected SIP response for unknown content type, got: %s", response)
		}
	})
}