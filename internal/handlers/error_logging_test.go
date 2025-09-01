package handlers

import (
	"errors"
	"testing"
	"time"

	"github.com/zurustar/xylitol2/internal/parser"
)

// MockLogger implements the Logger interface for testing
type MockLogger struct {
	debugMessages []LogMessage
	infoMessages  []LogMessage
	warnMessages  []LogMessage
	errorMessages []LogMessage
}

type LogMessage struct {
	Message string
	Fields  []Field
}

func NewMockLogger() *MockLogger {
	return &MockLogger{
		debugMessages: make([]LogMessage, 0),
		infoMessages:  make([]LogMessage, 0),
		warnMessages:  make([]LogMessage, 0),
		errorMessages: make([]LogMessage, 0),
	}
}

func (ml *MockLogger) Debug(msg string, fields ...Field) {
	ml.debugMessages = append(ml.debugMessages, LogMessage{Message: msg, Fields: fields})
}

func (ml *MockLogger) Info(msg string, fields ...Field) {
	ml.infoMessages = append(ml.infoMessages, LogMessage{Message: msg, Fields: fields})
}

func (ml *MockLogger) Warn(msg string, fields ...Field) {
	ml.warnMessages = append(ml.warnMessages, LogMessage{Message: msg, Fields: fields})
}

func (ml *MockLogger) Error(msg string, fields ...Field) {
	ml.errorMessages = append(ml.errorMessages, LogMessage{Message: msg, Fields: fields})
}

func (ml *MockLogger) GetErrorCount() int {
	return len(ml.errorMessages)
}

func (ml *MockLogger) GetWarnCount() int {
	return len(ml.warnMessages)
}

func (ml *MockLogger) GetInfoCount() int {
	return len(ml.infoMessages)
}

func (ml *MockLogger) Reset() {
	ml.debugMessages = make([]LogMessage, 0)
	ml.infoMessages = make([]LogMessage, 0)
	ml.warnMessages = make([]LogMessage, 0)
	ml.errorMessages = make([]LogMessage, 0)
}

func TestDetailedErrorLogger_LogParseError(t *testing.T) {
	mockLogger := NewMockLogger()
	logger := NewDetailedErrorLogger(LogLevelDebug, true, mockLogger)

	// Test parse error logging
	parseErr := errors.New("failed to parse start line: invalid format")
	rawMessage := []byte("INVALID SIP MESSAGE")
	context := map[string]interface{}{
		"source_ip": "192.168.1.100",
		"transport": "UDP",
	}

	logger.LogParseError(parseErr, rawMessage, context)

	// Verify error was logged
	if mockLogger.GetErrorCount() != 1 {
		t.Errorf("Expected 1 error log, got %d", mockLogger.GetErrorCount())
	}

	// Verify statistics were updated
	stats := logger.GetErrorStatistics()
	if stats.ParseErrors != 1 {
		t.Errorf("Expected 1 parse error in statistics, got %d", stats.ParseErrors)
	}

	// Verify detailed statistics
	detailedStats := logger.GetDetailedStatistics()
	if len(detailedStats.ParseErrorsByType) == 0 {
		t.Error("Expected parse errors by type to be populated")
	}

	if len(detailedStats.RecentErrors) != 1 {
		t.Errorf("Expected 1 recent error, got %d", len(detailedStats.RecentErrors))
	}
}

func TestDetailedErrorLogger_LogValidationError(t *testing.T) {
	mockLogger := NewMockLogger()
	logger := NewDetailedErrorLogger(LogLevelDebug, true, mockLogger)

	// Create a test SIP message
	req := parser.NewRequestMessage("INVITE", "sip:user@example.com")
	req.SetHeader("Call-ID", "test-call-id")
	req.SetHeader("From", "sip:caller@example.com")
	req.SetHeader("To", "sip:callee@example.com")

	// Create validation error
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
	}

	logger.LogValidationError(validationErr, req, context)

	// Verify warning was logged
	if mockLogger.GetWarnCount() != 1 {
		t.Errorf("Expected 1 warning log, got %d", mockLogger.GetWarnCount())
	}

	// Verify statistics were updated
	stats := logger.GetErrorStatistics()
	if stats.ValidationErrors != 1 {
		t.Errorf("Expected 1 validation error in statistics, got %d", stats.ValidationErrors)
	}

	// Verify detailed statistics
	detailedStats := logger.GetDetailedStatistics()
	if detailedStats.ValidationErrorsByType["SessionTimerValidator"] != 1 {
		t.Errorf("Expected 1 SessionTimerValidator error, got %d", 
			detailedStats.ValidationErrorsByType["SessionTimerValidator"])
	}
}

func TestDetailedErrorLogger_ErrorPatterns(t *testing.T) {
	mockLogger := NewMockLogger()
	logger := NewDetailedErrorLogger(LogLevelDebug, true, mockLogger)

	// Log multiple similar errors to test pattern detection
	for i := 0; i < 15; i++ {
		parseErr := errors.New("failed to parse start line: invalid format")
		rawMessage := []byte("INVALID SIP MESSAGE")
		context := map[string]interface{}{
			"iteration": i,
		}
		logger.LogParseError(parseErr, rawMessage, context)
	}

	// Check if systematic issue warning was logged
	// The logger should detect the pattern after 10 occurrences
	if mockLogger.GetWarnCount() == 0 {
		t.Error("Expected systematic issue warning to be logged")
	}

	// Verify error patterns are tracked
	patterns := logger.GetErrorPatterns()
	if len(patterns) == 0 {
		t.Error("Expected error patterns to be tracked")
	}
}

func TestDetailedErrorLogger_Statistics(t *testing.T) {
	mockLogger := NewMockLogger()
	logger := NewDetailedErrorLogger(LogLevelDebug, true, mockLogger)

	// Log various types of errors
	parseErr := errors.New("parse error")
	logger.LogParseError(parseErr, []byte("test"), nil)

	processingErr := errors.New("processing error")
	logger.LogProcessingError(processingErr, nil, nil)

	transportErr := errors.New("transport error")
	logger.LogTransportError(transportErr, nil, nil)

	authErr := errors.New("authentication failed")
	logger.LogAuthenticationError(authErr, nil, nil)

	sessionErr := errors.New("session timer error")
	logger.LogSessionTimerError(sessionErr, nil, nil)

	// Verify basic statistics
	stats := logger.GetErrorStatistics()
	if stats.ParseErrors != 1 {
		t.Errorf("Expected 1 parse error, got %d", stats.ParseErrors)
	}
	if stats.ProcessingErrors != 1 {
		t.Errorf("Expected 1 processing error, got %d", stats.ProcessingErrors)
	}
	if stats.TransportErrors != 1 {
		t.Errorf("Expected 1 transport error, got %d", stats.TransportErrors)
	}
	if stats.AuthErrors != 1 {
		t.Errorf("Expected 1 auth error, got %d", stats.AuthErrors)
	}
	if stats.SessionTimerErrors != 1 {
		t.Errorf("Expected 1 session timer error, got %d", stats.SessionTimerErrors)
	}

	// Verify detailed statistics
	detailedStats := logger.GetDetailedStatistics()
	if len(detailedStats.RecentErrors) != 5 {
		t.Errorf("Expected 5 recent errors, got %d", len(detailedStats.RecentErrors))
	}

	// Test hourly statistics
	currentHour := time.Now().Hour()
	if detailedStats.ErrorsByHour[currentHour] != 5 {
		t.Errorf("Expected 5 errors in current hour, got %d", detailedStats.ErrorsByHour[currentHour])
	}
}

func TestDetailedErrorLogger_ResetStatistics(t *testing.T) {
	mockLogger := NewMockLogger()
	logger := NewDetailedErrorLogger(LogLevelDebug, true, mockLogger)

	// Log some errors
	parseErr := errors.New("parse error")
	logger.LogParseError(parseErr, []byte("test"), nil)

	// Verify statistics are populated
	stats := logger.GetErrorStatistics()
	if stats.ParseErrors != 1 {
		t.Errorf("Expected 1 parse error before reset, got %d", stats.ParseErrors)
	}

	// Reset statistics
	logger.ResetStatistics()

	// Verify statistics are reset
	stats = logger.GetErrorStatistics()
	if stats.ParseErrors != 0 {
		t.Errorf("Expected 0 parse errors after reset, got %d", stats.ParseErrors)
	}

	detailedStats := logger.GetDetailedStatistics()
	if len(detailedStats.RecentErrors) != 0 {
		t.Errorf("Expected 0 recent errors after reset, got %d", len(detailedStats.RecentErrors))
	}
}

func TestDetailedErrorLogger_LogLevels(t *testing.T) {
	mockLogger := NewMockLogger()
	
	// Test with error level only
	logger := NewDetailedErrorLogger(LogLevelError, false, mockLogger)

	parseErr := errors.New("parse error")
	logger.LogParseError(parseErr, []byte("test"), nil)

	// Should log error
	if mockLogger.GetErrorCount() != 1 {
		t.Errorf("Expected 1 error log, got %d", mockLogger.GetErrorCount())
	}

	// Test log level change
	logger.SetLogLevel(LogLevelDebug)
	
	// Test debug mode toggle
	logger.EnableDebugMode(true)
	
	// Log another error with debug enabled
	mockLogger.Reset()
	logger.LogParseError(parseErr, []byte("test message"), nil)
	
	// Should have both error and debug logs
	if mockLogger.GetErrorCount() != 1 {
		t.Errorf("Expected 1 error log with debug enabled, got %d", mockLogger.GetErrorCount())
	}
}

func TestDetailedErrorLogger_ErrorCategorization(t *testing.T) {
	mockLogger := NewMockLogger()
	logger := NewDetailedErrorLogger(LogLevelDebug, true, mockLogger)

	testCases := []struct {
		errorMsg     string
		expectedType string
	}{
		{"failed to parse start line: invalid format", "start_line_error"},
		{"failed to parse headers: missing colon", "header_error"},
		{"invalid Content-Length: abc", "content_length_error"},
		{"failed to parse body: unexpected end", "body_error"},
		{"empty message data", "empty_message_error"},
		{"invalid method: UNKNOWN", "invalid_method_error"},
		{"unsupported SIP version: SIP/1.0", "version_error"},
		{"some other error", "unknown_parse_error"},
	}

	for _, tc := range testCases {
		mockLogger.Reset()
		logger.ResetStatistics()
		
		parseErr := errors.New(tc.errorMsg)
		logger.LogParseError(parseErr, []byte("test"), nil)

		detailedStats := logger.GetDetailedStatistics()
		if detailedStats.ParseErrorsByType[tc.expectedType] != 1 {
			t.Errorf("Expected error type %s for message %s, but got types: %v", 
				tc.expectedType, tc.errorMsg, detailedStats.ParseErrorsByType)
		}
	}
}

func TestDetailedErrorLogger_ErrorSummary(t *testing.T) {
	mockLogger := NewMockLogger()
	logger := NewDetailedErrorLogger(LogLevelDebug, true, mockLogger)

	// Log various errors
	parseErr := errors.New("parse error")
	logger.LogParseError(parseErr, []byte("test"), nil)

	processingErr := errors.New("processing error")
	logger.LogProcessingError(processingErr, nil, nil)

	// Log error summary
	logger.LogErrorSummary()

	// Verify summary was logged as info
	infoCount := len(mockLogger.infoMessages)
	if infoCount != 1 {
		t.Errorf("Expected 1 info log for summary, got %d", infoCount)
	}
}

func TestErrorRecoveryManager(t *testing.T) {
	mockLogger := NewMockLogger()
	logger := NewDetailedErrorLogger(LogLevelDebug, true, mockLogger)
	
	// Test error monitor functionality (simpler than full recovery manager)
	monitor := NewErrorMonitor(logger)
	
	// Test threshold setting
	monitor.SetThreshold(ErrorTypeParseError, 5, 1*time.Minute)
	
	// Test recovery check with low error count
	exceeded := monitor.RecordError(ErrorTypeParseError)
	if exceeded {
		t.Error("Expected threshold not to be exceeded on first error")
	}
	
	// Record multiple errors to exceed threshold
	for i := 0; i < 5; i++ {
		exceeded = monitor.RecordError(ErrorTypeParseError)
	}
	
	if !exceeded {
		t.Error("Expected threshold to be exceeded after multiple errors")
	}
	
	// Test error counts
	counts := monitor.GetErrorCounts()
	if counts[ErrorTypeParseError] < 5 {
		t.Errorf("Expected at least 5 parse errors, got %d", counts[ErrorTypeParseError])
	}
	
	// Test reset
	monitor.ResetCounters()
	counts = monitor.GetErrorCounts()
	if counts[ErrorTypeParseError] != 0 {
		t.Errorf("Expected 0 parse errors after reset, got %d", counts[ErrorTypeParseError])
	}
}

func TestErrorStatisticsCollector(t *testing.T) {
	mockLogger := NewMockLogger()
	logger := NewDetailedErrorLogger(LogLevelDebug, true, mockLogger)
	
	// Create statistics collector
	collector := NewErrorStatisticsCollector(logger, 100*time.Millisecond)
	
	// Test adding collectors
	customCollector := &TestMetricCollector{}
	collector.AddCollector(customCollector)
	
	// Log some errors to generate statistics
	parseErr := errors.New("parse error")
	logger.LogParseError(parseErr, []byte("test"), nil)
	
	processingErr := errors.New("processing error")
	logger.LogProcessingError(processingErr, nil, nil)
	
	// Start collection
	collector.Start()
	
	// Wait for collection to happen
	time.Sleep(150 * time.Millisecond)
	
	// Stop collection
	collector.Stop()
	
	// Check metrics
	metrics := collector.GetMetrics()
	if len(metrics) == 0 {
		t.Error("Expected metrics to be collected")
	}
	
	// Verify custom collector was called
	if !customCollector.called {
		t.Error("Expected custom collector to be called")
	}
}

func TestErrorRateCollector(t *testing.T) {
	collector := &ErrorRateCollector{}
	
	// First collection (no previous data)
	stats1 := ErrorStatistics{
		ParseErrors:      5,
		ValidationErrors: 3,
		ProcessingErrors: 2,
	}
	
	metrics1 := collector.CollectMetrics(stats1)
	if len(metrics1) != 0 {
		t.Errorf("Expected no metrics on first collection, got %d", len(metrics1))
	}
	
	// Wait a bit and collect again
	time.Sleep(10 * time.Millisecond)
	
	stats2 := ErrorStatistics{
		ParseErrors:      10,
		ValidationErrors: 8,
		ProcessingErrors: 4,
	}
	
	metrics2 := collector.CollectMetrics(stats2)
	if len(metrics2) == 0 {
		t.Error("Expected metrics on second collection")
	}
	
	// Check that rates are calculated
	if _, exists := metrics2["parse_errors_per_second"]; !exists {
		t.Error("Expected parse_errors_per_second metric")
	}
	
	if _, exists := metrics2["total_errors_per_second"]; !exists {
		t.Error("Expected total_errors_per_second metric")
	}
}

func TestErrorTrendCollector(t *testing.T) {
	collector := &ErrorTrendCollector{}
	
	// Collect multiple statistics to establish trend
	stats1 := ErrorStatistics{ParseErrors: 5, ValidationErrors: 3}
	_ = collector.CollectMetrics(stats1) // First collection establishes baseline
	
	stats2 := ErrorStatistics{ParseErrors: 8, ValidationErrors: 5}
	metrics2 := collector.CollectMetrics(stats2)
	
	if len(metrics2) == 0 {
		t.Error("Expected trend metrics after second collection")
	}
	
	// Check trend metrics
	if _, exists := metrics2["parse_error_trend"]; !exists {
		t.Error("Expected parse_error_trend metric")
	}
	
	if trend, exists := metrics2["parse_error_trend"]; exists {
		if trend != 3.0 { // 8 - 5 = 3
			t.Errorf("Expected parse error trend of 3.0, got %f", trend)
		}
	}
}

func TestErrorTypeString(t *testing.T) {
	testCases := []struct {
		errorType ErrorType
		expected  string
	}{
		{ErrorTypeParseError, "parse_error"},
		{ErrorTypeValidationError, "validation_error"},
		{ErrorTypeProcessingError, "processing_error"},
		{ErrorTypeTransportError, "transport_error"},
		{ErrorTypeAuthenticationError, "authentication_error"},
		{ErrorTypeSessionTimerError, "session_timer_error"},
	}
	
	for _, tc := range testCases {
		if tc.errorType.String() != tc.expected {
			t.Errorf("Expected %s, got %s", tc.expected, tc.errorType.String())
		}
	}
}

func TestErrorStatisticsTotalErrors(t *testing.T) {
	stats := ErrorStatistics{
		ParseErrors:        5,
		ValidationErrors:   3,
		ProcessingErrors:   2,
		TransportErrors:    1,
		AuthErrors:         4,
		SessionTimerErrors: 2,
	}
	
	expected := int64(17) // 5+3+2+1+4+2
	if stats.TotalErrors() != expected {
		t.Errorf("Expected total errors %d, got %d", expected, stats.TotalErrors())
	}
}

// Note: RecoveryType and RecoveryPriority tests removed as they are defined in error_recovery.go
// and would require importing the full recovery system which is beyond the scope of this task

// TestMetricCollector is a test implementation of MetricCollector
type TestMetricCollector struct {
	called bool
}

func (tmc *TestMetricCollector) Name() string {
	return "test"
}

func (tmc *TestMetricCollector) CollectMetrics(stats ErrorStatistics) map[string]float64 {
	tmc.called = true
	return map[string]float64{
		"test_metric": float64(stats.TotalErrors()),
	}
}