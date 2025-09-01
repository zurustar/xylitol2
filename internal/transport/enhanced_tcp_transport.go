package transport

import (
	"bufio"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"
)

// TCPErrorStatistics tracks error statistics for TCP transport
type TCPErrorStatistics struct {
	mu                    sync.RWMutex
	TimeoutErrors         int64
	ConnectionErrors      int64
	ReadErrors            int64
	WriteErrors           int64
	RecoveryAttempts      int64
	SuccessfulRecoveries  int64
	FailedRecoveries      int64
	LastErrorTime         time.Time
	LastRecoveryTime      time.Time
}

// EnhancedTCPTransport provides improved TCP transport with connection management,
// message framing, timeout handling, and error recovery
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
	errorStats        *TCPErrorStatistics
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
	
	// Error handling and recovery
	MaxRetries        int
	RetryDelay        time.Duration
	MaxRetryDelay     time.Duration
	BackoffMultiplier float64
	
	// Timeout recovery
	TimeoutRecoveryEnabled bool
	TimeoutRecoveryDelay   time.Duration
	MaxTimeoutRetries      int
	
	// Connection recovery
	ConnectionRecoveryEnabled bool
	ConnectionRecoveryDelay   time.Duration
	
	// Error logging
	DetailedErrorLogging bool
	ErrorStatistics      bool
	
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
		MaxRetryDelay:   30 * time.Second,
		BackoffMultiplier: 2.0,
		
		// Timeout recovery settings
		TimeoutRecoveryEnabled: true,
		TimeoutRecoveryDelay:   500 * time.Millisecond,
		MaxTimeoutRetries:      5,
		
		// Connection recovery settings
		ConnectionRecoveryEnabled: true,
		ConnectionRecoveryDelay:   2 * time.Second,
		
		// Error logging settings
		DetailedErrorLogging: true,
		ErrorStatistics:      true,
		
		Logger: &noOpLogger{},
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
		errorStats:        &TCPErrorStatistics{},
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

// SendMessage sends a SIP message over TCP with enhanced timeout handling and error recovery
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
	
	// Try to send with enhanced retries and backoff
	var lastErr error
	retryDelay := t.config.RetryDelay
	
	for attempt := 0; attempt < t.config.MaxRetries; attempt++ {
		if attempt > 0 {
			if t.config.DetailedErrorLogging {
				t.logger.Debug("Retrying TCP send with backoff", 
					"attempt", attempt+1, 
					"addr", tcpAddr, 
					"delay", retryDelay,
					"last_error", lastErr)
			}
			
			// Apply exponential backoff with jitter
			time.Sleep(retryDelay)
			retryDelay = time.Duration(float64(retryDelay) * t.config.BackoffMultiplier)
			if retryDelay > t.config.MaxRetryDelay {
				retryDelay = t.config.MaxRetryDelay
			}
		}
		
		err := t.sendMessageWithTimeoutAndRecovery(data, tcpAddr, attempt)
		if err == nil {
			if attempt > 0 {
				t.recordRecoveryAttempt(true)
				if t.config.DetailedErrorLogging {
					t.logger.Info("Successfully recovered TCP send after retries", 
						"addr", tcpAddr, 
						"size", len(data),
						"attempts", attempt+1)
				}
			}
			return nil
		}
		
		lastErr = err
		
		// Classify and record the error
		if netErr, ok := err.(net.Error); ok {
			if netErr.Timeout() {
				t.recordTimeoutError()
				if t.config.TimeoutRecoveryEnabled && attempt < t.config.MaxTimeoutRetries {
					if t.config.DetailedErrorLogging {
						t.logger.Warn("TCP send timeout, will retry with recovery", 
							"attempt", attempt+1, 
							"addr", tcpAddr, 
							"timeout", t.config.WriteTimeout,
							"error", err)
					}
					continue // Retry on timeout
				}
			} else {
				t.recordConnectionError()
			}
		} else {
			t.recordWriteError()
		}
		
		if t.config.DetailedErrorLogging {
			t.logger.Warn("Failed to send TCP message", 
				"attempt", attempt+1, 
				"addr", tcpAddr, 
				"error_type", fmt.Sprintf("%T", err),
				"error", err)
		}
		
		// Check if error is recoverable
		if !t.isRecoverableError(err) {
			if t.config.DetailedErrorLogging {
				t.logger.Error("Non-recoverable TCP send error, stopping retries", 
					"addr", tcpAddr, 
					"error", err)
			}
			break
		}
	}
	
	t.recordRecoveryAttempt(false)
	
	if t.config.DetailedErrorLogging {
		t.logger.Error("Failed to send TCP message after all retries", 
			"addr", tcpAddr, 
			"attempts", t.config.MaxRetries,
			"final_error", lastErr,
			"error_stats", t.GetErrorStatistics())
	}
	
	return fmt.Errorf("failed to send TCP message to %s after %d attempts: %w", tcpAddr, t.config.MaxRetries, lastErr)
}

// sendMessageWithTimeoutAndRecovery sends a message with enhanced timeout handling and recovery
func (t *EnhancedTCPTransport) sendMessageWithTimeoutAndRecovery(data []byte, addr *net.TCPAddr, attempt int) error {
	// Adjust timeout based on attempt number for progressive timeout increase
	writeTimeout := t.config.WriteTimeout
	if attempt > 0 && t.config.TimeoutRecoveryEnabled {
		// Increase timeout for retry attempts
		timeoutMultiplier := 1.0 + (float64(attempt) * 0.5)
		writeTimeout = time.Duration(float64(writeTimeout) * timeoutMultiplier)
		if writeTimeout > t.config.WriteTimeout*3 {
			writeTimeout = t.config.WriteTimeout * 3 // Cap at 3x original timeout
		}
	}
	
	// Create connection with timeout
	conn, err := net.DialTimeout("tcp4", addr.String(), writeTimeout)
	if err != nil {
		if t.config.DetailedErrorLogging {
			t.logger.Debug("Failed to establish TCP connection", 
				"addr", addr, 
				"timeout", writeTimeout,
				"attempt", attempt+1,
				"error", err)
		}
		return fmt.Errorf("failed to connect to %s: %w", addr, err)
	}
	defer func() {
		if closeErr := conn.Close(); closeErr != nil && t.config.DetailedErrorLogging {
			t.logger.Debug("Error closing TCP connection", "addr", addr, "error", closeErr)
		}
	}()
	
	// Set write timeout with recovery adjustment
	if writeTimeout > 0 {
		deadline := time.Now().Add(writeTimeout)
		err = conn.SetWriteDeadline(deadline)
		if err != nil {
			if t.config.DetailedErrorLogging {
				t.logger.Debug("Failed to set write deadline", 
					"addr", addr, 
					"timeout", writeTimeout,
					"deadline", deadline,
					"error", err)
			}
			return fmt.Errorf("failed to set write timeout: %w", err)
		}
	}
	
	// Set read timeout for potential response reading
	if t.config.ReadTimeout > 0 {
		err = conn.SetReadDeadline(time.Now().Add(t.config.ReadTimeout))
		if err != nil && t.config.DetailedErrorLogging {
			t.logger.Debug("Failed to set read deadline", "addr", addr, "error", err)
		}
	}
	
	// Create message writer with recovery features
	writer := bufio.NewWriter(conn)
	messageWriter := NewTCPMessageWriter(writer)
	
	// Send the message with detailed error tracking
	startTime := time.Now()
	err = messageWriter.WriteMessage(data)
	duration := time.Since(startTime)
	
	if err != nil {
		if t.config.DetailedErrorLogging {
			t.logger.Debug("Failed to write TCP message", 
				"addr", addr,
				"size", len(data),
				"duration", duration,
				"timeout", writeTimeout,
				"attempt", attempt+1,
				"error", err)
		}
		
		// Check if this is a timeout that might be recoverable
		if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
			return &RecoverableTimeoutError{
				Addr:            addr,
				TimeoutDuration: writeTimeout,
				Duration:        duration,
				Attempt:         attempt,
				Err:             err,
			}
		}
		
		return fmt.Errorf("failed to write message: %w", err)
	}
	
	if t.config.DetailedErrorLogging && attempt > 0 {
		t.logger.Debug("Successfully sent TCP message after retry", 
			"addr", addr,
			"size", len(data),
			"duration", duration,
			"timeout", writeTimeout,
			"attempt", attempt+1)
	}
	
	return nil
}

// RecoverableTimeoutError represents a timeout error that might be recoverable
type RecoverableTimeoutError struct {
	Addr            *net.TCPAddr
	TimeoutDuration time.Duration
	Duration        time.Duration
	Attempt         int
	Err             error
}

func (e *RecoverableTimeoutError) Error() string {
	return fmt.Sprintf("recoverable timeout error to %s after %v (timeout: %v, attempt: %d): %v", 
		e.Addr, e.Duration, e.TimeoutDuration, e.Attempt+1, e.Err)
}

func (e *RecoverableTimeoutError) Timeout() bool {
	return true
}

func (e *RecoverableTimeoutError) Temporary() bool {
	return true
}

// isRecoverableError determines if an error is recoverable and worth retrying
func (t *EnhancedTCPTransport) isRecoverableError(err error) bool {
	if err == nil {
		return false
	}
	
	// Check for recoverable timeout errors
	if _, ok := err.(*RecoverableTimeoutError); ok {
		return t.config.TimeoutRecoveryEnabled
	}
	
	// Check for network errors
	if netErr, ok := err.(net.Error); ok {
		// Timeout errors are recoverable if timeout recovery is enabled
		if netErr.Timeout() {
			return t.config.TimeoutRecoveryEnabled
		}
		
		// Temporary errors are recoverable if connection recovery is enabled
		if netErr.Temporary() {
			return t.config.ConnectionRecoveryEnabled
		}
	}
	
	// Check for specific error types that might be recoverable
	errStr := err.Error()
	recoverableErrors := []string{
		"connection refused",
		"connection reset",
		"broken pipe",
		"network is unreachable",
		"no route to host",
	}
	
	for _, recoverableErr := range recoverableErrors {
		if strings.Contains(strings.ToLower(errStr), recoverableErr) {
			return t.config.ConnectionRecoveryEnabled
		}
	}
	
	// Default to non-recoverable
	return false
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

// handleConnection handles a single TCP connection with enhanced timeout and error recovery
func (t *EnhancedTCPTransport) handleConnection(tcpConn *TCPConnection) {
	defer t.wg.Done()
	defer func() {
		t.connectionManager.RemoveConnection(tcpConn.GetID())
		if t.config.DetailedErrorLogging {
			t.logger.Debug("Closed TCP connection", "id", tcpConn.GetID())
		}
	}()
	
	connID := tcpConn.GetID()
	remoteAddr := tcpConn.GetRemoteAddr()
	
	if t.config.DetailedErrorLogging {
		t.logger.Debug("Starting TCP connection handler", 
			"connection_id", connID,
			"remote_addr", remoteAddr)
	}
	
	// Create streaming message reader
	streamReader := NewStreamingTCPMessageReader(tcpConn.reader)
	
	// Error recovery state
	consecutiveTimeouts := 0
	consecutiveErrors := 0
	
	for {
		select {
		case <-t.stopChan:
			if t.config.DetailedErrorLogging {
				t.logger.Debug("TCP connection handler stopping", "connection_id", connID)
			}
			return
		default:
		}
		
		// Set read timeout with recovery adjustment
		readTimeout := t.config.ReadTimeout
		if consecutiveTimeouts > 0 && t.config.TimeoutRecoveryEnabled {
			// Increase timeout after consecutive timeouts
			timeoutMultiplier := 1.0 + (float64(consecutiveTimeouts) * 0.2)
			readTimeout = time.Duration(float64(readTimeout) * timeoutMultiplier)
			if readTimeout > t.config.ReadTimeout*2 {
				readTimeout = t.config.ReadTimeout * 2 // Cap at 2x original timeout
			}
		}
		
		err := tcpConn.SetReadTimeout(readTimeout)
		if err != nil {
			t.recordReadError()
			if t.config.DetailedErrorLogging {
				t.logger.Error("Failed to set read timeout", 
					"connection_id", connID, 
					"timeout", readTimeout,
					"error", err)
			}
			return
		}
		
		// Read SIP message with enhanced timeout handling and recovery
		message, err := t.readMessageWithTimeout(streamReader, tcpConn)
		if err != nil {
			// Handle recoverable timeout errors
			if recoverableErr, ok := err.(*RecoverableReadTimeoutError); ok {
				consecutiveTimeouts++
				
				if consecutiveTimeouts <= t.config.MaxTimeoutRetries {
					if t.config.DetailedErrorLogging {
						t.logger.Debug("Recoverable read timeout, continuing", 
							"connection_id", connID,
							"consecutive_timeouts", consecutiveTimeouts,
							"max_retries", t.config.MaxTimeoutRetries,
							"error", recoverableErr)
					}
					continue // Continue with adjusted timeout
				} else {
					if t.config.DetailedErrorLogging {
						t.logger.Warn("Max timeout retries exceeded, closing connection", 
							"connection_id", connID,
							"consecutive_timeouts", consecutiveTimeouts,
							"max_retries", t.config.MaxTimeoutRetries)
					}
					return
				}
			}
			
			// Handle regular timeout errors
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				consecutiveTimeouts++
				
				if t.config.DetailedErrorLogging {
					t.logger.Debug("Read timeout on TCP connection", 
						"connection_id", connID,
						"timeout", readTimeout,
						"consecutive_timeouts", consecutiveTimeouts)
				}
				
				// Check if we should continue or give up
				if consecutiveTimeouts <= t.config.MaxTimeoutRetries {
					continue // Continue to check stop signal and try again
				} else {
					if t.config.DetailedErrorLogging {
						t.logger.Warn("Too many consecutive timeouts, closing connection", 
							"connection_id", connID,
							"consecutive_timeouts", consecutiveTimeouts)
					}
					return
				}
			}
			
			// Handle EOF (client disconnected)
			if err.Error() == "EOF" {
				if t.config.DetailedErrorLogging {
					t.logger.Debug("Client disconnected", "connection_id", connID)
				}
				return
			}
			
			// Handle other errors with recovery logic
			consecutiveErrors++
			
			if t.config.DetailedErrorLogging {
				t.logger.Error("Failed to read message from TCP connection", 
					"connection_id", connID,
					"remote_addr", remoteAddr,
					"consecutive_errors", consecutiveErrors,
					"error_type", fmt.Sprintf("%T", err),
					"error", err)
			}
			
			// Check if we should attempt recovery
			if t.config.ConnectionRecoveryEnabled && consecutiveErrors <= 3 {
				if t.config.DetailedErrorLogging {
					t.logger.Debug("Attempting connection recovery", 
						"connection_id", connID,
						"consecutive_errors", consecutiveErrors)
				}
				
				// Apply recovery delay
				time.Sleep(t.config.ConnectionRecoveryDelay)
				
				// Clear the reader buffer to prevent corruption
				streamReader.Clear()
				
				continue
			}
			
			// Too many errors, close connection
			if t.config.DetailedErrorLogging {
				t.logger.Error("Too many consecutive errors, closing connection", 
					"connection_id", connID,
					"consecutive_errors", consecutiveErrors)
			}
			return
		}
		
		// Reset error counters on successful read
		if consecutiveTimeouts > 0 || consecutiveErrors > 0 {
			if t.config.DetailedErrorLogging {
				t.logger.Debug("TCP connection recovered", 
					"connection_id", connID,
					"previous_timeouts", consecutiveTimeouts,
					"previous_errors", consecutiveErrors)
			}
			consecutiveTimeouts = 0
			consecutiveErrors = 0
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
					defer func() {
						if r := recover(); r != nil {
							if t.config.DetailedErrorLogging {
								t.logger.Error("Panic in message handler", 
									"connection_id", connID,
									"remote_addr", remoteAddr,
									"panic", r)
							}
						}
					}()
					
					err := handler.HandleMessage(msg, "TCP", remoteAddr)
					if err != nil {
						if t.config.DetailedErrorLogging {
							t.logger.Error("Error handling TCP message", 
								"connection_id", connID,
								"remote_addr", remoteAddr,
								"message_size", len(msg),
								"error", err)
						}
					}
				}(message, connID, remoteAddr)
			}
		}
	}
}

// readMessageWithTimeout reads a message with enhanced timeout handling and error recovery
func (t *EnhancedTCPTransport) readMessageWithTimeout(reader *StreamingTCPMessageReader, tcpConn *TCPConnection) ([]byte, error) {
	connID := tcpConn.GetID()
	
	// Create a channel to receive the result
	resultChan := make(chan struct {
		message []byte
		err     error
	}, 1)
	
	// Start reading in a goroutine with recovery
	go func() {
		defer func() {
			if r := recover(); r != nil {
				if t.config.DetailedErrorLogging {
					t.logger.Error("Panic during TCP message read", 
						"connection_id", connID,
						"panic", r)
				}
				resultChan <- struct {
					message []byte
					err     error
				}{nil, fmt.Errorf("panic during read: %v", r)}
			}
		}()
		
		message, err := reader.ReadMessage()
		resultChan <- struct {
			message []byte
			err     error
		}{message, err}
	}()
	
	// Wait for result or timeout with enhanced error handling
	select {
	case result := <-resultChan:
		if result.err != nil {
			// Classify and handle the error
			if t.isReadTimeoutError(result.err) {
				t.recordTimeoutError()
				
				if t.config.TimeoutRecoveryEnabled {
					// Attempt timeout recovery
					if t.config.DetailedErrorLogging {
						t.logger.Debug("Attempting timeout recovery for TCP read", 
							"connection_id", connID,
							"error", result.err)
					}
					
					// Clear the reader buffer to prevent corruption
					reader.Clear()
					
					// Apply recovery delay
					if t.config.TimeoutRecoveryDelay > 0 {
						time.Sleep(t.config.TimeoutRecoveryDelay)
					}
					
					return nil, &RecoverableReadTimeoutError{
						ConnectionID:    connID,
						TimeoutDuration: t.config.ReadTimeout,
						Err:             result.err,
					}
				}
			} else {
				t.recordReadError()
				
				if t.config.DetailedErrorLogging {
					t.logger.Debug("TCP read error", 
						"connection_id", connID,
						"error_type", fmt.Sprintf("%T", result.err),
						"error", result.err)
				}
			}
		}
		
		return result.message, result.err
		
	case <-time.After(t.config.ReadTimeout):
		// Timeout occurred
		t.recordTimeoutError()
		
		if t.config.DetailedErrorLogging {
			t.logger.Debug("TCP read timeout", 
				"connection_id", connID,
				"timeout", t.config.ReadTimeout)
		}
		
		// Clear the reader buffer on timeout to prevent corruption
		reader.Clear()
		
		if t.config.TimeoutRecoveryEnabled {
			return nil, &RecoverableReadTimeoutError{
				ConnectionID:    connID,
				TimeoutDuration: t.config.ReadTimeout,
				Err:             fmt.Errorf("read timeout after %v", t.config.ReadTimeout),
			}
		}
		
		return nil, fmt.Errorf("read timeout after %v", t.config.ReadTimeout)
		
	case <-t.stopChan:
		if t.config.DetailedErrorLogging {
			t.logger.Debug("TCP read interrupted by transport shutdown", 
				"connection_id", connID)
		}
		return nil, fmt.Errorf("transport stopping")
	}
}

// RecoverableReadTimeoutError represents a read timeout that might be recoverable
type RecoverableReadTimeoutError struct {
	ConnectionID    string
	TimeoutDuration time.Duration
	Err             error
}

func (e *RecoverableReadTimeoutError) Error() string {
	return fmt.Sprintf("recoverable read timeout on connection %s (timeout: %v): %v", 
		e.ConnectionID, e.TimeoutDuration, e.Err)
}

func (e *RecoverableReadTimeoutError) Timeout() bool {
	return true
}

func (e *RecoverableReadTimeoutError) Temporary() bool {
	return true
}

// isReadTimeoutError checks if an error is a read timeout error
func (t *EnhancedTCPTransport) isReadTimeoutError(err error) bool {
	if err == nil {
		return false
	}
	
	if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
		return true
	}
	
	// Check for timeout-related error messages
	errStr := strings.ToLower(err.Error())
	timeoutIndicators := []string{
		"timeout",
		"deadline exceeded",
		"i/o timeout",
	}
	
	for _, indicator := range timeoutIndicators {
		if strings.Contains(errStr, indicator) {
			return true
		}
	}
	
	return false
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

// GetStats returns transport statistics including error statistics
func (t *EnhancedTCPTransport) GetStats() map[string]interface{} {
	t.mu.RLock()
	defer t.mu.RUnlock()
	
	stats := map[string]interface{}{
		"running":                    t.running,
		"read_timeout":               t.config.ReadTimeout,
		"write_timeout":              t.config.WriteTimeout,
		"idle_timeout":               t.config.IdleTimeout,
		"max_connections":            t.config.MaxConnections,
		"max_retries":                t.config.MaxRetries,
		"max_retry_delay":            t.config.MaxRetryDelay,
		"backoff_multiplier":         t.config.BackoffMultiplier,
		"timeout_recovery_enabled":   t.config.TimeoutRecoveryEnabled,
		"timeout_recovery_delay":     t.config.TimeoutRecoveryDelay,
		"max_timeout_retries":        t.config.MaxTimeoutRetries,
		"connection_recovery_enabled": t.config.ConnectionRecoveryEnabled,
		"connection_recovery_delay":  t.config.ConnectionRecoveryDelay,
		"detailed_error_logging":     t.config.DetailedErrorLogging,
		"error_statistics_enabled":   t.config.ErrorStatistics,
	}
	
	if t.listener != nil {
		stats["local_addr"] = t.listener.Addr().String()
	}
	
	// Add connection manager stats
	connStats := t.connectionManager.GetStats()
	for k, v := range connStats {
		stats["conn_"+k] = v
	}
	
	// Add error statistics
	errorStats := t.GetErrorStatistics()
	for k, v := range errorStats {
		stats["error_"+k] = v
	}
	
	return stats
}

// GetConnectionManager returns the connection manager (for testing)
func (t *EnhancedTCPTransport) GetConnectionManager() *TCPConnectionManager {
	return t.connectionManager
}

// recordTimeoutError records a timeout error in statistics
func (t *EnhancedTCPTransport) recordTimeoutError() {
	if !t.config.ErrorStatistics {
		return
	}
	
	t.errorStats.mu.Lock()
	defer t.errorStats.mu.Unlock()
	
	t.errorStats.TimeoutErrors++
	t.errorStats.LastErrorTime = time.Now()
}

// recordConnectionError records a connection error in statistics
func (t *EnhancedTCPTransport) recordConnectionError() {
	if !t.config.ErrorStatistics {
		return
	}
	
	t.errorStats.mu.Lock()
	defer t.errorStats.mu.Unlock()
	
	t.errorStats.ConnectionErrors++
	t.errorStats.LastErrorTime = time.Now()
}

// recordReadError records a read error in statistics
func (t *EnhancedTCPTransport) recordReadError() {
	if !t.config.ErrorStatistics {
		return
	}
	
	t.errorStats.mu.Lock()
	defer t.errorStats.mu.Unlock()
	
	t.errorStats.ReadErrors++
	t.errorStats.LastErrorTime = time.Now()
}

// recordWriteError records a write error in statistics
func (t *EnhancedTCPTransport) recordWriteError() {
	if !t.config.ErrorStatistics {
		return
	}
	
	t.errorStats.mu.Lock()
	defer t.errorStats.mu.Unlock()
	
	t.errorStats.WriteErrors++
	t.errorStats.LastErrorTime = time.Now()
}

// recordRecoveryAttempt records a recovery attempt in statistics
func (t *EnhancedTCPTransport) recordRecoveryAttempt(successful bool) {
	if !t.config.ErrorStatistics {
		return
	}
	
	t.errorStats.mu.Lock()
	defer t.errorStats.mu.Unlock()
	
	t.errorStats.RecoveryAttempts++
	if successful {
		t.errorStats.SuccessfulRecoveries++
	} else {
		t.errorStats.FailedRecoveries++
	}
	t.errorStats.LastRecoveryTime = time.Now()
}

// GetErrorStatistics returns a copy of the current error statistics
func (t *EnhancedTCPTransport) GetErrorStatistics() map[string]interface{} {
	if !t.config.ErrorStatistics {
		return map[string]interface{}{"enabled": false}
	}
	
	t.errorStats.mu.RLock()
	defer t.errorStats.mu.RUnlock()
	
	return map[string]interface{}{
		"enabled":               true,
		"timeout_errors":        t.errorStats.TimeoutErrors,
		"connection_errors":     t.errorStats.ConnectionErrors,
		"read_errors":           t.errorStats.ReadErrors,
		"write_errors":          t.errorStats.WriteErrors,
		"recovery_attempts":     t.errorStats.RecoveryAttempts,
		"successful_recoveries": t.errorStats.SuccessfulRecoveries,
		"failed_recoveries":     t.errorStats.FailedRecoveries,
		"last_error_time":       t.errorStats.LastErrorTime,
		"last_recovery_time":    t.errorStats.LastRecoveryTime,
	}
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