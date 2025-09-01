package transport

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// TCPMessageFramer handles SIP message framing for TCP transport
type TCPMessageFramer struct {
	buffer       []byte
	headersDone  bool
	contentLength int
	expectedBodyLength int
	currentBodyLength  int
	maxBufferSize      int  // Maximum buffer size to prevent memory exhaustion
	maxMessageSize     int  // Maximum message size to prevent DoS attacks
}

// NewTCPMessageFramer creates a new TCP message framer
func NewTCPMessageFramer() *TCPMessageFramer {
	return &TCPMessageFramer{
		buffer:         make([]byte, 0, 4096), // Initial buffer capacity
		headersDone:    false,
		contentLength:  -1,
		maxBufferSize:  1024 * 1024, // 1MB max buffer size
		maxMessageSize: 10 * 1024 * 1024, // 10MB max message size
	}
}

// NewTCPMessageFramerWithLimits creates a new TCP message framer with custom limits
func NewTCPMessageFramerWithLimits(maxBufferSize, maxMessageSize int) *TCPMessageFramer {
	return &TCPMessageFramer{
		buffer:         make([]byte, 0, 4096),
		headersDone:    false,
		contentLength:  -1,
		maxBufferSize:  maxBufferSize,
		maxMessageSize: maxMessageSize,
	}
}

// FrameMessage processes incoming data and extracts complete SIP messages
// Returns complete messages and any error encountered
func (f *TCPMessageFramer) FrameMessage(data []byte) ([][]byte, error) {
	var messages [][]byte
	
	// Check buffer size limits before appending new data
	if len(f.buffer)+len(data) > f.maxBufferSize {
		return nil, fmt.Errorf("buffer size limit exceeded: current=%d, incoming=%d, max=%d", 
			len(f.buffer), len(data), f.maxBufferSize)
	}
	
	// Append new data to buffer
	f.buffer = append(f.buffer, data...)
	
	for {
		message, consumed, err := f.extractMessage()
		if err != nil {
			return messages, err
		}
		
		if message == nil {
			// No complete message available, need more data
			break
		}
		
		// Check message size limits
		if len(message) > f.maxMessageSize {
			return nil, fmt.Errorf("message size limit exceeded: size=%d, max=%d", 
				len(message), f.maxMessageSize)
		}
		
		messages = append(messages, message)
		
		// Remove consumed data from buffer
		if consumed > 0 {
			f.buffer = f.buffer[consumed:]
		}
		
		// Reset framer state for next message
		f.reset()
	}
	
	return messages, nil
}

// extractMessage attempts to extract a complete SIP message from the buffer
// Returns: message bytes, bytes consumed, error
func (f *TCPMessageFramer) extractMessage() ([]byte, int, error) {
	if len(f.buffer) == 0 {
		return nil, 0, nil
	}
	
	if !f.headersDone {
		// Look for end of headers (double CRLF)
		headerEnd := f.findHeaderEnd()
		if headerEnd == -1 {
			// Headers not complete yet
			return nil, 0, nil
		}
		
		// Extract headers
		headers := f.buffer[:headerEnd+4] // Include the double CRLF
		
		// Parse Content-Length from headers
		contentLength, err := f.parseContentLength(headers)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to parse Content-Length: %w", err)
		}
		
		f.contentLength = contentLength
		f.headersDone = true
		f.expectedBodyLength = contentLength
		f.currentBodyLength = 0
		
		// If no body expected, return the message
		if contentLength == 0 {
			message := make([]byte, headerEnd+4)
			copy(message, f.buffer[:headerEnd+4])
			return message, headerEnd + 4, nil
		}
		
		// Check if we already have the complete body
		bodyStart := headerEnd + 4
		availableBodyData := len(f.buffer) - bodyStart
		
		if availableBodyData >= contentLength {
			// Complete message available
			messageLength := bodyStart + contentLength
			message := make([]byte, messageLength)
			copy(message, f.buffer[:messageLength])
			return message, messageLength, nil
		}
		
		// Partial body, need more data
		f.currentBodyLength = availableBodyData
		return nil, 0, nil
	}
	
	// Headers are done, waiting for body completion
	headerLength := f.getHeaderLength()
	bodyStart := headerLength
	availableBodyData := len(f.buffer) - bodyStart
	
	if availableBodyData >= f.expectedBodyLength {
		// Complete message available
		messageLength := bodyStart + f.expectedBodyLength
		message := make([]byte, messageLength)
		copy(message, f.buffer[:messageLength])
		return message, messageLength, nil
	}
	
	// Still waiting for more body data
	f.currentBodyLength = availableBodyData
	return nil, 0, nil
}

// findHeaderEnd finds the position of the double CRLF that marks the end of headers
func (f *TCPMessageFramer) findHeaderEnd() int {
	// Look for \r\n\r\n
	doubleCRLF := []byte("\r\n\r\n")
	return bytes.Index(f.buffer, doubleCRLF)
}

// parseContentLength extracts the Content-Length value from headers
func (f *TCPMessageFramer) parseContentLength(headers []byte) (int, error) {
	headerStr := string(headers)
	lines := strings.Split(headerStr, "\r\n")
	
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(strings.ToLower(line), "content-length:") {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) != 2 {
				continue
			}
			
			lengthStr := strings.TrimSpace(parts[1])
			length, err := strconv.Atoi(lengthStr)
			if err != nil {
				return 0, fmt.Errorf("invalid Content-Length value: %s", lengthStr)
			}
			
			if length < 0 {
				return 0, fmt.Errorf("negative Content-Length value: %d", length)
			}
			
			// Check against maximum message size
			if length > f.maxMessageSize {
				return 0, fmt.Errorf("Content-Length exceeds maximum message size: %d > %d", 
					length, f.maxMessageSize)
			}
			
			return length, nil
		}
	}
	
	// No Content-Length header found, assume no body
	return 0, nil
}

// getHeaderLength returns the length of headers including the double CRLF
func (f *TCPMessageFramer) getHeaderLength() int {
	headerEnd := f.findHeaderEnd()
	if headerEnd == -1 {
		return 0
	}
	return headerEnd + 4
}

// reset resets the framer state for processing the next message
func (f *TCPMessageFramer) reset() {
	f.headersDone = false
	f.contentLength = -1
	f.expectedBodyLength = 0
	f.currentBodyLength = 0
}

// GetBufferSize returns the current buffer size
func (f *TCPMessageFramer) GetBufferSize() int {
	return len(f.buffer)
}

// GetStats returns framing statistics
func (f *TCPMessageFramer) GetStats() map[string]interface{} {
	return map[string]interface{}{
		"buffer_size":           len(f.buffer),
		"buffer_capacity":       cap(f.buffer),
		"headers_done":          f.headersDone,
		"content_length":        f.contentLength,
		"expected_body_length":  f.expectedBodyLength,
		"current_body_length":   f.currentBodyLength,
		"max_buffer_size":       f.maxBufferSize,
		"max_message_size":      f.maxMessageSize,
	}
}

// Clear clears the internal buffer (useful for error recovery)
func (f *TCPMessageFramer) Clear() {
	f.buffer = f.buffer[:0]
	f.reset()
}

// SetLimits updates the buffer and message size limits
func (f *TCPMessageFramer) SetLimits(maxBufferSize, maxMessageSize int) {
	f.maxBufferSize = maxBufferSize
	f.maxMessageSize = maxMessageSize
}

// GetLimits returns the current buffer and message size limits
func (f *TCPMessageFramer) GetLimits() (maxBufferSize, maxMessageSize int) {
	return f.maxBufferSize, f.maxMessageSize
}

// IsBufferFull checks if the buffer is approaching its limit
func (f *TCPMessageFramer) IsBufferFull() bool {
	return len(f.buffer) > f.maxBufferSize*8/10 // 80% threshold
}

// CompactBuffer compacts the buffer to reduce memory usage
func (f *TCPMessageFramer) CompactBuffer() {
	if cap(f.buffer) > len(f.buffer)*2 && cap(f.buffer) > 4096 {
		// Create a new buffer with appropriate capacity
		newBuffer := make([]byte, len(f.buffer), len(f.buffer)+1024)
		copy(newBuffer, f.buffer)
		f.buffer = newBuffer
	}
}

// StreamingTCPMessageReader provides a streaming interface for reading SIP messages from TCP
type StreamingTCPMessageReader struct {
	reader       *bufio.Reader
	framer       *TCPMessageFramer
	messageQueue [][]byte // Queue for buffered messages
}

// NewStreamingTCPMessageReader creates a new streaming TCP message reader
func NewStreamingTCPMessageReader(reader *bufio.Reader) *StreamingTCPMessageReader {
	return &StreamingTCPMessageReader{
		reader:       reader,
		framer:       NewTCPMessageFramer(),
		messageQueue: make([][]byte, 0),
	}
}

// ReadMessage reads the next complete SIP message from the stream
func (r *StreamingTCPMessageReader) ReadMessage() ([]byte, error) {
	for {
		// Check if we have queued messages first
		if len(r.messageQueue) > 0 {
			message := r.messageQueue[0]
			r.messageQueue = r.messageQueue[1:]
			return message, nil
		}
		
		// Try to extract messages from existing buffer
		messages, err := r.framer.FrameMessage(nil)
		if err != nil {
			return nil, err
		}
		
		if len(messages) > 0 {
			// Return first message, queue the rest
			if len(messages) > 1 {
				r.messageQueue = append(r.messageQueue, messages[1:]...)
			}
			return messages[0], nil
		}
		
		// Need more data, read from the stream
		data := make([]byte, 4096)
		n, err := r.reader.Read(data)
		if err != nil {
			if err == io.EOF {
				// Check if we have any buffered data that might form a complete message
				if r.framer.GetBufferSize() > 0 {
					// Try one more time to extract messages from buffer
					messages, frameErr := r.framer.FrameMessage(nil)
					if frameErr != nil {
						return nil, frameErr
					}
					if len(messages) > 0 {
						// Return first message, queue the rest
						if len(messages) > 1 {
							r.messageQueue = append(r.messageQueue, messages[1:]...)
						}
						return messages[0], nil
					}
					// Connection closed but we have incomplete data
					return nil, fmt.Errorf("connection closed with incomplete message")
				}
				// No buffered data, just EOF
				return nil, err
			}
			return nil, err
		}
		
		// Process the new data
		messages, err = r.framer.FrameMessage(data[:n])
		if err != nil {
			return nil, err
		}
		
		if len(messages) > 0 {
			// Return first message, queue the rest
			if len(messages) > 1 {
				r.messageQueue = append(r.messageQueue, messages[1:]...)
			}
			return messages[0], nil
		}
		
		// Continue reading more data
	}
}

// GetStats returns reader statistics
func (r *StreamingTCPMessageReader) GetStats() map[string]interface{} {
	return r.framer.GetStats()
}

// Clear clears the internal buffer and message queue
func (r *StreamingTCPMessageReader) Clear() {
	r.framer.Clear()
	r.messageQueue = r.messageQueue[:0]
}

// TCPMessageWriter provides a simple interface for writing SIP messages to TCP
type TCPMessageWriter struct {
	writer *bufio.Writer
}

// NewTCPMessageWriter creates a new TCP message writer
func NewTCPMessageWriter(writer *bufio.Writer) *TCPMessageWriter {
	return &TCPMessageWriter{
		writer: writer,
	}
}

// WriteMessage writes a SIP message to the TCP stream
func (w *TCPMessageWriter) WriteMessage(message []byte) error {
	_, err := w.writer.Write(message)
	if err != nil {
		return fmt.Errorf("failed to write message: %w", err)
	}
	
	// Flush to ensure message is sent immediately
	err = w.writer.Flush()
	if err != nil {
		return fmt.Errorf("failed to flush message: %w", err)
	}
	
	return nil
}

// Flush flushes any buffered data
func (w *TCPMessageWriter) Flush() error {
	return w.writer.Flush()
}