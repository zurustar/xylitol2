package handlers

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/zurustar/xylitol2/internal/parser"
)

func TestLogLevel_String(t *testing.T) {
	tests := []struct {
		level    LogLevel
		expected string
	}{
		{LogLevelError, "ERROR"},
		{LogLevelWarn, "WARN"},
		{LogLevelInfo, "INFO"},
		{LogLevelDebug, "DEBUG"},
		{LogLevel(999), "UNKNOWN"},
	}
	
	for _, test := range tests {
		result := test.level.String()
		if result != test.expected {
			t.Errorf("LogLevel.String() = %s, expected %s", result, test.expected)
		}
	}
}

func TestNewDetailedErrorLogger(t *testing.T) {
	logger := NewDetailedErrorLogger(LogLevelInfo, true)
	
	if logger == nil {
		t.Fatal("NewDetailedErrorLogger should not return nil")
	}
	
	if logger.logLevel != LogLevelInfo {
		t.Errorf("Log level should be Info, got %v", logger.logLevel)
	}
	
	if !logger.enableDebug {
		t.Error("Debug should be enabled")
	}
	
	// Check that statistics are initialized
	stats := logger.GetErrorStatistics()
	if stats.ParseErrors != 0 {
		t.Error("Initial parse errors should be 0")
	}
}

func TestDetailedErrorLogger_LogParseError(t *testing.T) {
	logger := NewDetailedErrorLogger(LogLevelError, true)
	
	parseErr := errors.New("invalid request line")
	rawMessage := []byte("INVALID SIP MESSAGE\r\nHeader: value\r\n")
	context := map[string]interface{}{
		"source": "test",
	}
	
	// Capture initial statistics
	initialStats := logger.GetErrorStatistics()
	
	logger.LogParseError(parseErr, rawMessage, context)
	
	// Check that statistics were updated
	stats := logger.GetErrorStatistics()
	if stats.ParseErrors != initialStats.ParseErrors+1 {
		t.Errorf("Parse error count should increase by 1, got %d", stats.ParseErrors)
	}
}

func TestDetailedErrorLogger_LogValidationError(t *testing.T) {
	logger := NewDetailedErrorLogger(LogLevelWarn, true)
	
	req := parser.NewRequestMessage(parser.MethodINVITE, "sip:test@example.com")
	req.SetHeader(parser.HeaderCallID, "test-call-id")
	req.SetHeader(parser.HeaderFrom, "sip:alice@example.com")
	req.SetHeader(parser.HeaderTo, "sip:bob@example.com")
	
	validationErr := &DetailedValidationError{
		ValidationError: &ValidationError{
			ValidatorName: "TestValidator",
			Code:          parser.StatusBadRequest,
			Reason:        "Test validation error",
		},
		MissingHeaders: []string{"Via"},
		InvalidHeaders: map[string]string{
			"CSeq": "invalid format",
		},
		Suggestions: []string{"Add Via header"},
	}
	
	context := map[string]interface{}{
		"test_context": "validation_test",
	}
	
	// Capture initial statistics
	initialStats := logger.GetErrorStatistics()
	
	logger.LogValidationError(validationErr, req, context)
	
	// Check that statistics were updated
	stats := logger.GetErrorStatistics()
	if stats.ValidationErrors != initialStats.ValidationErrors+1 {
		t.Errorf("Validation error count should increase by 1, got %d", stats.ValidationErrors)
	}
}

func TestDetailedErrorLogger_LogProcessingError(t *testing.T) {
	logger := NewDetailedErrorLogger(LogLevelError, false)
	
	req := parser.NewRequestMessage(parser.MethodINVITE, "sip:test@example.com")
	req.SetHeader(parser.HeaderCallID, "test-call-id")
	
	processingErr := errors.New("database connection failed")
	context := map[string]interface{}{
		"operation": "user_lookup",
	}
	
	// Capture initial statistics
	initialStats := logger.GetErrorStatistics()
	
	logger.LogProcessingError(processingErr, req, context)
	
	// Check that statistics were updated
	stats := logger.GetErrorStatistics()
	if stats.ProcessingErrors != initialStats.ProcessingErrors+1 {
		t.Errorf("Processing error count should increase by 1, got %d", stats.ProcessingErrors)
	}
}

func TestDetailedErrorLogger_LogTransportError(t *testing.T) {
	logger := NewDetailedErrorLogger(LogLevelError, false)
	
	req := parser.NewRequestMessage(parser.MethodINVITE, "sip:test@example.com")
	req.SetHeader(parser.HeaderCallID, "test-call-id")
	
	transportErr := errors.New("connection timeout")
	context := map[string]interface{}{
		"transport": "TCP",
		"remote_addr": "192.168.1.100:5060",
	}
	
	// Capture initial statistics
	initialStats := logger.GetErrorStatistics()
	
	logger.LogTransportError(transportErr, req, context)
	
	// Check that statistics were updated
	stats := logger.GetErrorStatistics()
	if stats.TransportErrors != initialStats.TransportErrors+1 {
		t.Errorf("Transport error count should increase by 1, got %d", stats.TransportErrors)
	}
}

func TestDetailedErrorLogger_ResetStatistics(t *testing.T) {
	logger := NewDetailedErrorLogger(LogLevelError, false)
	
	// Generate some errors
	logger.LogParseError(errors.New("test"), []byte("test"), nil)
	logger.LogProcessingError(errors.New("test"), nil, nil)
	
	// Check statistics are non-zero
	stats := logger.GetErrorStatistics()
	if stats.ParseErrors == 0 || stats.ProcessingErrors == 0 {
		t.Error("Statistics should be non-zero before reset")
	}
	
	// Reset statistics
	logger.ResetStatistics()
	
	// Check statistics are zero
	stats = logger.GetErrorStatistics()
	if stats.ParseErrors != 0 || stats.ProcessingErrors != 0 {
		t.Error("Statistics should be zero after reset")
	}
}

func TestDetailedErrorLogger_sanitizeRawMessage(t *testing.T) {
	logger := NewDetailedErrorLogger(LogLevelDebug, true)
	
	tests := []struct {
		input    []byte
		contains []string
		notContains []string
	}{
		{
			input:    []byte("INVITE sip:test@example.com SIP/2.0\r\nVia: SIP/2.0/UDP client:5060\r\n"),
			contains: []string{"INVITE", "sip:test@example.com", "\\r", "\\n"},
		},
		{
			input:    []byte("password=secret123"),
			notContains: []string{"secret123"},
			contains: []string{"password=***"},
		},
		{
			// Very long message
			input:    []byte(strings.Repeat("A", 300)),
			contains: []string{"..."},
		},
	}
	
	for _, test := range tests {
		result := logger.sanitizeRawMessage(test.input)
		
		for _, expected := range test.contains {
			if !strings.Contains(result, expected) {
				t.Errorf("Sanitized message should contain '%s', got: %s", expected, result)
			}
		}
		
		for _, notExpected := range test.notContains {
			if strings.Contains(result, notExpected) {
				t.Errorf("Sanitized message should not contain '%s', got: %s", notExpected, result)
			}
		}
	}
}

func TestDetailedErrorLogger_sanitizeHeaders(t *testing.T) {
	logger := NewDetailedErrorLogger(LogLevelDebug, true)
	
	req := parser.NewRequestMessage(parser.MethodINVITE, "sip:test@example.com")
	req.SetHeader(parser.HeaderVia, "SIP/2.0/UDP client.example.com:5060")
	req.SetHeader(parser.HeaderFrom, "sip:alice@example.com;tag=123")
	req.SetHeader(parser.HeaderTo, "sip:bob@example.com")
	req.SetHeader(parser.HeaderCallID, "test-call-id")
	req.SetHeader(parser.HeaderCSeq, "1 INVITE")
	req.SetHeader(parser.HeaderAuthorization, "Digest response=secret123")
	
	headers := logger.sanitizeHeaders(req)
	
	// Should include standard headers
	expectedHeaders := []string{
		parser.HeaderVia,
		parser.HeaderFrom,
		parser.HeaderTo,
		parser.HeaderCallID,
		parser.HeaderCSeq,
	}
	
	for _, header := range expectedHeaders {
		if _, exists := headers[header]; !exists {
			t.Errorf("Sanitized headers should include %s", header)
		}
	}
	
	// Should not include Authorization header (not in debug list)
	if _, exists := headers[parser.HeaderAuthorization]; exists {
		t.Error("Sanitized headers should not include Authorization header")
	}
	
	// Should sanitize sensitive information
	if authHeader, exists := headers[parser.HeaderAuthorization]; exists {
		if strings.Contains(authHeader, "secret123") {
			t.Error("Authorization header should be sanitized")
		}
	}
}

func TestDetailedErrorLogger_analyzeParseError(t *testing.T) {
	logger := NewDetailedErrorLogger(LogLevelDebug, true)
	
	tests := []struct {
		error       error
		rawMessage  []byte
		expectedKeys []string
	}{
		{
			error:       errors.New("invalid request line"),
			rawMessage:  []byte("INVALID REQUEST\r\nVia: SIP/2.0/UDP test\r\n"),
			expectedKeys: []string{"issue", "expected_format", "actual_first_line"},
		},
		{
			error:       errors.New("invalid status line"),
			rawMessage:  []byte("INVALID STATUS\r\nVia: SIP/2.0/UDP test\r\n"),
			expectedKeys: []string{"issue", "expected_format", "actual_first_line"},
		},
		{
			error:       errors.New("invalid header format"),
			rawMessage:  []byte("INVITE sip:test SIP/2.0\r\nInvalid Header Line\r\n"),
			expectedKeys: []string{"issue", "expected_format", "total_lines"},
		},
		{
			error:       errors.New("content-length mismatch"),
			rawMessage:  []byte("INVITE sip:test SIP/2.0\r\nContent-Length: 10\r\n\r\nshort"),
			expectedKeys: []string{"issue", "suggestion"},
		},
	}
	
	for _, test := range tests {
		analysis := logger.analyzeParseError(test.error, test.rawMessage)
		
		for _, key := range test.expectedKeys {
			if _, exists := analysis[key]; !exists {
				t.Errorf("Analysis should contain key '%s' for error '%s'", key, test.error.Error())
			}
		}
		
		// Should always have general statistics
		generalKeys := []string{"message_size", "line_count", "has_body"}
		for _, key := range generalKeys {
			if _, exists := analysis[key]; !exists {
				t.Errorf("Analysis should always contain key '%s'", key)
			}
		}
	}
}

func TestNewErrorMonitor(t *testing.T) {
	logger := NewDetailedErrorLogger(LogLevelError, false)
	monitor := NewErrorMonitor(logger)
	
	if monitor == nil {
		t.Fatal("NewErrorMonitor should not return nil")
	}
	
	if monitor.logger != logger {
		t.Error("Monitor should store the provided logger")
	}
	
	// Check that default thresholds are set
	counts := monitor.GetErrorCounts()
	if len(counts) == 0 {
		t.Error("Monitor should have initialized counters")
	}
}

func TestErrorMonitor_SetThreshold(t *testing.T) {
	logger := NewDetailedErrorLogger(LogLevelError, false)
	monitor := NewErrorMonitor(logger)
	
	monitor.SetThreshold(ErrorTypeParseError, 5, 2*time.Minute)
	
	// Verify threshold was set by checking if counter exists
	counts := monitor.GetErrorCounts()
	if _, exists := counts[ErrorTypeParseError]; !exists {
		t.Error("Counter should exist after setting threshold")
	}
}

func TestErrorMonitor_RecordError(t *testing.T) {
	logger := NewDetailedErrorLogger(LogLevelError, false)
	monitor := NewErrorMonitor(logger)
	
	// Set a low threshold for testing
	monitor.SetThreshold(ErrorTypeParseError, 2, 1*time.Minute)
	
	// Record first error - should not exceed threshold
	exceeded := monitor.RecordError(ErrorTypeParseError)
	if exceeded {
		t.Error("First error should not exceed threshold")
	}
	
	// Record second error - should exceed threshold
	exceeded = monitor.RecordError(ErrorTypeParseError)
	if !exceeded {
		t.Error("Second error should exceed threshold of 2")
	}
	
	// Check error count
	counts := monitor.GetErrorCounts()
	if counts[ErrorTypeParseError] != 2 {
		t.Errorf("Error count should be 2, got %d", counts[ErrorTypeParseError])
	}
}

func TestErrorMonitor_RecordError_WindowReset(t *testing.T) {
	logger := NewDetailedErrorLogger(LogLevelError, false)
	monitor := NewErrorMonitor(logger)
	
	// Set a very short window for testing
	monitor.SetThreshold(ErrorTypeParseError, 5, 1*time.Millisecond)
	
	// Record an error
	monitor.RecordError(ErrorTypeParseError)
	
	// Wait for window to expire
	time.Sleep(2 * time.Millisecond)
	
	// Record another error - counter should have reset
	monitor.RecordError(ErrorTypeParseError)
	
	counts := monitor.GetErrorCounts()
	if counts[ErrorTypeParseError] != 1 {
		t.Errorf("Error count should be 1 after window reset, got %d", counts[ErrorTypeParseError])
	}
}

func TestErrorMonitor_ResetCounters(t *testing.T) {
	logger := NewDetailedErrorLogger(LogLevelError, false)
	monitor := NewErrorMonitor(logger)
	
	// Record some errors
	monitor.RecordError(ErrorTypeParseError)
	monitor.RecordError(ErrorTypeValidationError)
	
	// Check counts are non-zero
	counts := monitor.GetErrorCounts()
	if counts[ErrorTypeParseError] == 0 || counts[ErrorTypeValidationError] == 0 {
		t.Error("Counts should be non-zero before reset")
	}
	
	// Reset counters
	monitor.ResetCounters()
	
	// Check counts are zero
	counts = monitor.GetErrorCounts()
	for errorType, count := range counts {
		if count != 0 {
			t.Errorf("Count for %s should be 0 after reset, got %d", errorType.String(), count)
		}
	}
}

func TestErrorCounter_WindowExpiration(t *testing.T) {
	counter := &ErrorCounter{
		count:       5,
		windowStart: time.Now().Add(-10 * time.Minute), // Old window
		window:      5 * time.Minute,
	}
	
	now := time.Now()
	
	// Check if window has expired
	if now.Sub(counter.windowStart) <= counter.window {
		t.Error("Window should have expired")
	}
	
	// Simulate window reset
	if now.Sub(counter.windowStart) > counter.window {
		counter.count = 0
		counter.windowStart = now
	}
	
	if counter.count != 0 {
		t.Error("Count should be reset to 0 after window expiration")
	}
}

func TestDetailedErrorLogger_createLogEntry(t *testing.T) {
	logger := NewDetailedErrorLogger(LogLevelInfo, false)
	
	context := map[string]interface{}{
		"test_key": "test_value",
		"number":   42,
	}
	
	entry := logger.createLogEntry(LogLevelError, "Test Category", "Test message", context)
	
	// Check required fields
	requiredFields := []string{"timestamp", "level", "category", "message"}
	for _, field := range requiredFields {
		if _, exists := entry[field]; !exists {
			t.Errorf("Log entry should contain field '%s'", field)
		}
	}
	
	// Check level
	if entry["level"] != "ERROR" {
		t.Errorf("Level should be 'ERROR', got %v", entry["level"])
	}
	
	// Check context was merged
	if entry["test_key"] != "test_value" {
		t.Error("Context should be merged into log entry")
	}
	if entry["number"] != 42 {
		t.Error("Context values should be preserved")
	}
}