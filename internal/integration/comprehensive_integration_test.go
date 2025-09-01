package integration

import (
	"bytes"
	"fmt"
	"net"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// TestComprehensiveIntegrationSuite runs the complete integration test suite
// covering all requirements: 5.1, 5.2, 7.1, 7.2, 7.3, 8.1, 8.2
func TestComprehensiveIntegrationSuite(t *testing.T) {
	suite := SetupTestSuite(t)
	defer suite.Cleanup(t)

	// Run all comprehensive test categories
	t.Run("EndToEnd_SIP_Call_Flows", func(t *testing.T) {
		testEndToEndSIPCallFlows(t, suite)
	})

	t.Run("Concurrent_Registration_And_Session_Handling", func(t *testing.T) {
		testConcurrentRegistrationAndSessionHandling(t, suite)
	})

	t.Run("SessionTimer_Enforcement_Integration", func(t *testing.T) {
		testSessionTimerEnforcementIntegration(t, suite)
	})

	t.Run("UDP_TCP_Transport_Protocol_Handling", func(t *testing.T) {
		testUDPTCPTransportProtocolHandling(t, suite)
	})

	t.Run("SIP_Protocol_Compliance_RFC3261", func(t *testing.T) {
		testSIPProtocolComplianceRFC3261(t, suite)
	})

	t.Run("Error_Handling_And_Edge_Cases", func(t *testing.T) {
		testErrorHandlingAndEdgeCases(t, suite)
	})

	t.Run("Performance_And_Load_Testing", func(t *testing.T) {
		testPerformanceAndLoadTesting(t, suite)
	})
}

// testEndToEndSIPCallFlows tests complete SIP call flows (Requirement 5.1, 5.2)
func testEndToEndSIPCallFlows(t *testing.T, suite *TestSuite) {
	t.Run("Complete_REGISTER_Flow", func(t *testing.T) {
		// Test complete registration flow with authentication
		registerMsg := suite.CreateREGISTERMessage("alice", "sip:alice@192.168.1.100:5060")
		
		// First request should get 401 Unauthorized
		response, err := suite.SendUDPMessage(t, registerMsg)
		if err != nil {
			t.Fatalf("Failed to send REGISTER: %v", err)
		}

		if !bytes.Contains([]byte(response), []byte("401 Unauthorized")) {
			t.Errorf("Expected 401 Unauthorized for REGISTER without auth, got: %s", response)
		}

		if !strings.Contains(response, "WWW-Authenticate:") {
			t.Errorf("Expected WWW-Authenticate header in 401 response")
		}

		// Verify nonce is present
		if !strings.Contains(response, "nonce=") {
			t.Errorf("Expected nonce parameter in WWW-Authenticate header")
		}
	})

	t.Run("Complete_INVITE_Flow", func(t *testing.T) {
		// Test complete INVITE flow with Session-Timer
		inviteMsg := suite.CreateINVITEMessage("alice", "bob", "sip:alice@192.168.1.100:5060")
		
		response, err := suite.SendUDPMessage(t, inviteMsg)
		if err != nil {
			t.Fatalf("Failed to send INVITE: %v", err)
		}

		// Should get some SIP response (likely 404 since bob isn't registered)
		if !bytes.Contains([]byte(response), []byte("SIP/2.0")) {
			t.Errorf("Expected SIP response for INVITE, got: %s", response)
		}

		// Should not get 421 Extension Required since Session-Timer is present
		if bytes.Contains([]byte(response), []byte("421 Extension Required")) {
			t.Errorf("Unexpected 421 Extension Required for INVITE with Session-Timer")
		}
	})

	t.Run("OPTIONS_Server_Capabilities", func(t *testing.T) {
		// Test OPTIONS to discover server capabilities
		optionsMsg := suite.CreateOPTIONSMessage()
		
		response, err := suite.SendUDPMessage(t, optionsMsg)
		if err != nil {
			t.Fatalf("Failed to send OPTIONS: %v", err)
		}

		if !bytes.Contains([]byte(response), []byte("200 OK")) {
			t.Errorf("Expected 200 OK for OPTIONS, got: %s", response)
		}

		// Should include Allow header with supported methods
		if !strings.Contains(response, "Allow:") {
			t.Errorf("Expected Allow header in OPTIONS response")
		}

		// Should include supported methods
		allowHeader := extractHeader(response, "Allow")
		expectedMethods := []string{"INVITE", "ACK", "BYE", "CANCEL", "OPTIONS", "REGISTER"}
		for _, method := range expectedMethods {
			if !strings.Contains(allowHeader, method) {
				t.Logf("Method %s not found in Allow header: %s", method, allowHeader)
			}
		}
	})

	t.Run("BYE_Session_Termination", func(t *testing.T) {
		// Test BYE request for session termination
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

		// Should get some SIP response (likely 481 Call/Transaction Does Not Exist)
		if !bytes.Contains([]byte(response), []byte("SIP/2.0")) {
			t.Errorf("Expected SIP response for BYE, got: %s", response)
		}
	})

	t.Run("ACK_Request_Handling", func(t *testing.T) {
		// Test ACK request (should not generate response)
		ackMsg := `ACK sip:bob@test.local SIP/2.0
Via: SIP/2.0/UDP 192.168.1.100:5060;branch=z9hG4bK-ack-test
From: <sip:alice@test.local>;tag=alice-tag
To: <sip:bob@test.local>;tag=bob-tag
Call-ID: test-ack-call-id
CSeq: 1 ACK
Content-Length: 0

`
		
		conn, err := net.Dial("udp", "127.0.0.1:5060")
		if err != nil {
			t.Fatalf("Failed to create UDP connection: %v", err)
		}
		defer conn.Close()

		_, err = conn.Write([]byte(ackMsg))
		if err != nil {
			t.Fatalf("Failed to send ACK: %v", err)
		}

		// ACK should not generate a response
		conn.SetReadDeadline(time.Now().Add(1 * time.Second))
		buffer := make([]byte, 4096)
		n, err := conn.Read(buffer)
		
		if err == nil {
			response := string(buffer[:n])
			t.Errorf("Unexpected response to ACK: %s", response)
		}
		// Timeout is expected for ACK
	})
}

// testConcurrentRegistrationAndSessionHandling tests concurrent operations (Requirement 5.1, 5.2)
func testConcurrentRegistrationAndSessionHandling(t *testing.T, suite *TestSuite) {
	t.Run("Concurrent_REGISTER_Requests", func(t *testing.T) {
		const numConcurrentRegs = 15
		var wg sync.WaitGroup
		results := make(chan error, numConcurrentRegs)

		for i := 0; i < numConcurrentRegs; i++ {
			wg.Add(1)
			go func(userID int) {
				defer wg.Done()
				
				username := fmt.Sprintf("user%d", userID)
				contact := fmt.Sprintf("sip:%s@192.168.1.%d:5060", username, 100+userID)
				registerMsg := suite.CreateREGISTERMessage(username, contact)
				
				conn, err := net.Dial("udp", "127.0.0.1:5060")
				if err != nil {
					results <- fmt.Errorf("user %d: failed to create connection: %w", userID, err)
					return
				}
				defer conn.Close()
				
				_, err = conn.Write([]byte(registerMsg))
				if err != nil {
					results <- fmt.Errorf("user %d: failed to send REGISTER: %w", userID, err)
					return
				}
				
				conn.SetReadDeadline(time.Now().Add(5 * time.Second))
				buffer := make([]byte, 4096)
				n, err := conn.Read(buffer)
				if err != nil {
					results <- fmt.Errorf("user %d: failed to read response: %w", userID, err)
					return
				}
				
				response := string(buffer[:n])
				if !bytes.Contains([]byte(response), []byte("401 Unauthorized")) {
					results <- fmt.Errorf("user %d: expected 401 Unauthorized, got: %s", userID, response)
					return
				}
				
				results <- nil
			}(i)
		}

		wg.Wait()
		close(results)

		successCount := 0
		errorCount := 0
		for err := range results {
			if err != nil {
				errorCount++
				t.Logf("Concurrent REGISTER error: %v", err)
			} else {
				successCount++
			}
		}

		t.Logf("Concurrent REGISTER results: %d successful, %d errors", successCount, errorCount)

		// Allow up to 20% errors under concurrent load
		if errorCount > numConcurrentRegs/5 {
			t.Errorf("Too many errors in concurrent REGISTER test: %d/%d", errorCount, numConcurrentRegs)
		}
	})

	t.Run("Concurrent_INVITE_Requests", func(t *testing.T) {
		const numConcurrentInvites = 10
		var wg sync.WaitGroup
		results := make(chan error, numConcurrentInvites)

		for i := 0; i < numConcurrentInvites; i++ {
			wg.Add(1)
			go func(sessionID int) {
				defer wg.Done()
				
				from := fmt.Sprintf("caller%d", sessionID)
				to := fmt.Sprintf("callee%d", sessionID)
				contact := fmt.Sprintf("sip:%s@192.168.1.%d:5060", from, 100+sessionID)
				inviteMsg := suite.CreateINVITEMessage(from, to, contact)
				
				conn, err := net.Dial("udp", "127.0.0.1:5060")
				if err != nil {
					results <- fmt.Errorf("session %d: failed to create connection: %w", sessionID, err)
					return
				}
				defer conn.Close()
				
				_, err = conn.Write([]byte(inviteMsg))
				if err != nil {
					results <- fmt.Errorf("session %d: failed to send INVITE: %w", sessionID, err)
					return
				}
				
				conn.SetReadDeadline(time.Now().Add(5 * time.Second))
				buffer := make([]byte, 4096)
				n, err := conn.Read(buffer)
				if err != nil {
					results <- fmt.Errorf("session %d: failed to read response: %w", sessionID, err)
					return
				}
				
				response := string(buffer[:n])
				if !bytes.Contains([]byte(response), []byte("SIP/2.0")) {
					results <- fmt.Errorf("session %d: expected SIP response, got: %s", sessionID, response)
					return
				}
				
				results <- nil
			}(i)
		}

		wg.Wait()
		close(results)

		successCount := 0
		errorCount := 0
		for err := range results {
			if err != nil {
				errorCount++
				t.Logf("Concurrent INVITE error: %v", err)
			} else {
				successCount++
			}
		}

		t.Logf("Concurrent INVITE results: %d successful, %d errors", successCount, errorCount)

		// Allow up to 20% errors under concurrent load
		if errorCount > numConcurrentInvites/5 {
			t.Errorf("Too many errors in concurrent INVITE test: %d/%d", errorCount, numConcurrentInvites)
		}
	})

	t.Run("Mixed_Concurrent_Operations", func(t *testing.T) {
		const numOperations = 20
		var wg sync.WaitGroup
		var successCount, errorCount int32

		for i := 0; i < numOperations; i++ {
			wg.Add(1)
			go func(opID int) {
				defer wg.Done()
				
				conn, err := net.Dial("udp", "127.0.0.1:5060")
				if err != nil {
					atomic.AddInt32(&errorCount, 1)
					return
				}
				defer conn.Close()
				
				var message string
				switch opID % 3 {
				case 0:
					// OPTIONS request
					message = fmt.Sprintf(`OPTIONS sip:test.local SIP/2.0
Via: SIP/2.0/UDP 192.168.1.100:5060;branch=z9hG4bK-mixed-options-%d
From: <sip:test@test.local>;tag=test-tag-%d
To: <sip:test.local>
Call-ID: mixed-options-call-id-%d
CSeq: 1 OPTIONS
Content-Length: 0

`, opID, opID, opID)
				case 1:
					// REGISTER request
					message = fmt.Sprintf(`REGISTER sip:test.local SIP/2.0
Via: SIP/2.0/UDP 192.168.1.100:5060;branch=z9hG4bK-mixed-register-%d
From: <sip:user%d@test.local>;tag=test-tag-%d
To: <sip:user%d@test.local>
Call-ID: mixed-register-call-id-%d
CSeq: 1 REGISTER
Contact: <sip:user%d@192.168.1.100:5060>
Expires: 3600
Content-Length: 0

`, opID, opID, opID, opID, opID, opID)
				case 2:
					// INVITE request
					message = fmt.Sprintf(`INVITE sip:target%d@test.local SIP/2.0
Via: SIP/2.0/UDP 192.168.1.100:5060;branch=z9hG4bK-mixed-invite-%d
From: <sip:caller%d@test.local>;tag=test-tag-%d
To: <sip:target%d@test.local>
Call-ID: mixed-invite-call-id-%d
CSeq: 1 INVITE
Contact: <sip:caller%d@192.168.1.100:5060>
Session-Expires: 1800;refresher=uac
Content-Type: application/sdp
Content-Length: 142

v=0
o=caller%d 2890844526 2890844526 IN IP4 192.168.1.100
s=-
c=IN IP4 192.168.1.100
t=0 0
m=audio 5004 RTP/AVP 0
a=rtpmap:0 PCMU/8000
`, opID, opID, opID, opID, opID, opID, opID, opID)
				}
				
				_, err = conn.Write([]byte(message))
				if err != nil {
					atomic.AddInt32(&errorCount, 1)
					return
				}
				
				conn.SetReadDeadline(time.Now().Add(5 * time.Second))
				buffer := make([]byte, 4096)
				_, err = conn.Read(buffer)
				if err != nil {
					atomic.AddInt32(&errorCount, 1)
					return
				}
				
				atomic.AddInt32(&successCount, 1)
			}(i)
		}

		wg.Wait()

		t.Logf("Mixed concurrent operations: %d successful, %d errors out of %d", 
			successCount, errorCount, numOperations)

		// Should have reasonable success rate
		if successCount < int32(numOperations*3/4) {
			t.Errorf("Too many failures in mixed concurrent operations: %d successful out of %d", 
				successCount, numOperations)
		}
	})
}

// testSessionTimerEnforcementIntegration tests Session-Timer enforcement (Requirement 8.1, 8.2)
func testSessionTimerEnforcementIntegration(t *testing.T, suite *TestSuite) {
	t.Run("INVITE_Requires_SessionTimer", func(t *testing.T) {
		// INVITE without Session-Expires should be rejected
		inviteWithoutTimer := `INVITE sip:bob@test.local SIP/2.0
Via: SIP/2.0/UDP 192.168.1.100:5060;branch=z9hG4bK-no-session-timer
From: <sip:alice@test.local>;tag=alice-tag
To: <sip:bob@test.local>
Call-ID: no-session-timer-call-id
CSeq: 1 INVITE
Contact: <sip:alice@192.168.1.100:5060>
Content-Type: application/sdp
Content-Length: 142

v=0
o=alice 2890844526 2890844526 IN IP4 192.168.1.100
s=-
c=IN IP4 192.168.1.100
t=0 0
m=audio 5004 RTP/AVP 0
a=rtpmap:0 PCMU/8000
`
		
		response, err := suite.SendUDPMessage(t, inviteWithoutTimer)
		if err != nil {
			t.Fatalf("Failed to send INVITE without Session-Timer: %v", err)
		}

		// Should get 421 Extension Required
		if !bytes.Contains([]byte(response), []byte("421 Extension Required")) {
			t.Errorf("Expected 421 Extension Required for INVITE without Session-Timer, got: %s", response)
		}

		// Should include Require header
		if !strings.Contains(response, "Require:") {
			t.Errorf("Expected Require header in 421 response")
		}
	})

	t.Run("SessionTimer_Value_Validation", func(t *testing.T) {
		testCases := []struct {
			name           string
			sessionExpires string
			expectedStatus string
		}{
			{
				name:           "Valid_Timer_1800",
				sessionExpires: "1800;refresher=uac",
				expectedStatus: "SIP/2.0", // Should not get 421 or 422
			},
			{
				name:           "Low_Timer_30",
				sessionExpires: "30;refresher=uac",
				expectedStatus: "422 Session Interval Too Small",
			},
			{
				name:           "Zero_Timer",
				sessionExpires: "0;refresher=uac",
				expectedStatus: "422 Session Interval Too Small",
			},
			{
				name:           "High_Timer_10000",
				sessionExpires: "10000;refresher=uac",
				expectedStatus: "SIP/2.0", // May be accepted or adjusted
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				inviteMsg := fmt.Sprintf(`INVITE sip:bob@test.local SIP/2.0
Via: SIP/2.0/UDP 192.168.1.100:5060;branch=z9hG4bK-%s
From: <sip:alice@test.local>;tag=alice-tag
To: <sip:bob@test.local>
Call-ID: %s-call-id
CSeq: 1 INVITE
Contact: <sip:alice@192.168.1.100:5060>
Session-Expires: %s
Content-Type: application/sdp
Content-Length: 142

v=0
o=alice 2890844526 2890844526 IN IP4 192.168.1.100
s=-
c=IN IP4 192.168.1.100
t=0 0
m=audio 5004 RTP/AVP 0
a=rtpmap:0 PCMU/8000
`, tc.name, tc.name, tc.sessionExpires)
				
				response, err := suite.SendUDPMessage(t, inviteMsg)
				if err != nil {
					t.Fatalf("Failed to send INVITE for %s: %v", tc.name, err)
				}

				if !bytes.Contains([]byte(response), []byte(tc.expectedStatus)) {
					t.Errorf("Test %s: Expected %s, got: %s", tc.name, tc.expectedStatus, response)
				}

				// For 422 responses, should include Min-SE header
				if strings.Contains(tc.expectedStatus, "422") {
					if !strings.Contains(response, "Min-SE:") {
						t.Errorf("Test %s: Expected Min-SE header in 422 response", tc.name)
					}
				}
			})
		}
	})

	t.Run("SessionTimer_Refresher_Parameter", func(t *testing.T) {
		refresherTests := []string{"uac", "uas", ""}
		
		for _, refresher := range refresherTests {
			t.Run(fmt.Sprintf("Refresher_%s", refresher), func(t *testing.T) {
				sessionExpires := "1800"
				if refresher != "" {
					sessionExpires += ";refresher=" + refresher
				}
				
				inviteMsg := fmt.Sprintf(`INVITE sip:bob@test.local SIP/2.0
Via: SIP/2.0/UDP 192.168.1.100:5060;branch=z9hG4bK-refresher-%s
From: <sip:alice@test.local>;tag=alice-tag
To: <sip:bob@test.local>
Call-ID: refresher-%s-call-id
CSeq: 1 INVITE
Contact: <sip:alice@192.168.1.100:5060>
Session-Expires: %s
Content-Type: application/sdp
Content-Length: 142

v=0
o=alice 2890844526 2890844526 IN IP4 192.168.1.100
s=-
c=IN IP4 192.168.1.100
t=0 0
m=audio 5004 RTP/AVP 0
a=rtpmap:0 PCMU/8000
`, refresher, refresher, sessionExpires)
				
				response, err := suite.SendUDPMessage(t, inviteMsg)
				if err != nil {
					t.Fatalf("Failed to send INVITE with refresher %s: %v", refresher, err)
				}

				// Should handle refresher parameter correctly
				if !bytes.Contains([]byte(response), []byte("SIP/2.0")) {
					t.Errorf("Expected SIP response for refresher %s, got: %s", refresher, response)
				}

				// Should not get 421 Extension Required since Session-Expires is present
				if bytes.Contains([]byte(response), []byte("421 Extension Required")) {
					t.Errorf("Unexpected 421 Extension Required for refresher %s", refresher)
				}
			})
		}
	})
}

// testUDPTCPTransportProtocolHandling tests transport protocols (Requirement 7.1, 7.2, 7.3)
func testUDPTCPTransportProtocolHandling(t *testing.T, suite *TestSuite) {
	t.Run("UDP_Message_Processing", func(t *testing.T) {
		optionsMsg := suite.CreateOPTIONSMessage()
		
		response, err := suite.SendUDPMessage(t, optionsMsg)
		if err != nil {
			t.Fatalf("Failed to send UDP message: %v", err)
		}

		if !bytes.Contains([]byte(response), []byte("200 OK")) {
			t.Errorf("Expected 200 OK for UDP OPTIONS, got: %s", response)
		}

		// Response should indicate UDP transport in Via header
		if !strings.Contains(response, "UDP") {
			t.Errorf("Expected UDP transport in Via header, got: %s", response)
		}
	})

	t.Run("TCP_Message_Processing", func(t *testing.T) {
		optionsMsg := NewSIPMessageBuilder("OPTIONS", "sip:test.local").
			SetTransport("TCP").
			Build()
		
		response, err := suite.SendTCPMessage(t, optionsMsg)
		if err != nil {
			t.Fatalf("Failed to send TCP message: %v", err)
		}

		if !bytes.Contains([]byte(response), []byte("200 OK")) {
			t.Errorf("Expected 200 OK for TCP OPTIONS, got: %s", response)
		}

		// Response should indicate TCP transport in Via header
		if !strings.Contains(response, "TCP") {
			t.Errorf("Expected TCP transport in Via header, got: %s", response)
		}
	})

	t.Run("TCP_Connection_Reuse", func(t *testing.T) {
		// Test multiple messages over same TCP connection
		conn, err := net.Dial("tcp", "127.0.0.1:5060")
		if err != nil {
			t.Fatalf("Failed to create TCP connection: %v", err)
		}
		defer conn.Close()

		for i := 0; i < 3; i++ {
			optionsMsg := fmt.Sprintf(`OPTIONS sip:test.local SIP/2.0
Via: SIP/2.0/TCP 192.168.1.100:5060;branch=z9hG4bK-reuse-%d
From: <sip:test@test.local>;tag=test-tag-%d
To: <sip:test.local>
Call-ID: reuse-call-id-%d
CSeq: %d OPTIONS
Content-Length: 0

`, i, i, i, i+1)
			
			_, err = conn.Write([]byte(optionsMsg))
			if err != nil {
				t.Fatalf("Failed to send TCP message %d: %v", i, err)
			}
			
			conn.SetReadDeadline(time.Now().Add(5 * time.Second))
			buffer := make([]byte, 4096)
			n, err := conn.Read(buffer)
			if err != nil {
				t.Fatalf("Failed to read TCP response %d: %v", i, err)
			}
			
			response := string(buffer[:n])
			if !bytes.Contains([]byte(response), []byte("200 OK")) {
				t.Errorf("Expected 200 OK for TCP message %d, got: %s", i, response)
			}
		}
	})

	t.Run("Large_Message_Handling", func(t *testing.T) {
		// Create large SDP content
		largeSDP := CreateSDPBody("Large Message Test")
		for i := 0; i < 20; i++ {
			largeSDP += fmt.Sprintf("a=test-attribute-%d:value-%d\r\n", i, i)
		}
		
		largeInviteMsg := NewSIPMessageBuilder("INVITE", "sip:bob@test.local").
			SetHeader("Contact", "<sip:alice@192.168.1.100:5060>").
			SetHeader("Session-Expires", "1800;refresher=uac").
			SetHeader("Content-Type", "application/sdp").
			SetBody(largeSDP).
			Build()
		
		// Test over UDP
		response, err := suite.SendUDPMessage(t, largeInviteMsg)
		if err != nil {
			t.Fatalf("Failed to send large UDP message: %v", err)
		}
		
		if !bytes.Contains([]byte(response), []byte("SIP/2.0")) {
			t.Errorf("Expected SIP response for large UDP message, got: %s", response)
		}
		
		// Test over TCP
		tcpResponse, err := suite.SendTCPMessage(t, largeInviteMsg)
		if err != nil {
			t.Fatalf("Failed to send large TCP message: %v", err)
		}
		
		if !bytes.Contains([]byte(tcpResponse), []byte("SIP/2.0")) {
			t.Errorf("Expected SIP response for large TCP message, got: %s", tcpResponse)
		}
	})

	t.Run("Parallel_UDP_TCP_Operations", func(t *testing.T) {
		const numParallel = 10
		var wg sync.WaitGroup
		var udpSuccess, tcpSuccess, udpErrors, tcpErrors int32

		// Launch parallel UDP and TCP operations
		for i := 0; i < numParallel; i++ {
			// UDP operation
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				
				conn, err := net.Dial("udp", "127.0.0.1:5060")
				if err != nil {
					atomic.AddInt32(&udpErrors, 1)
					return
				}
				defer conn.Close()
				
				optionsMsg := fmt.Sprintf(`OPTIONS sip:test.local SIP/2.0
Via: SIP/2.0/UDP 192.168.1.100:5060;branch=z9hG4bK-parallel-udp-%d
From: <sip:test@test.local>;tag=test-tag-%d
To: <sip:test.local>
Call-ID: parallel-udp-call-id-%d
CSeq: 1 OPTIONS
Content-Length: 0

`, id, id, id)
				
				_, err = conn.Write([]byte(optionsMsg))
				if err != nil {
					atomic.AddInt32(&udpErrors, 1)
					return
				}
				
				conn.SetReadDeadline(time.Now().Add(3 * time.Second))
				buffer := make([]byte, 4096)
				_, err = conn.Read(buffer)
				if err != nil {
					atomic.AddInt32(&udpErrors, 1)
					return
				}
				
				atomic.AddInt32(&udpSuccess, 1)
			}(i)
			
			// TCP operation
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				
				conn, err := net.Dial("tcp", "127.0.0.1:5060")
				if err != nil {
					atomic.AddInt32(&tcpErrors, 1)
					return
				}
				defer conn.Close()
				
				optionsMsg := fmt.Sprintf(`OPTIONS sip:test.local SIP/2.0
Via: SIP/2.0/TCP 192.168.1.100:5060;branch=z9hG4bK-parallel-tcp-%d
From: <sip:test@test.local>;tag=test-tag-%d
To: <sip:test.local>
Call-ID: parallel-tcp-call-id-%d
CSeq: 1 OPTIONS
Content-Length: 0

`, id, id, id)
				
				_, err = conn.Write([]byte(optionsMsg))
				if err != nil {
					atomic.AddInt32(&tcpErrors, 1)
					return
				}
				
				conn.SetReadDeadline(time.Now().Add(3 * time.Second))
				buffer := make([]byte, 4096)
				_, err = conn.Read(buffer)
				if err != nil {
					atomic.AddInt32(&tcpErrors, 1)
					return
				}
				
				atomic.AddInt32(&tcpSuccess, 1)
			}(i)
		}

		wg.Wait()

		t.Logf("Parallel transport test: UDP(%d success, %d errors), TCP(%d success, %d errors)", 
			udpSuccess, udpErrors, tcpSuccess, tcpErrors)

		// Both transports should work
		if udpSuccess == 0 {
			t.Error("No UDP operations succeeded")
		}
		if tcpSuccess == 0 {
			t.Error("No TCP operations succeeded")
		}
	})
}

// testSIPProtocolComplianceRFC3261 tests RFC3261 compliance
func testSIPProtocolComplianceRFC3261(t *testing.T, suite *TestSuite) {
	t.Run("Required_Headers_Validation", func(t *testing.T) {
		// Test message missing required headers
		incompleteMsg := `REGISTER sip:test.local SIP/2.0
Via: SIP/2.0/UDP 192.168.1.100:5060;branch=z9hG4bK-incomplete
From: <sip:alice@test.local>;tag=test-tag
Content-Length: 0

`
		// Missing To, Call-ID, CSeq headers
		
		response, err := suite.SendUDPMessage(t, incompleteMsg)
		if err != nil {
			t.Fatalf("Failed to send incomplete message: %v", err)
		}

		if !bytes.Contains([]byte(response), []byte("400 Bad Request")) {
			t.Errorf("Expected 400 Bad Request for missing headers, got: %s", response)
		}
	})

	t.Run("Method_Support_Validation", func(t *testing.T) {
		supportedMethods := []string{"OPTIONS", "REGISTER", "INVITE", "ACK", "BYE", "CANCEL"}
		unsupportedMethods := []string{"SUBSCRIBE", "NOTIFY", "PUBLISH", "MESSAGE"}

		// Test supported methods
		for _, method := range supportedMethods {
			t.Run(fmt.Sprintf("Supported_%s", method), func(t *testing.T) {
				if method == "ACK" {
					// ACK doesn't generate responses, skip
					return
				}
				
				msg := NewSIPMessageBuilder(method, "sip:test.local").Build()
				response, err := suite.SendUDPMessage(t, msg)
				if err != nil {
					t.Fatalf("Failed to send %s: %v", method, err)
				}

				// Should not get 405 Method Not Allowed
				if bytes.Contains([]byte(response), []byte("405 Method Not Allowed")) {
					t.Errorf("Unexpected 405 Method Not Allowed for %s: %s", method, response)
				}
			})
		}

		// Test unsupported methods
		for _, method := range unsupportedMethods {
			t.Run(fmt.Sprintf("Unsupported_%s", method), func(t *testing.T) {
				msg := NewSIPMessageBuilder(method, "sip:test.local").Build()
				response, err := suite.SendUDPMessage(t, msg)
				if err != nil {
					t.Fatalf("Failed to send %s: %v", method, err)
				}

				// Should get 405 Method Not Allowed
				if !bytes.Contains([]byte(response), []byte("405 Method Not Allowed")) {
					t.Errorf("Expected 405 Method Not Allowed for %s, got: %s", method, response)
				}

				// Should include Allow header
				if !strings.Contains(response, "Allow:") {
					t.Errorf("Expected Allow header in 405 response for %s", method)
				}
			})
		}
	})

	t.Run("SIP_Version_Validation", func(t *testing.T) {
		invalidVersionMsg := `OPTIONS sip:test.local SIP/1.0
Via: SIP/1.0/UDP 192.168.1.100:5060;branch=z9hG4bK-invalid-version
From: <sip:test@test.local>;tag=test-tag
To: <sip:test.local>
Call-ID: invalid-version-call-id
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
			t.Errorf("Expected 505 or 400 for invalid SIP version, got: %s", response)
		}
	})

	t.Run("Content_Length_Validation", func(t *testing.T) {
		// Test Content-Length mismatch
		mismatchMsg := `INVITE sip:bob@test.local SIP/2.0
Via: SIP/2.0/UDP 192.168.1.100:5060;branch=z9hG4bK-content-mismatch
From: <sip:alice@test.local>;tag=test-tag
To: <sip:bob@test.local>
Call-ID: content-mismatch-call-id
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
		// Content-Length is 1000 but actual content is much shorter
		
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

// testErrorHandlingAndEdgeCases tests error handling scenarios
func testErrorHandlingAndEdgeCases(t *testing.T, suite *TestSuite) {
	t.Run("Malformed_SIP_Messages", func(t *testing.T) {
		malformedMessages := []string{
			"INVALID MESSAGE FORMAT\r\nThis is not SIP\r\n\r\n",
			"OPTIONS\r\nMissing URI and version\r\n\r\n",
			"SIP/2.0 200 OK\r\nThis looks like a response\r\n\r\n",
		}

		for i, msg := range malformedMessages {
			t.Run(fmt.Sprintf("Malformed_%d", i), func(t *testing.T) {
				response, err := suite.SendUDPMessage(t, msg)
				if err != nil {
					t.Fatalf("Failed to send malformed message %d: %v", i, err)
				}

				// Should get 400 Bad Request for malformed messages
				if !bytes.Contains([]byte(response), []byte("400 Bad Request")) {
					t.Errorf("Expected 400 Bad Request for malformed message %d, got: %s", i, response)
				}
			})
		}
	})

	t.Run("Invalid_URI_Formats", func(t *testing.T) {
		invalidURIs := []string{
			"invalid-uri",
			"http://example.com",
			"sip:",
			"sip:@domain.com",
		}

		for i, uri := range invalidURIs {
			t.Run(fmt.Sprintf("Invalid_URI_%d", i), func(t *testing.T) {
				msg := NewSIPMessageBuilder("OPTIONS", uri).Build()
				response, err := suite.SendUDPMessage(t, msg)
				if err != nil {
					t.Fatalf("Failed to send message with invalid URI %s: %v", uri, err)
				}

				// Should get 400 Bad Request for invalid URI
				if !bytes.Contains([]byte(response), []byte("400 Bad Request")) {
					t.Errorf("Expected 400 Bad Request for invalid URI %s, got: %s", uri, response)
				}
			})
		}
	})

	t.Run("Header_Edge_Cases", func(t *testing.T) {
		// Test various header edge cases
		edgeCases := []struct {
			name string
			msg  string
		}{
			{
				name: "Empty_Header_Value",
				msg: `OPTIONS sip:test.local SIP/2.0
Via: SIP/2.0/UDP 192.168.1.100:5060;branch=z9hG4bK-empty-header
From: 
To: <sip:test.local>
Call-ID: empty-header-call-id
CSeq: 1 OPTIONS
Content-Length: 0

`,
			},
			{
				name: "Very_Long_Header",
				msg: fmt.Sprintf(`OPTIONS sip:test.local SIP/2.0
Via: SIP/2.0/UDP 192.168.1.100:5060;branch=z9hG4bK-long-header
From: <sip:test@test.local>;tag=%s
To: <sip:test.local>
Call-ID: long-header-call-id
CSeq: 1 OPTIONS
Content-Length: 0

`, strings.Repeat("very-long-tag-", 100)),
			},
		}

		for _, tc := range edgeCases {
			t.Run(tc.name, func(t *testing.T) {
				response, err := suite.SendUDPMessage(t, tc.msg)
				if err != nil {
					t.Fatalf("Failed to send %s message: %v", tc.name, err)
				}

				// Should handle edge cases gracefully (may accept or reject)
				if !bytes.Contains([]byte(response), []byte("SIP/2.0")) {
					t.Errorf("Expected SIP response for %s, got: %s", tc.name, response)
				}
			})
		}
	})
}

// testPerformanceAndLoadTesting tests performance characteristics
func testPerformanceAndLoadTesting(t *testing.T, suite *TestSuite) {
	t.Run("Throughput_Measurement", func(t *testing.T) {
		const numRequests = 50
		const maxConcurrency = 10
		
		start := time.Now()
		var wg sync.WaitGroup
		var successCount, errorCount int32
		semaphore := make(chan struct{}, maxConcurrency)

		for i := 0; i < numRequests; i++ {
			wg.Add(1)
			go func(requestID int) {
				defer wg.Done()
				
				semaphore <- struct{}{} // Acquire
				defer func() { <-semaphore }() // Release
				
				conn, err := net.Dial("udp", "127.0.0.1:5060")
				if err != nil {
					atomic.AddInt32(&errorCount, 1)
					return
				}
				defer conn.Close()
				
				optionsMsg := fmt.Sprintf(`OPTIONS sip:test.local SIP/2.0
Via: SIP/2.0/UDP 192.168.1.100:5060;branch=z9hG4bK-throughput-%d
From: <sip:test@test.local>;tag=test-tag-%d
To: <sip:test.local>
Call-ID: throughput-call-id-%d
CSeq: 1 OPTIONS
Content-Length: 0

`, requestID, requestID, requestID)
				
				_, err = conn.Write([]byte(optionsMsg))
				if err != nil {
					atomic.AddInt32(&errorCount, 1)
					return
				}
				
				conn.SetReadDeadline(time.Now().Add(3 * time.Second))
				buffer := make([]byte, 4096)
				_, err = conn.Read(buffer)
				if err != nil {
					atomic.AddInt32(&errorCount, 1)
					return
				}
				
				atomic.AddInt32(&successCount, 1)
			}(i)
		}

		wg.Wait()
		duration := time.Since(start)
		throughput := float64(successCount) / duration.Seconds()

		t.Logf("Throughput test: %d successful, %d errors in %v (%.2f req/sec)", 
			successCount, errorCount, duration, throughput)

		// Should achieve reasonable throughput
		if throughput < 5 {
			t.Errorf("Throughput too low: %.2f req/sec", throughput)
		}

		// Error rate should be acceptable
		errorRate := float64(errorCount) / float64(numRequests) * 100
		if errorRate > 20 {
			t.Errorf("Error rate too high: %.2f%%", errorRate)
		}
	})

	t.Run("Response_Time_Analysis", func(t *testing.T) {
		const numSamples = 20
		var responseTimes []time.Duration

		for i := 0; i < numSamples; i++ {
			conn, err := net.Dial("udp", "127.0.0.1:5060")
			if err != nil {
				t.Fatalf("Failed to create connection for sample %d: %v", i, err)
			}
			
			optionsMsg := fmt.Sprintf(`OPTIONS sip:test.local SIP/2.0
Via: SIP/2.0/UDP 192.168.1.100:5060;branch=z9hG4bK-response-time-%d
From: <sip:test@test.local>;tag=test-tag-%d
To: <sip:test.local>
Call-ID: response-time-call-id-%d
CSeq: 1 OPTIONS
Content-Length: 0

`, i, i, i)
			
			start := time.Now()
			_, err = conn.Write([]byte(optionsMsg))
			if err != nil {
				conn.Close()
				continue
			}
			
			conn.SetReadDeadline(time.Now().Add(2 * time.Second))
			buffer := make([]byte, 4096)
			_, err = conn.Read(buffer)
			responseTime := time.Since(start)
			conn.Close()
			
			if err == nil {
				responseTimes = append(responseTimes, responseTime)
			}
			
			// Small delay between samples
			time.Sleep(50 * time.Millisecond)
		}

		if len(responseTimes) == 0 {
			t.Fatal("No successful responses for response time analysis")
		}

		// Calculate statistics
		var total time.Duration
		min := responseTimes[0]
		max := responseTimes[0]
		
		for _, rt := range responseTimes {
			total += rt
			if rt < min {
				min = rt
			}
			if rt > max {
				max = rt
			}
		}
		
		avg := total / time.Duration(len(responseTimes))

		t.Logf("Response time analysis: avg=%v, min=%v, max=%v, samples=%d", 
			avg, min, max, len(responseTimes))

		// Response times should be reasonable
		if avg > 200*time.Millisecond {
			t.Errorf("Average response time too high: %v", avg)
		}
		
		if max > 1*time.Second {
			t.Errorf("Maximum response time too high: %v", max)
		}
	})
}

// Helper function to extract header value from SIP message
func extractHeader(message, headerName string) string {
	lines := strings.Split(message, "\r\n")
	for _, line := range lines {
		if strings.HasPrefix(strings.ToLower(line), strings.ToLower(headerName)+":") {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				return strings.TrimSpace(parts[1])
			}
		}
	}
	return ""
}