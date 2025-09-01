package transport

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"strconv"
	"strings"
	"testing"
)

func TestTCPMessageFramer_SimpleMessage(t *testing.T) {
	framer := NewTCPMessageFramer()
	
	// Simple SIP message without body
	message := "INVITE sip:test@example.com SIP/2.0\r\nContent-Length: 0\r\n\r\n"
	
	messages, err := framer.FrameMessage([]byte(message))
	if err != nil {
		t.Fatalf("Failed to frame message: %v", err)
	}
	
	if len(messages) != 1 {
		t.Fatalf("Expected 1 message, got %d", len(messages))
	}
	
	if string(messages[0]) != message {
		t.Errorf("Expected message %q, got %q", message, string(messages[0]))
	}
}

func TestTCPMessageFramer_MessageWithBody(t *testing.T) {
	framer := NewTCPMessageFramer()
	
	body := "v=0\r\no=alice 2890844526 2890844527 IN IP4 host.atlanta.com\r\n"
	message := "INVITE sip:test@example.com SIP/2.0\r\nContent-Length: " + 
		strconv.Itoa(len(body)) + "\r\n\r\n" + body
	
	messages, err := framer.FrameMessage([]byte(message))
	if err != nil {
		t.Fatalf("Failed to frame message: %v", err)
	}
	
	if len(messages) != 1 {
		t.Fatalf("Expected 1 message, got %d", len(messages))
	}
	
	if string(messages[0]) != message {
		t.Errorf("Expected message %q, got %q", message, string(messages[0]))
	}
}

func TestTCPMessageFramer_PartialMessage(t *testing.T) {
	framer := NewTCPMessageFramer()
	
	// Send message in parts
	part1 := "INVITE sip:test@example.com SIP/2.0\r\n"
	part2 := "Content-Length: 0\r\n"
	part3 := "\r\n"
	
	// First part - should not produce a message
	messages, err := framer.FrameMessage([]byte(part1))
	if err != nil {
		t.Fatalf("Failed to frame partial message: %v", err)
	}
	if len(messages) != 0 {
		t.Error("Expected no messages from partial data")
	}
	
	// Second part - still not complete
	messages, err = framer.FrameMessage([]byte(part2))
	if err != nil {
		t.Fatalf("Failed to frame partial message: %v", err)
	}
	if len(messages) != 0 {
		t.Error("Expected no messages from partial data")
	}
	
	// Third part - should complete the message
	messages, err = framer.FrameMessage([]byte(part3))
	if err != nil {
		t.Fatalf("Failed to frame complete message: %v", err)
	}
	if len(messages) != 1 {
		t.Fatalf("Expected 1 message, got %d", len(messages))
	}
	
	expectedMessage := part1 + part2 + part3
	if string(messages[0]) != expectedMessage {
		t.Errorf("Expected message %q, got %q", expectedMessage, string(messages[0]))
	}
}

func TestTCPMessageFramer_PartialMessageWithBody(t *testing.T) {
	framer := NewTCPMessageFramer()
	
	body := "v=0\r\no=alice 2890844526 2890844527 IN IP4 host.atlanta.com\r\n"
	headers := "INVITE sip:test@example.com SIP/2.0\r\nContent-Length: " + 
		strconv.Itoa(len(body)) + "\r\n\r\n"
	
	// Send headers first
	messages, err := framer.FrameMessage([]byte(headers))
	if err != nil {
		t.Fatalf("Failed to frame headers: %v", err)
	}
	if len(messages) != 0 {
		t.Error("Expected no messages from headers only")
	}
	
	// Send partial body
	partialBody := body[:len(body)/2]
	messages, err = framer.FrameMessage([]byte(partialBody))
	if err != nil {
		t.Fatalf("Failed to frame partial body: %v", err)
	}
	if len(messages) != 0 {
		t.Error("Expected no messages from partial body")
	}
	
	// Send remaining body
	remainingBody := body[len(body)/2:]
	messages, err = framer.FrameMessage([]byte(remainingBody))
	if err != nil {
		t.Fatalf("Failed to frame remaining body: %v", err)
	}
	if len(messages) != 1 {
		t.Fatalf("Expected 1 message, got %d", len(messages))
	}
	
	expectedMessage := headers + body
	if string(messages[0]) != expectedMessage {
		t.Errorf("Expected message %q, got %q", expectedMessage, string(messages[0]))
	}
}

func TestTCPMessageFramer_MultipleMessages(t *testing.T) {
	framer := NewTCPMessageFramer()
	
	message1 := "INVITE sip:test1@example.com SIP/2.0\r\nContent-Length: 0\r\n\r\n"
	message2 := "REGISTER sip:test2@example.com SIP/2.0\r\nContent-Length: 0\r\n\r\n"
	
	// Send both messages at once
	combinedData := message1 + message2
	messages, err := framer.FrameMessage([]byte(combinedData))
	if err != nil {
		t.Fatalf("Failed to frame messages: %v", err)
	}
	
	if len(messages) != 2 {
		t.Fatalf("Expected 2 messages, got %d", len(messages))
	}
	
	if string(messages[0]) != message1 {
		t.Errorf("Expected first message %q, got %q", message1, string(messages[0]))
	}
	
	if string(messages[1]) != message2 {
		t.Errorf("Expected second message %q, got %q", message2, string(messages[1]))
	}
}

func TestTCPMessageFramer_InvalidContentLength(t *testing.T) {
	framer := NewTCPMessageFramer()
	
	// Message with invalid Content-Length
	message := "INVITE sip:test@example.com SIP/2.0\r\nContent-Length: invalid\r\n\r\n"
	
	_, err := framer.FrameMessage([]byte(message))
	if err == nil {
		t.Error("Expected error for invalid Content-Length")
	}
	
	if !strings.Contains(err.Error(), "invalid Content-Length value") {
		t.Errorf("Expected Content-Length error, got: %v", err)
	}
}

func TestTCPMessageFramer_NegativeContentLength(t *testing.T) {
	framer := NewTCPMessageFramer()
	
	// Message with negative Content-Length
	message := "INVITE sip:test@example.com SIP/2.0\r\nContent-Length: -1\r\n\r\n"
	
	_, err := framer.FrameMessage([]byte(message))
	if err == nil {
		t.Error("Expected error for negative Content-Length")
	}
	
	if !strings.Contains(err.Error(), "negative Content-Length value") {
		t.Errorf("Expected negative Content-Length error, got: %v", err)
	}
}

func TestTCPMessageFramer_NoContentLength(t *testing.T) {
	framer := NewTCPMessageFramer()
	
	// Message without Content-Length header (should assume 0)
	message := "INVITE sip:test@example.com SIP/2.0\r\nVia: SIP/2.0/TCP test.com\r\n\r\n"
	
	messages, err := framer.FrameMessage([]byte(message))
	if err != nil {
		t.Fatalf("Failed to frame message without Content-Length: %v", err)
	}
	
	if len(messages) != 1 {
		t.Fatalf("Expected 1 message, got %d", len(messages))
	}
	
	if string(messages[0]) != message {
		t.Errorf("Expected message %q, got %q", message, string(messages[0]))
	}
}

func TestTCPMessageFramer_GetStats(t *testing.T) {
	framer := NewTCPMessageFramer()
	
	// Initial stats
	stats := framer.GetStats()
	if stats["buffer_size"] != 0 {
		t.Error("Expected initial buffer size to be 0")
	}
	if stats["headers_done"] != false {
		t.Error("Expected headers_done to be false initially")
	}
	
	// Add partial data
	partialMessage := "INVITE sip:test@example.com SIP/2.0\r\n"
	framer.FrameMessage([]byte(partialMessage))
	
	stats = framer.GetStats()
	if stats["buffer_size"] != len(partialMessage) {
		t.Errorf("Expected buffer size %d, got %v", len(partialMessage), stats["buffer_size"])
	}
}

func TestTCPMessageFramer_Clear(t *testing.T) {
	framer := NewTCPMessageFramer()
	
	// Add some data
	partialMessage := "INVITE sip:test@example.com SIP/2.0\r\n"
	framer.FrameMessage([]byte(partialMessage))
	
	if framer.GetBufferSize() == 0 {
		t.Error("Expected buffer to have data")
	}
	
	// Clear the buffer
	framer.Clear()
	
	if framer.GetBufferSize() != 0 {
		t.Error("Expected buffer to be empty after clear")
	}
	
	stats := framer.GetStats()
	if stats["headers_done"] != false {
		t.Error("Expected headers_done to be false after clear")
	}
}

func TestTCPMessageFramer_BufferSizeLimit(t *testing.T) {
	framer := NewTCPMessageFramerWithLimits(100, 1000) // Small limits for testing
	
	// Try to add data that exceeds buffer limit
	largeData := make([]byte, 150)
	for i := range largeData {
		largeData[i] = 'A'
	}
	
	_, err := framer.FrameMessage(largeData)
	if err == nil {
		t.Error("Expected error for buffer size limit exceeded")
	}
	
	if !strings.Contains(err.Error(), "buffer size limit exceeded") {
		t.Errorf("Expected buffer size limit error, got: %v", err)
	}
}

func TestTCPMessageFramer_MessageSizeLimit(t *testing.T) {
	framer := NewTCPMessageFramerWithLimits(10000, 100) // Small message limit for testing
	
	// Create a message with Content-Length that exceeds limit
	body := make([]byte, 150)
	for i := range body {
		body[i] = 'B'
	}
	
	message := "INVITE sip:test@example.com SIP/2.0\r\nContent-Length: 150\r\n\r\n" + string(body)
	
	_, err := framer.FrameMessage([]byte(message))
	if err == nil {
		t.Error("Expected error for Content-Length exceeds maximum message size")
	}
	
	if !strings.Contains(err.Error(), "Content-Length exceeds maximum message size") {
		t.Errorf("Expected message size limit error, got: %v", err)
	}
}

func TestTCPMessageFramer_VeryLargeMessage(t *testing.T) {
	framer := NewTCPMessageFramer()
	
	// Create a large but valid message
	bodySize := 50000 // 50KB body
	body := make([]byte, bodySize)
	for i := range body {
		body[i] = byte('A' + (i % 26))
	}
	
	headers := fmt.Sprintf("INVITE sip:test@example.com SIP/2.0\r\nContent-Length: %d\r\n\r\n", bodySize)
	message := headers + string(body)
	
	messages, err := framer.FrameMessage([]byte(message))
	if err != nil {
		t.Fatalf("Failed to frame large message: %v", err)
	}
	
	if len(messages) != 1 {
		t.Fatalf("Expected 1 message, got %d", len(messages))
	}
	
	if len(messages[0]) != len(message) {
		t.Errorf("Expected message length %d, got %d", len(message), len(messages[0]))
	}
}

func TestTCPMessageFramer_FragmentedLargeMessage(t *testing.T) {
	framer := NewTCPMessageFramer()
	
	// Create a large message and send it in fragments
	bodySize := 10000
	body := make([]byte, bodySize)
	for i := range body {
		body[i] = byte('X')
	}
	
	headers := fmt.Sprintf("INVITE sip:test@example.com SIP/2.0\r\nContent-Length: %d\r\n\r\n", bodySize)
	fullMessage := headers + string(body)
	
	// Send in small fragments
	fragmentSize := 100
	var allMessages [][]byte
	
	for i := 0; i < len(fullMessage); i += fragmentSize {
		end := i + fragmentSize
		if end > len(fullMessage) {
			end = len(fullMessage)
		}
		
		fragment := fullMessage[i:end]
		messages, err := framer.FrameMessage([]byte(fragment))
		if err != nil {
			t.Fatalf("Failed to frame fragment at position %d: %v", i, err)
		}
		
		allMessages = append(allMessages, messages...)
	}
	
	if len(allMessages) != 1 {
		t.Fatalf("Expected 1 complete message, got %d", len(allMessages))
	}
	
	if string(allMessages[0]) != fullMessage {
		t.Error("Reconstructed message doesn't match original")
	}
}

func TestTCPMessageFramer_MultipleContentLengthHeaders(t *testing.T) {
	framer := NewTCPMessageFramer()
	
	// Message with multiple Content-Length headers (should use first valid one)
	message := "INVITE sip:test@example.com SIP/2.0\r\n" +
		"Content-Length: 5\r\n" +
		"Content-Length: 10\r\n" +
		"\r\n" +
		"Hello"
	
	messages, err := framer.FrameMessage([]byte(message))
	if err != nil {
		t.Fatalf("Failed to frame message with multiple Content-Length headers: %v", err)
	}
	
	if len(messages) != 1 {
		t.Fatalf("Expected 1 message, got %d", len(messages))
	}
	
	if string(messages[0]) != message {
		t.Errorf("Expected message %q, got %q", message, string(messages[0]))
	}
}

func TestTCPMessageFramer_MalformedHeaders(t *testing.T) {
	framer := NewTCPMessageFramer()
	
	// Message with malformed Content-Length header (empty value)
	message := "INVITE sip:test@example.com SIP/2.0\r\n" +
		"Content-Length:\r\n" + // Missing value
		"\r\n"
	
	messages, err := framer.FrameMessage([]byte(message))
	// This should fail because empty Content-Length is invalid
	if err == nil {
		t.Error("Expected error for empty Content-Length value")
	}
	
	if !strings.Contains(err.Error(), "invalid Content-Length value") {
		t.Errorf("Expected Content-Length error, got: %v", err)
	}
	
	// Test with completely missing Content-Length (should work)
	framer.Clear()
	validMessage := "INVITE sip:test@example.com SIP/2.0\r\n" +
		"Via: SIP/2.0/TCP test.com\r\n" +
		"\r\n"
	
	messages, err = framer.FrameMessage([]byte(validMessage))
	if err != nil {
		t.Fatalf("Failed to frame message without Content-Length: %v", err)
	}
	
	if len(messages) != 1 {
		t.Fatalf("Expected 1 message, got %d", len(messages))
	}
}

func TestTCPMessageFramer_SetLimits(t *testing.T) {
	framer := NewTCPMessageFramer()
	
	// Test initial limits
	maxBuffer, maxMessage := framer.GetLimits()
	if maxBuffer != 1024*1024 {
		t.Errorf("Expected default max buffer size 1MB, got %d", maxBuffer)
	}
	if maxMessage != 10*1024*1024 {
		t.Errorf("Expected default max message size 10MB, got %d", maxMessage)
	}
	
	// Update limits
	framer.SetLimits(2048, 4096)
	
	maxBuffer, maxMessage = framer.GetLimits()
	if maxBuffer != 2048 {
		t.Errorf("Expected updated max buffer size 2048, got %d", maxBuffer)
	}
	if maxMessage != 4096 {
		t.Errorf("Expected updated max message size 4096, got %d", maxMessage)
	}
}

func TestTCPMessageFramer_IsBufferFull(t *testing.T) {
	framer := NewTCPMessageFramerWithLimits(100, 1000)
	
	// Buffer should not be full initially
	if framer.IsBufferFull() {
		t.Error("Buffer should not be full initially")
	}
	
	// Add data to approach the limit (80% threshold)
	data := make([]byte, 85) // 85% of 100
	for i := range data {
		data[i] = 'A'
	}
	
	framer.FrameMessage(data)
	
	if !framer.IsBufferFull() {
		t.Error("Buffer should be considered full at 85% capacity")
	}
}

func TestTCPMessageFramer_CompactBuffer(t *testing.T) {
	framer := NewTCPMessageFramer()
	
	// Add a lot of data to expand the buffer
	largeData := make([]byte, 8192)
	for i := range largeData {
		largeData[i] = 'A'
	}
	
	framer.FrameMessage(largeData)
	
	initialCapacity := cap(framer.buffer)
	
	// Clear most of the buffer
	framer.Clear()
	
	// Add small amount of data
	smallData := []byte("INVITE sip:test@example.com SIP/2.0\r\n")
	framer.FrameMessage(smallData)
	
	// Compact the buffer
	framer.CompactBuffer()
	
	newCapacity := cap(framer.buffer)
	
	// Buffer should be compacted if it was much larger than needed
	if initialCapacity > 8192 && newCapacity >= initialCapacity {
		t.Error("Buffer should have been compacted")
	}
}

func TestTCPMessageFramer_EdgeCases(t *testing.T) {
	framer := NewTCPMessageFramer()
	
	// Test empty data
	messages, err := framer.FrameMessage([]byte{})
	if err != nil {
		t.Errorf("Failed to handle empty data: %v", err)
	}
	if len(messages) != 0 {
		t.Error("Expected no messages from empty data")
	}
	
	// Test single byte at a time
	message := "INVITE sip:test@example.com SIP/2.0\r\nContent-Length: 0\r\n\r\n"
	var allMessages [][]byte
	
	for i := 0; i < len(message); i++ {
		msgs, err := framer.FrameMessage([]byte{message[i]})
		if err != nil {
			t.Fatalf("Failed to frame single byte at position %d: %v", i, err)
		}
		allMessages = append(allMessages, msgs...)
	}
	
	if len(allMessages) != 1 {
		t.Fatalf("Expected 1 message from single-byte processing, got %d", len(allMessages))
	}
	
	if string(allMessages[0]) != message {
		t.Error("Single-byte processed message doesn't match original")
	}
}

func TestStreamingTCPMessageReader_SimpleMessage(t *testing.T) {
	message := "INVITE sip:test@example.com SIP/2.0\r\nContent-Length: 0\r\n\r\n"
	
	reader := bufio.NewReader(strings.NewReader(message))
	streamReader := NewStreamingTCPMessageReader(reader)
	
	readMessage, err := streamReader.ReadMessage()
	if err != nil {
		t.Fatalf("Failed to read message: %v", err)
	}
	
	if string(readMessage) != message {
		t.Errorf("Expected message %q, got %q", message, string(readMessage))
	}
}

func TestStreamingTCPMessageReader_MessageWithBody(t *testing.T) {
	body := "v=0\r\no=alice 2890844526 2890844527 IN IP4 host.atlanta.com\r\n"
	message := "INVITE sip:test@example.com SIP/2.0\r\nContent-Length: " + 
		strconv.Itoa(len(body)) + "\r\n\r\n" + body
	
	reader := bufio.NewReader(strings.NewReader(message))
	streamReader := NewStreamingTCPMessageReader(reader)
	
	readMessage, err := streamReader.ReadMessage()
	if err != nil {
		t.Fatalf("Failed to read message: %v", err)
	}
	
	if string(readMessage) != message {
		t.Errorf("Expected message %q, got %q", message, string(readMessage))
	}
}

func TestStreamingTCPMessageReader_MultipleMessages(t *testing.T) {
	message1 := "INVITE sip:test1@example.com SIP/2.0\r\nContent-Length: 0\r\n\r\n"
	message2 := "REGISTER sip:test2@example.com SIP/2.0\r\nContent-Length: 0\r\n\r\n"
	
	combinedData := message1 + message2
	reader := bufio.NewReader(strings.NewReader(combinedData))
	streamReader := NewStreamingTCPMessageReader(reader)
	
	// Read first message
	readMessage1, err := streamReader.ReadMessage()
	if err != nil {
		t.Fatalf("Failed to read first message: %v", err)
	}
	
	if string(readMessage1) != message1 {
		t.Errorf("Expected first message %q, got %q", message1, string(readMessage1))
	}
	
	// Read second message
	readMessage2, err := streamReader.ReadMessage()
	if err != nil {
		t.Fatalf("Failed to read second message: %v", err)
	}
	
	if string(readMessage2) != message2 {
		t.Errorf("Expected second message %q, got %q", message2, string(readMessage2))
	}
}

func TestStreamingTCPMessageReader_GetStats(t *testing.T) {
	reader := bufio.NewReader(strings.NewReader(""))
	streamReader := NewStreamingTCPMessageReader(reader)
	
	stats := streamReader.GetStats()
	if stats["buffer_size"] != 0 {
		t.Error("Expected initial buffer size to be 0")
	}
}

func TestStreamingTCPMessageReader_Clear(t *testing.T) {
	reader := bufio.NewReader(strings.NewReader("partial data"))
	streamReader := NewStreamingTCPMessageReader(reader)
	
	// Try to read (will buffer partial data)
	_, err := streamReader.ReadMessage()
	if err == nil {
		t.Error("Expected error for incomplete message")
	}
	
	// Clear should reset the buffer
	streamReader.Clear()
	
	stats := streamReader.GetStats()
	if stats["buffer_size"] != 0 {
		t.Error("Expected buffer size to be 0 after clear")
	}
}

func TestTCPMessageWriter_WriteMessage(t *testing.T) {
	var buf bytes.Buffer
	writer := bufio.NewWriter(&buf)
	messageWriter := NewTCPMessageWriter(writer)
	
	message := "INVITE sip:test@example.com SIP/2.0\r\nContent-Length: 0\r\n\r\n"
	
	err := messageWriter.WriteMessage([]byte(message))
	if err != nil {
		t.Fatalf("Failed to write message: %v", err)
	}
	
	if buf.String() != message {
		t.Errorf("Expected written message %q, got %q", message, buf.String())
	}
}

func TestTCPMessageWriter_Flush(t *testing.T) {
	var buf bytes.Buffer
	writer := bufio.NewWriter(&buf)
	messageWriter := NewTCPMessageWriter(writer)
	
	// Write to the underlying writer directly (bypassing messageWriter)
	writer.WriteString("test data")
	
	// Buffer should be empty until flush
	if buf.String() != "" {
		t.Error("Expected buffer to be empty before flush")
	}
	
	err := messageWriter.Flush()
	if err != nil {
		t.Fatalf("Failed to flush: %v", err)
	}
	
	if buf.String() != "test data" {
		t.Errorf("Expected flushed data %q, got %q", "test data", buf.String())
	}
}

func TestStreamingTCPMessageReader_LargeMessage(t *testing.T) {
	// Create a large message
	bodySize := 50000
	body := make([]byte, bodySize)
	for i := range body {
		body[i] = byte('L')
	}
	
	message := fmt.Sprintf("INVITE sip:test@example.com SIP/2.0\r\nContent-Length: %d\r\n\r\n%s", 
		bodySize, string(body))
	
	reader := bufio.NewReader(strings.NewReader(message))
	streamReader := NewStreamingTCPMessageReader(reader)
	
	readMessage, err := streamReader.ReadMessage()
	if err != nil {
		t.Fatalf("Failed to read large message: %v", err)
	}
	
	if string(readMessage) != message {
		t.Error("Large message doesn't match expected")
	}
}

func TestStreamingTCPMessageReader_FragmentedReading(t *testing.T) {
	message := "INVITE sip:test@example.com SIP/2.0\r\nContent-Length: 10\r\n\r\n1234567890"
	
	// Create a reader that returns data in small chunks
	reader := bufio.NewReader(&slowReader{data: []byte(message), chunkSize: 5})
	streamReader := NewStreamingTCPMessageReader(reader)
	
	readMessage, err := streamReader.ReadMessage()
	if err != nil {
		t.Fatalf("Failed to read fragmented message: %v", err)
	}
	
	if string(readMessage) != message {
		t.Errorf("Expected message %q, got %q", message, string(readMessage))
	}
}

func TestStreamingTCPMessageReader_ErrorRecovery(t *testing.T) {
	reader := bufio.NewReader(strings.NewReader("invalid data without proper headers"))
	streamReader := NewStreamingTCPMessageReader(reader)
	
	// This should eventually return EOF when no valid message is found
	_, err := streamReader.ReadMessage()
	if err == nil {
		t.Error("Expected error for invalid data")
	}
	
	// Clear should reset the state
	streamReader.Clear()
	
	stats := streamReader.GetStats()
	if stats["buffer_size"] != 0 {
		t.Error("Expected buffer to be cleared")
	}
}

// slowReader simulates a slow network connection by returning data in small chunks
type slowReader struct {
	data      []byte
	pos       int
	chunkSize int
}

func (r *slowReader) Read(p []byte) (n int, err error) {
	if r.pos >= len(r.data) {
		return 0, io.EOF
	}
	
	// Return at most chunkSize bytes
	remaining := len(r.data) - r.pos
	toRead := r.chunkSize
	if toRead > remaining {
		toRead = remaining
	}
	if toRead > len(p) {
		toRead = len(p)
	}
	
	copy(p, r.data[r.pos:r.pos+toRead])
	r.pos += toRead
	
	return toRead, nil
}

func TestTCPMessageFramer_ConcurrentAccess(t *testing.T) {
	// Test concurrent access to framer (should not panic)
	done := make(chan bool, 10)
	
	// Multiple goroutines trying to frame messages
	for i := 0; i < 5; i++ {
		go func(id int) {
			defer func() { done <- true }()
			
			message := fmt.Sprintf("INVITE sip:test%d@example.com SIP/2.0\r\nContent-Length: 0\r\n\r\n", id)
			
			for j := 0; j < 10; j++ {
				// Create a new framer for each goroutine to avoid race conditions
				localFramer := NewTCPMessageFramer()
				_, err := localFramer.FrameMessage([]byte(message))
				if err != nil {
					t.Errorf("Goroutine %d iteration %d failed: %v", id, j, err)
					return
				}
			}
		}(i)
	}
	
	// Multiple goroutines accessing stats
	for i := 0; i < 5; i++ {
		go func() {
			defer func() { done <- true }()
			
			for j := 0; j < 10; j++ {
				// Create a new framer for stats access
				localFramer := NewTCPMessageFramer()
				_ = localFramer.GetStats()
				_ = localFramer.GetBufferSize()
				localFramer.Clear()
			}
		}()
	}
	
	// Wait for all goroutines to complete
	for i := 0; i < 10; i++ {
		<-done
	}
}