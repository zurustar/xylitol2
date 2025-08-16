package integration

import (
	"fmt"
	"net"
	"testing"
	"time"
)

// TestBasicServerResponse tests if the server responds to a simple OPTIONS request
func TestBasicServerResponse(t *testing.T) {
	// Create UDP connection
	conn, err := net.Dial("udp", "127.0.0.1:5060")
	if err != nil {
		t.Fatalf("Failed to create UDP connection: %v", err)
	}
	defer conn.Close()

	// Simple OPTIONS message
	message := `OPTIONS sip:test.local SIP/2.0
Via: SIP/2.0/UDP 192.168.1.100:5060;branch=z9hG4bK-debug-test
From: <sip:debug@test.local>;tag=debug-tag
To: <sip:test.local>
Call-ID: debug-call-id@test.local
CSeq: 1 OPTIONS
Content-Length: 0

`

	t.Logf("Sending message:\n%s", message)

	// Send message
	start := time.Now()
	_, err = conn.Write([]byte(message))
	if err != nil {
		t.Fatalf("Failed to send message: %v", err)
	}

	// Set read timeout
	conn.SetReadDeadline(time.Now().Add(5 * time.Second))

	// Read response
	buffer := make([]byte, 4096)
	n, err := conn.Read(buffer)
	duration := time.Since(start)

	if err != nil {
		t.Fatalf("Failed to read response after %v: %v", duration, err)
	}

	response := string(buffer[:n])
	t.Logf("Received response after %v:\n%s", duration, response)

	// Check if response is valid SIP response
	if len(response) < 10 {
		t.Errorf("Response too short: %d bytes", len(response))
	}

	if !contains(response, "SIP/2.0") {
		t.Errorf("Response doesn't contain SIP/2.0: %s", response)
	}
}

// TestServerProcessingTime tests how long it takes for the server to process a message
func TestServerProcessingTime(t *testing.T) {
	const numTests = 5

	for i := 0; i < numTests; i++ {
		t.Run(fmt.Sprintf("Test_%d", i+1), func(t *testing.T) {
			conn, err := net.Dial("udp", "127.0.0.1:5060")
			if err != nil {
				t.Fatalf("Failed to create UDP connection: %v", err)
			}
			defer conn.Close()

			message := fmt.Sprintf(`OPTIONS sip:test.local SIP/2.0
Via: SIP/2.0/UDP 192.168.1.100:5060;branch=z9hG4bK-timing-test-%d
From: <sip:timing@test.local>;tag=timing-tag-%d
To: <sip:test.local>
Call-ID: timing-call-id-%d@test.local
CSeq: 1 OPTIONS
Content-Length: 0

`, i, i, i)

			start := time.Now()
			_, err = conn.Write([]byte(message))
			if err != nil {
				t.Fatalf("Failed to send message: %v", err)
			}

			conn.SetReadDeadline(time.Now().Add(2 * time.Second))
			buffer := make([]byte, 4096)
			n, err := conn.Read(buffer)
			duration := time.Since(start)

			if err != nil {
				t.Errorf("Test %d: Failed to read response after %v: %v", i+1, duration, err)
				return
			}

			t.Logf("Test %d: Response received in %v", i+1, duration)

			if duration > 100*time.Millisecond {
				t.Logf("Test %d: Slow response time: %v", i+1, duration)
			}

			response := string(buffer[:n])
			if !contains(response, "200 OK") && !contains(response, "SIP/2.0") {
				t.Errorf("Test %d: Invalid response: %s", i+1, response)
			}
		})
	}
}

// TestConcurrentRequests tests concurrent request handling
func TestConcurrentRequests(t *testing.T) {
	const numConcurrent = 3

	results := make(chan string, numConcurrent)
	errors := make(chan error, numConcurrent)

	for i := 0; i < numConcurrent; i++ {
		go func(id int) {
			conn, err := net.Dial("udp", "127.0.0.1:5060")
			if err != nil {
				errors <- fmt.Errorf("worker %d: failed to create connection: %v", id, err)
				return
			}
			defer conn.Close()

			message := fmt.Sprintf(`OPTIONS sip:test.local SIP/2.0
Via: SIP/2.0/UDP 192.168.1.100:5060;branch=z9hG4bK-concurrent-test-%d
From: <sip:concurrent@test.local>;tag=concurrent-tag-%d
To: <sip:test.local>
Call-ID: concurrent-call-id-%d@test.local
CSeq: 1 OPTIONS
Content-Length: 0

`, id, id, id)

			start := time.Now()
			_, err = conn.Write([]byte(message))
			if err != nil {
				errors <- fmt.Errorf("worker %d: failed to send message: %v", id, err)
				return
			}

			conn.SetReadDeadline(time.Now().Add(3 * time.Second))
			buffer := make([]byte, 4096)
			n, err := conn.Read(buffer)
			duration := time.Since(start)

			if err != nil {
				errors <- fmt.Errorf("worker %d: failed to read response after %v: %v", id, duration, err)
				return
			}

			response := string(buffer[:n])
			results <- fmt.Sprintf("Worker %d: Success in %v", id, duration)
			t.Logf("Worker %d response: %s", id, response[:min(100, len(response))])
		}(i)
	}

	// Collect results
	successCount := 0
	errorCount := 0

	for i := 0; i < numConcurrent; i++ {
		select {
		case result := <-results:
			t.Log(result)
			successCount++
		case err := <-errors:
			t.Log(err)
			errorCount++
		case <-time.After(5 * time.Second):
			t.Errorf("Timeout waiting for worker %d", i)
			errorCount++
		}
	}

	t.Logf("Concurrent test results: %d success, %d errors", successCount, errorCount)

	if successCount == 0 {
		t.Error("No successful concurrent requests")
	}
}

// Helper functions
func contains(s, substr string) bool {
	return len(s) >= len(substr) && findSubstring(s, substr) >= 0
}

func findSubstring(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}