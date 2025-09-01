package handlers

import (
	"bytes"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/zurustar/xylitol2/internal/logging"
	"github.com/zurustar/xylitol2/internal/parser"
)

// TestLogger implements the Logger interface for testing
type TestLogger struct {
	buffer *bytes.Buffer
}

func NewTestLogger() *TestLogger {
	return &TestLogger{
		buffer: &bytes.Buffer{},
	}
}

func (tl *TestLogger) Debug(msg string, fields ...logging.Field) {
	tl.log("DEBUG", msg, fields...)
}

func (tl *TestLogger) Info(msg string, fields ...logging.Field) {
	tl.log("INFO", msg, fields...)
}

func (tl *TestLogger) Warn(msg string, fields ...logging.Field) {
	tl.log("WARN", msg, fields...)
}

func (tl *TestLogger) Error(msg string, fields ...logging.Field) {
	tl.log("ERROR", msg, fields...)
}

func (tl *TestLogger) log(level, msg string, fields ...logging.Field) {
	tl.buffer.WriteString(level + ": " + msg)
	for _, field := range fields {
		tl.buffer.WriteString(" " + field.Key + "=" + fmt.Sprintf("%v", field.Value))
	}
	tl.buffer.WriteString("\n")
}

func (tl *TestLogger) GetOutput() string {
	return tl.buffer.String()
}

func (tl *TestLogger) Reset() {
	tl.buffer.Reset()
}

func TestNewComprehensiveErrorHandler(t *testing.T) {
	logger := NewTestLogger()
	handler := NewComprehensiveErrorHandler(logger)
	
	if handler == nil {
		t.Fatal("Expected non-nil error handler")
	}
	
	if handler.responseBuilder == nil {
		t.Error("Expected response builder to be initialized")
	}
	
	if handler.logger == nil {
		t.Error("Expected logger to be set")
	}
	
	stats := handler.GetErrorStatistics()
	if stats.ParseErrors != 0 {
		t.Error("Expected initial parse errors to be 0")
	}
}

func TestHandleParseError(t *testing.T) {
	logger := NewTestLogger()
	handler := NewComprehensiveErrorHandler(logger)
	
	parseErr := errors.New("invalid SIP message format")
	rawMessage := []byte("INVALID SIP MESSAGE")
	
	response := handler.HandleParseError(parseErr, rawMessage)
	
	if response == nil {
		t.Fatal("Expected non-nil response")
	}
	
	if response.GetStatusCode() != parser.StatusBadRequest {
		t.Errorf("Expected status code %d, got %d", parser.StatusBadRequest, response.GetStatusCode())
	}
	
	// Check that error count was incremented
	stats := handler.GetErrorStatistics()
	if stats.ParseErrors != 1 {
		t.Errorf("Expected parse errors to be 1, got %d", stats.ParseErrors)
	}
}

func TestHandleValidationError(t *testing.T) {
	logger := NewTestLogger()
	handler := NewComprehensiveErrorHandler(logger)
	
	req := parser.NewRequestMessage(parser.MethodINVITE, "sip:test@example.com")
	req.SetHeader(parser.HeaderFrom, "sip:caller@example.com")
	req.SetHeader(parser.HeaderTo, "sip:test@example.com")
	req.SetHeader(parser.HeaderCallID, "test-call-id")
	req.SetHeader(parser.HeaderCSeq, "1 INVITE")
	req.SetHeader(parser.HeaderVia, "SIP/2.0/UDP 192.168.1.1:5060")
	
	validationErr := &DetailedValidationError{
		ValidationError: &ValidationError{
			ValidatorName: "TestValidator",
			Code:          parser.StatusExtensionRequired,
			Reason:        "Extension Required",
			Details:       "Session-Timer extension is required",
		},
		ErrorType: ErrorTypeValidationError,
		MissingHeaders: []string{"Session-Expires"},
	}
	
	response := handler.HandleValidationError(validationErr, req)
	
	if response == nil {
		t.Fatal("Expected non-nil response")
	}
	
	if response.GetStatusCode() != parser.StatusExtensionRequired {
		t.Errorf("Expected status code %d, got %d", parser.StatusExtensionRequired, response.GetStatusCode())
	}
	
	// Check that error count was incremented
	stats := handler.GetErrorStatistics()
	if stats.ValidationErrors != 1 {
		t.Errorf("Expected validation errors to be 1, got %d", stats.ValidationErrors)
	}
}

func TestHandleProcessingError(t *testing.T) {
	logger := NewTestLogger()
	handler := NewComprehensiveErrorHandler(logger)
	
	req := parser.NewRequestMessage(parser.MethodINVITE, "sip:test@example.com")
	processingErr := errors.New("database connection failed")
	
	response := handler.HandleProcessingError(processingErr, req)
	
	if response == nil {
		t.Fatal("Expected non-nil response")
	}
	
	if response.GetStatusCode() != parser.StatusServerInternalError {
		t.Errorf("Expected status code %d, got %d", parser.StatusServerInternalError, response.GetStatusCode())
	}
	
	// Check that error count was incremented
	stats := handler.GetErrorStatistics()
	if stats.ProcessingErrors != 1 {
		t.Errorf("Expected processing errors to be 1, got %d", stats.ProcessingErrors)
	}
}

func TestHandleTransportError(t *testing.T) {
	logger := NewTestLogger()
	handler := NewComprehensiveErrorHandler(logger)
	
	req := parser.NewRequestMessage(parser.MethodINVITE, "sip:test@example.com")
	transportErr := errors.New("connection refused")
	
	response := handler.HandleTransportError(transportErr, req)
	
	if response == nil {
		t.Fatal("Expected non-nil response")
	}
	
	if response.GetStatusCode() != parser.StatusServiceUnavailable {
		t.Errorf("Expected status code %d, got %d", parser.StatusServiceUnavailable, response.GetStatusCode())
	}
	
	// Check that error count was incremented
	stats := handler.GetErrorStatistics()
	if stats.TransportErrors != 1 {
		t.Errorf("Expected transport errors to be 1, got %d", stats.TransportErrors)
	}
}

func TestHandleTimeoutError(t *testing.T) {
	logger := NewTestLogger()
	handler := NewComprehensiveErrorHandler(logger)
	
	req := parser.NewRequestMessage(parser.MethodINVITE, "sip:test@example.com")
	timeoutErr := errors.New("request timeout")
	
	response := handler.HandleTimeoutError(timeoutErr, req)
	
	if response == nil {
		t.Fatal("Expected non-nil response")
	}
	
	if response.GetStatusCode() != parser.StatusRequestTimeout {
		t.Errorf("Expected status code %d, got %d", parser.StatusRequestTimeout, response.GetStatusCode())
	}
}

func TestHandleAuthenticationError(t *testing.T) {
	logger := NewTestLogger()
	handler := NewComprehensiveErrorHandler(logger)
	
	req := parser.NewRequestMessage(parser.MethodINVITE, "sip:test@example.com")
	req.SetHeader(parser.HeaderFrom, "sip:caller@example.com")
	authErr := errors.New("invalid credentials")
	
	response := handler.HandleAuthenticationError(authErr, req)
	
	if response == nil {
		t.Fatal("Expected non-nil response")
	}
	
	if response.GetStatusCode() != parser.StatusUnauthorized {
		t.Errorf("Expected status code %d, got %d", parser.StatusUnauthorized, response.GetStatusCode())
	}
	
	// Check that error count was incremented
	stats := handler.GetErrorStatistics()
	if stats.AuthErrors != 1 {
		t.Errorf("Expected auth errors to be 1, got %d", stats.AuthErrors)
	}
}

func TestHandleSessionTimerError(t *testing.T) {
	logger := NewTestLogger()
	handler := NewComprehensiveErrorHandler(logger)
	
	req := parser.NewRequestMessage(parser.MethodINVITE, "sip:test@example.com")
	
	tests := []struct {
		name           string
		error          error
		expectedStatus int
	}{
		{
			name:           "Extension Required",
			error:          errors.New("session timer not supported"),
			expectedStatus: parser.StatusExtensionRequired,
		},
		{
			name:           "Interval Too Brief",
			error:          errors.New("session interval too small"),
			expectedStatus: parser.StatusIntervalTooBrief,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			response := handler.HandleSessionTimerError(tt.error, req)
			
			if response == nil {
				t.Fatal("Expected non-nil response")
			}
			
			if response.GetStatusCode() != tt.expectedStatus {
				t.Errorf("Expected status code %d, got %d", tt.expectedStatus, response.GetStatusCode())
			}
		})
	}
	
	// Check that error count was incremented
	stats := handler.GetErrorStatistics()
	if stats.SessionTimerErrors != 2 {
		t.Errorf("Expected session timer errors to be 2, got %d", stats.SessionTimerErrors)
	}
}

func TestShouldLogError(t *testing.T) {
	logger := NewTestLogger()
	handler := NewComprehensiveErrorHandler(logger)
	
	tests := []struct {
		errorType    ErrorType
		statusCode   int
		shouldLog    bool
		description  string
	}{
		{ErrorTypeParseError, parser.StatusBadRequest, true, "Parse errors should always be logged"},
		{ErrorTypeValidationError, parser.StatusUnauthorized, false, "Unauthorized validation errors should not be logged"},
		{ErrorTypeValidationError, parser.StatusBadRequest, true, "Other validation errors should be logged"},
		{ErrorTypeProcessingError, parser.StatusServerInternalError, true, "Processing errors should always be logged"},
		{ErrorTypeTransportError, parser.StatusServiceUnavailable, true, "Transport errors should always be logged"},
		{ErrorTypeAuthenticationError, parser.StatusUnauthorized, false, "Unauthorized auth errors should not be logged"},
		{ErrorTypeSessionTimerError, parser.StatusExtensionRequired, true, "Session timer errors should always be logged"},
	}
	
	for _, tt := range tests {
		t.Run(tt.description, func(t *testing.T) {
			result := handler.ShouldLogError(tt.errorType, tt.statusCode)
			if result != tt.shouldLog {
				t.Errorf("Expected ShouldLogError to return %v, got %v", tt.shouldLog, result)
			}
		})
	}
}

func TestErrorStatistics(t *testing.T) {
	logger := NewTestLogger()
	handler := NewComprehensiveErrorHandler(logger)
	
	// Increment various error types
	handler.IncrementErrorCount(ErrorTypeParseError)
	handler.IncrementErrorCount(ErrorTypeValidationError)
	handler.IncrementErrorCount(ErrorTypeValidationError)
	handler.IncrementErrorCount(ErrorTypeProcessingError)
	
	stats := handler.GetErrorStatistics()
	
	if stats.ParseErrors != 1 {
		t.Errorf("Expected parse errors to be 1, got %d", stats.ParseErrors)
	}
	
	if stats.ValidationErrors != 2 {
		t.Errorf("Expected validation errors to be 2, got %d", stats.ValidationErrors)
	}
	
	if stats.ProcessingErrors != 1 {
		t.Errorf("Expected processing errors to be 1, got %d", stats.ProcessingErrors)
	}
	
	// Test reset
	handler.ResetStatistics()
	stats = handler.GetErrorStatistics()
	
	if stats.ParseErrors != 0 {
		t.Errorf("Expected parse errors to be 0 after reset, got %d", stats.ParseErrors)
	}
	
	if stats.ValidationErrors != 0 {
		t.Errorf("Expected validation errors to be 0 after reset, got %d", stats.ValidationErrors)
	}
}

func TestGetMessagePreview(t *testing.T) {
	logger := NewTestLogger()
	handler := NewComprehensiveErrorHandler(logger)
	
	tests := []struct {
		name     string
		input    []byte
		expected string
	}{
		{
			name:     "Empty message",
			input:    []byte{},
			expected: "<empty>",
		},
		{
			name:     "Short message",
			input:    []byte("INVITE sip:test@example.com SIP/2.0"),
			expected: "INVITE sip:test@example.com SIP/2.0",
		},
		{
			name:     "Long message",
			input:    []byte(strings.Repeat("A", 150)),
			expected: strings.Repeat("A", 100) + "...",
		},
		{
			name:     "Message with newlines",
			input:    []byte("INVITE sip:test@example.com SIP/2.0\r\nVia: SIP/2.0/UDP 192.168.1.1:5060"),
			expected: "INVITE sip:test@example.com SIP/2.0 Via: SIP/2.0/UDP 192.168.1.1:5060",
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := handler.getMessagePreview(tt.input)
			if result != tt.expected {
				t.Errorf("Expected preview %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestGetMethodFromRequest(t *testing.T) {
	logger := NewTestLogger()
	handler := NewComprehensiveErrorHandler(logger)
	
	tests := []struct {
		name     string
		req      *parser.SIPMessage
		expected string
	}{
		{
			name:     "Nil request",
			req:      nil,
			expected: "<unknown>",
		},
		{
			name:     "Valid INVITE request",
			req:      parser.NewRequestMessage(parser.MethodINVITE, "sip:test@example.com"),
			expected: parser.MethodINVITE,
		},
		{
			name:     "Valid REGISTER request",
			req:      parser.NewRequestMessage(parser.MethodREGISTER, "sip:test@example.com"),
			expected: parser.MethodREGISTER,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := handler.getMethodFromRequest(tt.req)
			if result != tt.expected {
				t.Errorf("Expected method %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestGetReasonPhrase(t *testing.T) {
	logger := NewTestLogger()
	handler := NewComprehensiveErrorHandler(logger)
	
	tests := []struct {
		statusCode int
		expected   string
	}{
		{parser.StatusBadRequest, "Bad Request"},
		{parser.StatusUnauthorized, "Unauthorized"},
		{parser.StatusNotFound, "Not Found"},
		{parser.StatusMethodNotAllowed, "Method Not Allowed"},
		{parser.StatusRequestTimeout, "Request Timeout"},
		{parser.StatusExtensionRequired, "Extension Required"},
		{parser.StatusIntervalTooBrief, "Session Interval Too Small"},
		{parser.StatusServerInternalError, "Internal Server Error"},
		{parser.StatusServiceUnavailable, "Service Unavailable"},
		{999, "Unknown Error"},
	}
	
	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := handler.getReasonPhrase(tt.statusCode)
			if result != tt.expected {
				t.Errorf("Expected reason phrase %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestValidationErrorInterface(t *testing.T) {
	ve := &ValidationError{
		ValidatorName: "TestValidator",
		Code:          parser.StatusBadRequest,
		Reason:        "Bad Request",
		Details:       "Test details",
	}
	
	expected := "validation failed in TestValidator: Bad Request - Test details"
	if ve.Error() != expected {
		t.Errorf("Expected error string %q, got %q", expected, ve.Error())
	}
	
	// Test without details
	ve.Details = ""
	expected = "validation failed in TestValidator: Bad Request"
	if ve.Error() != expected {
		t.Errorf("Expected error string %q, got %q", expected, ve.Error())
	}
}

func TestDetailedValidationErrorInterface(t *testing.T) {
	dve := &DetailedValidationError{
		ValidationError: &ValidationError{
			ValidatorName: "TestValidator",
			Reason:        "Test error",
		},
		ErrorType:      ErrorTypeValidationError,
		MissingHeaders: []string{"Content-Length", "From"},
		InvalidHeaders: map[string]string{
			"Via": "missing branch parameter",
		},
	}
	
	errorStr := dve.Error()
	
	if !strings.Contains(errorStr, "validation failed in TestValidator: Test error") {
		t.Error("Expected error string to contain validator info")
	}
	
	if !strings.Contains(errorStr, "missing headers: Content-Length, From") {
		t.Error("Expected error string to contain missing headers")
	}
	
	if !strings.Contains(errorStr, "invalid headers: Via (missing branch parameter)") {
		t.Error("Expected error string to contain invalid headers")
	}
}