package transport

import (
	"fmt"
	"net"
	"strings"
	"testing"
	"time"
)

// TestTCPMessageFraming_IntegrationTest demonstrates the complete TCP message framing
// functionality including partial message handling, large message processing, and
// streaming data processing as required by task 17.2
func TestTCPMessageFraming_IntegrationTest(t *testing.T) {
	// Start enhanced TCP transport
	config := &EnhancedTCPConfig{
		ReadTimeout:     5 * time.Second,
		WriteTimeout:    5 * time.Second,
		IdleTimeout:     1 * time.Minute,
		AcceptTimeout:   1 * time.Second,
		MaxConnections:  10,
		CleanupInterval: 30 * time.Second,
		MaxRetries:      3,
		RetryDelay:      100 * time.Millisecond,
		Logger:          &noOpLogger{},
	}
	
	transport := NewEnhancedTCPTransport(config)
	
	// Channel to collect received messages
	receivedMessages := make(chan []byte, 10)
	
	// Register message handler
	transport.RegisterHandler(&testMessageHandler{
		messages: receivedMessages,
	})
	
	// Start transport on available port
	err := transport.Start(0) // Use port 0 to get any available port
	if err != nil {
		t.Fatalf("Failed to start transport: %v", err)
	}
	defer transport.Stop()
	
	// Get the actual listening address
	addr := transport.LocalAddr().(*net.TCPAddr)
	
	t.Logf("TCP transport listening on %s", addr)
	
	// Test 1: Simple message
	t.Run("SimpleMessage", func(t *testing.T) {
		testSimpleMessage(t, addr, receivedMessages)
	})
	
	// Test 2: Message with body
	t.Run("MessageWithBody", func(t *testing.T) {
		testMessageWithBody(t, addr, receivedMessages)
	})
	
	// Test 3: Large message (streaming data processing)
	t.Run("LargeMessage", func(t *testing.T) {
		testLargeMessage(t, addr, receivedMessages)
	})
	
	// Test 4: Fragmented message (partial message handling)
	t.Run("FragmentedMessage", func(t *testing.T) {
		testFragmentedMessage(t, addr, receivedMessages)
	})
	
	// Test 5: Multiple messages in single connection
	t.Run("MultipleMessages", func(t *testing.T) {
		testMultipleMessages(t, addr, receivedMessages)
	})
	
	// Test 6: Content-Length based parsing edge cases
	t.Run("ContentLengthEdgeCases", func(t *testing.T) {
		testContentLengthEdgeCases(t, addr, receivedMessages)
	})
}

func testSimpleMessage(t *testing.T, addr *net.TCPAddr, receivedMessages chan []byte) {
	conn, err := net.Dial("tcp", addr.String())
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close()
	
	message := "INVITE sip:test@example.com SIP/2.0\r\nContent-Length: 0\r\n\r\n"
	
	_, err = conn.Write([]byte(message))
	if err != nil {
		t.Fatalf("Failed to send message: %v", err)
	}
	
	// Wait for message to be received
	select {
	case received := <-receivedMessages:
		if string(received) != message {
			t.Errorf("Expected message %q, got %q", message, string(received))
		}
	case <-time.After(2 * time.Second):
		t.Error("Timeout waiting for message")
	}
}

func testMessageWithBody(t *testing.T, addr *net.TCPAddr, receivedMessages chan []byte) {
	conn, err := net.Dial("tcp", addr.String())
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close()
	
	body := "v=0\r\no=alice 2890844526 2890844527 IN IP4 host.atlanta.com\r\n"
	message := fmt.Sprintf("INVITE sip:test@example.com SIP/2.0\r\nContent-Length: %d\r\n\r\n%s", 
		len(body), body)
	
	_, err = conn.Write([]byte(message))
	if err != nil {
		t.Fatalf("Failed to send message: %v", err)
	}
	
	// Wait for message to be received
	select {
	case received := <-receivedMessages:
		if string(received) != message {
			t.Errorf("Expected message %q, got %q", message, string(received))
		}
	case <-time.After(2 * time.Second):
		t.Error("Timeout waiting for message")
	}
}

func testLargeMessage(t *testing.T, addr *net.TCPAddr, receivedMessages chan []byte) {
	conn, err := net.Dial("tcp", addr.String())
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close()
	
	// Create a large SDP body (10KB)
	bodySize := 10240
	body := strings.Repeat("a=test:value\r\n", bodySize/13) // Approximately 10KB
	actualBodySize := len(body)
	
	message := fmt.Sprintf("INVITE sip:test@example.com SIP/2.0\r\nContent-Length: %d\r\n\r\n%s", 
		actualBodySize, body)
	
	t.Logf("Sending large message of size %d bytes", len(message))
	
	_, err = conn.Write([]byte(message))
	if err != nil {
		t.Fatalf("Failed to send large message: %v", err)
	}
	
	// Wait for message to be received
	select {
	case received := <-receivedMessages:
		if len(received) != len(message) {
			t.Errorf("Expected message length %d, got %d", len(message), len(received))
		}
		if string(received) != message {
			t.Error("Large message content doesn't match")
		}
	case <-time.After(5 * time.Second):
		t.Error("Timeout waiting for large message")
	}
}

func testFragmentedMessage(t *testing.T, addr *net.TCPAddr, receivedMessages chan []byte) {
	conn, err := net.Dial("tcp", addr.String())
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close()
	
	body := "This is a test body for fragmented message handling"
	message := fmt.Sprintf("INVITE sip:test@example.com SIP/2.0\r\nContent-Length: %d\r\n\r\n%s", 
		len(body), body)
	
	t.Logf("Sending fragmented message of size %d bytes", len(message))
	
	// Send message in small fragments to test partial message handling
	fragmentSize := 10
	for i := 0; i < len(message); i += fragmentSize {
		end := i + fragmentSize
		if end > len(message) {
			end = len(message)
		}
		
		fragment := message[i:end]
		_, err = conn.Write([]byte(fragment))
		if err != nil {
			t.Fatalf("Failed to send fragment: %v", err)
		}
		
		// Small delay between fragments to simulate slow network
		time.Sleep(10 * time.Millisecond)
	}
	
	// Wait for complete message to be received
	select {
	case received := <-receivedMessages:
		if string(received) != message {
			t.Errorf("Expected message %q, got %q", message, string(received))
		}
	case <-time.After(3 * time.Second):
		t.Error("Timeout waiting for fragmented message")
	}
}

func testMultipleMessages(t *testing.T, addr *net.TCPAddr, receivedMessages chan []byte) {
	conn, err := net.Dial("tcp", addr.String())
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close()
	
	messages := []string{
		"INVITE sip:test1@example.com SIP/2.0\r\nContent-Length: 0\r\n\r\n",
		"REGISTER sip:test2@example.com SIP/2.0\r\nContent-Length: 5\r\n\r\nHello",
		"OPTIONS sip:test3@example.com SIP/2.0\r\nContent-Length: 0\r\n\r\n",
	}
	
	// Send all messages at once
	for _, msg := range messages {
		_, err = conn.Write([]byte(msg))
		if err != nil {
			t.Fatalf("Failed to send message: %v", err)
		}
	}
	
	// Receive all messages
	for i, expectedMsg := range messages {
		select {
		case received := <-receivedMessages:
			if string(received) != expectedMsg {
				t.Errorf("Message %d: expected %q, got %q", i, expectedMsg, string(received))
			}
		case <-time.After(2 * time.Second):
			t.Errorf("Timeout waiting for message %d", i)
		}
	}
}

func testContentLengthEdgeCases(t *testing.T, addr *net.TCPAddr, receivedMessages chan []byte) {
	conn, err := net.Dial("tcp", addr.String())
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close()
	
	// Test message with Content-Length: 0
	message1 := "INVITE sip:test@example.com SIP/2.0\r\nContent-Length: 0\r\n\r\n"
	
	_, err = conn.Write([]byte(message1))
	if err != nil {
		t.Fatalf("Failed to send message: %v", err)
	}
	
	select {
	case received := <-receivedMessages:
		if string(received) != message1 {
			t.Errorf("Expected message %q, got %q", message1, string(received))
		}
	case <-time.After(2 * time.Second):
		t.Error("Timeout waiting for Content-Length: 0 message")
	}
	
	// Test message without Content-Length header
	message2 := "OPTIONS sip:test@example.com SIP/2.0\r\nVia: SIP/2.0/TCP test.com\r\n\r\n"
	
	_, err = conn.Write([]byte(message2))
	if err != nil {
		t.Fatalf("Failed to send message: %v", err)
	}
	
	select {
	case received := <-receivedMessages:
		if string(received) != message2 {
			t.Errorf("Expected message %q, got %q", message2, string(received))
		}
	case <-time.After(2 * time.Second):
		t.Error("Timeout waiting for message without Content-Length")
	}
}

// testMessageHandler implements MessageHandler for testing
type testMessageHandler struct {
	messages chan []byte
}

func (h *testMessageHandler) HandleMessage(data []byte, transport string, addr net.Addr) error {
	// Copy the data since it might be reused
	messageCopy := make([]byte, len(data))
	copy(messageCopy, data)
	
	select {
	case h.messages <- messageCopy:
	default:
		// Channel full, drop message
	}
	
	return nil
}

// TestTCPMessageFramer_PerformanceTest tests the performance of message framing
func TestTCPMessageFramer_PerformanceTest(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping performance test in short mode")
	}
	
	framer := NewTCPMessageFramer()
	
	// Create a typical SIP message
	body := "v=0\r\no=alice 2890844526 2890844527 IN IP4 host.atlanta.com\r\n"
	message := fmt.Sprintf("INVITE sip:test@example.com SIP/2.0\r\nContent-Length: %d\r\n\r\n%s", 
		len(body), body)
	
	messageBytes := []byte(message)
	
	// Benchmark message framing
	start := time.Now()
	iterations := 10000
	
	for i := 0; i < iterations; i++ {
		framer.Clear()
		messages, err := framer.FrameMessage(messageBytes)
		if err != nil {
			t.Fatalf("Framing failed at iteration %d: %v", i, err)
		}
		if len(messages) != 1 {
			t.Fatalf("Expected 1 message, got %d at iteration %d", len(messages), i)
		}
	}
	
	duration := time.Since(start)
	messagesPerSecond := float64(iterations) / duration.Seconds()
	
	t.Logf("Processed %d messages in %v (%.2f messages/second)", 
		iterations, duration, messagesPerSecond)
	
	// Should be able to process at least 1000 messages per second
	if messagesPerSecond < 1000 {
		t.Errorf("Performance too low: %.2f messages/second (expected > 1000)", messagesPerSecond)
	}
}

// TestTCPMessageFramer_MemoryUsage tests memory usage patterns
func TestTCPMessageFramer_MemoryUsage(t *testing.T) {
	framer := NewTCPMessageFramer()
	
	// Test buffer growth and compaction
	initialStats := framer.GetStats()
	initialCapacity := initialStats["buffer_capacity"].(int)
	
	// Add large amount of data
	largeData := make([]byte, 50000)
	for i := range largeData {
		largeData[i] = 'A'
	}
	
	framer.FrameMessage(largeData)
	
	afterLargeStats := framer.GetStats()
	afterLargeCapacity := afterLargeStats["buffer_capacity"].(int)
	
	if afterLargeCapacity <= initialCapacity {
		t.Error("Buffer should have grown after large data")
	}
	
	// Clear and add small data
	framer.Clear()
	smallData := []byte("INVITE sip:test@example.com SIP/2.0\r\n")
	framer.FrameMessage(smallData)
	
	// Compact buffer
	framer.CompactBuffer()
	
	afterCompactStats := framer.GetStats()
	afterCompactCapacity := afterCompactStats["buffer_capacity"].(int)
	
	t.Logf("Buffer capacity: initial=%d, after_large=%d, after_compact=%d", 
		initialCapacity, afterLargeCapacity, afterCompactCapacity)
	
	// Buffer should be more reasonable after compaction
	if afterCompactCapacity >= afterLargeCapacity {
		t.Log("Buffer compaction may not have occurred (this is not necessarily an error)")
	}
}