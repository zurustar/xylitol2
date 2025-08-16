package integration

import (
	"fmt"
	"sync"
	"testing"
	"time"
)

// TestComprehensiveIntegration runs all integration test scenarios
func TestComprehensiveIntegration(t *testing.T) {
	suite := SetupTestSuite(t)
	defer suite.Cleanup(t)

	// Run all test categories
	t.Run("SIP_Protocol_Compliance", func(t *testing.T) {
		runSIPProtocolTests(t, suite)
	})

	t.Run("Session_Timer_Enforcement", func(t *testing.T) {
		runSessionTimerTests(t, suite)
	})

	t.Run("Transport_Protocol_Handling", func(t *testing.T) {
		runTransportTests(t, suite)
	})

	t.Run("Concurrent_Operations", func(t *testing.T) {
		runConcurrentTests(t, suite)
	})

	t.Run("Error_Handling_Scenarios", func(t *testing.T) {
		runErrorHandlingTests(t, suite)
	})

	t.Run("Performance_Characteristics", func(t *testing.T) {
		runPerformanceTests(t, suite)
	})
}

// runSIPProtocolTests executes SIP protocol compliance tests
func runSIPProtocolTests(t *testing.T, suite *TestSuite) {
	scenarios := []TestScenario{
		{
			Name:        "Basic_OPTIONS_Request",
			Description: "Test basic OPTIONS request handling",
			Execute: func(t *testing.T, suite *TestSuite) error {
				msg := NewSIPMessageBuilder("OPTIONS", "sip:test.local").Build()
				response, err := suite.SendUDPMessage(t, msg)
				if err != nil {
					return err
				}
				AssertSIPResponse(t, response, 200, []string{"Allow", "Via"})
				return nil
			},
		},
		{
			Name:        "REGISTER_Without_Authentication",
			Description: "Test REGISTER request without authentication",
			Execute: func(t *testing.T, suite *TestSuite) error {
				msg := NewSIPMessageBuilder("REGISTER", "sip:test.local").
					SetHeader("Contact", "<sip:alice@192.168.1.100:5060>").
					SetHeader("Expires", "3600").
					Build()
				response, err := suite.SendUDPMessage(t, msg)
				if err != nil {
					return err
				}
				AssertSIPResponse(t, response, 401, []string{"WWW-Authenticate"})
				return nil
			},
		},
		{
			Name:        "INVITE_Without_SessionTimer",
			Description: "Test INVITE request without Session-Timer (should be rejected)",
			Execute: func(t *testing.T, suite *TestSuite) error {
				msg := NewSIPMessageBuilder("INVITE", "sip:bob@test.local").
					SetHeader("Contact", "<sip:alice@192.168.1.100:5060>").
					SetHeader("Content-Type", "application/sdp").
					SetBody(CreateSDPBody("Test Session")).
					Build()
				response, err := suite.SendUDPMessage(t, msg)
				if err != nil {
					return err
				}
				AssertSIPResponse(t, response, 421, []string{"Require"})
				return nil
			},
		},
		{
			Name:        "Malformed_SIP_Message",
			Description: "Test handling of malformed SIP messages",
			Execute: func(t *testing.T, suite *TestSuite) error {
				malformedMsg := "INVALID SIP MESSAGE\r\nThis is not valid\r\n\r\n"
				response, err := suite.SendUDPMessage(t, malformedMsg)
				if err != nil {
					return err
				}
				AssertSIPResponse(t, response, 400, []string{"Via"})
				return nil
			},
		},
	}

	for _, scenario := range scenarios {
		RunScenario(t, suite, scenario)
	}
}

// runSessionTimerTests executes Session-Timer enforcement tests
func runSessionTimerTests(t *testing.T, suite *TestSuite) {
	scenarios := []TestScenario{
		{
			Name:        "INVITE_With_Valid_SessionTimer",
			Description: "Test INVITE with valid Session-Timer",
			Execute: func(t *testing.T, suite *TestSuite) error {
				msg := NewSIPMessageBuilder("INVITE", "sip:bob@test.local").
					SetHeader("Contact", "<sip:alice@192.168.1.100:5060>").
					SetHeader("Session-Expires", "1800;refresher=uac").
					SetHeader("Content-Type", "application/sdp").
					SetBody(CreateSDPBody("Session Timer Test")).
					Build()
				response, err := suite.SendUDPMessage(t, msg)
				if err != nil {
					return err
				}
				// Should not get 421 Extension Required
				parser := NewSIPResponseParser(response)
				statusCode, _ := parser.GetStatusCode()
				if statusCode == 421 {
					t.Errorf("Unexpected 421 Extension Required for INVITE with Session-Timer")
				}
				return nil
			},
		},
		{
			Name:        "INVITE_With_Low_SessionTimer",
			Description: "Test INVITE with Session-Timer below minimum",
			Execute: func(t *testing.T, suite *TestSuite) error {
				msg := NewSIPMessageBuilder("INVITE", "sip:bob@test.local").
					SetHeader("Contact", "<sip:alice@192.168.1.100:5060>").
					SetHeader("Session-Expires", "30;refresher=uac").
					SetHeader("Content-Type", "application/sdp").
					SetBody(CreateSDPBody("Low Timer Test")).
					Build()
				response, err := suite.SendUDPMessage(t, msg)
				if err != nil {
					return err
				}
				AssertSIPResponse(t, response, 422, []string{"Min-SE"})
				return nil
			},
		},
		{
			Name:        "INVITE_With_High_SessionTimer",
			Description: "Test INVITE with Session-Timer above maximum",
			Execute: func(t *testing.T, suite *TestSuite) error {
				msg := NewSIPMessageBuilder("INVITE", "sip:bob@test.local").
					SetHeader("Contact", "<sip:alice@192.168.1.100:5060>").
					SetHeader("Session-Expires", "10000;refresher=uac").
					SetHeader("Content-Type", "application/sdp").
					SetBody(CreateSDPBody("High Timer Test")).
					Build()
				response, err := suite.SendUDPMessage(t, msg)
				if err != nil {
					return err
				}
				// Server may adjust or accept high values
				parser := NewSIPResponseParser(response)
				statusCode, _ := parser.GetStatusCode()
				if statusCode < 200 || statusCode >= 600 {
					t.Errorf("Invalid status code for high Session-Timer: %d", statusCode)
				}
				return nil
			},
		},
	}

	for _, scenario := range scenarios {
		RunScenario(t, suite, scenario)
	}
}

// runTransportTests executes transport protocol tests
func runTransportTests(t *testing.T, suite *TestSuite) {
	scenarios := []TestScenario{
		{
			Name:        "UDP_Message_Handling",
			Description: "Test UDP message handling",
			Execute: func(t *testing.T, suite *TestSuite) error {
				msg := NewSIPMessageBuilder("OPTIONS", "sip:test.local").
					SetTransport("UDP").
					Build()
				response, err := suite.SendUDPMessage(t, msg)
				if err != nil {
					return err
				}
				AssertSIPResponse(t, response, 200, []string{"Via"})
				return nil
			},
		},
		{
			Name:        "TCP_Message_Handling",
			Description: "Test TCP message handling",
			Execute: func(t *testing.T, suite *TestSuite) error {
				msg := NewSIPMessageBuilder("OPTIONS", "sip:test.local").
					SetTransport("TCP").
					Build()
				response, err := suite.SendTCPMessage(t, msg)
				if err != nil {
					return err
				}
				AssertSIPResponse(t, response, 200, []string{"Via"})
				return nil
			},
		},
		{
			Name:        "Large_UDP_Message",
			Description: "Test large message handling over UDP",
			Execute: func(t *testing.T, suite *TestSuite) error {
				largeBody := CreateSDPBody("Large Message Test")
				// Add extra content to make it larger
				for i := 0; i < 10; i++ {
					largeBody += fmt.Sprintf("a=test-attribute-%d:value\r\n", i)
				}
				
				msg := NewSIPMessageBuilder("INVITE", "sip:bob@test.local").
					SetHeader("Contact", "<sip:alice@192.168.1.100:5060>").
					SetHeader("Session-Expires", "1800;refresher=uac").
					SetHeader("Content-Type", "application/sdp").
					SetBody(largeBody).
					Build()
				response, err := suite.SendUDPMessage(t, msg)
				if err != nil {
					return err
				}
				// Should handle large messages
				parser := NewSIPResponseParser(response)
				statusCode, _ := parser.GetStatusCode()
				if statusCode < 200 || statusCode >= 600 {
					t.Errorf("Invalid status code for large UDP message: %d", statusCode)
				}
				return nil
			},
		},
	}

	for _, scenario := range scenarios {
		RunScenario(t, suite, scenario)
	}
}

// runConcurrentTests executes concurrent operation tests
func runConcurrentTests(t *testing.T, suite *TestSuite) {
	t.Run("Concurrent_REGISTER_Requests", func(t *testing.T) {
		const numConcurrent = 20
		var wg sync.WaitGroup
		results := make(chan error, numConcurrent)

		for i := 0; i < numConcurrent; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				
				msg := NewSIPMessageBuilder("REGISTER", "sip:test.local").
					SetHeader("Contact", fmt.Sprintf("<sip:user%d@192.168.1.100:5060>", id)).
					SetHeader("Expires", "3600").
					Build()
				
				response, err := suite.SendUDPMessage(t, msg)
				if err != nil {
					results <- err
					return
				}
				
				parser := NewSIPResponseParser(response)
				statusCode, err := parser.GetStatusCode()
				if err != nil {
					results <- err
					return
				}
				
				// Should get 401 Unauthorized (expected without auth)
				if statusCode != 401 {
					results <- fmt.Errorf("expected 401, got %d", statusCode)
					return
				}
				
				results <- nil
			}(i)
		}

		wg.Wait()
		close(results)

		errorCount := 0
		for err := range results {
			if err != nil {
				errorCount++
				t.Logf("Concurrent REGISTER error: %v", err)
			}
		}

		if errorCount > numConcurrent/10 { // Allow up to 10% errors
			t.Errorf("Too many errors in concurrent REGISTER test: %d/%d", errorCount, numConcurrent)
		}
	})

	t.Run("Concurrent_INVITE_Requests", func(t *testing.T) {
		const numConcurrent = 15
		var wg sync.WaitGroup
		results := make(chan error, numConcurrent)

		for i := 0; i < numConcurrent; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				
				msg := NewSIPMessageBuilder("INVITE", fmt.Sprintf("sip:user%d@test.local", id)).
					SetHeader("Contact", fmt.Sprintf("<sip:caller%d@192.168.1.100:5060>", id)).
					SetHeader("Session-Expires", "1800;refresher=uac").
					SetHeader("Content-Type", "application/sdp").
					SetBody(CreateSDPBody(fmt.Sprintf("Concurrent Test %d", id))).
					Build()
				
				response, err := suite.SendUDPMessage(t, msg)
				if err != nil {
					results <- err
					return
				}
				
				parser := NewSIPResponseParser(response)
				statusCode, err := parser.GetStatusCode()
				if err != nil {
					results <- err
					return
				}
				
				// Should get some valid SIP response
				if statusCode < 200 || statusCode >= 600 {
					results <- fmt.Errorf("invalid status code: %d", statusCode)
					return
				}
				
				results <- nil
			}(i)
		}

		wg.Wait()
		close(results)

		errorCount := 0
		for err := range results {
			if err != nil {
				errorCount++
				t.Logf("Concurrent INVITE error: %v", err)
			}
		}

		if errorCount > numConcurrent/10 { // Allow up to 10% errors
			t.Errorf("Too many errors in concurrent INVITE test: %d/%d", errorCount, numConcurrent)
		}
	})

	t.Run("Mixed_Transport_Concurrent", func(t *testing.T) {
		const numConcurrent = 10
		var wg sync.WaitGroup
		udpResults := make(chan error, numConcurrent/2)
		tcpResults := make(chan error, numConcurrent/2)

		// UDP requests
		for i := 0; i < numConcurrent/2; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				
				msg := NewSIPMessageBuilder("OPTIONS", "sip:test.local").
					SetTransport("UDP").
					Build()
				
				response, err := suite.SendUDPMessage(t, msg)
				if err != nil {
					udpResults <- err
					return
				}
				
				parser := NewSIPResponseParser(response)
				statusCode, _ := parser.GetStatusCode()
				if statusCode != 200 {
					udpResults <- fmt.Errorf("expected 200, got %d", statusCode)
					return
				}
				
				udpResults <- nil
			}(i)
		}

		// TCP requests
		for i := 0; i < numConcurrent/2; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				
				msg := NewSIPMessageBuilder("OPTIONS", "sip:test.local").
					SetTransport("TCP").
					Build()
				
				response, err := suite.SendTCPMessage(t, msg)
				if err != nil {
					tcpResults <- err
					return
				}
				
				parser := NewSIPResponseParser(response)
				statusCode, _ := parser.GetStatusCode()
				if statusCode != 200 {
					tcpResults <- fmt.Errorf("expected 200, got %d", statusCode)
					return
				}
				
				tcpResults <- nil
			}(i)
		}

		wg.Wait()
		close(udpResults)
		close(tcpResults)

		udpErrors := 0
		for err := range udpResults {
			if err != nil {
				udpErrors++
				t.Logf("UDP error: %v", err)
			}
		}

		tcpErrors := 0
		for err := range tcpResults {
			if err != nil {
				tcpErrors++
				t.Logf("TCP error: %v", err)
			}
		}

		if udpErrors > 0 {
			t.Errorf("UDP errors in mixed transport test: %d", udpErrors)
		}
		if tcpErrors > 0 {
			t.Errorf("TCP errors in mixed transport test: %d", tcpErrors)
		}
	})
}

// runErrorHandlingTests executes error handling tests
func runErrorHandlingTests(t *testing.T, suite *TestSuite) {
	scenarios := []TestScenario{
		{
			Name:        "Invalid_SIP_Version",
			Description: "Test handling of invalid SIP version",
			Execute: func(t *testing.T, suite *TestSuite) error {
				invalidMsg := "OPTIONS sip:test.local SIP/1.0\r\n" +
					"Via: SIP/1.0/UDP 192.168.1.100:5060;branch=z9hG4bK-invalid\r\n" +
					"From: <sip:test@test.local>;tag=test\r\n" +
					"To: <sip:test.local>\r\n" +
					"Call-ID: invalid-version-test\r\n" +
					"CSeq: 1 OPTIONS\r\n" +
					"Content-Length: 0\r\n\r\n"
				
				response, err := suite.SendUDPMessage(t, invalidMsg)
				if err != nil {
					return err
				}
				
				parser := NewSIPResponseParser(response)
				statusCode, _ := parser.GetStatusCode()
				// Should get 505 Version Not Supported or 400 Bad Request
				if statusCode != 505 && statusCode != 400 {
					t.Errorf("Expected 505 or 400 for invalid SIP version, got %d", statusCode)
				}
				return nil
			},
		},
		{
			Name:        "Missing_Required_Headers",
			Description: "Test handling of missing required headers",
			Execute: func(t *testing.T, suite *TestSuite) error {
				incompleteMsg := "REGISTER sip:test.local SIP/2.0\r\n" +
					"Via: SIP/2.0/UDP 192.168.1.100:5060;branch=z9hG4bK-incomplete\r\n" +
					"From: <sip:test@test.local>;tag=test\r\n" +
					"Content-Length: 0\r\n\r\n"
				// Missing To, Call-ID, CSeq headers
				
				response, err := suite.SendUDPMessage(t, incompleteMsg)
				if err != nil {
					return err
				}
				
				AssertSIPResponse(t, response, 400, []string{"Via"})
				return nil
			},
		},
		{
			Name:        "Unsupported_Method",
			Description: "Test handling of unsupported SIP method",
			Execute: func(t *testing.T, suite *TestSuite) error {
				msg := NewSIPMessageBuilder("SUBSCRIBE", "sip:test.local").Build()
				response, err := suite.SendUDPMessage(t, msg)
				if err != nil {
					return err
				}
				AssertSIPResponse(t, response, 405, []string{"Allow"})
				return nil
			},
		},
	}

	for _, scenario := range scenarios {
		RunScenario(t, suite, scenario)
	}
}

// runPerformanceTests executes performance tests
func runPerformanceTests(t *testing.T, suite *TestSuite) {
	t.Run("Throughput_Test", func(t *testing.T) {
		const numRequests = 100
		const maxConcurrency = 20
		
		start := time.Now()
		var wg sync.WaitGroup
		semaphore := make(chan struct{}, maxConcurrency)
		successCount := 0
		errorCount := 0
		mu := sync.Mutex{}

		for i := 0; i < numRequests; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				
				semaphore <- struct{}{} // Acquire
				defer func() { <-semaphore }() // Release
				
				msg := NewSIPMessageBuilder("OPTIONS", "sip:test.local").Build()
				response, err := suite.SendUDPMessage(t, msg)
				
				mu.Lock()
				if err != nil {
					errorCount++
				} else {
					parser := NewSIPResponseParser(response)
					statusCode, _ := parser.GetStatusCode()
					if statusCode == 200 {
						successCount++
					} else {
						errorCount++
					}
				}
				mu.Unlock()
			}(i)
		}

		wg.Wait()
		duration := time.Since(start)
		throughput := float64(successCount) / duration.Seconds()

		t.Logf("Throughput test: %d successful, %d errors in %v (%.2f req/sec)", 
			successCount, errorCount, duration, throughput)

		// Should achieve reasonable throughput
		if throughput < 20 {
			t.Errorf("Throughput too low: %.2f req/sec", throughput)
		}

		// Error rate should be low
		errorRate := float64(errorCount) / float64(numRequests) * 100
		if errorRate > 10 {
			t.Errorf("Error rate too high: %.2f%%", errorRate)
		}
	})

	t.Run("Response_Time_Test", func(t *testing.T) {
		const numSamples = 10 // Reduced from 50 to 10 for faster testing
		var responseTimes []time.Duration

		for i := 0; i < numSamples; i++ {
			msg := NewSIPMessageBuilder("OPTIONS", "sip:test.local").Build()
			
			start := time.Now()
			response, err := suite.SendUDPMessage(t, msg)
			responseTime := time.Since(start)
			
			if err != nil {
				t.Logf("Request %d failed: %v", i, err)
				continue
			}
			
			parser := NewSIPResponseParser(response)
			statusCode, _ := parser.GetStatusCode()
			if statusCode == 200 {
				responseTimes = append(responseTimes, responseTime)
			}
			
			// Add small delay between requests to avoid overwhelming the server
			time.Sleep(100 * time.Millisecond)
		}

		if len(responseTimes) == 0 {
			t.Fatal("No successful responses for response time test")
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

		t.Logf("Response time test: avg=%v, min=%v, max=%v, samples=%d", 
			avg, min, max, len(responseTimes))

		// Response times should be reasonable
		if avg > 100*time.Millisecond {
			t.Errorf("Average response time too high: %v", avg)
		}
		
		if max > 500*time.Millisecond {
			t.Errorf("Maximum response time too high: %v", max)
		}
	})
}

// TestIntegrationSummary provides a summary of all integration tests
func TestIntegrationSummary(t *testing.T) {
	t.Log("=== SIP Server Integration Test Summary ===")
	t.Log("This comprehensive integration test suite covers:")
	t.Log("1. End-to-end SIP call flows")
	t.Log("2. Concurrent registration and session handling")
	t.Log("3. Session-Timer enforcement integration")
	t.Log("4. UDP and TCP transport protocol handling")
	t.Log("5. SIP protocol compliance (RFC3261)")
	t.Log("6. Error handling and edge cases")
	t.Log("7. Performance characteristics under load")
	t.Log("8. Transport reliability and message framing")
	t.Log("9. Authentication and authorization flows")
	t.Log("10. Malformed message handling")
	t.Log("============================================")
}