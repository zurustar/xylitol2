package transport

import (
	"net"
	"testing"
	"time"
)

// TestTCPTimeoutAndRecoveryIntegration demonstrates the complete timeout and error recovery functionality
func TestTCPTimeoutAndRecoveryIntegration(t *testing.T) {
	// Create a logger to capture detailed logs
	logger := &mockTimeoutLogger{}
	
	// Configure transport with timeout and recovery settings
	config := DefaultEnhancedTCPConfig()
	config.Logger = logger
	config.ReadTimeout = 200 * time.Millisecond
	config.WriteTimeout = 200 * time.Millisecond
	config.MaxRetries = 3
	config.RetryDelay = 50 * time.Millisecond
	config.MaxRetryDelay = 200 * time.Millisecond
	config.BackoffMultiplier = 1.5
	config.TimeoutRecoveryEnabled = true
	config.TimeoutRecoveryDelay = 25 * time.Millisecond
	config.MaxTimeoutRetries = 2
	config.ConnectionRecoveryEnabled = true
	config.ConnectionRecoveryDelay = 100 * time.Millisecond
	config.DetailedErrorLogging = true
	config.ErrorStatistics = true
	
	transport := NewEnhancedTCPTransport(config)
	err := transport.Start(0)
	if err != nil {
		t.Fatalf("Failed to start transport: %v", err)
	}
	defer transport.Stop()
	
	// Test 1: Verify configuration is applied correctly
	stats := transport.GetStats()
	if stats["timeout_recovery_enabled"] != true {
		t.Error("Expected timeout recovery to be enabled")
	}
	if stats["connection_recovery_enabled"] != true {
		t.Error("Expected connection recovery to be enabled")
	}
	if stats["detailed_error_logging"] != true {
		t.Error("Expected detailed error logging to be enabled")
	}
	if stats["error_statistics_enabled"] != true {
		t.Error("Expected error statistics to be enabled")
	}
	
	// Test 2: Test send message with connection error and recovery
	testMessage := []byte("INVITE sip:test@example.com SIP/2.0\r\nContent-Length: 0\r\n\r\n")
	unreachableAddr, _ := net.ResolveTCPAddr("tcp", "127.0.0.1:9999") // Non-existent port
	
	startTime := time.Now()
	err = transport.SendMessage(testMessage, unreachableAddr)
	duration := time.Since(startTime)
	
	// Should fail but with retry attempts
	if err == nil {
		t.Error("Expected error when sending to unreachable address")
	}
	
	// Should have taken time for retries
	if duration < 100*time.Millisecond {
		t.Errorf("Expected retries to take some time, took: %v", duration)
	}
	
	// Test 3: Verify error statistics were collected
	errorStats := transport.GetErrorStatistics()
	if errorStats["enabled"] != true {
		t.Error("Expected error statistics to be enabled")
	}
	
	// Should have recorded some errors
	totalErrors := int64(0)
	if connectionErrors, ok := errorStats["connection_errors"].(int64); ok {
		totalErrors += connectionErrors
	}
	if writeErrors, ok := errorStats["write_errors"].(int64); ok {
		totalErrors += writeErrors
	}
	
	if totalErrors == 0 {
		t.Errorf("Expected some errors to be recorded, got stats: %v", errorStats)
	}
	
	// Should have attempted recovery
	if recoveryAttempts, ok := errorStats["recovery_attempts"].(int64); ok && recoveryAttempts > 0 {
		// Good, recovery was attempted
	} else {
		t.Logf("No recovery attempts recorded, which is acceptable for connection refused errors")
	}
	
	// Test 4: Test timeout handling with a real connection
	localAddr := transport.LocalAddr().(*net.TCPAddr)
	
	// Create a connection that will timeout
	clientConn, err := net.DialTCP("tcp", nil, localAddr)
	if err != nil {
		t.Fatalf("Failed to create client connection: %v", err)
	}
	defer clientConn.Close()
	
	// Send partial data to trigger read timeout
	partialMessage := "INVITE sip:test@example.com SIP/2.0\r\n"
	_, err = clientConn.Write([]byte(partialMessage))
	if err != nil {
		t.Fatalf("Failed to send partial message: %v", err)
	}
	
	// Wait for timeout handling
	time.Sleep(600 * time.Millisecond)
	
	// Check that timeout errors were recorded
	errorStats = transport.GetErrorStatistics()
	timeoutErrors, ok := errorStats["timeout_errors"].(int64)
	if !ok || timeoutErrors == 0 {
		t.Errorf("Expected timeout errors to be recorded, got: %v", errorStats["timeout_errors"])
	}
	
	// Test 5: Verify detailed logging occurred
	debug, info, warn, errorMsgs := logger.getMessages()
	
	// Should have debug messages about timeouts
	foundTimeoutMessage := false
	for _, msg := range debug {
		if containsAny(msg, []string{"timeout", "TCP read timeout", "Recoverable read timeout"}) {
			foundTimeoutMessage = true
			break
		}
	}
	
	if !foundTimeoutMessage {
		t.Logf("Debug messages: %v", debug)
		t.Error("Expected timeout-related debug messages")
	}
	
	// Should have some informational or warning messages
	totalLogMessages := len(debug) + len(info) + len(warn) + len(errorMsgs)
	if totalLogMessages == 0 {
		t.Error("Expected some log messages to be generated")
	}
	
	// Test 6: Verify transport can still handle normal messages after errors
	handler := &mockMessageHandler{}
	transport.RegisterHandler(handler)
	
	// Create a fresh connection for the valid message test
	freshConn, err := net.DialTCP("tcp", nil, localAddr)
	if err != nil {
		t.Fatalf("Failed to create fresh client connection: %v", err)
	}
	defer freshConn.Close()
	
	// Send a complete valid message
	validMessage := []byte("OPTIONS sip:test@example.com SIP/2.0\r\nContent-Length: 0\r\n\r\n")
	_, err = freshConn.Write(validMessage)
	if err != nil {
		t.Fatalf("Failed to send valid message: %v", err)
	}
	
	// Wait for message processing
	time.Sleep(200 * time.Millisecond)
	
	// Should have received the message
	messages := handler.getMessages()
	if len(messages) == 0 {
		t.Error("Expected to receive the valid message after error recovery")
	}
	
	t.Logf("Integration test completed successfully")
	t.Logf("Final error statistics: %v", transport.GetErrorStatistics())
	t.Logf("Final transport stats: timeout_recovery_enabled=%v, connection_recovery_enabled=%v", 
		stats["timeout_recovery_enabled"], stats["connection_recovery_enabled"])
}

// containsAny checks if a string contains any of the given substrings
func containsAny(s string, substrings []string) bool {
	for _, substr := range substrings {
		if len(substr) > 0 && len(s) >= len(substr) {
			for i := 0; i <= len(s)-len(substr); i++ {
				if s[i:i+len(substr)] == substr {
					return true
				}
			}
		}
	}
	return false
}

// TestTCPTimeoutConfigurationValidation tests that timeout configuration is properly validated and applied
func TestTCPTimeoutConfigurationValidation(t *testing.T) {
	// Test with custom timeout configuration
	config := &EnhancedTCPConfig{
		ReadTimeout:               1 * time.Second,
		WriteTimeout:              2 * time.Second,
		IdleTimeout:               10 * time.Minute,
		AcceptTimeout:             500 * time.Millisecond,
		MaxConnections:            100,
		CleanupInterval:           30 * time.Second,
		MaxRetries:                5,
		RetryDelay:                200 * time.Millisecond,
		MaxRetryDelay:             10 * time.Second,
		BackoffMultiplier:         2.5,
		TimeoutRecoveryEnabled:    true,
		TimeoutRecoveryDelay:      100 * time.Millisecond,
		MaxTimeoutRetries:         3,
		ConnectionRecoveryEnabled: true,
		ConnectionRecoveryDelay:   1 * time.Second,
		DetailedErrorLogging:      true,
		ErrorStatistics:           true,
		Logger:                    &mockTimeoutLogger{},
	}
	
	transport := NewEnhancedTCPTransport(config)
	defer transport.Stop()
	
	// Verify configuration was applied
	stats := transport.GetStats()
	
	if stats["read_timeout"] != 1*time.Second {
		t.Errorf("Expected read timeout 1s, got: %v", stats["read_timeout"])
	}
	
	if stats["write_timeout"] != 2*time.Second {
		t.Errorf("Expected write timeout 2s, got: %v", stats["write_timeout"])
	}
	
	if stats["max_retries"] != 5 {
		t.Errorf("Expected max retries 5, got: %v", stats["max_retries"])
	}
	
	if stats["backoff_multiplier"] != 2.5 {
		t.Errorf("Expected backoff multiplier 2.5, got: %v", stats["backoff_multiplier"])
	}
	
	if stats["timeout_recovery_enabled"] != true {
		t.Error("Expected timeout recovery to be enabled")
	}
	
	if stats["max_timeout_retries"] != 3 {
		t.Errorf("Expected max timeout retries 3, got: %v", stats["max_timeout_retries"])
	}
	
	// Test updating timeouts
	transport.SetTimeouts(5*time.Second, 10*time.Second, 30*time.Minute)
	
	updatedStats := transport.GetStats()
	if updatedStats["read_timeout"] != 5*time.Second {
		t.Errorf("Expected updated read timeout 5s, got: %v", updatedStats["read_timeout"])
	}
	
	if updatedStats["write_timeout"] != 10*time.Second {
		t.Errorf("Expected updated write timeout 10s, got: %v", updatedStats["write_timeout"])
	}
}