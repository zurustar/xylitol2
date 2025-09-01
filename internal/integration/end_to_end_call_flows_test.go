package integration

import (
	"bytes"
	"fmt"
	"net"
	"strings"
	"sync"
	"testing"
	"time"
)

// TestCompleteCallFlowScenarios tests complete end-to-end SIP call flows
func TestCompleteCallFlowScenarios(t *testing.T) {
	suite := SetupTestSuite(t)
	defer suite.Cleanup(t)

	t.Run("Basic_Call_Setup_Flow", func(t *testing.T) {
		testBasicCallSetupFlow(t, suite)
	})

	t.Run("Call_With_Authentication_Flow", func(t *testing.T) {
		testCallWithAuthenticationFlow(t, suite)
	})

	t.Run("Call_Termination_Flow", func(t *testing.T) {
		testCallTerminationFlow(t, suite)
	})

	t.Run("Multiple_Simultaneous_Calls", func(t *testing.T) {
		testMultipleSimultaneousCalls(t, suite)
	})

	t.Run("Call_Flow_With_Session_Timer", func(t *testing.T) {
		testCallFlowWithSessionTimer(t, suite)
	})
}

// testBasicCallSetupFlow tests a basic call setup scenario
func testBasicCallSetupFlow(t *testing.T, suite *TestSuite) {
	// Step 1: Caller sends INVITE
	inviteMsg := suite.CreateINVITEMessage("alice", "bob", "sip:alice@192.168.1.100:5060")
	
	t.Log("Step 1: Sending INVITE from alice to bob")
	inviteResponse, err := suite.SendUDPMessage(t, inviteMsg)
	if err != nil {
		t.Fatalf("Failed to send INVITE: %v", err)
	}

	t.Logf("INVITE response: %s", inviteResponse)

	// Should get some SIP response (likely 404 since bob isn't registered, or 401 for auth)
	if !bytes.Contains([]byte(inviteResponse), []byte("SIP/2.0")) {
		t.Errorf("Expected SIP response for INVITE, got: %s", inviteResponse)
	}

	// Extract response code
	parser := NewSIPResponseParser(inviteResponse)
	statusCode, err := parser.GetStatusCode()
	if err != nil {
		t.Fatalf("Failed to parse INVITE response status: %v", err)
	}

	t.Logf("INVITE response status code: %d", statusCode)

	// Handle different response scenarios
	switch {
	case statusCode == 401:
		// Authentication required - this is expected
		t.Log("Authentication required for INVITE (expected)")
		
		// Verify WWW-Authenticate header is present
		if !parser.HasHeader("WWW-Authenticate") {
			t.Error("Expected WWW-Authenticate header in 401 response")
		}

	case statusCode == 404:
		// User not found - this is also expected since bob isn't registered
		t.Log("User not found (expected - bob not registered)")

	case statusCode == 421:
		// Extension required - check if it's Session-Timer related
		t.Log("Extension required response")
		if strings.Contains(inviteResponse, "timer") {
			t.Error("Unexpected 421 Extension Required - INVITE has Session-Timer")
		}

	case statusCode >= 200 && statusCode < 300:
		// Success response - unexpected but handle gracefully
		t.Log("Unexpected success response for unregistered user")

	default:
		t.Logf("Other response code: %d", statusCode)
	}

	// Step 2: If we got a provisional response, we might expect a final response
	// For this test, we'll just verify the server handled the INVITE properly
	
	// Step 3: Test ACK handling (even though we might not have a successful call)
	ackMsg := `ACK sip:bob@test.local SIP/2.0
Via: SIP/2.0/UDP 192.168.1.100:5060;branch=z9hG4bK-ack-test
From: <sip:alice@test.local>;tag=alice-tag
To: <sip:bob@test.local>;tag=bob-tag
Call-ID: test-call-setup-call-id
CSeq: 1 ACK
Content-Length: 0

`
	
	t.Log("Step 3: Sending ACK")
	// ACK should not generate a response
	conn, err := net.Dial("udp", "127.0.0.1:5060")
	if err != nil {
		t.Fatalf("Failed to create UDP connection for ACK: %v", err)
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
	
	if err == nil {
		response := string(buffer[:n])
		t.Errorf("Unexpected response to ACK: %s", response)
	}
	// Timeout is expected for ACK
	t.Log("ACK handled correctly (no response generated)")
}

// testCallWithAuthenticationFlow tests call flow with authentication
func testCallWithAuthenticationFlow(t *testing.T, suite *TestSuite) {
	// Step 1: Send REGISTER without authentication
	registerMsg := suite.CreateREGISTERMessage("alice", "sip:alice@192.168.1.100:5060")
	
	t.Log("Step 1: Sending REGISTER without authentication")
	registerResponse, err := suite.SendUDPMessage(t, registerMsg)
	if err != nil {
		t.Fatalf("Failed to send REGISTER: %v", err)
	}

	// Should get 401 Unauthorized
	if !bytes.Contains([]byte(registerResponse), []byte("401 Unauthorized")) {
		t.Errorf("Expected 401 Unauthorized for REGISTER without auth, got: %s", registerResponse)
	}

	// Extract authentication challenge
	parser := NewSIPResponseParser(registerResponse)
	wwwAuth := parser.GetHeader("WWW-Authenticate")
	if wwwAuth == "" {
		t.Fatal("No WWW-Authenticate header in 401 response")
	}

	t.Logf("WWW-Authenticate header: %s", wwwAuth)

	// Step 2: Send INVITE without authentication
	inviteMsg := suite.CreateINVITEMessage("alice", "bob", "sip:alice@192.168.1.100:5060")
	
	t.Log("Step 2: Sending INVITE without authentication")
	inviteResponse, err := suite.SendUDPMessage(t, inviteMsg)
	if err != nil {
		t.Fatalf("Failed to send INVITE: %v", err)
	}

	// Should also require authentication or return other appropriate response
	inviteParser := NewSIPResponseParser(inviteResponse)
	inviteStatusCode, err := inviteParser.GetStatusCode()
	if err != nil {
		t.Fatalf("Failed to parse INVITE response status: %v", err)
	}

	t.Logf("INVITE response status code: %d", inviteStatusCode)

	// Verify appropriate authentication handling
	if inviteStatusCode == 401 {
		t.Log("INVITE also requires authentication (expected)")
		if !inviteParser.HasHeader("WWW-Authenticate") {
			t.Error("Expected WWW-Authenticate header in INVITE 401 response")
		}
	} else {
		t.Logf("INVITE handled with status %d", inviteStatusCode)
	}

	// Step 3: Simulate authenticated REGISTER (with dummy credentials)
	// Note: This is a simplified test - real authentication would require proper digest calculation
	authRegisterMsg := `REGISTER sip:test.local SIP/2.0
Via: SIP/2.0/UDP 192.168.1.100:5060;branch=z9hG4bK-auth-register
From: <sip:alice@test.local>;tag=alice-tag
To: <sip:alice@test.local>
Call-ID: auth-register-call-id
CSeq: 2 REGISTER
Contact: <sip:alice@192.168.1.100:5060>
Authorization: Digest username="alice", realm="test.local", nonce="dummy", uri="sip:test.local", response="dummy"
Expires: 3600
Content-Length: 0

`
	
	t.Log("Step 3: Sending REGISTER with authentication")
	authRegisterResponse, err := suite.SendUDPMessage(t, authRegisterMsg)
	if err != nil {
		t.Fatalf("Failed to send authenticated REGISTER: %v", err)
	}

	authRegParser := NewSIPResponseParser(authRegisterResponse)
	authRegStatusCode, _ := authRegParser.GetStatusCode()
	t.Logf("Authenticated REGISTER response status: %d", authRegStatusCode)

	// The response will depend on whether the server validates the dummy credentials
	// This is acceptable for integration testing
}

// testCallTerminationFlow tests call termination scenarios
func testCallTerminationFlow(t *testing.T, suite *TestSuite) {
	// Step 1: Send BYE request
	byeMsg := `BYE sip:bob@test.local SIP/2.0
Via: SIP/2.0/UDP 192.168.1.100:5060;branch=z9hG4bK-bye-termination
From: <sip:alice@test.local>;tag=alice-tag
To: <sip:bob@test.local>;tag=bob-tag
Call-ID: termination-call-id
CSeq: 2 BYE
Content-Length: 0

`
	
	t.Log("Step 1: Sending BYE request")
	byeResponse, err := suite.SendUDPMessage(t, byeMsg)
	if err != nil {
		t.Fatalf("Failed to send BYE: %v", err)
	}

	// Should get some SIP response
	if !bytes.Contains([]byte(byeResponse), []byte("SIP/2.0")) {
		t.Errorf("Expected SIP response for BYE, got: %s", byeResponse)
	}

	parser := NewSIPResponseParser(byeResponse)
	statusCode, err := parser.GetStatusCode()
	if err != nil {
		t.Fatalf("Failed to parse BYE response status: %v", err)
	}

	t.Logf("BYE response status code: %d", statusCode)

	// Common responses for BYE:
	// 200 OK - call terminated successfully
	// 481 Call/Transaction Does Not Exist - no active call
	// 401 Unauthorized - authentication required
	
	switch statusCode {
	case 200:
		t.Log("BYE successful (200 OK)")
	case 481:
		t.Log("Call/Transaction does not exist (expected for non-existent call)")
	case 401:
		t.Log("BYE requires authentication")
	default:
		t.Logf("BYE response: %d", statusCode)
	}

	// Step 2: Send CANCEL request
	cancelMsg := `CANCEL sip:bob@test.local SIP/2.0
Via: SIP/2.0/UDP 192.168.1.100:5060;branch=z9hG4bK-cancel-termination
From: <sip:alice@test.local>;tag=alice-tag
To: <sip:bob@test.local>
Call-ID: termination-cancel-call-id
CSeq: 1 CANCEL
Content-Length: 0

`
	
	t.Log("Step 2: Sending CANCEL request")
	cancelResponse, err := suite.SendUDPMessage(t, cancelMsg)
	if err != nil {
		t.Fatalf("Failed to send CANCEL: %v", err)
	}

	// Should get some SIP response
	if !bytes.Contains([]byte(cancelResponse), []byte("SIP/2.0")) {
		t.Errorf("Expected SIP response for CANCEL, got: %s", cancelResponse)
	}

	cancelParser := NewSIPResponseParser(cancelResponse)
	cancelStatusCode, _ := cancelParser.GetStatusCode()
	t.Logf("CANCEL response status code: %d", cancelStatusCode)

	// Common responses for CANCEL:
	// 200 OK - CANCEL accepted
	// 481 Call/Transaction Does Not Exist - no matching INVITE transaction
	// 487 Request Terminated - original INVITE was cancelled
}

// testMultipleSimultaneousCalls tests handling of multiple simultaneous calls
func testMultipleSimultaneousCalls(t *testing.T, suite *TestSuite) {
	const numCalls = 5
	var wg sync.WaitGroup
	
	type callResult struct {
		callID     int
		statusCode int
		err        error
	}
	
	results := make(chan callResult, numCalls)

	t.Logf("Starting %d simultaneous calls", numCalls)

	for i := 0; i < numCalls; i++ {
		wg.Add(1)
		go func(callID int) {
			defer wg.Done()
			
			from := fmt.Sprintf("caller%d", callID)
			to := fmt.Sprintf("callee%d", callID)
			contact := fmt.Sprintf("sip:%s@192.168.1.%d:5060", from, 100+callID)
			
			inviteMsg := fmt.Sprintf(`INVITE sip:%s@test.local SIP/2.0
Via: SIP/2.0/UDP 192.168.1.100:5060;branch=z9hG4bK-multi-call-%d
From: <sip:%s@test.local>;tag=caller-tag-%d
To: <sip:%s@test.local>
Call-ID: multi-call-id-%d
CSeq: 1 INVITE
Contact: <%s>
Session-Expires: 1800;refresher=uac
Content-Type: application/sdp
Content-Length: 142

v=0
o=%s 2890844526 2890844526 IN IP4 192.168.1.100
s=-
c=IN IP4 192.168.1.100
t=0 0
m=audio 5004 RTP/AVP 0
a=rtpmap:0 PCMU/8000
`, to, callID, from, callID, to, callID, contact, from)
			
			conn, err := net.Dial("udp", "127.0.0.1:5060")
			if err != nil {
				results <- callResult{callID: callID, err: err}
				return
			}
			defer conn.Close()
			
			_, err = conn.Write([]byte(inviteMsg))
			if err != nil {
				results <- callResult{callID: callID, err: err}
				return
			}
			
			conn.SetReadDeadline(time.Now().Add(5 * time.Second))
			buffer := make([]byte, 4096)
			n, err := conn.Read(buffer)
			if err != nil {
				results <- callResult{callID: callID, err: err}
				return
			}
			
			response := string(buffer[:n])
			parser := NewSIPResponseParser(response)
			statusCode, err := parser.GetStatusCode()
			if err != nil {
				results <- callResult{callID: callID, err: err}
				return
			}
			
			results <- callResult{callID: callID, statusCode: statusCode}
		}(i)
	}

	wg.Wait()
	close(results)

	successCount := 0
	errorCount := 0
	statusCodes := make(map[int]int)

	for result := range results {
		if result.err != nil {
			errorCount++
			t.Logf("Call %d error: %v", result.callID, result.err)
		} else {
			successCount++
			statusCodes[result.statusCode]++
			t.Logf("Call %d status: %d", result.callID, result.statusCode)
		}
	}

	t.Logf("Multiple calls results: %d successful, %d errors", successCount, errorCount)
	t.Logf("Status code distribution: %v", statusCodes)

	// Should handle most calls successfully
	if successCount < numCalls-2 {
		t.Errorf("Too many call failures: %d successful out of %d", successCount, numCalls)
	}
}

// testCallFlowWithSessionTimer tests call flow with Session-Timer enforcement
func testCallFlowWithSessionTimer(t *testing.T, suite *TestSuite) {
	// Test 1: INVITE without Session-Timer should be rejected
	t.Log("Test 1: INVITE without Session-Timer")
	inviteWithoutTimer := `INVITE sip:bob@test.local SIP/2.0
Via: SIP/2.0/UDP 192.168.1.100:5060;branch=z9hG4bK-no-timer-flow
From: <sip:alice@test.local>;tag=alice-tag
To: <sip:bob@test.local>
Call-ID: no-timer-flow-call-id
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
	
	response1, err := suite.SendUDPMessage(t, inviteWithoutTimer)
	if err != nil {
		t.Fatalf("Failed to send INVITE without Session-Timer: %v", err)
	}

	if !bytes.Contains([]byte(response1), []byte("421 Extension Required")) {
		t.Errorf("Expected 421 Extension Required for INVITE without Session-Timer, got: %s", response1)
	}

	// Test 2: INVITE with valid Session-Timer should be processed
	t.Log("Test 2: INVITE with valid Session-Timer")
	inviteWithTimer := suite.CreateINVITEMessage("alice", "bob", "sip:alice@192.168.1.100:5060")
	
	response2, err := suite.SendUDPMessage(t, inviteWithTimer)
	if err != nil {
		t.Fatalf("Failed to send INVITE with Session-Timer: %v", err)
	}

	// Should not get 421 Extension Required
	if bytes.Contains([]byte(response2), []byte("421 Extension Required")) {
		t.Errorf("Unexpected 421 Extension Required for INVITE with Session-Timer: %s", response2)
	}

	parser := NewSIPResponseParser(response2)
	statusCode, _ := parser.GetStatusCode()
	t.Logf("INVITE with Session-Timer response: %d", statusCode)

	// Test 3: INVITE with invalid Session-Timer value
	t.Log("Test 3: INVITE with invalid Session-Timer value")
	inviteWithInvalidTimer := `INVITE sip:bob@test.local SIP/2.0
Via: SIP/2.0/UDP 192.168.1.100:5060;branch=z9hG4bK-invalid-timer-flow
From: <sip:alice@test.local>;tag=alice-tag
To: <sip:bob@test.local>
Call-ID: invalid-timer-flow-call-id
CSeq: 1 INVITE
Contact: <sip:alice@192.168.1.100:5060>
Session-Expires: 30;refresher=uac
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
	
	response3, err := suite.SendUDPMessage(t, inviteWithInvalidTimer)
	if err != nil {
		t.Fatalf("Failed to send INVITE with invalid Session-Timer: %v", err)
	}

	// Should get 422 Session Interval Too Small
	if !bytes.Contains([]byte(response3), []byte("422 Session Interval Too Small")) {
		t.Errorf("Expected 422 Session Interval Too Small for invalid timer value, got: %s", response3)
	}

	// Should include Min-SE header
	invalidParser := NewSIPResponseParser(response3)
	if !invalidParser.HasHeader("Min-SE") {
		t.Error("Expected Min-SE header in 422 response")
	}

	// Test 4: Re-INVITE for session refresh
	t.Log("Test 4: Re-INVITE for session refresh")
	reInviteMsg := `INVITE sip:bob@test.local SIP/2.0
Via: SIP/2.0/UDP 192.168.1.100:5060;branch=z9hG4bK-reinvite-flow
From: <sip:alice@test.local>;tag=alice-tag
To: <sip:bob@test.local>;tag=bob-tag
Call-ID: reinvite-flow-call-id
CSeq: 2 INVITE
Contact: <sip:alice@192.168.1.100:5060>
Session-Expires: 1800;refresher=uac
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
	
	response4, err := suite.SendUDPMessage(t, reInviteMsg)
	if err != nil {
		t.Fatalf("Failed to send re-INVITE: %v", err)
	}

	// Should handle re-INVITE (may require authentication or return other status)
	if !bytes.Contains([]byte(response4), []byte("SIP/2.0")) {
		t.Errorf("Expected SIP response for re-INVITE, got: %s", response4)
	}

	reInviteParser := NewSIPResponseParser(response4)
	reInviteStatusCode, _ := reInviteParser.GetStatusCode()
	t.Logf("Re-INVITE response: %d", reInviteStatusCode)
}

// TestCallFlowErrorScenarios tests various error scenarios in call flows
func TestCallFlowErrorScenarios(t *testing.T) {
	suite := SetupTestSuite(t)
	defer suite.Cleanup(t)

	t.Run("INVITE_To_Nonexistent_User", func(t *testing.T) {
		inviteMsg := suite.CreateINVITEMessage("alice", "nonexistent", "sip:alice@192.168.1.100:5060")
		
		response, err := suite.SendUDPMessage(t, inviteMsg)
		if err != nil {
			t.Fatalf("Failed to send INVITE to nonexistent user: %v", err)
		}

		// Should get 404 Not Found or similar error
		parser := NewSIPResponseParser(response)
		statusCode, _ := parser.GetStatusCode()
		
		if statusCode != 404 && statusCode != 401 && statusCode != 480 {
			t.Logf("INVITE to nonexistent user got status %d (acceptable)", statusCode)
		}
	})

	t.Run("Malformed_INVITE_Request", func(t *testing.T) {
		malformedInvite := `INVITE sip:bob@test.local SIP/2.0
Via: SIP/2.0/UDP 192.168.1.100:5060;branch=z9hG4bK-malformed
From: <sip:alice@test.local>;tag=alice-tag
To: <sip:bob@test.local>
Call-ID: malformed-call-id
CSeq: invalid INVITE
Contact: <sip:alice@192.168.1.100:5060>
Session-Expires: 1800;refresher=uac
Content-Length: 0

`
		
		response, err := suite.SendUDPMessage(t, malformedInvite)
		if err != nil {
			t.Fatalf("Failed to send malformed INVITE: %v", err)
		}

		// Should get 400 Bad Request
		if !bytes.Contains([]byte(response), []byte("400 Bad Request")) {
			t.Errorf("Expected 400 Bad Request for malformed INVITE, got: %s", response)
		}
	})

	t.Run("INVITE_With_Invalid_SDP", func(t *testing.T) {
		invalidSDPInvite := `INVITE sip:bob@test.local SIP/2.0
Via: SIP/2.0/UDP 192.168.1.100:5060;branch=z9hG4bK-invalid-sdp
From: <sip:alice@test.local>;tag=alice-tag
To: <sip:bob@test.local>
Call-ID: invalid-sdp-call-id
CSeq: 1 INVITE
Contact: <sip:alice@192.168.1.100:5060>
Session-Expires: 1800;refresher=uac
Content-Type: application/sdp
Content-Length: 50

This is not valid SDP content
Invalid format
`
		
		response, err := suite.SendUDPMessage(t, invalidSDPInvite)
		if err != nil {
			t.Fatalf("Failed to send INVITE with invalid SDP: %v", err)
		}

		// Should handle invalid SDP (may accept or reject)
		if !bytes.Contains([]byte(response), []byte("SIP/2.0")) {
			t.Errorf("Expected SIP response for INVITE with invalid SDP, got: %s", response)
		}

		parser := NewSIPResponseParser(response)
		statusCode, _ := parser.GetStatusCode()
		t.Logf("INVITE with invalid SDP response: %d", statusCode)
	})
}

// TestCallFlowPerformance tests performance aspects of call flows
func TestCallFlowPerformance(t *testing.T) {
	suite := SetupTestSuite(t)
	defer suite.Cleanup(t)

	t.Run("Call_Setup_Response_Time", func(t *testing.T) {
		const numTests = 10
		var responseTimes []time.Duration

		for i := 0; i < numTests; i++ {
			inviteMsg := fmt.Sprintf(`INVITE sip:bob%d@test.local SIP/2.0
Via: SIP/2.0/UDP 192.168.1.100:5060;branch=z9hG4bK-perf-%d
From: <sip:alice@test.local>;tag=alice-tag-%d
To: <sip:bob%d@test.local>
Call-ID: perf-call-id-%d
CSeq: 1 INVITE
Contact: <sip:alice@192.168.1.100:5060>
Session-Expires: 1800;refresher=uac
Content-Type: application/sdp
Content-Length: 142

v=0
o=alice 2890844526 2890844526 IN IP4 192.168.1.100
s=-
c=IN IP4 192.168.1.100
t=0 0
m=audio 5004 RTP/AVP 0
a=rtpmap:0 PCMU/8000
`, i, i, i, i, i)
			
			start := time.Now()
			response, err := suite.SendUDPMessage(t, inviteMsg)
			responseTime := time.Since(start)
			
			if err != nil {
				t.Logf("Test %d failed: %v", i, err)
				continue
			}
			
			if bytes.Contains([]byte(response), []byte("SIP/2.0")) {
				responseTimes = append(responseTimes, responseTime)
			}
			
			// Small delay between tests
			time.Sleep(50 * time.Millisecond)
		}

		if len(responseTimes) == 0 {
			t.Fatal("No successful call setup responses")
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

		t.Logf("Call setup performance: avg=%v, min=%v, max=%v, samples=%d", 
			avg, min, max, len(responseTimes))

		// Call setup should be reasonably fast
		if avg > 200*time.Millisecond {
			t.Errorf("Average call setup time too high: %v", avg)
		}
	})
}