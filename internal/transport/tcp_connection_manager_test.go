package transport

import (
	"net"
	"testing"
	"time"
)

// mockLogger implements the Logger interface for testing
type mockLogger struct {
	debugMsgs []string
	infoMsgs  []string
	warnMsgs  []string
	errorMsgs []string
}

func (l *mockLogger) Debug(msg string, fields ...interface{}) {
	l.debugMsgs = append(l.debugMsgs, msg)
}

func (l *mockLogger) Info(msg string, fields ...interface{}) {
	l.infoMsgs = append(l.infoMsgs, msg)
}

func (l *mockLogger) Warn(msg string, fields ...interface{}) {
	l.warnMsgs = append(l.warnMsgs, msg)
}

func (l *mockLogger) Error(msg string, fields ...interface{}) {
	l.errorMsgs = append(l.errorMsgs, msg)
}

func TestTCPConnection_NewTCPConnection(t *testing.T) {
	// Create a mock connection using a pipe
	server, client := net.Pipe()
	defer server.Close()
	defer client.Close()

	tcpConn := NewTCPConnection(server)

	if tcpConn.conn != server {
		t.Error("Expected connection to be set correctly")
	}

	if tcpConn.reader == nil {
		t.Error("Expected reader to be initialized")
	}

	if tcpConn.writer == nil {
		t.Error("Expected writer to be initialized")
	}

	if tcpConn.GetRemoteAddr() != server.RemoteAddr() {
		t.Error("Expected remote address to be set correctly")
	}

	if tcpConn.GetID() == "" {
		t.Error("Expected connection ID to be generated")
	}
}

func TestTCPConnection_ActivityTracking(t *testing.T) {
	server, client := net.Pipe()
	defer server.Close()
	defer client.Close()

	tcpConn := NewTCPConnection(server)
	
	initialActivity := tcpConn.GetLastActivity()
	
	// Wait a bit and update activity
	time.Sleep(10 * time.Millisecond)
	tcpConn.UpdateActivity()
	
	updatedActivity := tcpConn.GetLastActivity()
	
	if !updatedActivity.After(initialActivity) {
		t.Error("Expected activity timestamp to be updated")
	}
}

func TestTCPConnection_IdleCheck(t *testing.T) {
	server, client := net.Pipe()
	defer server.Close()
	defer client.Close()

	tcpConn := NewTCPConnection(server)
	
	// Should not be idle immediately
	if tcpConn.IsIdle(1 * time.Second) {
		t.Error("Connection should not be idle immediately")
	}
	
	// Wait and check if idle
	time.Sleep(10 * time.Millisecond)
	if !tcpConn.IsIdle(5 * time.Millisecond) {
		t.Error("Connection should be idle after timeout")
	}
}

func TestTCPConnection_Timeouts(t *testing.T) {
	server, client := net.Pipe()
	defer server.Close()
	defer client.Close()

	tcpConn := NewTCPConnection(server)
	
	// Test setting read timeout
	err := tcpConn.SetReadTimeout(1 * time.Second)
	if err != nil {
		t.Errorf("Failed to set read timeout: %v", err)
	}
	
	// Test setting write timeout
	err = tcpConn.SetWriteTimeout(1 * time.Second)
	if err != nil {
		t.Errorf("Failed to set write timeout: %v", err)
	}
	
	// Test clearing timeouts
	err = tcpConn.SetReadTimeout(0)
	if err != nil {
		t.Errorf("Failed to clear read timeout: %v", err)
	}
	
	err = tcpConn.SetWriteTimeout(0)
	if err != nil {
		t.Errorf("Failed to clear write timeout: %v", err)
	}
}

func TestTCPConnectionManager_NewManager(t *testing.T) {
	config := &TCPConnectionManagerConfig{
		IdleTimeout:     1 * time.Minute,
		ReadTimeout:     30 * time.Second,
		WriteTimeout:    30 * time.Second,
		CleanupInterval: 30 * time.Second,
		Logger:          &mockLogger{},
	}
	
	manager := NewTCPConnectionManager(config)
	defer manager.Stop()
	
	if manager.idleTimeout != config.IdleTimeout {
		t.Error("Expected idle timeout to be set correctly")
	}
	
	if manager.readTimeout != config.ReadTimeout {
		t.Error("Expected read timeout to be set correctly")
	}
	
	if manager.writeTimeout != config.WriteTimeout {
		t.Error("Expected write timeout to be set correctly")
	}
	
	if manager.GetConnectionCount() != 0 {
		t.Error("Expected no connections initially")
	}
}

func TestTCPConnectionManager_DefaultConfig(t *testing.T) {
	manager := NewTCPConnectionManager(nil)
	defer manager.Stop()
	
	// Should use default configuration
	if manager.idleTimeout != 5*time.Minute {
		t.Error("Expected default idle timeout")
	}
	
	if manager.readTimeout != 30*time.Second {
		t.Error("Expected default read timeout")
	}
	
	if manager.writeTimeout != 30*time.Second {
		t.Error("Expected default write timeout")
	}
}

func TestTCPConnectionManager_AddRemoveConnection(t *testing.T) {
	logger := &mockLogger{}
	config := &TCPConnectionManagerConfig{
		IdleTimeout:     1 * time.Minute,
		ReadTimeout:     30 * time.Second,
		WriteTimeout:    30 * time.Second,
		CleanupInterval: 30 * time.Second,
		Logger:          logger,
	}
	
	manager := NewTCPConnectionManager(config)
	defer manager.Stop()
	
	// Create a mock connection
	server, client := net.Pipe()
	defer server.Close()
	defer client.Close()
	
	// Add connection
	tcpConn := manager.AddConnection(server)
	
	if manager.GetConnectionCount() != 1 {
		t.Error("Expected 1 connection after adding")
	}
	
	// Verify connection can be retrieved
	retrievedConn, exists := manager.GetConnection(tcpConn.GetID())
	if !exists {
		t.Error("Expected to find added connection")
	}
	
	if retrievedConn != tcpConn {
		t.Error("Expected retrieved connection to match added connection")
	}
	
	// Remove connection
	manager.RemoveConnection(tcpConn.GetID())
	
	if manager.GetConnectionCount() != 0 {
		t.Error("Expected 0 connections after removing")
	}
	
	// Verify connection cannot be retrieved
	_, exists = manager.GetConnection(tcpConn.GetID())
	if exists {
		t.Error("Expected connection to be removed")
	}
	
	// Check that debug messages were logged
	if len(logger.debugMsgs) < 2 {
		t.Error("Expected debug messages for add and remove operations")
	}
}

func TestTCPConnectionManager_GetAllConnections(t *testing.T) {
	manager := NewTCPConnectionManager(nil)
	defer manager.Stop()
	
	// Add multiple connections
	var connections []*TCPConnection
	for i := 0; i < 3; i++ {
		server, client := net.Pipe()
		defer server.Close()
		defer client.Close()
		
		conn := manager.AddConnection(server)
		connections = append(connections, conn)
	}
	
	allConnections := manager.GetAllConnections()
	
	if len(allConnections) != 3 {
		t.Errorf("Expected 3 connections, got %d", len(allConnections))
	}
	
	// Verify all connections are present
	for _, conn := range connections {
		if _, exists := allConnections[conn.GetID()]; !exists {
			t.Errorf("Expected to find connection %s", conn.GetID())
		}
	}
}

func TestTCPConnectionManager_IdleCleanup(t *testing.T) {
	logger := &mockLogger{}
	config := &TCPConnectionManagerConfig{
		IdleTimeout:     50 * time.Millisecond, // Very short for testing
		ReadTimeout:     30 * time.Second,
		WriteTimeout:    30 * time.Second,
		CleanupInterval: 1 * time.Hour, // Very long to prevent automatic cleanup during test
		Logger:          logger,
	}
	
	manager := NewTCPConnectionManager(config)
	defer manager.Stop()
	
	// Add a connection
	server, client := net.Pipe()
	defer server.Close()
	defer client.Close()
	
	tcpConn := manager.AddConnection(server)
	
	if manager.GetConnectionCount() != 1 {
		t.Error("Expected 1 connection after adding")
	}
	
	// Wait for connection to become idle
	time.Sleep(100 * time.Millisecond)
	
	// Verify the connection is actually idle
	if !tcpConn.IsIdle(config.IdleTimeout) {
		t.Errorf("Connection should be idle. Last activity: %v, idle timeout: %v, time since: %v", 
			tcpConn.GetLastActivity(), config.IdleTimeout, time.Since(tcpConn.GetLastActivity()))
	}
	
	// Manual cleanup to test the functionality
	cleanedCount := manager.CleanupIdleConnections()
	
	if cleanedCount != 1 {
		t.Errorf("Expected 1 connection to be cleaned up, got %d", cleanedCount)
	}
	
	if manager.GetConnectionCount() != 0 {
		t.Error("Expected 0 connections after cleanup")
	}
	
	// Verify connection cannot be retrieved
	_, exists := manager.GetConnection(tcpConn.GetID())
	if exists {
		t.Error("Expected connection to be cleaned up")
	}
}

func TestTCPConnectionManager_ActivityUpdate(t *testing.T) {
	config := &TCPConnectionManagerConfig{
		IdleTimeout:     100 * time.Millisecond,
		ReadTimeout:     30 * time.Second,
		WriteTimeout:    30 * time.Second,
		CleanupInterval: 30 * time.Second,
		Logger:          &mockLogger{},
	}
	
	manager := NewTCPConnectionManager(config)
	defer manager.Stop()
	
	// Add a connection
	server, client := net.Pipe()
	defer server.Close()
	defer client.Close()
	
	tcpConn := manager.AddConnection(server)
	
	// Wait a bit, then update activity
	time.Sleep(50 * time.Millisecond)
	manager.UpdateConnectionActivity(tcpConn.GetID())
	
	// Wait more, but connection should not be idle due to activity update
	time.Sleep(60 * time.Millisecond)
	
	cleanedCount := manager.CleanupIdleConnections()
	
	if cleanedCount != 0 {
		t.Error("Expected no connections to be cleaned up due to recent activity")
	}
	
	if manager.GetConnectionCount() != 1 {
		t.Error("Expected 1 connection to remain after cleanup")
	}
}

func TestTCPConnectionManager_SetTimeouts(t *testing.T) {
	manager := NewTCPConnectionManager(nil)
	defer manager.Stop()
	
	// Add a connection
	server, client := net.Pipe()
	defer server.Close()
	defer client.Close()
	
	manager.AddConnection(server)
	
	// Update timeouts
	newReadTimeout := 1 * time.Minute
	newWriteTimeout := 2 * time.Minute
	
	manager.SetTimeouts(newReadTimeout, newWriteTimeout)
	
	if manager.readTimeout != newReadTimeout {
		t.Error("Expected read timeout to be updated")
	}
	
	if manager.writeTimeout != newWriteTimeout {
		t.Error("Expected write timeout to be updated")
	}
}

func TestTCPConnectionManager_GetStats(t *testing.T) {
	config := &TCPConnectionManagerConfig{
		IdleTimeout:     50 * time.Millisecond, // Short for testing
		ReadTimeout:     30 * time.Second,
		WriteTimeout:    30 * time.Second,
		CleanupInterval: 30 * time.Second,
		Logger:          &mockLogger{},
	}
	
	manager := NewTCPConnectionManager(config)
	defer manager.Stop()
	
	// Add connections
	server1, client1 := net.Pipe()
	defer server1.Close()
	defer client1.Close()
	
	server2, client2 := net.Pipe()
	defer server2.Close()
	defer client2.Close()
	
	manager.AddConnection(server1)
	manager.AddConnection(server2)
	
	// Get initial stats
	stats := manager.GetStats()
	
	if stats["total_connections"] != 2 {
		t.Errorf("Expected 2 total connections, got %v", stats["total_connections"])
	}
	
	if stats["idle_timeout"] != config.IdleTimeout {
		t.Error("Expected idle timeout to match config")
	}
	
	if stats["read_timeout"] != config.ReadTimeout {
		t.Error("Expected read timeout to match config")
	}
	
	if stats["write_timeout"] != config.WriteTimeout {
		t.Error("Expected write timeout to match config")
	}
	
	// Wait for connections to become idle
	time.Sleep(60 * time.Millisecond)
	
	stats = manager.GetStats()
	
	if stats["idle_connections"] != 2 {
		t.Errorf("Expected 2 idle connections, got %v", stats["idle_connections"])
	}
	
	if stats["active_connections"] != 0 {
		t.Errorf("Expected 0 active connections, got %v", stats["active_connections"])
	}
}

func TestTCPConnectionManager_Stop(t *testing.T) {
	logger := &mockLogger{}
	config := &TCPConnectionManagerConfig{
		IdleTimeout:     1 * time.Minute,
		ReadTimeout:     30 * time.Second,
		WriteTimeout:    30 * time.Second,
		CleanupInterval: 30 * time.Second,
		Logger:          logger,
	}
	
	manager := NewTCPConnectionManager(config)
	
	// Add connections
	server1, client1 := net.Pipe()
	defer client1.Close()
	
	server2, client2 := net.Pipe()
	defer client2.Close()
	
	manager.AddConnection(server1)
	manager.AddConnection(server2)
	
	if manager.GetConnectionCount() != 2 {
		t.Error("Expected 2 connections before stop")
	}
	
	// Stop the manager
	err := manager.Stop()
	if err != nil {
		t.Errorf("Failed to stop manager: %v", err)
	}
	
	if manager.GetConnectionCount() != 0 {
		t.Error("Expected 0 connections after stop")
	}
	
	// Check that info messages were logged
	if len(logger.infoMsgs) < 2 {
		t.Error("Expected info messages for start and stop operations")
	}
}

func TestTCPConnectionManager_ConcurrentAccess(t *testing.T) {
	manager := NewTCPConnectionManager(nil)
	defer manager.Stop()
	
	// Test concurrent add/remove operations
	done := make(chan bool, 10)
	
	// Add connections concurrently
	for i := 0; i < 5; i++ {
		go func() {
			server, client := net.Pipe()
			defer server.Close()
			defer client.Close()
			
			conn := manager.AddConnection(server)
			time.Sleep(10 * time.Millisecond)
			manager.RemoveConnection(conn.GetID())
			done <- true
		}()
	}
	
	// Get stats concurrently
	for i := 0; i < 5; i++ {
		go func() {
			for j := 0; j < 10; j++ {
				manager.GetStats()
				manager.GetConnectionCount()
				time.Sleep(1 * time.Millisecond)
			}
			done <- true
		}()
	}
	
	// Wait for all goroutines to complete
	for i := 0; i < 10; i++ {
		<-done
	}
	
	// Should not panic and should have consistent state
	if manager.GetConnectionCount() < 0 {
		t.Error("Connection count should not be negative")
	}
}