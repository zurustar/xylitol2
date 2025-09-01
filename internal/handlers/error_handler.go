package handlers

import (
	"fmt"
	"strings"
	"time"

	"github.com/zurustar/xylitol2/internal/parser"
)

// ErrorType represents different categories of errors
type ErrorType int

const (
	ErrorTypeParseError ErrorType = iota
	ErrorTypeValidationError
	ErrorTypeProcessingError
	ErrorTypeTransportError
	ErrorTypeAuthenticationError
	ErrorTypeSessionTimerError
)

// String returns the string representation of ErrorType
func (et ErrorType) String() string {
	switch et {
	case ErrorTypeParseError:
		return "parse_error"
	case ErrorTypeValidationError:
		return "validation_error"
	case ErrorTypeProcessingError:
		return "processing_error"
	case ErrorTypeTransportError:
		return "transport_error"
	case ErrorTypeAuthenticationError:
		return "authentication_error"
	case ErrorTypeSessionTimerError:
		return "session_timer_error"
	default:
		return "unknown_error"
	}
}

// DetailedValidationError extends ValidationError with more detailed information
type DetailedValidationError struct {
	*ValidationError
	ErrorType     ErrorType
	MissingHeaders []string
	InvalidHeaders map[string]string
	Suggestions   []string
	Context       map[string]interface{}
}



// Error implements the error interface
func (dve *DetailedValidationError) Error() string {
	var parts []string
	parts = append(parts, fmt.Sprintf("validation failed in %s: %s", dve.ValidatorName, dve.Reason))
	
	if len(dve.MissingHeaders) > 0 {
		parts = append(parts, fmt.Sprintf("missing headers: %s", strings.Join(dve.MissingHeaders, ", ")))
	}
	
	if len(dve.InvalidHeaders) > 0 {
		var invalid []string
		for header, reason := range dve.InvalidHeaders {
			invalid = append(invalid, fmt.Sprintf("%s (%s)", header, reason))
		}
		parts = append(parts, fmt.Sprintf("invalid headers: %s", strings.Join(invalid, ", ")))
	}
	
	return strings.Join(parts, "; ")
}

// ErrorHandler defines the interface for handling different types of errors
type ErrorHandler interface {
	// HandleParseError handles SIP message parsing errors
	HandleParseError(err error, rawMessage []byte) *parser.SIPMessage
	
	// HandleValidationError handles request validation errors
	HandleValidationError(err *DetailedValidationError, req *parser.SIPMessage) *parser.SIPMessage
	
	// HandleProcessingError handles general processing errors
	HandleProcessingError(err error, req *parser.SIPMessage) *parser.SIPMessage
	
	// HandleTransportError handles transport-related errors
	HandleTransportError(err error, req *parser.SIPMessage) *parser.SIPMessage
	
	// HandleTimeoutError handles timeout-related errors
	HandleTimeoutError(err error, req *parser.SIPMessage) *parser.SIPMessage
	
	// HandleAuthenticationError handles authentication-related errors
	HandleAuthenticationError(err error, req *parser.SIPMessage) *parser.SIPMessage
	
	// HandleSessionTimerError handles session timer-related errors
	HandleSessionTimerError(err error, req *parser.SIPMessage) *parser.SIPMessage
	
	// ShouldLogError determines if an error should be logged
	ShouldLogError(errorType ErrorType, statusCode int) bool
	
	// GetErrorStatistics returns error statistics
	GetErrorStatistics() ErrorStatistics
	
	// ResetStatistics resets error statistics
	ResetStatistics()
	
	// IncrementErrorCount increments the error count for a specific type
	IncrementErrorCount(errorType ErrorType)
}

// ErrorStatistics holds statistics about errors
type ErrorStatistics struct {
	ParseErrors       int64
	ValidationErrors  int64
	ProcessingErrors  int64
	TransportErrors   int64
	AuthErrors        int64
	SessionTimerErrors int64
	LastReset         time.Time
}

// TotalErrors returns the total number of errors across all types
func (es *ErrorStatistics) TotalErrors() int64 {
	return es.ParseErrors + es.ValidationErrors + es.ProcessingErrors + 
		   es.TransportErrors + es.AuthErrors + es.SessionTimerErrors
}

// ErrorResponseBuilder builds appropriate error responses
type ErrorResponseBuilder struct {
	templates map[int]ResponseTemplate
}

// ResponseTemplate defines a template for error responses
type ResponseTemplate struct {
	StatusCode   int
	ReasonPhrase string
	Headers      map[string]string
	BodyTemplate string
}

// NewErrorResponseBuilder creates a new error response builder
func NewErrorResponseBuilder() *ErrorResponseBuilder {
	builder := &ErrorResponseBuilder{
		templates: make(map[int]ResponseTemplate),
	}
	
	// Initialize default templates
	builder.initializeDefaultTemplates()
	
	return builder
}

// initializeDefaultTemplates sets up default response templates
func (erb *ErrorResponseBuilder) initializeDefaultTemplates() {
	// 400 Bad Request
	erb.templates[parser.StatusBadRequest] = ResponseTemplate{
		StatusCode:   parser.StatusBadRequest,
		ReasonPhrase: "Bad Request",
		Headers: map[string]string{
			parser.HeaderContentType: "text/plain",
		},
		BodyTemplate: "Invalid SIP message format",
	}
	
	// 405 Method Not Allowed
	erb.templates[parser.StatusMethodNotAllowed] = ResponseTemplate{
		StatusCode:   parser.StatusMethodNotAllowed,
		ReasonPhrase: "Method Not Allowed",
		Headers: map[string]string{
			parser.HeaderContentType: "text/plain",
		},
		BodyTemplate: "Method not supported",
	}
	
	// 421 Extension Required
	erb.templates[parser.StatusExtensionRequired] = ResponseTemplate{
		StatusCode:   parser.StatusExtensionRequired,
		ReasonPhrase: "Extension Required",
		Headers: map[string]string{
			parser.HeaderRequire:   "timer",
			parser.HeaderSupported: "timer",
		},
		BodyTemplate: "Session-Timer extension is required",
	}
	
	// 422 Session Interval Too Small
	erb.templates[parser.StatusIntervalTooBrief] = ResponseTemplate{
		StatusCode:   parser.StatusIntervalTooBrief,
		ReasonPhrase: "Session Interval Too Small",
		Headers: map[string]string{
			parser.HeaderContentType: "text/plain",
		},
		BodyTemplate: "Session interval is too small",
	}
	
	// 500 Internal Server Error
	erb.templates[parser.StatusServerInternalError] = ResponseTemplate{
		StatusCode:   parser.StatusServerInternalError,
		ReasonPhrase: "Internal Server Error",
		Headers: map[string]string{
			parser.HeaderContentType: "text/plain",
		},
		BodyTemplate: "Internal server error occurred",
	}
	
	// 503 Service Unavailable
	erb.templates[parser.StatusServiceUnavailable] = ResponseTemplate{
		StatusCode:   parser.StatusServiceUnavailable,
		ReasonPhrase: "Service Unavailable",
		Headers: map[string]string{
			parser.HeaderContentType: "text/plain",
		},
		BodyTemplate: "Service temporarily unavailable",
	}
}

// BuildErrorResponse builds an error response using the appropriate template
func (erb *ErrorResponseBuilder) BuildErrorResponse(statusCode int, req *parser.SIPMessage, details *DetailedValidationError) *parser.SIPMessage {
	template, exists := erb.templates[statusCode]
	if !exists {
		// Fallback to generic 500 error
		template = erb.templates[parser.StatusServerInternalError]
		template.StatusCode = statusCode
		template.ReasonPhrase = "Unknown Error"
	}
	
	response := parser.NewResponseMessage(template.StatusCode, template.ReasonPhrase)
	
	// Copy mandatory headers from request if available
	if req != nil {
		copyResponseHeaders(req, response)
	}
	
	// Add template headers
	for header, value := range template.Headers {
		response.SetHeader(header, value)
	}
	
	// Add specific headers based on error details
	if details != nil {
		erb.addDetailedHeaders(response, details)
	}
	
	// Set body if template has one
	if template.BodyTemplate != "" {
		body := erb.buildErrorBody(template.BodyTemplate, details)
		response.Body = []byte(body)
		response.SetHeader(parser.HeaderContentLength, fmt.Sprintf("%d", len(body)))
	} else {
		response.SetHeader(parser.HeaderContentLength, "0")
	}
	
	return response
}

// addDetailedHeaders adds specific headers based on error details
func (erb *ErrorResponseBuilder) addDetailedHeaders(response *parser.SIPMessage, details *DetailedValidationError) {
	switch details.Code {
	case parser.StatusExtensionRequired:
		response.SetHeader(parser.HeaderRequire, "timer")
		response.SetHeader(parser.HeaderSupported, "timer")
		
	case parser.StatusIntervalTooBrief:
		if minSE, exists := details.Context["min_se"]; exists {
			response.SetHeader(parser.HeaderMinSE, fmt.Sprintf("%v", minSE))
		}
		
	case parser.StatusMethodNotAllowed:
		if allowedMethods, exists := details.Context["allowed_methods"]; exists {
			if methods, ok := allowedMethods.([]string); ok {
				response.SetHeader(parser.HeaderAllow, strings.Join(methods, ", "))
			}
		}
		
	case parser.StatusBadRequest:
		// Add custom header for detailed error information since Warning header is not defined
		if len(details.MissingHeaders) > 0 || len(details.InvalidHeaders) > 0 {
			warning := erb.buildWarningHeader(details)
			response.SetHeader("X-Error-Details", warning)
		}
	}
}

// buildWarningHeader builds a detailed error information string
func (erb *ErrorResponseBuilder) buildWarningHeader(details *DetailedValidationError) string {
	var warnings []string
	
	if len(details.MissingHeaders) > 0 {
		warnings = append(warnings, fmt.Sprintf("Missing required headers: %s", 
			strings.Join(details.MissingHeaders, ", ")))
	}
	
	if len(details.InvalidHeaders) > 0 {
		for header, reason := range details.InvalidHeaders {
			warnings = append(warnings, fmt.Sprintf("Invalid header %s: %s", header, reason))
		}
	}
	
	return strings.Join(warnings, "; ")
}

// buildErrorBody builds the error response body
func (erb *ErrorResponseBuilder) buildErrorBody(template string, details *DetailedValidationError) string {
	if details == nil {
		return template
	}
	
	body := template
	
	// Add detailed information if available
	if details.Details != "" {
		body += "\n" + details.Details
	}
	
	if len(details.Suggestions) > 0 {
		body += "\nSuggestions:\n"
		for _, suggestion := range details.Suggestions {
			body += "- " + suggestion + "\n"
		}
	}
	
	return body
}

// SetTemplate allows customizing response templates
func (erb *ErrorResponseBuilder) SetTemplate(statusCode int, template ResponseTemplate) {
	erb.templates[statusCode] = template
}

// GetTemplate returns the template for a status code
func (erb *ErrorResponseBuilder) GetTemplate(statusCode int) (ResponseTemplate, bool) {
	template, exists := erb.templates[statusCode]
	return template, exists
}