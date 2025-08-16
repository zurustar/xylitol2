package handlers

import (
	"errors"
	"strings"
	"testing"

	"github.com/zurustar/xylitol2/internal/parser"
)

func TestErrorType_String(t *testing.T) {
	tests := []struct {
		errorType ErrorType
		expected  string
	}{
		{ErrorTypeParseError, "ParseError"},
		{ErrorTypeValidationError, "ValidationError"},
		{ErrorTypeProcessingError, "ProcessingError"},
		{ErrorTypeTransportError, "TransportError"},
		{ErrorTypeAuthenticationError, "AuthenticationError"},
		{ErrorTypeSessionTimerError, "SessionTimerError"},
		{ErrorType(999), "UnknownError"},
	}
	
	for _, test := range tests {
		result := test.errorType.String()
		if result != test.expected {
			t.Errorf("ErrorType.String() = %s, expected %s", result, test.expected)
		}
	}
}

func TestDetailedValidationError_Error(t *testing.T) {
	err := &DetailedValidationError{
		ValidationError: &ValidationError{
			ValidatorName: "TestValidator",
			Reason:        "Test error",
		},
		MissingHeaders: []string{"Via", "From"},
		InvalidHeaders: map[string]string{
			"CSeq":           "invalid format",
			"Content-Length": "not a number",
		},
	}
	
	result := err.Error()
	
	// Check that all components are included
	if !strings.Contains(result, "TestValidator") {
		t.Error("Error message should contain validator name")
	}
	if !strings.Contains(result, "Test error") {
		t.Error("Error message should contain reason")
	}
	if !strings.Contains(result, "Via, From") {
		t.Error("Error message should contain missing headers")
	}
	if !strings.Contains(result, "CSeq") || !strings.Contains(result, "Content-Length") {
		t.Error("Error message should contain invalid headers")
	}
}

func TestNewErrorResponseBuilder(t *testing.T) {
	builder := NewErrorResponseBuilder()
	
	if builder == nil {
		t.Fatal("NewErrorResponseBuilder should not return nil")
	}
	
	if builder.templates == nil {
		t.Error("Builder should have initialized templates")
	}
	
	// Check that default templates are initialized
	expectedCodes := []int{
		parser.StatusBadRequest,
		parser.StatusMethodNotAllowed,
		parser.StatusExtensionRequired,
		parser.StatusIntervalTooBrief,
		parser.StatusServerInternalError,
		parser.StatusServiceUnavailable,
	}
	
	for _, code := range expectedCodes {
		if _, exists := builder.templates[code]; !exists {
			t.Errorf("Default template for status code %d should be initialized", code)
		}
	}
}

func TestErrorResponseBuilder_BuildErrorResponse(t *testing.T) {
	builder := NewErrorResponseBuilder()
	
	// Create a test request
	req := parser.NewRequestMessage(parser.MethodINVITE, "sip:test@example.com")
	req.SetHeader(parser.HeaderVia, "SIP/2.0/UDP client.example.com:5060")
	req.SetHeader(parser.HeaderFrom, "sip:alice@example.com;tag=123")
	req.SetHeader(parser.HeaderTo, "sip:bob@example.com")
	req.SetHeader(parser.HeaderCallID, "test-call-id")
	req.SetHeader(parser.HeaderCSeq, "1 INVITE")
	
	// Create error details
	details := &DetailedValidationError{
		ValidationError: &ValidationError{
			ValidatorName: "TestValidator",
			Code:          parser.StatusBadRequest,
			Reason:        "Test Error",
			Details:       "Detailed error information",
		},
		MissingHeaders: []string{"Session-Expires"},
		Suggestions:    []string{"Add Session-Expires header"},
	}
	
	response := builder.BuildErrorResponse(parser.StatusBadRequest, req, details)
	
	// Verify response structure
	if response.GetStatusCode() != parser.StatusBadRequest {
		t.Errorf("Response status code = %d, expected %d", response.GetStatusCode(), parser.StatusBadRequest)
	}
	
	// Verify mandatory headers are copied
	if response.GetHeader(parser.HeaderVia) != req.GetHeader(parser.HeaderVia) {
		t.Error("Via header should be copied from request")
	}
	if response.GetHeader(parser.HeaderFrom) != req.GetHeader(parser.HeaderFrom) {
		t.Error("From header should be copied from request")
	}
	if response.GetHeader(parser.HeaderCallID) != req.GetHeader(parser.HeaderCallID) {
		t.Error("Call-ID header should be copied from request")
	}
	
	// Verify Content-Length is set
	if response.GetHeader(parser.HeaderContentLength) == "" {
		t.Error("Content-Length header should be set")
	}
}

func TestErrorResponseBuilder_BuildErrorResponse_WithoutRequest(t *testing.T) {
	builder := NewErrorResponseBuilder()
	
	response := builder.BuildErrorResponse(parser.StatusBadRequest, nil, nil)
	
	if response.GetStatusCode() != parser.StatusBadRequest {
		t.Errorf("Response status code = %d, expected %d", response.GetStatusCode(), parser.StatusBadRequest)
	}
	
	// Should still have Content-Length
	if response.GetHeader(parser.HeaderContentLength) == "" {
		t.Error("Content-Length header should be set even without request")
	}
}

func TestErrorResponseBuilder_BuildErrorResponse_ExtensionRequired(t *testing.T) {
	builder := NewErrorResponseBuilder()
	
	req := parser.NewRequestMessage(parser.MethodINVITE, "sip:test@example.com")
	details := &DetailedValidationError{
		ValidationError: &ValidationError{
			Code: parser.StatusExtensionRequired,
		},
	}
	
	response := builder.BuildErrorResponse(parser.StatusExtensionRequired, req, details)
	
	// Should have Require and Supported headers
	if response.GetHeader(parser.HeaderRequire) != "timer" {
		t.Error("Response should have Require: timer header")
	}
	if response.GetHeader(parser.HeaderSupported) != "timer" {
		t.Error("Response should have Supported: timer header")
	}
}

func TestErrorResponseBuilder_BuildErrorResponse_IntervalTooBrief(t *testing.T) {
	builder := NewErrorResponseBuilder()
	
	req := parser.NewRequestMessage(parser.MethodINVITE, "sip:test@example.com")
	details := &DetailedValidationError{
		ValidationError: &ValidationError{
			Code: parser.StatusIntervalTooBrief,
		},
		Context: map[string]interface{}{
			"min_se": "90",
		},
	}
	
	response := builder.BuildErrorResponse(parser.StatusIntervalTooBrief, req, details)
	
	// Should have Min-SE header
	if response.GetHeader(parser.HeaderMinSE) != "90" {
		t.Errorf("Response should have Min-SE: 90 header, got %s", response.GetHeader(parser.HeaderMinSE))
	}
}

func TestErrorResponseBuilder_BuildErrorResponse_MethodNotAllowed(t *testing.T) {
	builder := NewErrorResponseBuilder()
	
	req := parser.NewRequestMessage("UNKNOWN", "sip:test@example.com")
	details := &DetailedValidationError{
		ValidationError: &ValidationError{
			Code: parser.StatusMethodNotAllowed,
		},
		Context: map[string]interface{}{
			"allowed_methods": []string{"INVITE", "REGISTER", "OPTIONS"},
		},
	}
	
	response := builder.BuildErrorResponse(parser.StatusMethodNotAllowed, req, details)
	
	// Should have Allow header
	allowHeader := response.GetHeader(parser.HeaderAllow)
	if !strings.Contains(allowHeader, "INVITE") || !strings.Contains(allowHeader, "REGISTER") {
		t.Errorf("Response should have Allow header with supported methods, got %s", allowHeader)
	}
}

func TestNewDefaultErrorHandler(t *testing.T) {
	handler := NewDefaultErrorHandler()
	
	if handler == nil {
		t.Fatal("NewDefaultErrorHandler should not return nil")
	}
	
	if handler.responseBuilder == nil {
		t.Error("Handler should have response builder")
	}
	
	if handler.logThreshold == nil {
		t.Error("Handler should have log threshold map")
	}
	
	// Check default log thresholds
	if !handler.logThreshold[ErrorTypeParseError] {
		t.Error("Parse errors should be logged by default")
	}
	if handler.logThreshold[ErrorTypeAuthenticationError] {
		t.Error("Authentication errors should not be logged by default (too noisy)")
	}
}

func TestDefaultErrorHandler_HandleParseError(t *testing.T) {
	handler := NewDefaultErrorHandler()
	
	parseErr := errors.New("invalid request line")
	rawMessage := []byte("INVALID SIP MESSAGE")
	
	response := handler.HandleParseError(parseErr, rawMessage)
	
	if response.GetStatusCode() != parser.StatusBadRequest {
		t.Errorf("Parse error should result in 400 Bad Request, got %d", response.GetStatusCode())
	}
	
	// Check statistics
	stats := handler.GetErrorStatistics()
	if stats.ParseErrors != 1 {
		t.Errorf("Parse error count should be 1, got %d", stats.ParseErrors)
	}
}

func TestDefaultErrorHandler_HandleValidationError(t *testing.T) {
	handler := NewDefaultErrorHandler()
	
	req := parser.NewRequestMessage(parser.MethodINVITE, "sip:test@example.com")
	req.SetHeader(parser.HeaderCallID, "test-call-id")
	
	validationErr := &DetailedValidationError{
		ValidationError: &ValidationError{
			ValidatorName: "TestValidator",
			Code:          parser.StatusExtensionRequired,
			Reason:        "Session-Timer required",
		},
		ErrorType: ErrorTypeSessionTimerError,
	}
	
	response := handler.HandleValidationError(validationErr, req)
	
	if response.GetStatusCode() != parser.StatusExtensionRequired {
		t.Errorf("Validation error should preserve status code, got %d", response.GetStatusCode())
	}
	
	// Check that context was added
	if validationErr.Context["method"] != parser.MethodINVITE {
		t.Error("Context should include method from request")
	}
	if validationErr.Context["call_id"] != "test-call-id" {
		t.Error("Context should include call-id from request")
	}
	
	// Check statistics
	stats := handler.GetErrorStatistics()
	if stats.ValidationErrors != 1 {
		t.Errorf("Validation error count should be 1, got %d", stats.ValidationErrors)
	}
}

func TestDefaultErrorHandler_HandleProcessingError(t *testing.T) {
	handler := NewDefaultErrorHandler()
	
	req := parser.NewRequestMessage(parser.MethodINVITE, "sip:test@example.com")
	processingErr := errors.New("database connection failed")
	
	response := handler.HandleProcessingError(processingErr, req)
	
	if response.GetStatusCode() != parser.StatusServerInternalError {
		t.Errorf("Processing error should result in 500 Internal Server Error, got %d", response.GetStatusCode())
	}
	
	// Check statistics
	stats := handler.GetErrorStatistics()
	if stats.ProcessingErrors != 1 {
		t.Errorf("Processing error count should be 1, got %d", stats.ProcessingErrors)
	}
}

func TestDefaultErrorHandler_HandleTransportError(t *testing.T) {
	handler := NewDefaultErrorHandler()
	
	req := parser.NewRequestMessage(parser.MethodINVITE, "sip:test@example.com")
	transportErr := errors.New("connection timeout")
	
	response := handler.HandleTransportError(transportErr, req)
	
	if response.GetStatusCode() != parser.StatusServiceUnavailable {
		t.Errorf("Transport error should result in 503 Service Unavailable, got %d", response.GetStatusCode())
	}
	
	// Check statistics
	stats := handler.GetErrorStatistics()
	if stats.TransportErrors != 1 {
		t.Errorf("Transport error count should be 1, got %d", stats.TransportErrors)
	}
}

func TestDefaultErrorHandler_ShouldLogError(t *testing.T) {
	handler := NewDefaultErrorHandler()
	
	// Server errors should always be logged
	if !handler.ShouldLogError(ErrorTypeProcessingError, 500) {
		t.Error("Server errors (5xx) should always be logged")
	}
	
	// Check type-specific thresholds
	if !handler.ShouldLogError(ErrorTypeParseError, 400) {
		t.Error("Parse errors should be logged by default")
	}
	
	if handler.ShouldLogError(ErrorTypeAuthenticationError, 401) {
		t.Error("Authentication errors should not be logged by default")
	}
	
	// Test setting threshold
	handler.SetLogThreshold(ErrorTypeAuthenticationError, true)
	if !handler.ShouldLogError(ErrorTypeAuthenticationError, 401) {
		t.Error("Authentication errors should be logged after setting threshold")
	}
}

func TestDefaultErrorHandler_CreateMissingHeaderError(t *testing.T) {
	handler := NewDefaultErrorHandler()
	
	req := parser.NewRequestMessage(parser.MethodINVITE, "sip:test@example.com")
	missingHeaders := []string{"Via", "From", "Session-Expires"}
	
	err := handler.CreateMissingHeaderError("TestValidator", missingHeaders, req)
	
	if err.Code != parser.StatusBadRequest {
		t.Errorf("Missing header error should be 400 Bad Request, got %d", err.Code)
	}
	
	if len(err.MissingHeaders) != 3 {
		t.Errorf("Should have 3 missing headers, got %d", len(err.MissingHeaders))
	}
	
	if len(err.Suggestions) == 0 {
		t.Error("Should have suggestions for missing headers")
	}
	
	// Check that suggestions are relevant
	suggestionText := strings.Join(err.Suggestions, " ")
	if !strings.Contains(suggestionText, "Via") {
		t.Error("Should have suggestion for Via header")
	}
	if !strings.Contains(suggestionText, "Session-Expires") {
		t.Error("Should have suggestion for Session-Expires header")
	}
}

func TestDefaultErrorHandler_CreateInvalidHeaderError(t *testing.T) {
	handler := NewDefaultErrorHandler()
	
	req := parser.NewRequestMessage(parser.MethodINVITE, "sip:test@example.com")
	invalidHeaders := map[string]string{
		"CSeq":           "invalid format",
		"Content-Length": "not a number",
		"Session-Expires": "negative value",
	}
	
	err := handler.CreateInvalidHeaderError("TestValidator", invalidHeaders, req)
	
	if err.Code != parser.StatusBadRequest {
		t.Errorf("Invalid header error should be 400 Bad Request, got %d", err.Code)
	}
	
	if len(err.InvalidHeaders) != 3 {
		t.Errorf("Should have 3 invalid headers, got %d", len(err.InvalidHeaders))
	}
	
	if len(err.Suggestions) == 0 {
		t.Error("Should have suggestions for invalid headers")
	}
	
	// Check that suggestions are relevant
	suggestionText := strings.Join(err.Suggestions, " ")
	if !strings.Contains(suggestionText, "CSeq") {
		t.Error("Should have suggestion for CSeq header")
	}
	if !strings.Contains(suggestionText, "Content-Length") {
		t.Error("Should have suggestion for Content-Length header")
	}
}

func TestDefaultErrorHandler_ResetStatistics(t *testing.T) {
	handler := NewDefaultErrorHandler()
	
	// Generate some errors
	handler.HandleParseError(errors.New("test"), []byte("test"))
	handler.HandleProcessingError(errors.New("test"), nil)
	
	// Check statistics are non-zero
	stats := handler.GetErrorStatistics()
	if stats.ParseErrors == 0 || stats.ProcessingErrors == 0 {
		t.Error("Statistics should be non-zero before reset")
	}
	
	// Reset statistics
	handler.ResetStatistics()
	
	// Check statistics are zero
	stats = handler.GetErrorStatistics()
	if stats.ParseErrors != 0 || stats.ProcessingErrors != 0 {
		t.Error("Statistics should be zero after reset")
	}
}

func TestDefaultErrorHandler_tryExtractBasicInfo(t *testing.T) {
	handler := NewDefaultErrorHandler()
	
	rawMessage := []byte("INVITE sip:test@example.com SIP/2.0\r\n" +
		"Via: SIP/2.0/UDP client.example.com:5060\r\n" +
		"From: sip:alice@example.com;tag=123\r\n" +
		"To: sip:bob@example.com\r\n" +
		"Call-ID: test-call-id\r\n" +
		"CSeq: 1 INVITE\r\n" +
		"\r\n")
	
	msg := handler.tryExtractBasicInfo(rawMessage)
	
	if msg == nil {
		t.Fatal("Should extract basic info from valid message")
	}
	
	if msg.GetHeader("Via") == "" {
		t.Error("Should extract Via header")
	}
	if msg.GetHeader("From") == "" {
		t.Error("Should extract From header")
	}
	if msg.GetHeader("Call-ID") == "" {
		t.Error("Should extract Call-ID header")
	}
}

func TestDefaultErrorHandler_tryExtractBasicInfo_EmptyMessage(t *testing.T) {
	handler := NewDefaultErrorHandler()
	
	msg := handler.tryExtractBasicInfo([]byte(""))
	
	// For empty message, it should return a message with no headers
	if msg == nil {
		t.Error("Should return empty message for empty input, not nil")
	}
	
	// Should have no headers extracted
	if msg.GetHeader("Via") != "" {
		t.Error("Empty message should not have Via header")
	}
}