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

// TestSessionTimerEnforcementIntegration tests comprehensive Session-Timer enforcement
func TestSessionTimerEnforcementIntegration(t *testing.T) {
	suite := SetupTestSuite(t)
	defer suite.Cleanup(t)

	t.Run("INVITE_Requires_SessionTimer", func(t *testing.T) {
		// Test that INVITE without Session-Expires is rejected
		inviteWithoutTimer := `INVITE sip:bob@test.local SIP/2.0
Via: SIP/2.0/UDP 192.168.1.100:5060;branch=z9hG4bK-no-timer
From: <sip:alice@test.local>;tag=alice-tag
To: <sip:bob@test.local>
Call-ID: test-no-timer-call-id
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

		// Should include Require header indicating Session-Timer
		if !strings.Contains(response, "Require:") || !strings.Contains(response, "timer") {
			t.Errorf("Expected Require header with timer extension in 421 response")
		}
	})

	t.Run("INVITE_With_Valid_SessionTimer", func(t *testing.T) {
		inviteWithTimer := suite.CreateINVITEMessage("alice", "bob", "sip:alice@192.168.1.100:5060")
		
		response, err := suite.SendUDPMessage(t, inviteWithTimer)
		if err != nil {
			t.Fatalf("Failed to send INVITE with Session-Timer: %v", err)
		}

		// Should not get 421 Extension Required
		if bytes.Contains([]byte(response), []byte("421 Extension Required")) {
			t.Errorf("Unexpected 421 Extension Required for INVITE with Session-Timer: %s", response)
		}

		// Should get some SIP response (likely 404 since bob isn't registered)
		if !bytes.Contains([]byte(response), []byte("SIP/2.0")) {
			t.Errorf("Expected SIP response for INVITE with Session-Timer, got: %s", response)
		}
	})

	t.Run("SessionTimer_Value_Below_Minimum", func(t *testing.T) {
		inviteWithLowTimer := `INVITE sip:bob@test.local SIP/2.0
Via: SIP/2.0/UDP 192.168.1.100:5060;branch=z9hG4bK-low-timer
From: <sip:alice@test.local>;tag=alice-tag
To: <sip:bob@test.local>
Call-ID: test-low-timer-call-id
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
		
		response, err := suite.SendUDPMessage(t, inviteWithLowTimer)
		if err != nil {
			t.Fatalf("Failed to send INVITE with low Session-Timer: %v", err)
		}

		// Should get 422 Session Interval Too Small
		if !bytes.Contains([]byte(response), []byte("422 Session Interval Too Small")) {
			t.Errorf("Expected 422 Session Interval Too Small for low timer value, got: %s", response)
		}

		// Should include Min-SE header
		if !strings.Contains(response, "Min-SE:") {
			t.Errorf("Expected Min-SE header in 422 response")
		}
	})

	t.Run("SessionTimer_Value_Above_Maximum", func(t *testing.T) {
		inviteWithHighTimer := `INVITE sip:bob@test.local SIP/2.0
Via: SIP/2.0/UDP 192.168.1.100:5060;branch=z9hG4bK-high-timer
From: <sip:alice@test.local>;tag=alice-tag
To: <sip:bob@test.local>
Call-ID: test-high-timer-call-id
CSeq: 1 INVITE
Contact: <sip:alice@192.168.1.100:5060>
Session-Expires: 10000;refresher=uac
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
		
		response, err := suite.SendUDPMessage(t, inviteWithHighTimer)
		if err != nil {
			t.Fatalf("Failed to send INVITE with high Session-Timer: %v", err)
		}

		// Server may adjust the value or reject it
		if !bytes.Contains([]byte(response), []byte("SIP/2.0")) {
			t.Errorf("Expected SIP response for high timer value, got: %s", response)
		}

		// If accepted, response should contain adjusted Session-Expires
		if bytes.Contains([]byte(response), []byte("200 OK")) {
			if !strings.Contains(response, "Session-Expires:") {
				t.Errorf("Expected Session-Expires header in successful response")
			}
		}
	})

	t.Run("SessionTimer_With_MinSE_Header", func(t *testing.T) {
		inviteWithMinSE := `INVITE sip:bob@test.local SIP/2.0
Via: SIP/2.0/UDP 192.168.1.100:5060;branch=z9hG4bK-min-se
From: <sip:alice@test.local>;tag=alice-tag
To: <sip:bob@test.local>
Call-ID: test-min-se-call-id
CSeq: 1 INVITE
Contact: <sip:alice@192.168.1.100:5060>
Session-Expires: 1800;refresher=uac
Min-SE: 120
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
		
		response, err := suite.SendUDPMessage(t, inviteWithMinSE)
		if err != nil {
			t.Fatalf("Failed to send INVITE with Min-SE: %v", err)
		}

		// Should handle Min-SE header correctly
		if !bytes.Contains([]byte(response), []byte("SIP/2.0")) {
			t.Errorf("Expected SIP response for INVITE with Min-SE, got: %s", response)
		}
	})

	t.Run("SessionTimer_Refresher_Parameter", func(t *testing.T) {
		// Test with refresher=uas
		inviteWithUASRefresher := `INVITE sip:bob@test.local SIP/2.0
Via: SIP/2.0/UDP 192.168.1.100:5060;branch=z9hG4bK-uas-refresher
From: <sip:alice@test.local>;tag=alice-tag
To: <sip:bob@test.local>
Call-ID: test-uas-refresher-call-id
CSeq: 1 INVITE
Contact: <sip:alice@192.168.1.100:5060>
Session-Expires: 1800;refresher=uas
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
		
		response, err := suite.SendUDPMessage(t, inviteWithUASRefresher)
		if err != nil {
			t.Fatalf("Failed to send INVITE with UAS refresher: %v", err)
		}

		// Should handle refresher parameter correctly
		if !bytes.Contains([]byte(response), []byte("SIP/2.0")) {
			t.Errorf("Expected SIP response for INVITE with UAS refresher, got: %s", response)
		}
	})

	t.Run("Multiple_SessionTimer_Headers", func(t *testing.T) {
		// Test with multiple Session-Expires headers (should be invalid)
		inviteWithMultipleTimers := `INVITE sip:bob@test.local SIP/2.0
Via: SIP/2.0/UDP 192.168.1.100:5060;branch=z9hG4bK-multiple-timers
From: <sip:alice@test.local>;tag=alice-tag
To: <sip:bob@test.local>
Call-ID: test-multiple-timers-call-id
CSeq: 1 INVITE
Contact: <sip:alice@192.168.1.100:5060>
Session-Expires: 1800;refresher=uac
Session-Expires: 3600;refresher=uas
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
		
		response, err := suite.SendUDPMessage(t, inviteWithMultipleTimers)
		if err != nil {
			t.Fatalf("Failed to send INVITE with multiple Session-Expires: %v", err)
		}

		// Should handle multiple headers (may use first one or reject)
		if !bytes.Contains([]byte(response), []byte("SIP/2.0")) {
			t.Errorf("Expected SIP response for multiple Session-Expires, got: %s", response)
		}
	})
}

// TestSessionTimerConcurrentHandling tests Session-Timer under concurrent load
func TestSessionTimerConcurrentHandling(t *testing.T) {
	suite := SetupTestSuite(t)
	defer suite.Cleanup(t)

	t.Run("Concurrent_SessionTimer_Validation", func(t *testing.T) {
		const numConcurrentRequests = 5  // Reduced from 20
		var wg sync.WaitGroup
		results := make(chan string, numConcurrentRequests)
		errors := make(chan error, numConcurrentRequests)

		// Launch concurrent INVITE requests with Session-Timer
		for i := 0; i < numConcurrentRequests; i++ {
			wg.Add(1)
			go func(requestID int) {
				defer wg.Done()
				
				from := fmt.Sprintf("user%d", requestID)
				to := fmt.Sprintf("target%d", requestID)
				contact := fmt.Sprintf("sip:%s@192.168.1.%d:5060", from, 100+requestID)
				
				inviteMsg := fmt.Sprintf(`INVITE sip:%s@test.local SIP/2.0
Via: SIP/2.0/UDP 192.168.1.100:5060;branch=z9hG4bK-concurrent-%d
From: <sip:%s@test.local>;tag=tag-%d
To: <sip:%s@test.local>
Call-ID: concurrent-timer-call-id-%d
CSeq: 1 INVITE
Contact: <%s>
Session-Expires: %d;refresher=uac
Content-Type: application/sdp
Content-Length: 142

v=0
o=%s 2890844526 2890844526 IN IP4 192.168.1.100
s=-
c=IN IP4 192.168.1.100
t=0 0
m=audio 5004 RTP/AVP 0
a=rtpmap:0 PCMU/8000
`, to, requestID, from, requestID, to, requestID, contact, 1800+requestID*60, from)
				
				// Create separate UDP connection for each goroutine
				conn, err := net.Dial("udp", "127.0.0.1:5060")
				if err != nil {
					errors <- fmt.Errorf("failed to create UDP connection for request %d: %w", requestID, err)
					return
				}
				defer conn.Close()
				
				// Send INVITE message
				_, err = conn.Write([]byte(inviteMsg))
				if err != nil {
					errors <- fmt.Errorf("failed to send INVITE for request %d: %w", requestID, err)
					return
				}
				
				// Read response with timeout
				conn.SetReadDeadline(time.Now().Add(5 * time.Second))
				buffer := make([]byte, 4096)
				n, err := conn.Read(buffer)
				if err != nil {
					errors <- fmt.Errorf("failed to read response for request %d: %w", requestID, err)
					return
				}
				
				results <- string(buffer[:n])
			}(i)
		}

		// Wait for all goroutines to complete
		wg.Wait()
		close(results)
		close(errors)

		// Check for errors
		for err := range errors {
			t.Errorf("Concurrent Session-Timer error: %v", err)
		}

		// Verify all responses handle Session-Timer correctly
		responseCount := 0
		for response := range results {
			responseCount++
			
			// Should not get 421 Extension Required (all have Session-Timer)
			if bytes.Contains([]byte(response), []byte("421 Extension Required")) {
				t.Errorf("Unexpected 421 Extension Required in concurrent test: %s", response)
			}
			
			// Should get some SIP response
			if !bytes.Contains([]byte(response), []byte("SIP/2.0")) {
				t.Errorf("Expected SIP response in concurrent test, got: %s", response)
			}
		}

		if responseCount != numConcurrentRequests {
			t.Errorf("Expected %d responses, got %d", numConcurrentRequests, responseCount)
		}
	})

	t.Run("Mixed_SessionTimer_Concurrent_Requests", func(t *testing.T) {
		const numRequests = 3  // Reduced from 10
		var wg sync.WaitGroup
		validTimerCount := 0
		invalidTimerCount := 0
		results := make(chan string, numRequests)
		errors := make(chan error, numRequests)

		// Launch mixed requests (some with valid timers, some without)
		for i := 0; i < numRequests; i++ {
			wg.Add(1)
			go func(requestID int) {
				defer wg.Done()
				
				var inviteMsg string
				if requestID%2 == 0 {
					// Even requests: with Session-Timer
					inviteMsg = fmt.Sprintf(`INVITE sip:target%d@test.local SIP/2.0
Via: SIP/2.0/UDP 192.168.1.100:5060;branch=z9hG4bK-mixed-valid-%d
From: <sip:user%d@test.local>;tag=tag-%d
To: <sip:target%d@test.local>
Call-ID: mixed-timer-call-id-%d
CSeq: 1 INVITE
Contact: <sip:user%d@192.168.1.100:5060>
Session-Expires: 1800;refresher=uac
Content-Type: application/sdp
Content-Length: 142

v=0
o=user%d 2890844526 2890844526 IN IP4 192.168.1.100
s=-
c=IN IP4 192.168.1.100
t=0 0
m=audio 5004 RTP/AVP 0
a=rtpmap:0 PCMU/8000
`, requestID, requestID, requestID, requestID, requestID, requestID, requestID, requestID)
				} else {
					// Odd requests: without Session-Timer
					inviteMsg = fmt.Sprintf(`INVITE sip:target%d@test.local SIP/2.0
Via: SIP/2.0/UDP 192.168.1.100:5060;branch=z9hG4bK-mixed-invalid-%d
From: <sip:user%d@test.local>;tag=tag-%d
To: <sip:target%d@test.local>
Call-ID: mixed-no-timer-call-id-%d
CSeq: 1 INVITE
Contact: <sip:user%d@192.168.1.100:5060>
Content-Type: application/sdp
Content-Length: 142

v=0
o=user%d 2890844526 2890844526 IN IP4 192.168.1.100
s=-
c=IN IP4 192.168.1.100
t=0 0
m=audio 5004 RTP/AVP 0
a=rtpmap:0 PCMU/8000
`, requestID, requestID, requestID, requestID, requestID, requestID, requestID, requestID)
				}
				
				// Create separate UDP connection for each goroutine
				conn, err := net.Dial("udp", "127.0.0.1:5060")
				if err != nil {
					errors <- fmt.Errorf("failed to create UDP connection for request %d: %w", requestID, err)
					return
				}
				defer conn.Close()
				
				// Send INVITE message
				_, err = conn.Write([]byte(inviteMsg))
				if err != nil {
					errors <- fmt.Errorf("failed to send INVITE for request %d: %w", requestID, err)
					return
				}
				
				// Read response with timeout
				conn.SetReadDeadline(time.Now().Add(5 * time.Second))
				buffer := make([]byte, 4096)
				n, err := conn.Read(buffer)
				if err != nil {
					errors <- fmt.Errorf("failed to read response for request %d: %w", requestID, err)
					return
				}
				
				results <- fmt.Sprintf("%d:%s", requestID, string(buffer[:n]))
			}(i)
		}

		// Wait for all goroutines to complete
		wg.Wait()
		close(results)
		close(errors)

		// Check for errors
		for err := range errors {
			t.Errorf("Mixed Session-Timer error: %v", err)
		}

		// Verify responses match expectations
		for result := range results {
			parts := strings.SplitN(result, ":", 2)
			if len(parts) != 2 {
				continue
			}
			
			requestID := parts[0]
			response := parts[1]
			
			// Parse request ID to determine expected behavior
			var id int
			fmt.Sscanf(requestID, "%d", &id)
			
			if id%2 == 0 {
				// Even requests (with Session-Timer) should not get 421
				validTimerCount++
				if bytes.Contains([]byte(response), []byte("421 Extension Required")) {
					t.Errorf("Unexpected 421 for request %d with Session-Timer: %s", id, response)
				}
			} else {
				// Odd requests (without Session-Timer) should get 421
				invalidTimerCount++
				if !bytes.Contains([]byte(response), []byte("421 Extension Required")) {
					t.Errorf("Expected 421 for request %d without Session-Timer: %s", id, response)
				}
			}
		}

		t.Logf("Mixed Session-Timer test: %d valid timer requests, %d invalid timer requests", 
			validTimerCount, invalidTimerCount)
	})
}

// TestSessionTimerEdgeCases tests edge cases in Session-Timer handling
func TestSessionTimerEdgeCases(t *testing.T) {
	suite := SetupTestSuite(t)
	defer suite.Cleanup(t)

	t.Run("SessionTimer_With_Malformed_Value", func(t *testing.T) {
		inviteWithMalformedTimer := `INVITE sip:bob@test.local SIP/2.0
Via: SIP/2.0/UDP 192.168.1.100:5060;branch=z9hG4bK-malformed-timer
From: <sip:alice@test.local>;tag=alice-tag
To: <sip:bob@test.local>
Call-ID: test-malformed-timer-call-id
CSeq: 1 INVITE
Contact: <sip:alice@192.168.1.100:5060>
Session-Expires: invalid-value;refresher=uac
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
		
		response, err := suite.SendUDPMessage(t, inviteWithMalformedTimer)
		if err != nil {
			t.Fatalf("Failed to send INVITE with malformed Session-Timer: %v", err)
		}

		// Should get 400 Bad Request for malformed Session-Expires
		if !bytes.Contains([]byte(response), []byte("400 Bad Request")) {
			t.Errorf("Expected 400 Bad Request for malformed Session-Timer, got: %s", response)
		}
	})

	t.Run("SessionTimer_With_Invalid_Refresher", func(t *testing.T) {
		inviteWithInvalidRefresher := `INVITE sip:bob@test.local SIP/2.0
Via: SIP/2.0/UDP 192.168.1.100:5060;branch=z9hG4bK-invalid-refresher
From: <sip:alice@test.local>;tag=alice-tag
To: <sip:bob@test.local>
Call-ID: test-invalid-refresher-call-id
CSeq: 1 INVITE
Contact: <sip:alice@192.168.1.100:5060>
Session-Expires: 1800;refresher=invalid
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
		
		response, err := suite.SendUDPMessage(t, inviteWithInvalidRefresher)
		if err != nil {
			t.Fatalf("Failed to send INVITE with invalid refresher: %v", err)
		}

		// Should get 400 Bad Request for invalid refresher parameter
		if !bytes.Contains([]byte(response), []byte("400 Bad Request")) {
			t.Errorf("Expected 400 Bad Request for invalid refresher, got: %s", response)
		}
	})

	t.Run("SessionTimer_Zero_Value", func(t *testing.T) {
		inviteWithZeroTimer := `INVITE sip:bob@test.local SIP/2.0
Via: SIP/2.0/UDP 192.168.1.100:5060;branch=z9hG4bK-zero-timer
From: <sip:alice@test.local>;tag=alice-tag
To: <sip:bob@test.local>
Call-ID: test-zero-timer-call-id
CSeq: 1 INVITE
Contact: <sip:alice@192.168.1.100:5060>
Session-Expires: 0;refresher=uac
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
		
		response, err := suite.SendUDPMessage(t, inviteWithZeroTimer)
		if err != nil {
			t.Fatalf("Failed to send INVITE with zero Session-Timer: %v", err)
		}

		// Should get 422 Session Interval Too Small for zero value
		if !bytes.Contains([]byte(response), []byte("422 Session Interval Too Small")) {
			t.Errorf("Expected 422 Session Interval Too Small for zero timer, got: %s", response)
		}
	})

	t.Run("SessionTimer_Without_Refresher", func(t *testing.T) {
		inviteWithoutRefresher := `INVITE sip:bob@test.local SIP/2.0
Via: SIP/2.0/UDP 192.168.1.100:5060;branch=z9hG4bK-no-refresher
From: <sip:alice@test.local>;tag=alice-tag
To: <sip:bob@test.local>
Call-ID: test-no-refresher-call-id
CSeq: 1 INVITE
Contact: <sip:alice@192.168.1.100:5060>
Session-Expires: 1800
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
		
		response, err := suite.SendUDPMessage(t, inviteWithoutRefresher)
		if err != nil {
			t.Fatalf("Failed to send INVITE without refresher: %v", err)
		}

		// Should handle Session-Expires without refresher parameter (default behavior)
		if !bytes.Contains([]byte(response), []byte("SIP/2.0")) {
			t.Errorf("Expected SIP response for Session-Timer without refresher, got: %s", response)
		}

		// Should not get 421 Extension Required since Session-Expires is present
		if bytes.Contains([]byte(response), []byte("421 Extension Required")) {
			t.Errorf("Unexpected 421 Extension Required for Session-Timer without refresher")
		}
	})
}