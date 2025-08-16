package transport

import (
	"fmt"
	"net"
	"sync"
	"testing"
	"time"
)

// mockMessageHandler implements MessageHandler for testing
type mockMessageHandler struct {
	messages []mockMessage
	mu       sync.Mutex
}

type mockMessage struct {
	data      []byte
	transport string
	addr      net.Addr
}

func (m *mockMessageHandler) HandleMessage(data []byte, transport string, addr net.Addr) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	// Make a copy of the data
	dataCopy := make([]byte, len(data))
	copy(dataCopy, data)
	
	m.messages = append(m.messages, mockMessage{
		data:      dataCopy,
		transport: transport,
		addr:      addr,
	})
	return nil
}

func (m *mockMessageHandler) getMessages() []mockMessage {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	result := make([]mockMessage, len(m.messages))
	copy(result, m.messages)
	return result
}

func (m *mockMessageHandler) reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.messages = nil
}

func TestUDPTransport_StartStop(t *testing.T) {
	transport := NewUDPTransport()

	// Test starting on a free port
	err := transport.Start(0) // Port 0 means system will choose a free port
	if err != nil {
		t.Fatalf("Failed to start UDP transport: %v", err)
	}

	if !transport.IsRunning() {
		t.Error("Transport should be running after start")
	}

	// Test stopping
	err = transport.Stop()
	if err != nil {
		t.Fatalf("Failed to stop UDP transport: %v", err)
	}

	if transport.IsRunning() {
		t.Error("Transport should not be running after stop")
	}
}

func TestUDPTransport_StartTwice(t *testing.T) {
	transport := NewUDPTransport()

	err := transport.Start(0)
	if err != nil {
		t.Fatalf("Failed to start UDP transport: %v", err)
	}
	defer transport.Stop()

	// Try to start again - should fail
	err = transport.Start(0)
	if err == nil {
		t.Error("Expected error when starting transport twice")
	}
}

func TestUDPTransport_SendMessage(t *testing.T) {
	// Start a UDP transport
	transport := NewUDPTransport()
	err := transport.Start(0)
	if err != nil {
		t.Fatalf("Failed to start UDP transport: %v", err)
	}
	defer transport.Stop()

	// Get the local address and convert to IPv4 if needed
	localAddr := transport.LocalAddr().(*net.UDPAddr)
	
	// Create IPv4 address to avoid IPv6 routing issues in tests
	testAddr := &net.UDPAddr{
		IP:   net.IPv4(127, 0, 0, 1),
		Port: localAddr.Port,
	}

	// Create a test message
	testMessage := []byte("INVITE sip:test@example.com SIP/2.0\r\n\r\n")

	// Send message to localhost
	err = transport.SendMessage(testMessage, testAddr)
	if err != nil {
		t.Fatalf("Failed to send message: %v", err)
	}
}

func TestUDPTransport_SendMessageNotRunning(t *testing.T) {
	transport := NewUDPTransport()

	testMessage := []byte("test message")
	addr, _ := net.ResolveUDPAddr("udp", "127.0.0.1:5060")

	err := transport.SendMessage(testMessage, addr)
	if err == nil {
		t.Error("Expected error when sending message on stopped transport")
	}
}

func TestUDPTransport_SendMessageInvalidAddress(t *testing.T) {
	transport := NewUDPTransport()
	err := transport.Start(0)
	if err != nil {
		t.Fatalf("Failed to start UDP transport: %v", err)
	}
	defer transport.Stop()

	testMessage := []byte("test message")
	// Use TCP address instead of UDP
	addr, _ := net.ResolveTCPAddr("tcp", "127.0.0.1:5060")

	err = transport.SendMessage(testMessage, addr)
	if err == nil {
		t.Error("Expected error when sending message with invalid address type")
	}
}

func TestUDPTransport_ReceiveMessage(t *testing.T) {
	// Start a UDP transport
	transport := NewUDPTransport()
	err := transport.Start(0)
	if err != nil {
		t.Fatalf("Failed to start UDP transport: %v", err)
	}
	defer transport.Stop()

	// Register a mock handler
	handler := &mockMessageHandler{}
	transport.RegisterHandler(handler)

	// Get the local address
	localAddr := transport.LocalAddr().(*net.UDPAddr)

	// Create a client connection to send a message
	clientConn, err := net.DialUDP("udp", nil, localAddr)
	if err != nil {
		t.Fatalf("Failed to create client connection: %v", err)
	}
	defer clientConn.Close()

	// Send a test message
	testMessage := []byte("REGISTER sip:test@example.com SIP/2.0\r\n\r\n")
	_, err = clientConn.Write(testMessage)
	if err != nil {
		t.Fatalf("Failed to send test message: %v", err)
	}

	// Wait for message to be received and processed
	time.Sleep(100 * time.Millisecond)

	// Check that the message was received
	messages := handler.getMessages()
	if len(messages) != 1 {
		t.Fatalf("Expected 1 message, got %d", len(messages))
	}

	if string(messages[0].data) != string(testMessage) {
		t.Errorf("Expected message %q, got %q", string(testMessage), string(messages[0].data))
	}

	if messages[0].transport != "UDP" {
		t.Errorf("Expected transport 'UDP', got %q", messages[0].transport)
	}

	if messages[0].addr == nil {
		t.Error("Expected non-nil address")
	}
}

func TestUDPTransport_MultipleMessages(t *testing.T) {
	// Start a UDP transport
	transport := NewUDPTransport()
	err := transport.Start(0)
	if err != nil {
		t.Fatalf("Failed to start UDP transport: %v", err)
	}
	defer transport.Stop()

	// Register a mock handler
	handler := &mockMessageHandler{}
	transport.RegisterHandler(handler)

	// Get the local address
	localAddr := transport.LocalAddr().(*net.UDPAddr)

	// Create a client connection
	clientConn, err := net.DialUDP("udp", nil, localAddr)
	if err != nil {
		t.Fatalf("Failed to create client connection: %v", err)
	}
	defer clientConn.Close()

	// Send multiple test messages
	testMessages := []string{
		"INVITE sip:test1@example.com SIP/2.0\r\n\r\n",
		"REGISTER sip:test2@example.com SIP/2.0\r\n\r\n",
		"OPTIONS sip:test3@example.com SIP/2.0\r\n\r\n",
	}

	for _, msg := range testMessages {
		_, err = clientConn.Write([]byte(msg))
		if err != nil {
			t.Fatalf("Failed to send test message: %v", err)
		}
	}

	// Wait for messages to be received and processed
	time.Sleep(200 * time.Millisecond)

	// Check that all messages were received
	messages := handler.getMessages()
	if len(messages) != len(testMessages) {
		t.Fatalf("Expected %d messages, got %d", len(testMessages), len(messages))
	}

	// Verify each message (order might not be preserved due to goroutines)
	receivedMessages := make(map[string]bool)
	for _, msg := range messages {
		receivedMessages[string(msg.data)] = true
		if msg.transport != "UDP" {
			t.Errorf("Expected transport 'UDP', got %q", msg.transport)
		}
	}

	for _, expected := range testMessages {
		if !receivedMessages[expected] {
			t.Errorf("Expected message %q not received", expected)
		}
	}
}

func TestUDPTransport_LocalAddr(t *testing.T) {
	transport := NewUDPTransport()

	// Should return nil when not running
	if addr := transport.LocalAddr(); addr != nil {
		t.Error("Expected nil address when transport not running")
	}

	// Start transport
	err := transport.Start(0)
	if err != nil {
		t.Fatalf("Failed to start UDP transport: %v", err)
	}
	defer transport.Stop()

	// Should return valid address when running
	addr := transport.LocalAddr()
	if addr == nil {
		t.Error("Expected non-nil address when transport running")
	}

	udpAddr, ok := addr.(*net.UDPAddr)
	if !ok {
		t.Errorf("Expected UDP address, got %T", addr)
	}

	if udpAddr.Port == 0 {
		t.Error("Expected non-zero port")
	}
}

func TestUDPTransport_ConcurrentOperations(t *testing.T) {
	transport := NewUDPTransport()
	err := transport.Start(0)
	if err != nil {
		t.Fatalf("Failed to start UDP transport: %v", err)
	}
	defer transport.Stop()

	handler := &mockMessageHandler{}
	transport.RegisterHandler(handler)

	localAddr := transport.LocalAddr().(*net.UDPAddr)

	// Test concurrent sends and receives
	var wg sync.WaitGroup
	numGoroutines := 10
	messagesPerGoroutine := 5

	// Start sender goroutines
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			
			clientConn, err := net.DialUDP("udp", nil, localAddr)
			if err != nil {
				t.Errorf("Failed to create client connection: %v", err)
				return
			}
			defer clientConn.Close()

			for j := 0; j < messagesPerGoroutine; j++ {
				msg := []byte(fmt.Sprintf("Message from goroutine %d, message %d", id, j))
				_, err := clientConn.Write(msg)
				if err != nil {
					t.Errorf("Failed to send message: %v", err)
				}
			}
		}(i)
	}

	wg.Wait()

	// Wait for all messages to be processed
	time.Sleep(500 * time.Millisecond)

	// Verify we received all messages
	messages := handler.getMessages()
	expectedCount := numGoroutines * messagesPerGoroutine
	if len(messages) != expectedCount {
		t.Errorf("Expected %d messages, got %d", expectedCount, len(messages))
	}
}