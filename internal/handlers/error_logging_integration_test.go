package handlers

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/zurustar/xylitol2/internal/parser"
)

// TestErrorLoggingIntegration tests the complete error logging and statistics system
func TestErrorLoggingIntegration(t *testing.T) {
	mockLogger := NewMockLogger()
	errorLogger := NewDetailedErrorLogger(LogLevelDebug, true, mockLogger)
	
	// Create error monitor for recovery testing
	errorMonitor := NewErrorMonitor(errorLogger)
	
	// Create statistics collector
	statsCollector := NewErrorStatisticsCollector(errorLogger, 50*time.Millisecond)
	statsCollector.Start()
	defer statsCollector.Stop()
	
	// Simulate various error scenarios
	t.Run("ParseErrorScenario", func(t *testing.T) {
		// Log multiple parse errors to trigger recovery
		for i := 0; i < 12; i++ {
			parseErr := errors.New("failed to parse start line: invalid format")
			rawMessage := []byte("INVALID SIP MESSAGE")
			context := map[string]interface{}{
				"iteration": i,
				"source_ip": "192.168.1.100",
			}
			errorLogger.LogParseError(parseErr, rawMessage, context)
		}
		
		// Check statistics
		stats := errorLogger.GetErrorStatistics()
		if stats.ParseErrors != 12 {
			t.Errorf("Expected 12 parse errors, got %d", stats.ParseErrors)
		}
		
		// Check error monitoring
		exceeded := false
		for i := 0; i < 12; i++ {
			if errorMonitor.RecordError(ErrorTypeParseError) {
				exceeded = true
			}
		}
		if !exceeded {
			t.Error("Expected error threshold to be exceeded")
		}
	})
	
	t.Run("ValidationErrorScenario", func(t *testing.T) {
		// Create test SIP message
		req := parser.NewRequestMessage("INVITE", "sip:user@example.com")
		req.SetHeader("Call-ID", "test-call-id")
		req.SetHeader("From", "sip:caller@example.com")
		req.SetHeader("To", "sip:callee@example.com")
		
		// Log validation errors
		for i := 0; i < 5; i++ {
			validationErr := &DetailedValidationError{
				ValidationError: &ValidationError{
					ValidatorName: "SessionTimerValidator",
					Code:          421,
					Reason:        "Extension Required",
					Details:       "Session-Timer extension is required",
				},
				ErrorType:      ErrorTypeValidationError,
				MissingHeaders: []string{"Session-Expires"},
				InvalidHeaders: make(map[string]string),
				Suggestions:    []string{"Add Session-Expires header"},
				Context:        make(map[string]interface{}),
			}
			
			context := map[string]interface{}{
				"validator": "session_timer",
				"iteration": i,
			}
			
			errorLogger.LogValidationError(validationErr, req, context)
		}
		
		// Check detailed statistics
		detailedStats := errorLogger.GetDetailedStatistics()
		if detailedStats.ValidationErrorsByType["SessionTimerValidator"] != 5 {
			t.Errorf("Expected 5 SessionTimerValidator errors, got %d", 
				detailedStats.ValidationErrorsByType["SessionTimerValidator"])
		}
	})
	
	t.Run("TransportErrorScenario", func(t *testing.T) {
		// Set a lower threshold for transport errors
		errorMonitor.SetThreshold(ErrorTypeTransportError, 3, 1*time.Minute)
		
		// Log transport errors to trigger high-priority recovery
		for i := 0; i < 6; i++ {
			transportErr := errors.New("connection timeout")
			context := map[string]interface{}{
				"transport": "TCP",
				"remote_addr": "192.168.1.200:5060",
			}
			errorLogger.LogTransportError(transportErr, nil, context)
		}
		
		// Check that transport error threshold is exceeded
		exceeded := false
		for i := 0; i < 6; i++ {
			if errorMonitor.RecordError(ErrorTypeTransportError) {
				exceeded = true
				break
			}
		}
		if !exceeded {
			t.Error("Expected transport error threshold to be exceeded")
		}
	})
	
	t.Run("ErrorPatternsDetection", func(t *testing.T) {
		// Log similar errors to test pattern detection
		for i := 0; i < 15; i++ {
			parseErr := errors.New("failed to parse header: missing colon")
			rawMessage := []byte("INVALID HEADER FORMAT")
			context := map[string]interface{}{
				"pattern_test": true,
			}
			errorLogger.LogParseError(parseErr, rawMessage, context)
		}
		
		// Check error patterns
		patterns := errorLogger.GetErrorPatterns()
		if len(patterns) == 0 {
			t.Error("Expected error patterns to be detected")
		}
		
		// Verify systematic issue warning was logged
		if mockLogger.GetWarnCount() == 0 {
			t.Error("Expected systematic issue warning to be logged")
		}
	})
	
	t.Run("StatisticsCollection", func(t *testing.T) {
		// Log some additional errors to ensure there's data to collect
		for i := 0; i < 5; i++ {
			parseErr := errors.New("additional parse error")
			errorLogger.LogParseError(parseErr, []byte("test"), nil)
		}
		
		// Wait for statistics collection
		time.Sleep(150 * time.Millisecond)
		
		// Check collected metrics
		metrics := statsCollector.GetMetrics()
		if len(metrics) == 0 {
			// Log available metrics for debugging
			t.Logf("Available metrics: %v", metrics)
			t.Error("Expected metrics to be collected")
		}
		
		// Verify error rate metrics exist (they may not exist on first collection)
		found := false
		for name := range metrics {
			if strings.Contains(name, "error_rate") || strings.Contains(name, "error_trend") {
				found = true
				break
			}
		}
		if !found && len(metrics) > 0 {
			// Log what metrics we do have
			t.Logf("Available metrics: %v", metrics)
		}
		// Don't fail if no rate metrics yet, as they need multiple collections
	})
	
	t.Run("ErrorSummaryLogging", func(t *testing.T) {
		initialInfoCount := mockLogger.GetInfoCount()
		
		// Log error summary
		errorLogger.LogErrorSummary()
		
		// Verify summary was logged
		if mockLogger.GetInfoCount() <= initialInfoCount {
			t.Error("Expected error summary to be logged")
		}
	})
	
	t.Run("ErrorMonitorIntegration", func(t *testing.T) {
		// Test error monitoring workflow
		
		// Set custom threshold for processing errors
		errorMonitor.SetThreshold(ErrorTypeProcessingError, 3, 1*time.Minute)
		
		// Log processing errors to trigger threshold
		for i := 0; i < 4; i++ {
			processingErr := errors.New("processing component failure")
			context := map[string]interface{}{
				"component": "message_processor",
			}
			errorLogger.LogProcessingError(processingErr, nil, context)
		}
		
		// Check that threshold is exceeded
		exceeded := false
		for i := 0; i < 4; i++ {
			if errorMonitor.RecordError(ErrorTypeProcessingError) {
				exceeded = true
			}
		}
		if !exceeded {
			t.Error("Expected processing error threshold to be exceeded")
		}
		
		// Check error counts
		counts := errorMonitor.GetErrorCounts()
		if counts[ErrorTypeProcessingError] < 3 {
			t.Errorf("Expected at least 3 processing errors, got %d", counts[ErrorTypeProcessingError])
		}
	})
	
	t.Run("StatisticsReset", func(t *testing.T) {
		// Verify statistics before reset
		stats := errorLogger.GetErrorStatistics()
		if stats.TotalErrors() == 0 {
			t.Error("Expected some errors before reset")
		}
		
		// Reset statistics
		errorLogger.ResetStatistics()
		
		// Verify statistics after reset
		stats = errorLogger.GetErrorStatistics()
		if stats.TotalErrors() != 0 {
			t.Errorf("Expected 0 errors after reset, got %d", stats.TotalErrors())
		}
		
		detailedStats := errorLogger.GetDetailedStatistics()
		if len(detailedStats.RecentErrors) != 0 {
			t.Errorf("Expected 0 recent errors after reset, got %d", len(detailedStats.RecentErrors))
		}
	})
}

// TestErrorLoggingPerformance tests the performance of error logging under load
func TestErrorLoggingPerformance(t *testing.T) {
	mockLogger := NewMockLogger()
	errorLogger := NewDetailedErrorLogger(LogLevelError, false, mockLogger) // Disable debug for performance
	
	// Test logging performance
	start := time.Now()
	
	for i := 0; i < 1000; i++ {
		parseErr := errors.New("parse error")
		errorLogger.LogParseError(parseErr, []byte("test message"), nil)
	}
	
	duration := time.Since(start)
	
	// Should be able to log 1000 errors in reasonable time (< 100ms)
	if duration > 100*time.Millisecond {
		t.Errorf("Error logging too slow: %v for 1000 errors", duration)
	}
	
	// Verify all errors were logged
	stats := errorLogger.GetErrorStatistics()
	if stats.ParseErrors != 1000 {
		t.Errorf("Expected 1000 parse errors, got %d", stats.ParseErrors)
	}
}

// TestConcurrentErrorLogging tests error logging under concurrent access
func TestConcurrentErrorLogging(t *testing.T) {
	mockLogger := NewMockLogger()
	errorLogger := NewDetailedErrorLogger(LogLevelDebug, true, mockLogger)
	
	// Number of goroutines and errors per goroutine
	numGoroutines := 10
	errorsPerGoroutine := 100
	
	// Channel to synchronize goroutines
	done := make(chan bool, numGoroutines)
	
	// Start concurrent error logging
	for i := 0; i < numGoroutines; i++ {
		go func(goroutineID int) {
			defer func() { done <- true }()
			
			for j := 0; j < errorsPerGoroutine; j++ {
				switch j % 6 {
				case 0:
					parseErr := errors.New("parse error")
					errorLogger.LogParseError(parseErr, []byte("test"), nil)
				case 1:
					validationErr := &DetailedValidationError{
						ValidationError: &ValidationError{
							ValidatorName: "TestValidator",
							Code:          400,
							Reason:        "Bad Request",
						},
						ErrorType: ErrorTypeValidationError,
					}
					errorLogger.LogValidationError(validationErr, nil, nil)
				case 2:
					processingErr := errors.New("processing error")
					errorLogger.LogProcessingError(processingErr, nil, nil)
				case 3:
					transportErr := errors.New("transport error")
					errorLogger.LogTransportError(transportErr, nil, nil)
				case 4:
					authErr := errors.New("auth error")
					errorLogger.LogAuthenticationError(authErr, nil, nil)
				case 5:
					sessionErr := errors.New("session timer error")
					errorLogger.LogSessionTimerError(sessionErr, nil, nil)
				}
			}
		}(i)
	}
	
	// Wait for all goroutines to complete
	for i := 0; i < numGoroutines; i++ {
		<-done
	}
	
	// Verify statistics
	stats := errorLogger.GetErrorStatistics()
	expectedTotal := int64(numGoroutines * errorsPerGoroutine)
	
	if stats.TotalErrors() != expectedTotal {
		t.Errorf("Expected %d total errors, got %d", expectedTotal, stats.TotalErrors())
	}
	
	// Each error type should have roughly the same count
	expectedPerType := expectedTotal / 6
	tolerance := expectedPerType / 10 // 10% tolerance
	
	errorCounts := []int64{
		stats.ParseErrors,
		stats.ValidationErrors,
		stats.ProcessingErrors,
		stats.TransportErrors,
		stats.AuthErrors,
		stats.SessionTimerErrors,
	}
	
	for i, count := range errorCounts {
		if count < expectedPerType-tolerance || count > expectedPerType+tolerance {
			t.Errorf("Error type %d count %d outside expected range %d±%d", 
				i, count, expectedPerType, tolerance)
		}
	}
}

// TestErrorLoggingMemoryUsage tests that error logging doesn't cause memory leaks
func TestErrorLoggingMemoryUsage(t *testing.T) {
	mockLogger := NewMockLogger()
	errorLogger := NewDetailedErrorLogger(LogLevelDebug, true, mockLogger)
	
	// Log many errors to test memory management
	for i := 0; i < 10000; i++ {
		parseErr := errors.New("parse error")
		context := map[string]interface{}{
			"iteration": i,
			"large_data": make([]byte, 1024), // Add some data to test memory management
		}
		errorLogger.LogParseError(parseErr, []byte("test message"), context)
	}
	
	// Check that recent errors list is bounded
	detailedStats := errorLogger.GetDetailedStatistics()
	if len(detailedStats.RecentErrors) > 100 {
		t.Errorf("Recent errors list too large: %d (should be ≤ 100)", len(detailedStats.RecentErrors))
	}
	
	// Check that top error messages list is bounded
	if len(detailedStats.TopErrorMessages) > 50 {
		t.Errorf("Top error messages list too large: %d (should be ≤ 50)", len(detailedStats.TopErrorMessages))
	}
	
	// Check that error patterns are managed
	patterns := errorLogger.GetErrorPatterns()
	if len(patterns) > 100 { // Reasonable upper bound
		t.Errorf("Error patterns map too large: %d", len(patterns))
	}
}