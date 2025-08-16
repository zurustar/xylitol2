package transport

import (
	"bufio"
	"fmt"
	"net"
	"sync"
	"sync/atomic"
	"time"
)

// connectionIDCounter is used to generate unique connection IDs
var connectionIDCounter int64

// TCPConnection represents a managed TCP connection with metadata
type TCPConnection struct {
	conn         net.Conn
	reader       *bufio.Reader
	writer       *bufio.Writer
	lastActivity time.Time
	remoteAddr   net.Addr
	id           string
	mu           sync.RWMutex
}

// NewTCPConnection creates a new managed TCP connection
func NewTCPConnection(conn net.Conn) *TCPConnection {
	// Generate unique ID using counter to avoid collisions with pipe connections
	connID := atomic.AddInt64(&connectionIDCounter, 1)
	
	return &TCPConnection{
		conn:         conn,
		reader:       bufio.NewReader(conn),
		writer:       bufio.NewWriter(conn),
		lastActivity: time.Now(),
		remoteAddr:   conn.RemoteAddr(),
		id:           fmt.Sprintf("conn-%d-%s->%s", connID, conn.LocalAddr(), conn.RemoteAddr()),
	}
}

// UpdateActivity updates the last activity timestamp
func (tc *TCPConnection) UpdateActivity() {
	tc.mu.Lock()
	defer tc.mu.Unlock()
	tc.lastActivity = time.Now()
}

// GetLastActivity returns the last activity timestamp
func (tc *TCPConnection) GetLastActivity() time.Time {
	tc.mu.RLock()
	defer tc.mu.RUnlock()
	return tc.lastActivity
}

// IsIdle checks if the connection has been idle for the specified duration
func (tc *TCPConnection) IsIdle(idleTimeout time.Duration) bool {
	tc.mu.RLock()
	defer tc.mu.RUnlock()
	return time.Since(tc.lastActivity) > idleTimeout
}

// Close closes the underlying connection
func (tc *TCPConnection) Close() error {
	return tc.conn.Close()
}

// GetID returns the connection identifier
func (tc *TCPConnection) GetID() string {
	return tc.id
}

// GetRemoteAddr returns the remote address
func (tc *TCPConnection) GetRemoteAddr() net.Addr {
	return tc.remoteAddr
}

// SetReadTimeout sets a read timeout on the connection
func (tc *TCPConnection) SetReadTimeout(timeout time.Duration) error {
	if timeout > 0 {
		return tc.conn.SetReadDeadline(time.Now().Add(timeout))
	}
	return tc.conn.SetReadDeadline(time.Time{})
}

// SetWriteTimeout sets a write timeout on the connection
func (tc *TCPConnection) SetWriteTimeout(timeout time.Duration) error {
	if timeout > 0 {
		return tc.conn.SetWriteDeadline(time.Now().Add(timeout))
	}
	return tc.conn.SetWriteDeadline(time.Time{})
}

// TCPConnectionManager manages a pool of TCP connections with lifecycle management
type TCPConnectionManager struct {
	connections    map[string]*TCPConnection
	mu             sync.RWMutex
	idleTimeout    time.Duration
	readTimeout    time.Duration
	writeTimeout   time.Duration
	cleanupTicker  *time.Ticker
	stopCleanup    chan struct{}
	cleanupRunning bool
	logger         Logger // Interface for logging
}

// Logger interface for connection manager logging
type Logger interface {
	Debug(msg string, fields ...interface{})
	Info(msg string, fields ...interface{})
	Warn(msg string, fields ...interface{})
	Error(msg string, fields ...interface{})
}

// TCPConnectionManagerConfig holds configuration for the connection manager
type TCPConnectionManagerConfig struct {
	IdleTimeout     time.Duration
	ReadTimeout     time.Duration
	WriteTimeout    time.Duration
	CleanupInterval time.Duration
	Logger          Logger
}

// DefaultTCPConnectionManagerConfig returns default configuration
func DefaultTCPConnectionManagerConfig() *TCPConnectionManagerConfig {
	return &TCPConnectionManagerConfig{
		IdleTimeout:     5 * time.Minute,
		ReadTimeout:     30 * time.Second,
		WriteTimeout:    30 * time.Second,
		CleanupInterval: 1 * time.Minute,
		Logger:          &noOpLogger{},
	}
}

// noOpLogger is a no-operation logger implementation
type noOpLogger struct{}

func (l *noOpLogger) Debug(msg string, fields ...interface{}) {}
func (l *noOpLogger) Info(msg string, fields ...interface{})  {}
func (l *noOpLogger) Warn(msg string, fields ...interface{})  {}
func (l *noOpLogger) Error(msg string, fields ...interface{}) {}

// NewTCPConnectionManager creates a new TCP connection manager
func NewTCPConnectionManager(config *TCPConnectionManagerConfig) *TCPConnectionManager {
	if config == nil {
		config = DefaultTCPConnectionManagerConfig()
	}

	manager := &TCPConnectionManager{
		connections:  make(map[string]*TCPConnection),
		idleTimeout:  config.IdleTimeout,
		readTimeout:  config.ReadTimeout,
		writeTimeout: config.WriteTimeout,
		stopCleanup:  make(chan struct{}),
		logger:       config.Logger,
	}

	// Start cleanup routine
	manager.startCleanupRoutine(config.CleanupInterval)

	return manager
}

// AddConnection adds a new connection to the pool
func (cm *TCPConnectionManager) AddConnection(conn net.Conn) *TCPConnection {
	tcpConn := NewTCPConnection(conn)
	
	cm.mu.Lock()
	defer cm.mu.Unlock()
	
	cm.connections[tcpConn.GetID()] = tcpConn
	
	// Set initial timeouts
	if cm.readTimeout > 0 {
		tcpConn.SetReadTimeout(cm.readTimeout)
	}
	if cm.writeTimeout > 0 {
		tcpConn.SetWriteTimeout(cm.writeTimeout)
	}
	
	cm.logger.Debug("Added TCP connection", "id", tcpConn.GetID(), "remote", tcpConn.GetRemoteAddr())
	
	return tcpConn
}

// RemoveConnection removes a connection from the pool
func (cm *TCPConnectionManager) RemoveConnection(connectionID string) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	
	if conn, exists := cm.connections[connectionID]; exists {
		conn.Close()
		delete(cm.connections, connectionID)
		cm.logger.Debug("Removed TCP connection", "id", connectionID)
	}
}

// GetConnection retrieves a connection by ID
func (cm *TCPConnectionManager) GetConnection(connectionID string) (*TCPConnection, bool) {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	
	conn, exists := cm.connections[connectionID]
	if exists {
		conn.UpdateActivity()
	}
	
	return conn, exists
}

// GetConnectionCount returns the number of active connections
func (cm *TCPConnectionManager) GetConnectionCount() int {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	return len(cm.connections)
}

// GetAllConnections returns a copy of all active connections
func (cm *TCPConnectionManager) GetAllConnections() map[string]*TCPConnection {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	
	connections := make(map[string]*TCPConnection)
	for id, conn := range cm.connections {
		connections[id] = conn
	}
	
	return connections
}

// CleanupIdleConnections removes connections that have been idle for too long
func (cm *TCPConnectionManager) CleanupIdleConnections() int {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	
	var toRemove []string
	
	for id, conn := range cm.connections {
		if conn.IsIdle(cm.idleTimeout) {
			toRemove = append(toRemove, id)
			cm.logger.Info("Marking idle connection for cleanup", 
				"id", id, 
				"idle_duration", time.Since(conn.GetLastActivity()))
		}
	}
	
	// Remove idle connections
	for _, id := range toRemove {
		if conn, exists := cm.connections[id]; exists {
			conn.Close()
			delete(cm.connections, id)
			cm.logger.Debug("Cleaned up idle connection", "id", id)
		}
	}
	
	if len(toRemove) > 0 {
		cm.logger.Info("Cleaned up idle connections", "count", len(toRemove))
	}
	
	return len(toRemove)
}

// startCleanupRoutine starts the periodic cleanup routine
func (cm *TCPConnectionManager) startCleanupRoutine(interval time.Duration) {
	cm.cleanupTicker = time.NewTicker(interval)
	cm.cleanupRunning = true
	
	go func() {
		defer func() {
			cm.cleanupRunning = false
		}()
		
		for {
			select {
			case <-cm.cleanupTicker.C:
				cm.CleanupIdleConnections()
			case <-cm.stopCleanup:
				return
			}
		}
	}()
	
	cm.logger.Info("Started TCP connection cleanup routine", "interval", interval)
}

// Stop stops the connection manager and closes all connections
func (cm *TCPConnectionManager) Stop() error {
	cm.logger.Info("Stopping TCP connection manager")
	
	// Stop cleanup routine
	if cm.cleanupRunning {
		close(cm.stopCleanup)
		cm.cleanupTicker.Stop()
	}
	
	// Close all connections
	cm.mu.Lock()
	defer cm.mu.Unlock()
	
	var errors []error
	for id, conn := range cm.connections {
		if err := conn.Close(); err != nil {
			errors = append(errors, fmt.Errorf("failed to close connection %s: %w", id, err))
		}
	}
	
	// Clear the connections map
	cm.connections = make(map[string]*TCPConnection)
	
	cm.logger.Info("Stopped TCP connection manager", "closed_connections", len(cm.connections))
	
	if len(errors) > 0 {
		return fmt.Errorf("errors closing connections: %v", errors)
	}
	
	return nil
}

// UpdateConnectionActivity updates the activity timestamp for a connection
func (cm *TCPConnectionManager) UpdateConnectionActivity(connectionID string) {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	
	if conn, exists := cm.connections[connectionID]; exists {
		conn.UpdateActivity()
	}
}

// SetTimeouts updates the timeout settings for all existing connections
func (cm *TCPConnectionManager) SetTimeouts(readTimeout, writeTimeout time.Duration) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	
	cm.readTimeout = readTimeout
	cm.writeTimeout = writeTimeout
	
	// Apply to existing connections
	for _, conn := range cm.connections {
		if readTimeout > 0 {
			conn.SetReadTimeout(readTimeout)
		}
		if writeTimeout > 0 {
			conn.SetWriteTimeout(writeTimeout)
		}
	}
	
	cm.logger.Info("Updated connection timeouts", 
		"read_timeout", readTimeout, 
		"write_timeout", writeTimeout)
}

// GetStats returns statistics about the connection pool
func (cm *TCPConnectionManager) GetStats() map[string]interface{} {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	
	stats := map[string]interface{}{
		"total_connections": len(cm.connections),
		"idle_timeout":      cm.idleTimeout,
		"read_timeout":      cm.readTimeout,
		"write_timeout":     cm.writeTimeout,
	}
	
	// Count idle connections
	idleCount := 0
	for _, conn := range cm.connections {
		if conn.IsIdle(cm.idleTimeout) {
			idleCount++
		}
	}
	stats["idle_connections"] = idleCount
	stats["active_connections"] = len(cm.connections) - idleCount
	
	return stats
}