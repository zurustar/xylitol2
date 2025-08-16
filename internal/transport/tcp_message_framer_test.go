package transport

import (
	"bufio"
	"bytes"
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