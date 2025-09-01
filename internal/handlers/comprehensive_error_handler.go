package handlers

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/zurustar/xylitol2/internal/logging"
	"github.com/zurustar/xylitol2/internal/parser"
)

// ComprehensiveErrorHandler implements the ErrorHandler interface with full functionality
type ComprehensiveErrorHandler struct {
	responseBuilder *ErrorResponseBuilder
	logger          logging.Logger
	statistics      ErrorStatistics
	statsMutex      sync.RWMutex
}

// NewComprehensiveErrorHandler creates a new comprehensive error handler
func NewComprehensiveErrorHandler(logger logging.Logger) *ComprehensiveErrorHandler {
	return &ComprehensiveErrorHandler{
		responseBuilder: NewErrorResponseBuilder(),
		logger:          logger,
		statistics: ErrorStatistics{
			LastReset: time.Now(),
		},
	}
}

// HandleParseError handles SIP message parsing errors
func (ceh *ComprehensiveErrorHandler) HandleParseError(err error, rawMessage []byte) *parser.SIPMessage {
	ceh.IncrementErrorCount(ErrorTypeParseError)
	
	// Log the parse error with context
	ceh.logger.Error("SIP message parse error",
		logging.StringField("error", err.Error()),
		logging.IntField("message_length", len(rawMessage)),
		logging.StringField("raw_message_preview", ceh.getMessagePreview(rawMessage)),
	)
	
	// Create a generic 400 Bad Request response
	details := &DetailedValidationError{
		ValidationError: &ValidationError{
			ValidatorName: "MessageParser",
			Code:          parser.StatusBadRequest,
			Reason:        "Bad Request",
			Details:       fmt.Sprintf("Failed to parse SIP message: %s", err.Error()),
			Suggestions:   []string{"Check message format according to RFC3261"},
			Context: map[string]interface{}{
				"parse_error":      err.Error(),
				"message_length":   len(rawMessage),
				"timestamp":        time.Now(),
			},
		},
		ErrorType: ErrorTypeParseError,
	}
	
	return ceh.responseBuilder.BuildErrorResponse(parser.StatusBadRequest, nil, details)
}

// HandleValidationError handles request validation errors
func (ceh *ComprehensiveErrorHandler) HandleValidationError(err *DetailedValidationError, req *parser.SIPMessage) *parser.SIPMessage {
	ceh.IncrementErrorCount(ErrorTypeValidationError)
	
	// Log the validation error with detailed context
	ceh.logger.Warn("SIP request validation failed",
		logging.StringField("validator", err.ValidatorName),
		logging.IntField("status_code", err.Code),
		logging.StringField("reason", err.Reason),
		logging.StringField("details", err.Details),
		logging.StringField("missing_headers", strings.Join(err.MissingHeaders, ", ")),
		logging.StringField("method", ceh.getMethodFromRequest(req)),
		logging.StringField("request_uri", ceh.getRequestURIFromRequest(req)),
	)
	
	return ceh.responseBuilder.BuildErrorResponse(err.Code, req, err)
}

// HandleProcessingError handles general processing errors
func (ceh *ComprehensiveErrorHandler) HandleProcessingError(err error, req *parser.SIPMessage) *parser.SIPMessage {
	ceh.IncrementErrorCount(ErrorTypeProcessingError)
	
	// Log the processing error
	ceh.logger.Error("SIP request processing error",
		logging.StringField("error", err.Error()),
		logging.StringField("method", ceh.getMethodFromRequest(req)),
		logging.StringField("request_uri", ceh.getRequestURIFromRequest(req)),
	)
	
	// Create a 500 Internal Server Error response
	details := &DetailedValidationError{
		ValidationError: &ValidationError{
			ValidatorName: "RequestProcessor",
			Code:          parser.StatusServerInternalError,
			Reason:        "Internal Server Error",
			Details:       "An internal error occurred while processing the request",
			Context: map[string]interface{}{
				"processing_error": err.Error(),
				"timestamp":        time.Now(),
			},
		},
		ErrorType: ErrorTypeProcessingError,
	}
	
	return ceh.responseBuilder.BuildErrorResponse(parser.StatusServerInternalError, req, details)
}

// HandleTransportError handles transport-related errors
func (ceh *ComprehensiveErrorHandler) HandleTransportError(err error, req *parser.SIPMessage) *parser.SIPMessage {
	ceh.IncrementErrorCount(ErrorTypeTransportError)
	
	// Log the transport error
	ceh.logger.Error("SIP transport error",
		logging.StringField("error", err.Error()),
		logging.StringField("method", ceh.getMethodFromRequest(req)),
	)
	
	// Create a 503 Service Unavailable response
	details := &DetailedValidationError{
		ValidationError: &ValidationError{
			ValidatorName: "TransportLayer",
			Code:          parser.StatusServiceUnavailable,
			Reason:        "Service Unavailable",
			Details:       "Transport layer error occurred",
			Context: map[string]interface{}{
				"transport_error": err.Error(),
				"timestamp":       time.Now(),
			},
		},
		ErrorType: ErrorTypeTransportError,
	}
	
	return ceh.responseBuilder.BuildErrorResponse(parser.StatusServiceUnavailable, req, details)
}

// HandleTimeoutError handles timeout-related errors
func (ceh *ComprehensiveErrorHandler) HandleTimeoutError(err error, req *parser.SIPMessage) *parser.SIPMessage {
	ceh.IncrementErrorCount(ErrorTypeTransportError) // Timeouts are a subset of transport errors
	
	// Log the timeout error
	ceh.logger.Warn("SIP request timeout",
		logging.StringField("error", err.Error()),
		logging.StringField("method", ceh.getMethodFromRequest(req)),
	)
	
	// Create a 408 Request Timeout response
	details := &DetailedValidationError{
		ValidationError: &ValidationError{
			ValidatorName: "TimeoutHandler",
			Code:          parser.StatusRequestTimeout,
			Reason:        "Request Timeout",
			Details:       "Request processing timed out",
			Context: map[string]interface{}{
				"timeout_error": err.Error(),
				"timestamp":     time.Now(),
			},
		},
		ErrorType: ErrorTypeTransportError,
	}
	
	return ceh.responseBuilder.BuildErrorResponse(parser.StatusRequestTimeout, req, details)
}

// HandleAuthenticationError handles authentication-related errors
func (ceh *ComprehensiveErrorHandler) HandleAuthenticationError(err error, req *parser.SIPMessage) *parser.SIPMessage {
	ceh.IncrementErrorCount(ErrorTypeAuthenticationError)
	
	// Log the authentication error
	ceh.logger.Info("SIP authentication failed",
		logging.StringField("error", err.Error()),
		logging.StringField("method", ceh.getMethodFromRequest(req)),
		logging.StringField("from", req.GetHeader(parser.HeaderFrom)),
	)
	
	// Create a 401 Unauthorized response
	details := &DetailedValidationError{
		ValidationError: &ValidationError{
			ValidatorName: "AuthenticationHandler",
			Code:          parser.StatusUnauthorized,
			Reason:        "Unauthorized",
			Details:       "Authentication required",
			Context: map[string]interface{}{
				"auth_error": err.Error(),
				"timestamp":  time.Now(),
			},
		},
		ErrorType: ErrorTypeAuthenticationError,
	}
	
	return ceh.responseBuilder.BuildErrorResponse(parser.StatusUnauthorized, req, details)
}

// HandleSessionTimerError handles session timer-related errors
func (ceh *ComprehensiveErrorHandler) HandleSessionTimerError(err error, req *parser.SIPMessage) *parser.SIPMessage {
	ceh.IncrementErrorCount(ErrorTypeSessionTimerError)
	
	// Log the session timer error
	ceh.logger.Warn("SIP session timer error",
		logging.StringField("error", err.Error()),
		logging.StringField("method", ceh.getMethodFromRequest(req)),
	)
	
	// Determine appropriate response code based on error
	statusCode := parser.StatusExtensionRequired
	if strings.Contains(err.Error(), "too small") || strings.Contains(err.Error(), "brief") {
		statusCode = parser.StatusIntervalTooBrief
	}
	
	details := &DetailedValidationError{
		ValidationError: &ValidationError{
			ValidatorName: "SessionTimerHandler",
			Code:          statusCode,
			Reason:        ceh.getReasonPhrase(statusCode),
			Details:       err.Error(),
			Context: map[string]interface{}{
				"session_timer_error": err.Error(),
				"timestamp":           time.Now(),
			},
		},
		ErrorType: ErrorTypeSessionTimerError,
	}
	
	return ceh.responseBuilder.BuildErrorResponse(statusCode, req, details)
}

// ShouldLogError determines if an error should be logged based on type and status code
func (ceh *ComprehensiveErrorHandler) ShouldLogError(errorType ErrorType, statusCode int) bool {
	switch errorType {
	case ErrorTypeParseError:
		return true // Always log parse errors
	case ErrorTypeValidationError:
		// Log validation errors except for common client errors
		return statusCode != parser.StatusUnauthorized
	case ErrorTypeProcessingError:
		return true // Always log processing errors
	case ErrorTypeTransportError:
		return true // Always log transport errors
	case ErrorTypeAuthenticationError:
		// Only log authentication errors at INFO level for security monitoring
		return statusCode != parser.StatusUnauthorized
	case ErrorTypeSessionTimerError:
		return true // Always log session timer errors
	default:
		return true
	}
}

// GetErrorStatistics returns current error statistics
func (ceh *ComprehensiveErrorHandler) GetErrorStatistics() ErrorStatistics {
	ceh.statsMutex.RLock()
	defer ceh.statsMutex.RUnlock()
	
	return ceh.statistics
}

// ResetStatistics resets all error statistics
func (ceh *ComprehensiveErrorHandler) ResetStatistics() {
	ceh.statsMutex.Lock()
	defer ceh.statsMutex.Unlock()
	
	ceh.statistics = ErrorStatistics{
		LastReset: time.Now(),
	}
}

// IncrementErrorCount increments the error count for a specific type
func (ceh *ComprehensiveErrorHandler) IncrementErrorCount(errorType ErrorType) {
	ceh.statsMutex.Lock()
	defer ceh.statsMutex.Unlock()
	
	switch errorType {
	case ErrorTypeParseError:
		ceh.statistics.ParseErrors++
	case ErrorTypeValidationError:
		ceh.statistics.ValidationErrors++
	case ErrorTypeProcessingError:
		ceh.statistics.ProcessingErrors++
	case ErrorTypeTransportError:
		ceh.statistics.TransportErrors++
	case ErrorTypeAuthenticationError:
		ceh.statistics.AuthErrors++
	case ErrorTypeSessionTimerError:
		ceh.statistics.SessionTimerErrors++
	}
}

// Helper methods

// getMessagePreview returns a safe preview of the raw message for logging
func (ceh *ComprehensiveErrorHandler) getMessagePreview(rawMessage []byte) string {
	const maxPreviewLength = 100
	
	if len(rawMessage) == 0 {
		return "<empty>"
	}
	
	preview := string(rawMessage)
	if len(preview) > maxPreviewLength {
		preview = preview[:maxPreviewLength] + "..."
	}
	
	// Replace newlines with spaces for single-line logging
	preview = strings.ReplaceAll(preview, "\r\n", " ")
	preview = strings.ReplaceAll(preview, "\n", " ")
	
	return preview
}

// getMethodFromRequest safely extracts the method from a SIP request
func (ceh *ComprehensiveErrorHandler) getMethodFromRequest(req *parser.SIPMessage) string {
	if req == nil {
		return "<unknown>"
	}
	
	if req.IsRequest() {
		return req.GetMethod()
	}
	
	return "<unknown>"
}

// getRequestURIFromRequest safely extracts the request URI from a SIP request
func (ceh *ComprehensiveErrorHandler) getRequestURIFromRequest(req *parser.SIPMessage) string {
	if req == nil {
		return "<unknown>"
	}
	
	if req.IsRequest() {
		return req.GetRequestURI()
	}
	
	return "<unknown>"
}

// getReasonPhrase returns the standard reason phrase for a status code
func (ceh *ComprehensiveErrorHandler) getReasonPhrase(statusCode int) string {
	switch statusCode {
	case parser.StatusBadRequest:
		return "Bad Request"
	case parser.StatusUnauthorized:
		return "Unauthorized"
	case parser.StatusNotFound:
		return "Not Found"
	case parser.StatusMethodNotAllowed:
		return "Method Not Allowed"
	case parser.StatusRequestTimeout:
		return "Request Timeout"
	case parser.StatusExtensionRequired:
		return "Extension Required"
	case parser.StatusIntervalTooBrief:
		return "Session Interval Too Small"
	case parser.StatusServerInternalError:
		return "Internal Server Error"
	case parser.StatusServiceUnavailable:
		return "Service Unavailable"
	default:
		return "Unknown Error"
	}
}

// SetResponseBuilder allows customizing the response builder
func (ceh *ComprehensiveErrorHandler) SetResponseBuilder(builder *ErrorResponseBuilder) {
	ceh.responseBuilder = builder
}

// GetResponseBuilder returns the current response builder
func (ceh *ComprehensiveErrorHandler) GetResponseBuilder() *ErrorResponseBuilder {
	return ceh.responseBuilder
}