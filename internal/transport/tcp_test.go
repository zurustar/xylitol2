package transport

import (
	"fmt"
	"net"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestTCPTransport_StartStop(t *testing.T) {
	transport := NewTCPTransport()

	// Test starting on a free port
	err := transport.Start(0) // Port 0 means system will choose a free port
	if err != nil {
		t.Fatalf("Failed to start TCP transport: %v", err)
	}

	if !transport.IsRunning() {
		t.Error("Transport should be running after start")
	}

	// Test stopping
	err = transport.Stop()
	if err != nil {
		t.Fatalf("Failed to stop TCP transport: %v", err)
	}

	if transport.IsRunning() {
		t.Error("Transport should not be running after stop")
	}
}

func TestTCPTransport_StartTwice(t *testing.T) {
	transport := NewTCPTransport()

	err := transport.Start(0)
	if err != nil {
		t.Fatalf("Failed to start TCP transport: %v", err)
	}
	defer transport.Stop()

	// Try to start again - should fail
	err = transport.Start(0)
	if err == nil {
		t.Error("Expected error when starting transport twice")
	}
}

func TestTCPTransport_SendMessage(t *testing.T) {
	// Start a TCP transport
	transport := NewTCPTransport()
	err := transport.Start(0)
	if err != nil {
		t.Fatalf("Failed to start TCP transport: %v", err)
	}
	defer transport.Stop()

	// Get the local address
	localAddr := transport.LocalAddr().(*net.TCPAddr)

	// Create a test message
	testMessage := []byte("INVITE sip:test@example.com SIP/2.0\r\nContent-Length: 0\r\n\r\n")

	// Send message to ourselves
	err = transport.SendMessage(testMessage, localAddr)
	if err != nil {
		t.Fatalf("Failed to send message: %v", err)
	}
}

func TestTCPTransport_SendMessageNotRunning(t *testing.T) {
	transport := NewTCPTransport()

	testMessage := []byte("test message")
	addr, _ := net.ResolveTCPAddr("tcp", "127.0.0.1:5060")

	err := transport.SendMessage(testMessage, addr)
	if err == nil {
		t.Error("Expected error when sending message on stopped transport")
	}
}

func TestTCPTransport_SendMessageInvalidAddress(t *testing.T) {
	transport := NewTCPTransport()
	err := transport.Start(0)
	if err != nil {
		t.Fatalf("Failed to start TCP transport: %v", err)
	}
	defer transport.Stop()

	testMessage := []byte("test message")
	// Use UDP address instead of TCP
	addr, _ := net.ResolveUDPAddr("udp", "127.0.0.1:5060")

	err = transport.SendMessage(testMessage, addr)
	if err == nil {
		t.Error("Expected error when sending message with invalid address type")
	}
}

func TestTCPTransport_ReceiveMessage(t *testing.T) {
	// Start a TCP transport
	transport := NewTCPTransport()
	err := transport.Start(0)
	if err != nil {
		t.Fatalf("Failed to start TCP transport: %v", err)
	}
	defer transport.Stop()

	// Register a mock handler
	handler := &mockMessageHandler{}
	transport.RegisterHandler(handler)

	// Get the local address
	localAddr := transport.LocalAddr().(*net.TCPAddr)

	// Create a client connection
	clientConn, err := net.DialTCP("tcp", nil, localAddr)
	if err != nil {
		t.Fatalf("Failed to create client connection: %v", err)
	}
	defer clientConn.Close()

	// Send a test message
	testMessage := []byte("REGISTER sip:test@example.com SIP/2.0\r\nContent-Length: 0\r\n\r\n")
	_, err = clientConn.Write(testMessage)
	if err != nil {
		t.Fatalf("Failed to send test message: %v", err)
	}

	// Wait for message to be received and processed
	time.Sleep(200 * time.Millisecond)

	// Check that the message was received
	messages := handler.getMessages()
	if len(messages) != 1 {
		t.Fatalf("Expected 1 message, got %d", len(messages))
	}

	if string(messages[0].data) != string(testMessage) {
		t.Errorf("Expected message %q, got %q", string(testMessage), string(messages[0].data))
	}

	if messages[0].transport != "TCP" {
		t.Errorf("Expected transport 'TCP', got %q", messages[0].transport)
	}

	if messages[0].addr == nil {
		t.Error("Expected non-nil address")
	}
}

func TestTCPTransport_ReceiveMessageWithBody(t *testing.T) {
	// Start a TCP transport
	transport := NewTCPTransport()
	err := transport.Start(0)
	if err != nil {
		t.Fatalf("Failed to start TCP transport: %v", err)
	}
	defer transport.Stop()

	// Register a mock handler
	handler := &mockMessageHandler{}
	transport.RegisterHandler(handler)

	// Get the local address
	localAddr := transport.LocalAddr().(*net.TCPAddr)

	// Create a client connection
	clientConn, err := net.DialTCP("tcp", nil, localAddr)
	if err != nil {
		t.Fatalf("Failed to create client connection: %v", err)
	}
	defer clientConn.Close()

	// Send a test message with body
	body := "v=0\r\no=alice 2890844526 2890844527 IN IP4 host.atlanta.com\r\n"
	testMessage := fmt.Sprintf("INVITE sip:test@example.com SIP/2.0\r\nContent-Length: %d\r\n\r\n%s", len(body), body)
	
	_, err = clientConn.Write([]byte(testMessage))
	if err != nil {
		t.Fatalf("Failed to send test message: %v", err)
	}

	// Wait for message to be received and processed
	time.Sleep(200 * time.Millisecond)

	// Check that the message was received
	messages := handler.getMessages()
	if len(messages) != 1 {
		t.Fatalf("Expected 1 message, got %d", len(messages))
	}

	if string(messages[0].data) != testMessage {
		t.Errorf("Expected message %q, got %q", testMessage, string(messages[0].data))
	}

	if messages[0].transport != "TCP" {
		t.Errorf("Expected transport 'TCP', got %q", messages[0].transport)
	}
}

func TestTCPTransport_MultipleMessages(t *testing.T) {
	// Start a TCP transport
	transport := NewTCPTransport()
	err := transport.Start(0)
	if err != nil {
		t.Fatalf("Failed to start TCP transport: %v", err)
	}
	defer transport.Stop()

	// Register a mock handler
	handler := &mockMessageHandler{}
	transport.RegisterHandler(handler)

	// Get the local address
	localAddr := transport.LocalAddr().(*net.TCPAddr)

	// Create a client connection
	clientConn, err := net.DialTCP("tcp", nil, localAddr)
	if err != nil {
		t.Fatalf("Failed to create client connection: %v", err)
	}
	defer clientConn.Close()

	// Send multiple test messages
	testMessages := []string{
		"INVITE sip:test1@example.com SIP/2.0\r\nContent-Length: 0\r\n\r\n",
		"REGISTER sip:test2@example.com SIP/2.0\r\nContent-Length: 0\r\n\r\n",
		"OPTIONS sip:test3@example.com SIP/2.0\r\nContent-Length: 0\r\n\r\n",
	}

	for _, msg := range testMessages {
		_, err = clientConn.Write([]byte(msg))
		if err != nil {
			t.Fatalf("Failed to send test message: %v", err)
		}
		// Small delay between messages to ensure proper framing
		time.Sleep(10 * time.Millisecond)
	}

	// Wait for messages to be received and processed
	time.Sleep(300 * time.Millisecond)

	// Check that all messages were received
	messages := handler.getMessages()
	if len(messages) != len(testMessages) {
		t.Fatalf("Expected %d messages, got %d", len(testMessages), len(messages))
	}

	// Verify each message (order should be preserved for TCP)
	for i, msg := range messages {
		if string(msg.data) != testMessages[i] {
			t.Errorf("Expected message %d to be %q, got %q", i, testMessages[i], string(msg.data))
		}
		if msg.transport != "TCP" {
			t.Errorf("Expected transport 'TCP', got %q", msg.transport)
		}
	}
}

func TestTCPTransport_MultipleConnections(t *testing.T) {
	// Start a TCP transport
	transport := NewTCPTransport()
	err := transport.Start(0)
	if err != nil {
		t.Fatalf("Failed to start TCP transport: %v", err)
	}
	defer transport.Stop()

	// Register a mock handler
	handler := &mockMessageHandler{}
	transport.RegisterHandler(handler)

	// Get the local address
	localAddr := transport.LocalAddr().(*net.TCPAddr)

	// Create multiple client connections
	numConnections := 5
	var wg sync.WaitGroup

	for i := 0; i < numConnections; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			clientConn, err := net.DialTCP("tcp", nil, localAddr)
			if err != nil {
				t.Errorf("Failed to create client connection %d: %v", id, err)
				return
			}
			defer clientConn.Close()

			// Send a test message
			testMessage := fmt.Sprintf("REGISTER sip:test%d@example.com SIP/2.0\r\nContent-Length: 0\r\n\r\n", id)
			_, err = clientConn.Write([]byte(testMessage))
			if err != nil {
				t.Errorf("Failed to send test message from connection %d: %v", id, err)
			}
		}(i)
	}

	wg.Wait()

	// Wait for messages to be received and processed
	time.Sleep(300 * time.Millisecond)

	// Check that all messages were received
	messages := handler.getMessages()
	if len(messages) != numConnections {
		t.Fatalf("Expected %d messages, got %d", numConnections, len(messages))
	}

	// Verify all messages are from TCP transport
	for _, msg := range messages {
		if msg.transport != "TCP" {
			t.Errorf("Expected transport 'TCP', got %q", msg.transport)
		}
		if !strings.Contains(string(msg.data), "REGISTER sip:test") {
			t.Errorf("Unexpected message content: %q", string(msg.data))
		}
	}
}

func TestTCPTransport_LocalAddr(t *testing.T) {
	transport := NewTCPTransport()

	// Should return nil when not running
	if addr := transport.LocalAddr(); addr != nil {
		t.Error("Expected nil address when transport not running")
	}

	// Start transport
	err := transport.Start(0)
	if err != nil {
		t.Fatalf("Failed to start TCP transport: %v", err)
	}
	defer transport.Stop()

	// Should return valid address when running
	addr := transport.LocalAddr()
	if addr == nil {
		t.Error("Expected non-nil address when transport running")
	}

	tcpAddr, ok := addr.(*net.TCPAddr)
	if !ok {
		t.Errorf("Expected TCP address, got %T", addr)
	}

	if tcpAddr.Port == 0 {
		t.Error("Expected non-zero port")
	}
}

func TestTCPTransport_ConnectionCleanup(t *testing.T) {
	// Start a TCP transport
	transport := NewTCPTransport()
	err := transport.Start(0)
	if err != nil {
		t.Fatalf("Failed to start TCP transport: %v", err)
	}
	defer transport.Stop()

	// Get the local address
	localAddr := transport.LocalAddr().(*net.TCPAddr)

	// Create a client connection
	clientConn, err := net.DialTCP("tcp", nil, localAddr)
	if err != nil {
		t.Fatalf("Failed to create client connection: %v", err)
	}

	// Send a message to establish the connection on server side
	testMessage := []byte("OPTIONS sip:test@example.com SIP/2.0\r\nContent-Length: 0\r\n\r\n")
	_, err = clientConn.Write(testMessage)
	if err != nil {
		t.Fatalf("Failed to send test message: %v", err)
	}

	// Wait for connection to be established
	time.Sleep(100 * time.Millisecond)

	// Check that connection is tracked
	transport.connMu.RLock()
	connectionCount := len(transport.connections)
	transport.connMu.RUnlock()

	if connectionCount == 0 {
		t.Error("Expected at least one tracked connection")
	}

	// Close client connection
	clientConn.Close()

	// Wait for cleanup
	time.Sleep(200 * time.Millisecond)

	// Check that connection was cleaned up
	transport.connMu.RLock()
	connectionCount = len(transport.connections)
	transport.connMu.RUnlock()

	if connectionCount != 0 {
		t.Errorf("Expected 0 tracked connections after cleanup, got %d", connectionCount)
	}
}