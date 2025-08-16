package transport

import (
	"bufio"
	"fmt"
	"net"
	"sync"
	"time"
)

// EnhancedTCPTransport provides improved TCP transport with connection management,
// message framing, and timeout handling
type EnhancedTCPTransport struct {
	listener          net.Listener
	handler           MessageHandler
	connectionManager *TCPConnectionManager
	running           bool
	mu                sync.RWMutex
	wg                sync.WaitGroup
	stopChan          chan struct{}
	config            *EnhancedTCPConfig
	logger            Logger
}

// EnhancedTCPConfig holds configuration for the enhanced TCP transport
type EnhancedTCPConfig struct {
	// Connection timeouts
	ReadTimeout     time.Duration
	WriteTimeout    time.Duration
	IdleTimeout     time.Duration
	AcceptTimeout   time.Duration
	
	// Connection management
	MaxConnections    int
	CleanupInterval   time.Duration
	
	// Error handling
	MaxRetries        int
	RetryDelay        time.Duration
	
	// Logging
	Logger Logger
}

// DefaultEnhancedTCPConfig returns default configuration for enhanced TCP transport
func DefaultEnhancedTCPConfig() *EnhancedTCPConfig {
	return &EnhancedTCPConfig{
		ReadTimeout:     30 * time.Second,
		WriteTimeout:    30 * time.Second,
		IdleTimeout:     5 * time.Minute,
		AcceptTimeout:   1 * time.Second,
		MaxConnections:  1000,
		CleanupInterval: 1 * time.Minute,
		MaxRetries:      3,
		RetryDelay:      1 * time.Second,
		Logger:          &noOpLogger{},
	}
}

// NewEnhancedTCPTransport creates a new enhanced TCP transport
func NewEnhancedTCPTransport(config *EnhancedTCPConfig) *EnhancedTCPTransport {
	if config == nil {
		config = DefaultEnhancedTCPConfig()
	}
	
	connManagerConfig := &TCPConnectionManagerConfig{
		IdleTimeout:     config.IdleTimeout,
		ReadTimeout:     config.ReadTimeout,
		WriteTimeout:    config.WriteTimeout,
		CleanupInterval: config.CleanupInterval,
		Logger:          config.Logger,
	}
	
	return &EnhancedTCPTransport{
		connectionManager: NewTCPConnectionManager(connManagerConfig),
		stopChan:          make(chan struct{}),
		config:            config,
		logger:            config.Logger,
	}
}

// Start starts the enhanced TCP transport on the specified port
func (t *EnhancedTCPTransport) Start(port int) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	
	if t.running {
		return fmt.Errorf("enhanced TCP transport already running")
	}
	
	listener, err := net.Listen("tcp4", fmt.Sprintf(":%d", port))
	if err != nil {
		t.logger.Error("Failed to start TCP listener", "port", port, "error", err)
		return fmt.Errorf("failed to listen on TCP port %d: %w", port, err)
	}
	
	t.listener = listener
	t.running = true
	
	t.logger.Info("Started enhanced TCP transport", "port", port, "local_addr", listener.Addr())
	
	// Start the connection accepting goroutine
	t.wg.Add(1)
	go t.acceptConnections()
	
	return nil
}

// Stop stops the enhanced TCP transport
func (t *EnhancedTCPTransport) Stop() error {
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
	
	t.mu.Unlock()
	
	// Wait for all goroutines to finish
	t.wg.Wait()
	
	// Stop the connection manager
	err := t.connectionManager.Stop()
	if err != nil {
		t.logger.Error("Error stopping connection manager", "error", err)
	}
	
	t.logger.Info("Stopped enhanced TCP transport")
	return err
}

// SendMessage sends a SIP message over TCP with timeout handling
func (t *EnhancedTCPTransport) SendMessage(data []byte, addr net.Addr) error {
	t.mu.RLock()
	defer t.mu.RUnlock()
	
	if !t.running {
		return fmt.Errorf("enhanced TCP transport not running")
	}
	
	tcpAddr, ok := addr.(*net.TCPAddr)
	if !ok {
		return fmt.Errorf("invalid address type for TCP transport: %T", addr)
	}
	
	// Try to send with retries
	var lastErr error
	for attempt := 0; attempt < t.config.MaxRetries; attempt++ {
		if attempt > 0 {
			t.logger.Debug("Retrying TCP send", "attempt", attempt+1, "addr", tcpAddr)
			time.Sleep(t.config.RetryDelay)
		}
		
		err := t.sendMessageWithTimeout(data, tcpAddr)
		if err == nil {
			t.logger.Debug("Successfully sent TCP message", "addr", tcpAddr, "size", len(data))
			return nil
		}
		
		lastErr = err
		t.logger.Warn("Failed to send TCP message", "attempt", attempt+1, "addr", tcpAddr, "error", err)
		
		// Check if it's a timeout error that might be recoverable
		if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
			continue // Retry on timeout
		}
		
		// For non-timeout errors, don't retry
		break
	}
	
	t.logger.Error("Failed to send TCP message after retries", "addr", tcpAddr, "error", lastErr)
	return fmt.Errorf("failed to send TCP message to %s after %d attempts: %w", tcpAddr, t.config.MaxRetries, lastErr)
}

// sendMessageWithTimeout sends a message with timeout handling
func (t *EnhancedTCPTransport) sendMessageWithTimeout(data []byte, addr *net.TCPAddr) error {
	// Create connection with timeout
	conn, err := net.DialTimeout("tcp4", addr.String(), t.config.WriteTimeout)
	if err != nil {
		return fmt.Errorf("failed to connect to %s: %w", addr, err)
	}
	defer conn.Close()
	
	// Set write timeout
	if t.config.WriteTimeout > 0 {
		err = conn.SetWriteDeadline(time.Now().Add(t.config.WriteTimeout))
		if err != nil {
			return fmt.Errorf("failed to set write timeout: %w", err)
		}
	}
	
	// Create message writer
	writer := bufio.NewWriter(conn)
	messageWriter := NewTCPMessageWriter(writer)
	
	// Send the message
	err = messageWriter.WriteMessage(data)
	if err != nil {
		return fmt.Errorf("failed to write message: %w", err)
	}
	
	return nil
}

// RegisterHandler registers a message handler for incoming messages
func (t *EnhancedTCPTransport) RegisterHandler(handler MessageHandler) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.handler = handler
}

// acceptConnections handles incoming TCP connections
func (t *EnhancedTCPTransport) acceptConnections() {
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
			tcpListener.SetDeadline(time.Now().Add(t.config.AcceptTimeout))
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
			
			t.logger.Warn("Failed to accept TCP connection", "error", err)
			continue
		}
		
		// Check connection limit
		if t.connectionManager.GetConnectionCount() >= t.config.MaxConnections {
			t.logger.Warn("Rejecting connection due to limit", 
				"current_connections", t.connectionManager.GetConnectionCount(),
				"max_connections", t.config.MaxConnections,
				"remote_addr", conn.RemoteAddr())
			conn.Close()
			continue
		}
		
		t.logger.Debug("Accepted TCP connection", "remote_addr", conn.RemoteAddr())
		
		// Add connection to manager
		tcpConn := t.connectionManager.AddConnection(conn)
		
		// Handle the connection in a separate goroutine
		t.wg.Add(1)
		go t.handleConnection(tcpConn)
	}
}

// handleConnection handles a single TCP connection with timeout and error recovery
func (t *EnhancedTCPTransport) handleConnection(tcpConn *TCPConnection) {
	defer t.wg.Done()
	defer func() {
		t.connectionManager.RemoveConnection(tcpConn.GetID())
		t.logger.Debug("Closed TCP connection", "id", tcpConn.GetID())
	}()
	
	// Create streaming message reader
	streamReader := NewStreamingTCPMessageReader(tcpConn.reader)
	
	for {
		select {
		case <-t.stopChan:
			return
		default:
		}
		
		// Set read timeout
		err := tcpConn.SetReadTimeout(t.config.ReadTimeout)
		if err != nil {
			t.logger.Error("Failed to set read timeout", "connection_id", tcpConn.GetID(), "error", err)
			return
		}
		
		// Read SIP message with timeout handling
		message, err := t.readMessageWithTimeout(streamReader, tcpConn)
		if err != nil {
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				t.logger.Debug("Read timeout on TCP connection", 
					"connection_id", tcpConn.GetID(),
					"timeout", t.config.ReadTimeout)
				continue // Continue to check stop signal and try again
			}
			
			// Check for EOF (client disconnected)
			if err.Error() == "EOF" {
				t.logger.Debug("Client disconnected", "connection_id", tcpConn.GetID())
				return
			}
			
			t.logger.Error("Failed to read message from TCP connection", 
				"connection_id", tcpConn.GetID(), 
				"error", err)
			return
		}
		
		if len(message) > 0 {
			// Update connection activity
			tcpConn.UpdateActivity()
			
			// Get handler
			t.mu.RLock()
			handler := t.handler
			t.mu.RUnlock()
			
			if handler != nil {
				// Handle message in a separate goroutine to avoid blocking
				go func(msg []byte, connID string, remoteAddr net.Addr) {
					err := handler.HandleMessage(msg, "TCP", remoteAddr)
					if err != nil {
						t.logger.Error("Error handling TCP message", 
							"connection_id", connID,
							"remote_addr", remoteAddr,
							"error", err)
					}
				}(message, tcpConn.GetID(), tcpConn.GetRemoteAddr())
			}
		}
	}
}

// readMessageWithTimeout reads a message with timeout and error recovery
func (t *EnhancedTCPTransport) readMessageWithTimeout(reader *StreamingTCPMessageReader, tcpConn *TCPConnection) ([]byte, error) {
	// Create a channel to receive the result
	resultChan := make(chan struct {
		message []byte
		err     error
	}, 1)
	
	// Start reading in a goroutine
	go func() {
		message, err := reader.ReadMessage()
		resultChan <- struct {
			message []byte
			err     error
		}{message, err}
	}()
	
	// Wait for result or timeout
	select {
	case result := <-resultChan:
		return result.message, result.err
	case <-time.After(t.config.ReadTimeout):
		// Clear the reader buffer on timeout to prevent corruption
		reader.Clear()
		return nil, fmt.Errorf("read timeout after %v", t.config.ReadTimeout)
	case <-t.stopChan:
		return nil, fmt.Errorf("transport stopping")
	}
}

// IsRunning returns true if the enhanced TCP transport is running
func (t *EnhancedTCPTransport) IsRunning() bool {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.running
}

// LocalAddr returns the local address of the TCP listener
func (t *EnhancedTCPTransport) LocalAddr() net.Addr {
	t.mu.RLock()
	defer t.mu.RUnlock()
	if t.listener != nil {
		return t.listener.Addr()
	}
	return nil
}

// GetStats returns transport statistics
func (t *EnhancedTCPTransport) GetStats() map[string]interface{} {
	t.mu.RLock()
	defer t.mu.RUnlock()
	
	stats := map[string]interface{}{
		"running":         t.running,
		"read_timeout":    t.config.ReadTimeout,
		"write_timeout":   t.config.WriteTimeout,
		"idle_timeout":    t.config.IdleTimeout,
		"max_connections": t.config.MaxConnections,
		"max_retries":     t.config.MaxRetries,
	}
	
	if t.listener != nil {
		stats["local_addr"] = t.listener.Addr().String()
	}
	
	// Add connection manager stats
	connStats := t.connectionManager.GetStats()
	for k, v := range connStats {
		stats["conn_"+k] = v
	}
	
	return stats
}

// GetConnectionManager returns the connection manager (for testing)
func (t *EnhancedTCPTransport) GetConnectionManager() *TCPConnectionManager {
	return t.connectionManager
}

// SetTimeouts updates the timeout configuration
func (t *EnhancedTCPTransport) SetTimeouts(readTimeout, writeTimeout, idleTimeout time.Duration) {
	t.mu.Lock()
	defer t.mu.Unlock()
	
	t.config.ReadTimeout = readTimeout
	t.config.WriteTimeout = writeTimeout
	t.config.IdleTimeout = idleTimeout
	
	// Update connection manager timeouts
	t.connectionManager.SetTimeouts(readTimeout, writeTimeout)
	
	t.logger.Info("Updated TCP timeouts", 
		"read_timeout", readTimeout,
		"write_timeout", writeTimeout,
		"idle_timeout", idleTimeout)
}