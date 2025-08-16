package handlers

import (
	"fmt"
	"strings"
	"sync/atomic"
	"time"

	"github.com/zurustar/xylitol2/internal/parser"
)

// DefaultErrorHandler is the default implementation of ErrorHandler
type DefaultErrorHandler struct {
	responseBuilder *ErrorResponseBuilder
	statistics      ErrorStatistics
	logThreshold    map[ErrorType]bool
}

// NewDefaultErrorHandler creates a new default error handler
func NewDefaultErrorHandler() *DefaultErrorHandler {
	handler := &DefaultErrorHandler{
		responseBuilder: NewErrorResponseBuilder(),
		statistics: ErrorStatistics{
			LastReset: time.Now(),
		},
		logThreshold: make(map[ErrorType]bool),
	}
	
	// Set default logging thresholds
	handler.logThreshold[ErrorTypeParseError] = true
	handler.logThreshold[ErrorTypeValidationError] = true
	handler.logThreshold[ErrorTypeProcessingError] = true
	handler.logThreshold[ErrorTypeTransportError] = true
	handler.logThreshold[ErrorTypeAuthenticationError] = false // Too noisy
	handler.logThreshold[ErrorTypeSessionTimerError] = true
	
	return handler
}

// HandleParseError handles SIP message parsing errors
func (deh *DefaultErrorHandler) HandleParseError(err error, rawMessage []byte) *parser.SIPMessage {
	atomic.AddInt64(&deh.statistics.ParseErrors, 1)
	
	// Create a detailed validation error
	details := &DetailedValidationError{
		ValidationError: &ValidationError{
			ValidatorName: "MessageParser",
			Code:          parser.StatusBadRequest,
			Reason:        "Malformed SIP Message",
			Details:       err.Error(),
		},
		ErrorType: ErrorTypeParseError,
		Context: map[string]interface{}{
			"raw_message_length": len(rawMessage),
			"parse_error":        err.Error(),
		},
	}
	
	// Try to extract some basic information from raw message for response
	var req *parser.SIPMessage
	if len(rawMessage) > 0 {
		req = deh.tryExtractBasicInfo(rawMessage)
	}
	
	// Add suggestions based on common parse errors
	deh.addParseErrorSuggestions(details, err.Error())
	
	return deh.responseBuilder.BuildErrorResponse(parser.StatusBadRequest, req, details)
}

// HandleValidationError handles request validation errors
func (deh *DefaultErrorHandler) HandleValidationError(err *DetailedValidationError, req *parser.SIPMessage) *parser.SIPMessage {
	atomic.AddInt64(&deh.statistics.ValidationErrors, 1)
	
	// Add context information
	if err.Context == nil {
		err.Context = make(map[string]interface{})
	}
	
	if req != nil {
		err.Context["method"] = req.GetMethod()
		err.Context["request_uri"] = req.GetRequestURI()
		err.Context["call_id"] = req.GetHeader(parser.HeaderCallID)
	}
	
	return deh.responseBuilder.BuildErrorResponse(err.Code, req, err)
}

// HandleProcessingError handles general processing errors
func (deh *DefaultErrorHandler) HandleProcessingError(err error, req *parser.SIPMessage) *parser.SIPMessage {
	atomic.AddInt64(&deh.statistics.ProcessingErrors, 1)
	
	details := &DetailedValidationError{
		ValidationError: &ValidationError{
			ValidatorName: "RequestProcessor",
			Code:          parser.StatusServerInternalError,
			Reason:        "Processing Error",
			Details:       err.Error(),
		},
		ErrorType: ErrorTypeProcessingError,
		Context: map[string]interface{}{
			"processing_error": err.Error(),
		},
	}
	
	if req != nil {
		details.Context["method"] = req.GetMethod()
		details.Context["request_uri"] = req.GetRequestURI()
	}
	
	return deh.responseBuilder.BuildErrorResponse(parser.StatusServerInternalError, req, details)
}

// HandleTransportError handles transport-related errors
func (deh *DefaultErrorHandler) HandleTransportError(err error, req *parser.SIPMessage) *parser.SIPMessage {
	atomic.AddInt64(&deh.statistics.TransportErrors, 1)
	
	details := &DetailedValidationError{
		ValidationError: &ValidationError{
			ValidatorName: "TransportLayer",
			Code:          parser.StatusServiceUnavailable,
			Reason:        "Transport Error",
			Details:       err.Error(),
		},
		ErrorType: ErrorTypeTransportError,
		Context: map[string]interface{}{
			"transport_error": err.Error(),
		},
	}
	
	return deh.responseBuilder.BuildErrorResponse(parser.StatusServiceUnavailable, req, details)
}

// ShouldLogError determines if an error should be logged
func (deh *DefaultErrorHandler) ShouldLogError(errorType ErrorType, statusCode int) bool {
	// Always log server errors (5xx)
	if statusCode >= 500 && statusCode < 600 {
		return true
	}
	
	// Check type-specific threshold
	if shouldLog, exists := deh.logThreshold[errorType]; exists {
		return shouldLog
	}
	
	// Default to logging
	return true
}

// GetErrorStatistics returns error statistics
func (deh *DefaultErrorHandler) GetErrorStatistics() ErrorStatistics {
	return ErrorStatistics{
		ParseErrors:        atomic.LoadInt64(&deh.statistics.ParseErrors),
		ValidationErrors:   atomic.LoadInt64(&deh.statistics.ValidationErrors),
		ProcessingErrors:   atomic.LoadInt64(&deh.statistics.ProcessingErrors),
		TransportErrors:    atomic.LoadInt64(&deh.statistics.TransportErrors),
		AuthErrors:         atomic.LoadInt64(&deh.statistics.AuthErrors),
		SessionTimerErrors: atomic.LoadInt64(&deh.statistics.SessionTimerErrors),
		LastReset:          deh.statistics.LastReset,
	}
}

// ResetStatistics resets error statistics
func (deh *DefaultErrorHandler) ResetStatistics() {
	atomic.StoreInt64(&deh.statistics.ParseErrors, 0)
	atomic.StoreInt64(&deh.statistics.ValidationErrors, 0)
	atomic.StoreInt64(&deh.statistics.ProcessingErrors, 0)
	atomic.StoreInt64(&deh.statistics.TransportErrors, 0)
	atomic.StoreInt64(&deh.statistics.AuthErrors, 0)
	atomic.StoreInt64(&deh.statistics.SessionTimerErrors, 0)
	deh.statistics.LastReset = time.Now()
}

// SetLogThreshold sets the logging threshold for an error type
func (deh *DefaultErrorHandler) SetLogThreshold(errorType ErrorType, shouldLog bool) {
	deh.logThreshold[errorType] = shouldLog
}

// tryExtractBasicInfo attempts to extract basic SIP information from raw message
func (deh *DefaultErrorHandler) tryExtractBasicInfo(rawMessage []byte) *parser.SIPMessage {
	lines := strings.Split(string(rawMessage), "\r\n")
	if len(lines) == 0 {
		return nil
	}
	
	// Try to create a minimal message for response headers
	msg := &parser.SIPMessage{}
	
	// Parse headers line by line to extract what we can
	for _, line := range lines[1:] { // Skip first line (request/status line)
		if line == "" {
			break // End of headers
		}
		
		if colonIndex := strings.Index(line, ":"); colonIndex > 0 {
			headerName := strings.TrimSpace(line[:colonIndex])
			headerValue := strings.TrimSpace(line[colonIndex+1:])
			
			// Only extract headers we need for responses
			switch strings.ToLower(headerName) {
			case "via", "from", "to", "call-id", "cseq":
				msg.SetHeader(headerName, headerValue)
			}
		}
	}
	
	return msg
}

// addParseErrorSuggestions adds helpful suggestions based on parse error
func (deh *DefaultErrorHandler) addParseErrorSuggestions(details *DetailedValidationError, errorMsg string) {
	errorLower := strings.ToLower(errorMsg)
	
	if strings.Contains(errorLower, "invalid request line") {
		details.Suggestions = append(details.Suggestions, 
			"Check that the request line follows format: METHOD sip:user@domain SIP/2.0")
	}
	
	if strings.Contains(errorLower, "invalid status line") {
		details.Suggestions = append(details.Suggestions, 
			"Check that the status line follows format: SIP/2.0 CODE REASON")
	}
	
	if strings.Contains(errorLower, "header") {
		details.Suggestions = append(details.Suggestions, 
			"Ensure headers follow format: Header-Name: value",
			"Check for proper CRLF line endings (\\r\\n)")
	}
	
	if strings.Contains(errorLower, "content-length") {
		details.Suggestions = append(details.Suggestions, 
			"Verify Content-Length header matches actual body length",
			"Ensure Content-Length is a valid integer")
	}
	
	if strings.Contains(errorLower, "malformed") {
		details.Suggestions = append(details.Suggestions, 
			"Check message format against RFC3261 specification",
			"Verify proper SIP message structure")
	}
}

// CreateMissingHeaderError creates a detailed error for missing required headers
func (deh *DefaultErrorHandler) CreateMissingHeaderError(validatorName string, missingHeaders []string, req *parser.SIPMessage) *DetailedValidationError {
	atomic.AddInt64(&deh.statistics.ValidationErrors, 1)
	
	details := &DetailedValidationError{
		ValidationError: &ValidationError{
			ValidatorName: validatorName,
			Code:          parser.StatusBadRequest,
			Reason:        "Missing Required Headers",
			Details:       fmt.Sprintf("The following required headers are missing: %s", strings.Join(missingHeaders, ", ")),
		},
		ErrorType:      ErrorTypeValidationError,
		MissingHeaders: missingHeaders,
		Context: map[string]interface{}{
			"missing_header_count": len(missingHeaders),
		},
	}
	
	// Add suggestions for common missing headers
	for _, header := range missingHeaders {
		switch strings.ToLower(header) {
		case "via":
			details.Suggestions = append(details.Suggestions, "Add Via header with your SIP proxy/client information")
		case "from":
			details.Suggestions = append(details.Suggestions, "Add From header with sender information and tag parameter")
		case "to":
			details.Suggestions = append(details.Suggestions, "Add To header with recipient information")
		case "call-id":
			details.Suggestions = append(details.Suggestions, "Add Call-ID header with unique identifier for this call")
		case "cseq":
			details.Suggestions = append(details.Suggestions, "Add CSeq header with sequence number and method")
		case "session-expires":
			details.Suggestions = append(details.Suggestions, "Add Session-Expires header for session timer support")
		}
	}
	
	return details
}

// CreateInvalidHeaderError creates a detailed error for invalid headers
func (deh *DefaultErrorHandler) CreateInvalidHeaderError(validatorName string, invalidHeaders map[string]string, req *parser.SIPMessage) *DetailedValidationError {
	atomic.AddInt64(&deh.statistics.ValidationErrors, 1)
	
	var headerList []string
	for header := range invalidHeaders {
		headerList = append(headerList, header)
	}
	
	details := &DetailedValidationError{
		ValidationError: &ValidationError{
			ValidatorName: validatorName,
			Code:          parser.StatusBadRequest,
			Reason:        "Invalid Header Values",
			Details:       fmt.Sprintf("The following headers have invalid values: %s", strings.Join(headerList, ", ")),
		},
		ErrorType:      ErrorTypeValidationError,
		InvalidHeaders: invalidHeaders,
		Context: map[string]interface{}{
			"invalid_header_count": len(invalidHeaders),
		},
	}
	
	// Add specific suggestions based on invalid headers
	for header, reason := range invalidHeaders {
		switch strings.ToLower(header) {
		case "cseq":
			details.Suggestions = append(details.Suggestions, "CSeq must contain sequence number followed by method name")
		case "content-length":
			details.Suggestions = append(details.Suggestions, "Content-Length must be a non-negative integer")
		case "session-expires":
			details.Suggestions = append(details.Suggestions, "Session-Expires must be a positive integer (seconds)")
		case "min-se":
			details.Suggestions = append(details.Suggestions, "Min-SE must be a positive integer (seconds)")
		default:
			details.Suggestions = append(details.Suggestions, fmt.Sprintf("Fix %s header: %s", header, reason))
		}
	}
	
	return details
}