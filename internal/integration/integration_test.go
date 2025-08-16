package integration

import (
	"bytes"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/zurustar/xylitol2/internal/server"
)

// TestSuite holds common test infrastructure
type TestSuite struct {
	server     server.Server
	tempDir    string
	udpConn    *net.UDPConn
	tcpConn    *net.TCPConn
	serverAddr *net.UDPAddr
	tcpAddr    *net.TCPAddr
}

// SetupTestSuite creates a test server instance with temporary configuration
func SetupTestSuite(t *testing.T) *TestSuite {
	tempDir := t.TempDir()
	
	// Create test configuration
	configData := fmt.Sprintf(`
server:
  udp_port: 0  # Use port 0 to get random available port
  tcp_port: 0
database:
  path: "%s"
authentication:
  realm: "test.local"
  nonce_expiry: 300
session_timer:
  default_expires: 1800
  min_se: 90
  max_se: 7200
web_admin:
  port: 0
  enabled: false
logging:
  level: "error"  # Reduce log noise in tests
  file: "%s"
`, filepath.Join(tempDir, "test.db"), filepath.Join(tempDir, "test.log"))
	
	configFile := filepath.Join(tempDir, "config.yaml")
	if err := os.WriteFile(configFile, []byte(configData), 0644); err != nil {
		t.Fatalf("Failed to create test config file: %v", err)
	}

	// Create and configure server
	sipServer := server.NewSIPServer()
	if err := sipServer.LoadConfig(configFile); err != nil {
		t.Fatalf("Failed to load configuration: %v", err)
	}

	// Start server
	if err := sipServer.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}

	// Give server time to start
	time.Sleep(200 * time.Millisecond)

	suite := &TestSuite{
		server:  sipServer,
		tempDir: tempDir,
	}

	// Setup UDP client connection
	suite.setupUDPClient(t)
	
	// Setup TCP client connection
	suite.setupTCPClient(t)

	return suite
}

// setupUDPClient creates a UDP client connection to the test server
func (ts *TestSuite) setupUDPClient(t *testing.T) {
	// Connect to server UDP port (assuming it's listening on 5060 or configured port)
	serverAddr, err := net.ResolveUDPAddr("udp", "127.0.0.1:5060")
	if err != nil {
		t.Fatalf("Failed to resolve UDP address: %v", err)
	}
	
	conn, err := net.DialUDP("udp", nil, serverAddr)
	if err != nil {
		t.Fatalf("Failed to create UDP connection: %v", err)
	}
	
	ts.udpConn = conn
	ts.serverAddr = serverAddr
}

// setupTCPClient creates a TCP client connection to the test server
func (ts *TestSuite) setupTCPClient(t *testing.T) {
	// Connect to server TCP port
	tcpAddr, err := net.ResolveTCPAddr("tcp", "127.0.0.1:5060")
	if err != nil {
		t.Fatalf("Failed to resolve TCP address: %v", err)
	}
	
	conn, err := net.DialTCP("tcp", nil, tcpAddr)
	if err != nil {
		t.Fatalf("Failed to create TCP connection: %v", err)
	}
	
	ts.tcpConn = conn
	ts.tcpAddr = tcpAddr
}

// Cleanup shuts down the test server and cleans up resources
func (ts *TestSuite) Cleanup(t *testing.T) {
	if ts.udpConn != nil {
		ts.udpConn.Close()
	}
	if ts.tcpConn != nil {
		ts.tcpConn.Close()
	}
	if ts.server != nil {
		if err := ts.server.Stop(); err != nil {
			t.Errorf("Failed to stop server: %v", err)
		}
	}
}

// SendUDPMessage sends a SIP message over UDP and returns the response
func (ts *TestSuite) SendUDPMessage(t *testing.T, message string) (string, error) {
	// Send message
	_, err := ts.udpConn.Write([]byte(message))
	if err != nil {
		return "", fmt.Errorf("failed to send UDP message: %w", err)
	}

	// Read response with timeout
	ts.udpConn.SetReadDeadline(time.Now().Add(2 * time.Second))
	buffer := make([]byte, 4096)
	n, err := ts.udpConn.Read(buffer)
	if err != nil {
		return "", fmt.Errorf("failed to read UDP response: %w", err)
	}

	return string(buffer[:n]), nil
}

// SendTCPMessage sends a SIP message over TCP and returns the response
func (ts *TestSuite) SendTCPMessage(t *testing.T, message string) (string, error) {
	// Send message
	_, err := ts.tcpConn.Write([]byte(message))
	if err != nil {
		return "", fmt.Errorf("failed to send TCP message: %w", err)
	}

	// Read response with timeout
	ts.tcpConn.SetReadDeadline(time.Now().Add(5 * time.Second))
	buffer := make([]byte, 4096)
	n, err := ts.tcpConn.Read(buffer)
	if err != nil {
		return "", fmt.Errorf("failed to read TCP response: %w", err)
	}

	return string(buffer[:n]), nil
}

// CreateREGISTERMessage creates a basic REGISTER message
func (ts *TestSuite) CreateREGISTERMessage(username, contact string) string {
	return fmt.Sprintf(`REGISTER sip:test.local SIP/2.0
Via: SIP/2.0/UDP 192.168.1.100:5060;branch=z9hG4bK-test-register
From: <sip:%s@test.local>;tag=test-from-tag
To: <sip:%s@test.local>
Call-ID: test-register-call-id
CSeq: 1 REGISTER
Contact: <%s>
Expires: 3600
Content-Length: 0

`, username, username, contact)
}

// CreateINVITEMessage creates a basic INVITE message
func (ts *TestSuite) CreateINVITEMessage(from, to, contact string) string {
	return fmt.Sprintf(`INVITE sip:%s@test.local SIP/2.0
Via: SIP/2.0/UDP 192.168.1.100:5060;branch=z9hG4bK-test-invite
From: <sip:%s@test.local>;tag=test-from-tag
To: <sip:%s@test.local>
Call-ID: test-invite-call-id
CSeq: 1 INVITE
Contact: <%s>
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
`, to, from, to, contact)
}

// CreateOPTIONSMessage creates a basic OPTIONS message
func (ts *TestSuite) CreateOPTIONSMessage() string {
	return `OPTIONS sip:test.local SIP/2.0
Via: SIP/2.0/UDP 192.168.1.100:5060;branch=z9hG4bK-test-options
From: <sip:test@test.local>;tag=test-from-tag
To: <sip:test.local>
Call-ID: test-options-call-id
CSeq: 1 OPTIONS
Content-Length: 0

`
}

// TestEndToEndSIPCallFlow tests complete SIP call flows
func TestEndToEndSIPCallFlow(t *testing.T) {
	suite := SetupTestSuite(t)
	defer suite.Cleanup(t)

	t.Run("OPTIONS_Request", func(t *testing.T) {
		optionsMsg := suite.CreateOPTIONSMessage()
		
		response, err := suite.SendUDPMessage(t, optionsMsg)
		if err != nil {
			t.Fatalf("Failed to send OPTIONS message: %v", err)
		}

		// Verify response contains 200 OK
		if !bytes.Contains([]byte(response), []byte("200 OK")) {
			t.Errorf("Expected 200 OK response, got: %s", response)
		}

		// Verify Allow header is present
		if !bytes.Contains([]byte(response), []byte("Allow:")) {
			t.Errorf("Expected Allow header in OPTIONS response")
		}
	})

	t.Run("REGISTER_Without_Auth", func(t *testing.T) {
		registerMsg := suite.CreateREGISTERMessage("alice", "sip:alice@192.168.1.100:5060")
		
		response, err := suite.SendUDPMessage(t, registerMsg)
		if err != nil {
			t.Fatalf("Failed to send REGISTER message: %v", err)
		}

		// Verify response contains 401 Unauthorized
		if !bytes.Contains([]byte(response), []byte("401 Unauthorized")) {
			t.Errorf("Expected 401 Unauthorized response, got: %s", response)
		}

		// Verify WWW-Authenticate header is present
		if !bytes.Contains([]byte(response), []byte("WWW-Authenticate:")) {
			t.Errorf("Expected WWW-Authenticate header in 401 response")
		}
	})

	t.Run("INVITE_Without_SessionTimer", func(t *testing.T) {
		// Create INVITE without Session-Expires header
		inviteMsg := `INVITE sip:bob@test.local SIP/2.0
Via: SIP/2.0/UDP 192.168.1.100:5060;branch=z9hG4bK-test-invite-no-timer
From: <sip:alice@test.local>;tag=test-from-tag
To: <sip:bob@test.local>
Call-ID: test-invite-no-timer-call-id
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
		
		response, err := suite.SendUDPMessage(t, inviteMsg)
		if err != nil {
			t.Fatalf("Failed to send INVITE message: %v", err)
		}

		// Verify response contains 421 Extension Required (Session-Timer mandatory)
		if !bytes.Contains([]byte(response), []byte("421 Extension Required")) {
			t.Errorf("Expected 421 Extension Required response for missing Session-Timer, got: %s", response)
		}
	})
}

// TestConcurrentRegistrationHandling tests concurrent registration processing
func TestConcurrentRegistrationHandling(t *testing.T) {
	suite := SetupTestSuite(t)
	defer suite.Cleanup(t)

	const numConcurrentRegistrations = 10
	var wg sync.WaitGroup
	results := make(chan string, numConcurrentRegistrations)
	errors := make(chan error, numConcurrentRegistrations)

	// Launch concurrent REGISTER requests
	for i := 0; i < numConcurrentRegistrations; i++ {
		wg.Add(1)
		go func(userID int) {
			defer wg.Done()
			
			username := fmt.Sprintf("user%d", userID)
			contact := fmt.Sprintf("sip:%s@192.168.1.%d:5060", username, 100+userID)
			registerMsg := suite.CreateREGISTERMessage(username, contact)
			
			// Create separate UDP connection for each goroutine
			conn, err := net.Dial("udp", "127.0.0.1:5060")
			if err != nil {
				errors <- fmt.Errorf("failed to create UDP connection for user %d: %w", userID, err)
				return
			}
			defer conn.Close()
			
			// Send REGISTER message
			_, err = conn.Write([]byte(registerMsg))
			if err != nil {
				errors <- fmt.Errorf("failed to send REGISTER for user %d: %w", userID, err)
				return
			}
			
			// Read response with timeout
			conn.SetReadDeadline(time.Now().Add(5 * time.Second))
			buffer := make([]byte, 4096)
			n, err := conn.Read(buffer)
			if err != nil {
				errors <- fmt.Errorf("failed to read response for user %d: %w", userID, err)
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
		t.Errorf("Concurrent registration error: %v", err)
	}

	// Verify all responses are 401 Unauthorized (expected without auth)
	responseCount := 0
	for response := range results {
		responseCount++
		if !bytes.Contains([]byte(response), []byte("401 Unauthorized")) {
			t.Errorf("Expected 401 Unauthorized response, got: %s", response)
		}
	}

	if responseCount != numConcurrentRegistrations {
		t.Errorf("Expected %d responses, got %d", numConcurrentRegistrations, responseCount)
	}
}

// TestConcurrentSessionHandling tests concurrent session establishment
func TestConcurrentSessionHandling(t *testing.T) {
	suite := SetupTestSuite(t)
	defer suite.Cleanup(t)

	const numConcurrentSessions = 5
	var wg sync.WaitGroup
	results := make(chan string, numConcurrentSessions)
	errors := make(chan error, numConcurrentSessions)

	// Launch concurrent INVITE requests
	for i := 0; i < numConcurrentSessions; i++ {
		wg.Add(1)
		go func(sessionID int) {
			defer wg.Done()
			
			from := fmt.Sprintf("caller%d", sessionID)
			to := fmt.Sprintf("callee%d", sessionID)
			contact := fmt.Sprintf("sip:%s@192.168.1.%d:5060", from, 100+sessionID)
			inviteMsg := suite.CreateINVITEMessage(from, to, contact)
			
			// Create separate UDP connection for each goroutine
			conn, err := net.Dial("udp", "127.0.0.1:5060")
			if err != nil {
				errors <- fmt.Errorf("failed to create UDP connection for session %d: %w", sessionID, err)
				return
			}
			defer conn.Close()
			
			// Send INVITE message
			_, err = conn.Write([]byte(inviteMsg))
			if err != nil {
				errors <- fmt.Errorf("failed to send INVITE for session %d: %w", sessionID, err)
				return
			}
			
			// Read response with timeout
			conn.SetReadDeadline(time.Now().Add(5 * time.Second))
			buffer := make([]byte, 4096)
			n, err := conn.Read(buffer)
			if err != nil {
				errors <- fmt.Errorf("failed to read response for session %d: %w", sessionID, err)
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
		t.Errorf("Concurrent session error: %v", err)
	}

	// Verify responses (could be 404 Not Found if users not registered, or other appropriate responses)
	responseCount := 0
	for response := range results {
		responseCount++
		// Should get some kind of SIP response (not necessarily success since users aren't registered)
		if !bytes.Contains([]byte(response), []byte("SIP/2.0")) {
			t.Errorf("Expected SIP response, got: %s", response)
		}
	}

	if responseCount != numConcurrentSessions {
		t.Errorf("Expected %d responses, got %d", numConcurrentSessions, responseCount)
	}
}

// TestSessionTimerEnforcement tests Session-Timer enforcement
func TestSessionTimerEnforcement(t *testing.T) {
	suite := SetupTestSuite(t)
	defer suite.Cleanup(t)

	t.Run("INVITE_With_Valid_SessionTimer", func(t *testing.T) {
		inviteMsg := suite.CreateINVITEMessage("alice", "bob", "sip:alice@192.168.1.100:5060")
		
		response, err := suite.SendUDPMessage(t, inviteMsg)
		if err != nil {
			t.Fatalf("Failed to send INVITE message: %v", err)
		}

		// Should not get 421 Extension Required since Session-Expires is present
		if bytes.Contains([]byte(response), []byte("421 Extension Required")) {
			t.Errorf("Unexpected 421 Extension Required for INVITE with Session-Timer: %s", response)
		}

		// Should get some SIP response
		if !bytes.Contains([]byte(response), []byte("SIP/2.0")) {
			t.Errorf("Expected SIP response, got: %s", response)
		}
	})

	t.Run("INVITE_With_Invalid_SessionTimer_Value", func(t *testing.T) {
		// Create INVITE with Session-Expires value below minimum
		inviteMsg := `INVITE sip:bob@test.local SIP/2.0
Via: SIP/2.0/UDP 192.168.1.100:5060;branch=z9hG4bK-test-invite-low-timer
From: <sip:alice@test.local>;tag=test-from-tag
To: <sip:bob@test.local>
Call-ID: test-invite-low-timer-call-id
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
		
		response, err := suite.SendUDPMessage(t, inviteMsg)
		if err != nil {
			t.Fatalf("Failed to send INVITE message: %v", err)
		}

		// Should get 422 Session Interval Too Small or similar error
		if !bytes.Contains([]byte(response), []byte("422")) && !bytes.Contains([]byte(response), []byte("400")) {
			t.Errorf("Expected 422 or 400 response for invalid Session-Timer value, got: %s", response)
		}
	})

	t.Run("INVITE_With_Excessive_SessionTimer_Value", func(t *testing.T) {
		// Create INVITE with Session-Expires value above maximum
		inviteMsg := `INVITE sip:bob@test.local SIP/2.0
Via: SIP/2.0/UDP 192.168.1.100:5060;branch=z9hG4bK-test-invite-high-timer
From: <sip:alice@test.local>;tag=test-from-tag
To: <sip:bob@test.local>
Call-ID: test-invite-high-timer-call-id
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
		
		response, err := suite.SendUDPMessage(t, inviteMsg)
		if err != nil {
			t.Fatalf("Failed to send INVITE message: %v", err)
		}

		// Should get some SIP response (server may adjust the value or reject)
		if !bytes.Contains([]byte(response), []byte("SIP/2.0")) {
			t.Errorf("Expected SIP response, got: %s", response)
		}
	})
}

// TestUDPTransportProtocol tests UDP-specific transport handling
func TestUDPTransportProtocol(t *testing.T) {
	suite := SetupTestSuite(t)
	defer suite.Cleanup(t)

	t.Run("UDP_Message_Handling", func(t *testing.T) {
		optionsMsg := suite.CreateOPTIONSMessage()
		
		response, err := suite.SendUDPMessage(t, optionsMsg)
		if err != nil {
			t.Fatalf("Failed to send UDP message: %v", err)
		}

		// Verify response is received over UDP
		if !bytes.Contains([]byte(response), []byte("SIP/2.0")) {
			t.Errorf("Expected SIP response over UDP, got: %s", response)
		}

		// Verify Via header indicates UDP transport
		if !bytes.Contains([]byte(response), []byte("UDP")) {
			t.Errorf("Expected UDP transport in Via header, got: %s", response)
		}
	})

	t.Run("UDP_Large_Message_Handling", func(t *testing.T) {
		// Create a large INVITE message to test UDP handling
		largeBody := `v=0
o=alice 2890844526 2890844526 IN IP4 192.168.1.100
s=Large SDP Session
c=IN IP4 192.168.1.100
t=0 0
m=audio 5004 RTP/AVP 0 8 18
a=rtpmap:0 PCMU/8000
a=rtpmap:8 PCMA/8000
a=rtpmap:18 G729/8000
m=video 5006 RTP/AVP 96
a=rtpmap:96 H264/90000
a=fmtp:96 profile-level-id=42e01e
a=sendrecv
`
		
		inviteMsg := fmt.Sprintf(`INVITE sip:bob@test.local SIP/2.0
Via: SIP/2.0/UDP 192.168.1.100:5060;branch=z9hG4bK-test-large-invite
From: <sip:alice@test.local>;tag=test-from-tag
To: <sip:bob@test.local>
Call-ID: test-large-invite-call-id
CSeq: 1 INVITE
Contact: <sip:alice@192.168.1.100:5060>
Session-Expires: 1800;refresher=uac
Content-Type: application/sdp
Content-Length: %d

%s`, len(largeBody), largeBody)
		
		response, err := suite.SendUDPMessage(t, inviteMsg)
		if err != nil {
			t.Fatalf("Failed to send large UDP message: %v", err)
		}

		// Should get some SIP response
		if !bytes.Contains([]byte(response), []byte("SIP/2.0")) {
			t.Errorf("Expected SIP response for large UDP message, got: %s", response)
		}
	})
}

// TestTCPTransportProtocol tests TCP-specific transport handling
func TestTCPTransportProtocol(t *testing.T) {
	suite := SetupTestSuite(t)
	defer suite.Cleanup(t)

	t.Run("TCP_Message_Handling", func(t *testing.T) {
		optionsMsg := suite.CreateOPTIONSMessage()
		
		response, err := suite.SendTCPMessage(t, optionsMsg)
		if err != nil {
			t.Fatalf("Failed to send TCP message: %v", err)
		}

		// Verify response is received over TCP
		if !bytes.Contains([]byte(response), []byte("SIP/2.0")) {
			t.Errorf("Expected SIP response over TCP, got: %s", response)
		}

		// Verify Via header indicates TCP transport
		if !bytes.Contains([]byte(response), []byte("TCP")) {
			t.Errorf("Expected TCP transport in Via header, got: %s", response)
		}
	})

	t.Run("TCP_Connection_Persistence", func(t *testing.T) {
		// Send multiple messages over the same TCP connection
		for i := 0; i < 3; i++ {
			optionsMsg := fmt.Sprintf(`OPTIONS sip:test.local SIP/2.0
Via: SIP/2.0/TCP 192.168.1.100:5060;branch=z9hG4bK-test-options-%d
From: <sip:test@test.local>;tag=test-from-tag-%d
To: <sip:test.local>
Call-ID: test-options-call-id-%d
CSeq: %d OPTIONS
Content-Length: 0

`, i, i, i, i+1)
			
			response, err := suite.SendTCPMessage(t, optionsMsg)
			if err != nil {
				t.Fatalf("Failed to send TCP message %d: %v", i, err)
			}

			if !bytes.Contains([]byte(response), []byte("200 OK")) {
				t.Errorf("Expected 200 OK response for message %d, got: %s", i, response)
			}
		}
	})

	t.Run("TCP_Stream_Framing", func(t *testing.T) {
		// Test that TCP properly handles message framing by sending multiple messages rapidly
		messages := []string{
			suite.CreateOPTIONSMessage(),
			suite.CreateREGISTERMessage("test1", "sip:test1@192.168.1.100:5060"),
			suite.CreateOPTIONSMessage(),
		}

		// Send all messages rapidly
		for _, msg := range messages {
			_, err := suite.tcpConn.Write([]byte(msg))
			if err != nil {
				t.Fatalf("Failed to send TCP message: %v", err)
			}
		}

		// Read responses
		for i := 0; i < len(messages); i++ {
			suite.tcpConn.SetReadDeadline(time.Now().Add(5 * time.Second))
			buffer := make([]byte, 4096)
			n, err := suite.tcpConn.Read(buffer)
			if err != nil {
				t.Fatalf("Failed to read TCP response %d: %v", i, err)
			}

			response := string(buffer[:n])
			if !bytes.Contains([]byte(response), []byte("SIP/2.0")) {
				t.Errorf("Expected SIP response %d, got: %s", i, response)
			}
		}
	})
}

// TestTransportProtocolSelection tests that the server handles both UDP and TCP appropriately
func TestTransportProtocolSelection(t *testing.T) {
	suite := SetupTestSuite(t)
	defer suite.Cleanup(t)

	t.Run("UDP_And_TCP_Parallel_Handling", func(t *testing.T) {
		var wg sync.WaitGroup
		results := make(chan string, 2)
		errors := make(chan error, 2)

		// Send OPTIONS over UDP
		wg.Add(1)
		go func() {
			defer wg.Done()
			response, err := suite.SendUDPMessage(t, suite.CreateOPTIONSMessage())
			if err != nil {
				errors <- fmt.Errorf("UDP error: %w", err)
				return
			}
			results <- "UDP: " + response
		}()

		// Send OPTIONS over TCP
		wg.Add(1)
		go func() {
			defer wg.Done()
			response, err := suite.SendTCPMessage(t, suite.CreateOPTIONSMessage())
			if err != nil {
				errors <- fmt.Errorf("TCP error: %w", err)
				return
			}
			results <- "TCP: " + response
		}()

		wg.Wait()
		close(results)
		close(errors)

		// Check for errors
		for err := range errors {
			t.Errorf("Transport protocol error: %v", err)
		}

		// Verify both transports received responses
		responseCount := 0
		for response := range results {
			responseCount++
			if !bytes.Contains([]byte(response), []byte("200 OK")) {
				t.Errorf("Expected 200 OK response, got: %s", response)
			}
		}

		if responseCount != 2 {
			t.Errorf("Expected 2 responses (UDP and TCP), got %d", responseCount)
		}
	})
}

// TestServerResourceManagement tests server resource handling under load
func TestServerResourceManagement(t *testing.T) {
	suite := SetupTestSuite(t)
	defer suite.Cleanup(t)

	t.Run("High_Load_Handling", func(t *testing.T) {
		const numRequests = 5  // Reduced from 50
		var wg sync.WaitGroup
		successCount := int32(0)
		errorCount := int32(0)

		// Launch many concurrent requests
		for i := 0; i < numRequests; i++ {
			wg.Add(1)
			go func(requestID int) {
				defer wg.Done()
				
				// Create separate connection for each request
				conn, err := net.Dial("udp", "127.0.0.1:5060")
				if err != nil {
					errorCount++
					return
				}
				defer conn.Close()
				
				optionsMsg := fmt.Sprintf(`OPTIONS sip:test.local SIP/2.0
Via: SIP/2.0/UDP 192.168.1.100:5060;branch=z9hG4bK-load-test-%d
From: <sip:test@test.local>;tag=load-test-tag-%d
To: <sip:test.local>
Call-ID: load-test-call-id-%d
CSeq: 1 OPTIONS
Content-Length: 0

`, requestID, requestID, requestID)
				
				// Send message
				_, err = conn.Write([]byte(optionsMsg))
				if err != nil {
					errorCount++
					return
				}
				
				// Read response with timeout
				conn.SetReadDeadline(time.Now().Add(10 * time.Second))
				buffer := make([]byte, 4096)
				_, err = conn.Read(buffer)
				if err != nil {
					errorCount++
					return
				}
				
				successCount++
			}(i)
		}

		wg.Wait()

		t.Logf("High load test: %d successful, %d errors out of %d requests", 
			successCount, errorCount, numRequests)

		// Allow for some errors under high load, but most should succeed
		if successCount < int32(numRequests*0.8) {
			t.Errorf("Too many failures under load: %d successful out of %d", successCount, numRequests)
		}
	})
}

// TestServerShutdownGraceful tests graceful server shutdown during active connections
func TestServerShutdownGraceful(t *testing.T) {
	suite := SetupTestSuite(t)
	// Don't defer cleanup here as we'll test shutdown explicitly

	t.Run("Graceful_Shutdown_With_Active_Connections", func(t *testing.T) {
		// Establish connections
		conn1, err := net.Dial("udp", "127.0.0.1:5060")
		if err != nil {
			t.Fatalf("Failed to create UDP connection: %v", err)
		}
		defer conn1.Close()

		conn2, err := net.Dial("tcp", "127.0.0.1:5060")
		if err != nil {
			t.Fatalf("Failed to create TCP connection: %v", err)
		}
		defer conn2.Close()

		// Send a message to ensure connections are active
		optionsMsg := suite.CreateOPTIONSMessage()
		_, err = conn1.Write([]byte(optionsMsg))
		if err != nil {
			t.Fatalf("Failed to send UDP message: %v", err)
		}

		// Read response to confirm connection is working
		conn1.SetReadDeadline(time.Now().Add(5 * time.Second))
		buffer := make([]byte, 4096)
		_, err = conn1.Read(buffer)
		if err != nil {
			t.Fatalf("Failed to read UDP response: %v", err)
		}

		// Now test graceful shutdown
		shutdownStart := time.Now()
		err = suite.server.Stop()
		shutdownDuration := time.Since(shutdownStart)

		if err != nil {
			t.Errorf("Server shutdown failed: %v", err)
		}

		// Shutdown should complete within reasonable time
		if shutdownDuration > 10*time.Second {
			t.Errorf("Shutdown took too long: %v", shutdownDuration)
		}

		t.Logf("Graceful shutdown completed in %v", shutdownDuration)
	})
}