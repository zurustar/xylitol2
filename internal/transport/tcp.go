package transport

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"
)

// TCPTransport handles TCP transport for SIP messages
type TCPTransport struct {
	listener    net.Listener
	handler     MessageHandler
	running     bool
	mu          sync.RWMutex
	wg          sync.WaitGroup
	stopChan    chan struct{}
	connections map[net.Conn]struct{}
	connMu      sync.RWMutex
}

// NewTCPTransport creates a new TCP transport handler
func NewTCPTransport() *TCPTransport {
	return &TCPTransport{
		stopChan:    make(chan struct{}),
		connections: make(map[net.Conn]struct{}),
	}
}

// Start starts the TCP listener on the specified port
func (t *TCPTransport) Start(port int) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.running {
		return fmt.Errorf("TCP transport already running")
	}

	listener, err := net.Listen("tcp4", fmt.Sprintf(":%d", port))
	if err != nil {
		return fmt.Errorf("failed to listen on TCP port %d: %w", port, err)
	}

	t.listener = listener
	t.running = true

	// Start the connection accepting goroutine
	t.wg.Add(1)
	go t.acceptConnections()

	return nil
}

// Stop stops the TCP transport
func (t *TCPTransport) Stop() error {
	t.mu.Lock()
	if !t.running {
		t.mu.Unlock()
		return nil
	}

	t.running = false
	close(t.stopChan)

	if t.listener != nil {
		t.listener.Close()
	}

	// Close all active connections
	t.connMu.Lock()
	for conn := range t.connections {
		conn.Close()
	}
	t.connMu.Unlock()

	t.mu.Unlock()

	t.wg.Wait()
	return nil
}

// SendMessage sends a SIP message over TCP
func (t *TCPTransport) SendMessage(data []byte, addr net.Addr) error {
	t.mu.RLock()
	defer t.mu.RUnlock()

	if !t.running {
		return fmt.Errorf("TCP transport not running")
	}

	tcpAddr, ok := addr.(*net.TCPAddr)
	if !ok {
		return fmt.Errorf("invalid address type for TCP transport: %T", addr)
	}

	// Create a new connection for sending
	conn, err := net.DialTCP("tcp4", nil, tcpAddr)
	if err != nil {
		return fmt.Errorf("failed to connect to %s: %w", tcpAddr, err)
	}
	defer conn.Close()

	_, err = conn.Write(data)
	if err != nil {
		return fmt.Errorf("failed to send TCP message: %w", err)
	}

	return nil
}

// RegisterHandler registers a message handler for incoming messages
func (t *TCPTransport) RegisterHandler(handler MessageHandler) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.handler = handler
}

// acceptConnections handles incoming TCP connections
func (t *TCPTransport) acceptConnections() {
	defer t.wg.Done()

	for {
		select {
		case <-t.stopChan:
			return
		default:
		}

		t.mu.RLock()
		listener := t.listener
		t.mu.RUnlock()

		if listener == nil {
			break
		}

		// Set accept timeout to allow periodic checking of stop signal
		if tcpListener, ok := listener.(*net.TCPListener); ok {
			tcpListener.SetDeadline(time.Now().Add(1 * time.Second))
		}

		conn, err := listener.Accept()
		if err != nil {
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				// Timeout is expected, continue loop to check stop signal
				continue
			}
			// Check if we're stopping
			select {
			case <-t.stopChan:
				return
			default:
			}
			// Log error but continue accepting
			continue
		}

		// Track the connection
		t.connMu.Lock()
		t.connections[conn] = struct{}{}
		t.connMu.Unlock()

		// Handle the connection in a separate goroutine
		t.wg.Add(1)
		go t.handleConnection(conn)
	}
}

// handleConnection handles a single TCP connection
func (t *TCPTransport) handleConnection(conn net.Conn) {
	defer t.wg.Done()
	defer func() {
		conn.Close()
		t.connMu.Lock()
		delete(t.connections, conn)
		t.connMu.Unlock()
	}()

	reader := bufio.NewReader(conn)

	for {
		select {
		case <-t.stopChan:
			return
		default:
		}

		// Set read timeout
		conn.SetReadDeadline(time.Now().Add(30 * time.Second))

		// Read SIP message from TCP stream
		message, err := t.readSIPMessage(reader)
		if err != nil {
			if err == io.EOF {
				// Connection closed by client
				return
			}
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				// Timeout, continue to check stop signal
				continue
			}
			// Other error, close connection
			return
		}

		if len(message) > 0 {
			// Get handler
			t.mu.RLock()
			handler := t.handler
			t.mu.RUnlock()

			if handler != nil {
				go func() {
					if err := handler.HandleMessage(message, "TCP", conn.RemoteAddr()); err != nil {
						// Log error handling message
						// In a real implementation, this would use proper logging
					}
				}()
			}
		}
	}
}

// readSIPMessage reads a complete SIP message from TCP stream
func (t *TCPTransport) readSIPMessage(reader *bufio.Reader) ([]byte, error) {
	var message []byte
	var contentLength int = -1
	headersDone := false

	for {
		line, err := reader.ReadBytes('\n')
		if err != nil {
			return nil, err
		}

		message = append(message, line...)

		// Check for end of headers (empty line)
		if !headersDone && (len(line) == 2 && line[0] == '\r' && line[1] == '\n') {
			headersDone = true

			// Parse Content-Length from headers if not already found
			if contentLength == -1 {
				contentLength = t.parseContentLength(message)
			}

			// If no body expected, return the message
			if contentLength == 0 {
				return message, nil
			}

			// If Content-Length not found, assume no body
			if contentLength == -1 {
				return message, nil
			}

			// Read the body
			if contentLength > 0 {
				body := make([]byte, contentLength)
				_, err := io.ReadFull(reader, body)
				if err != nil {
					return nil, err
				}
				message = append(message, body...)
			}

			return message, nil
		}

		// Parse Content-Length header while reading headers
		if !headersDone && contentLength == -1 {
			lineStr := string(line)
			if strings.HasPrefix(strings.ToLower(lineStr), "content-length:") {
				parts := strings.SplitN(lineStr, ":", 2)
				if len(parts) == 2 {
					lengthStr := strings.TrimSpace(parts[1])
					lengthStr = strings.TrimSuffix(lengthStr, "\r\n")
					lengthStr = strings.TrimSuffix(lengthStr, "\n")
					if length, err := strconv.Atoi(lengthStr); err == nil {
						contentLength = length
					}
				}
			}
		}
	}
}

// parseContentLength parses the Content-Length header from the message
func (t *TCPTransport) parseContentLength(message []byte) int {
	messageStr := string(message)
	lines := strings.Split(messageStr, "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(strings.ToLower(line), "content-length:") {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				lengthStr := strings.TrimSpace(parts[1])
				if length, err := strconv.Atoi(lengthStr); err == nil {
					return length
				}
			}
		}
	}

	return 0 // Default to no body if Content-Length not found
}

// IsRunning returns true if the TCP transport is running
func (t *TCPTransport) IsRunning() bool {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.running
}

// LocalAddr returns the local address of the TCP listener
func (t *TCPTransport) LocalAddr() net.Addr {
	t.mu.RLock()
	defer t.mu.RUnlock()
	if t.listener != nil {
		return t.listener.Addr()
	}
	return nil
}