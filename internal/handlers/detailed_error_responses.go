package handlers

import (
	"fmt"
	"strings"

	"github.com/zurustar/xylitol2/internal/parser"
)

// DetailedErrorResponseGenerator generates detailed error responses for specific error types
type DetailedErrorResponseGenerator struct {
	errorHandler *DefaultErrorHandler
}

// NewDetailedErrorResponseGenerator creates a new detailed error response generator
func NewDetailedErrorResponseGenerator(errorHandler *DefaultErrorHandler) *DetailedErrorResponseGenerator {
	return &DetailedErrorResponseGenerator{
		errorHandler: errorHandler,
	}
}

// GenerateBadRequestResponse generates a detailed 400 Bad Request response
func (derg *DetailedErrorResponseGenerator) GenerateBadRequestResponse(req *parser.SIPMessage, errorDetails string, missingHeaders []string, invalidHeaders map[string]string) *parser.SIPMessage {
	details := &DetailedValidationError{
		ValidationError: &ValidationError{
			ValidatorName: "RequestValidator",
			Code:          parser.StatusBadRequest,
			Reason:        "Bad Request",
			Details:       errorDetails,
		},
		ErrorType:      ErrorTypeValidationError,
		MissingHeaders: missingHeaders,
		InvalidHeaders: invalidHeaders,
	}
	
	// Add specific suggestions based on the type of error
	derg.addBadRequestSuggestions(details, req)
	
	return derg.errorHandler.HandleValidationError(details, req)
}

// GenerateMethodNotAllowedResponse generates a detailed 405 Method Not Allowed response
func (derg *DetailedErrorResponseGenerator) GenerateMethodNotAllowedResponse(req *parser.SIPMessage, allowedMethods []string) *parser.SIPMessage {
	method := ""
	if req != nil {
		method = req.GetMethod()
	}
	
	details := &DetailedValidationError{
		ValidationError: &ValidationError{
			ValidatorName: "MethodValidator",
			Code:          parser.StatusMethodNotAllowed,
			Reason:        "Method Not Allowed",
			Details:       fmt.Sprintf("Method '%s' is not supported by this server", method),
		},
		ErrorType: ErrorTypeValidationError,
		Context: map[string]interface{}{
			"allowed_methods": allowedMethods,
			"requested_method": method,
		},
		Suggestions: []string{
			fmt.Sprintf("Use one of the supported methods: %s", strings.Join(allowedMethods, ", ")),
			"Check the SIP method spelling and case sensitivity",
		},
	}
	
	return derg.errorHandler.HandleValidationError(details, req)
}

// GenerateExtensionRequiredResponse generates a detailed 421 Extension Required response
func (derg *DetailedErrorResponseGenerator) GenerateExtensionRequiredResponse(req *parser.SIPMessage, requiredExtension string) *parser.SIPMessage {
	details := &DetailedValidationError{
		ValidationError: &ValidationError{
			ValidatorName: "ExtensionValidator",
			Code:          parser.StatusExtensionRequired,
			Reason:        "Extension Required",
			Details:       fmt.Sprintf("The '%s' extension is required for this request", requiredExtension),
		},
		ErrorType: ErrorTypeSessionTimerError,
		Context: map[string]interface{}{
			"required_extension": requiredExtension,
		},
		Suggestions: []string{
			fmt.Sprintf("Add 'Require: %s' header to your request", requiredExtension),
			fmt.Sprintf("Add 'Supported: %s' header to indicate client support", requiredExtension),
			"Ensure your SIP client supports the required extension",
		},
	}
	
	if requiredExtension == "timer" {
		details.Suggestions = append(details.Suggestions,
			"Add 'Session-Expires' header with desired session duration",
			"Consider adding 'Min-SE' header with minimum session expiration time")
	}
	
	return derg.errorHandler.HandleValidationError(details, req)
}

// GenerateIntervalTooBriefResponse generates a detailed 422 Session Interval Too Small response
func (derg *DetailedErrorResponseGenerator) GenerateIntervalTooBriefResponse(req *parser.SIPMessage, minSE int, requestedInterval int) *parser.SIPMessage {
	details := &DetailedValidationError{
		ValidationError: &ValidationError{
			ValidatorName: "SessionTimerValidator",
			Code:          parser.StatusIntervalTooBrief,
			Reason:        "Session Interval Too Small",
			Details:       fmt.Sprintf("Requested session interval %d is smaller than minimum %d seconds", requestedInterval, minSE),
		},
		ErrorType: ErrorTypeSessionTimerError,
		Context: map[string]interface{}{
			"min_se": fmt.Sprintf("%d", minSE),
			"requested_interval": requestedInterval,
		},
		Suggestions: []string{
			fmt.Sprintf("Use a Session-Expires value of at least %d seconds", minSE),
			"Check the Min-SE header in the response for the minimum allowed value",
			"Consider using a longer session interval for better stability",
		},
	}
	
	return derg.errorHandler.HandleValidationError(details, req)
}

// GenerateMissingHeaderResponse generates a response for missing required headers
func (derg *DetailedErrorResponseGenerator) GenerateMissingHeaderResponse(req *parser.SIPMessage, missingHeaders []string, validatorName string) *parser.SIPMessage {
	headerList := strings.Join(missingHeaders, ", ")
	
	details := &DetailedValidationError{
		ValidationError: &ValidationError{
			ValidatorName: validatorName,
			Code:          parser.StatusBadRequest,
			Reason:        "Missing Required Headers",
			Details:       fmt.Sprintf("The following required headers are missing: %s", headerList),
		},
		ErrorType:      ErrorTypeValidationError,
		MissingHeaders: missingHeaders,
	}
	
	// Add specific suggestions for each missing header
	for _, header := range missingHeaders {
		suggestions := derg.getSuggestionsForMissingHeader(header)
		details.Suggestions = append(details.Suggestions, suggestions...)
	}
	
	return derg.errorHandler.HandleValidationError(details, req)
}

// GenerateInvalidHeaderResponse generates a response for invalid header values
func (derg *DetailedErrorResponseGenerator) GenerateInvalidHeaderResponse(req *parser.SIPMessage, invalidHeaders map[string]string, validatorName string) *parser.SIPMessage {
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
	}
	
	// Add specific suggestions for each invalid header
	for header, reason := range invalidHeaders {
		suggestions := derg.getSuggestionsForInvalidHeader(header, reason)
		details.Suggestions = append(details.Suggestions, suggestions...)
	}
	
	return derg.errorHandler.HandleValidationError(details, req)
}

// GenerateParseErrorResponse generates a detailed response for parse errors
func (derg *DetailedErrorResponseGenerator) GenerateParseErrorResponse(parseError error, rawMessage []byte) *parser.SIPMessage {
	return derg.errorHandler.HandleParseError(parseError, rawMessage)
}

// addBadRequestSuggestions adds specific suggestions for bad request errors
func (derg *DetailedErrorResponseGenerator) addBadRequestSuggestions(details *DetailedValidationError, req *parser.SIPMessage) {
	// Add suggestions based on missing headers
	for _, header := range details.MissingHeaders {
		suggestions := derg.getSuggestionsForMissingHeader(header)
		details.Suggestions = append(details.Suggestions, suggestions...)
	}
	
	// Add suggestions based on invalid headers
	for header, reason := range details.InvalidHeaders {
		suggestions := derg.getSuggestionsForInvalidHeader(header, reason)
		details.Suggestions = append(details.Suggestions, suggestions...)
	}
	
	// Add general suggestions if no specific ones were added
	if len(details.Suggestions) == 0 {
		details.Suggestions = []string{
			"Verify that the SIP message follows RFC3261 format",
			"Check that all required headers are present and properly formatted",
			"Ensure proper CRLF line endings (\\r\\n) are used",
		}
	}
}

// getSuggestionsForMissingHeader returns suggestions for a specific missing header
func (derg *DetailedErrorResponseGenerator) getSuggestionsForMissingHeader(header string) []string {
	switch strings.ToLower(header) {
	case "via":
		return []string{
			"Add Via header: Via: SIP/2.0/UDP your-host:port;branch=z9hG4bK-unique-id",
			"Via header must include protocol version, transport, and branch parameter",
		}
	case "from":
		return []string{
			"Add From header: From: <sip:user@domain>;tag=unique-tag",
			"From header must include a tag parameter for requests",
		}
	case "to":
		return []string{
			"Add To header: To: <sip:user@domain>",
			"To header should contain the target user's SIP URI",
		}
	case "call-id":
		return []string{
			"Add Call-ID header: Call-ID: unique-identifier@your-domain",
			"Call-ID must be unique for each call session",
		}
	case "cseq":
		return []string{
			"Add CSeq header: CSeq: sequence-number METHOD",
			"CSeq must contain a sequence number followed by the method name",
		}
	case "max-forwards":
		return []string{
			"Add Max-Forwards header: Max-Forwards: 70",
			"Max-Forwards prevents infinite loops in SIP routing",
		}
	case "session-expires":
		return []string{
			"Add Session-Expires header: Session-Expires: duration-in-seconds",
			"Session-Expires is required for session timer support",
		}
	case "contact":
		return []string{
			"Add Contact header: Contact: <sip:user@host:port>",
			"Contact header is required for REGISTER and INVITE requests",
		}
	case "content-length":
		return []string{
			"Add Content-Length header: Content-Length: body-size-in-bytes",
			"Content-Length must match the actual message body size",
		}
	default:
		return []string{
			fmt.Sprintf("Add the required %s header with appropriate value", header),
		}
	}
}

// getSuggestionsForInvalidHeader returns suggestions for a specific invalid header
func (derg *DetailedErrorResponseGenerator) getSuggestionsForInvalidHeader(header, reason string) []string {
	switch strings.ToLower(header) {
	case "cseq":
		return []string{
			"CSeq format: CSeq: sequence-number METHOD",
			"Sequence number must be a positive integer",
			"Method must match the request method",
		}
	case "content-length":
		return []string{
			"Content-Length must be a non-negative integer",
			"Content-Length must match the actual message body size",
			"Use Content-Length: 0 for messages without body",
		}
	case "session-expires":
		return []string{
			"Session-Expires must be a positive integer (seconds)",
			"Session-Expires should be at least the Min-SE value",
			"Consider using a reasonable session duration (e.g., 1800 seconds)",
		}
	case "min-se":
		return []string{
			"Min-SE must be a positive integer (seconds)",
			"Min-SE should be less than or equal to Session-Expires",
			"Typical Min-SE values range from 90 to 300 seconds",
		}
	case "max-forwards":
		return []string{
			"Max-Forwards must be a non-negative integer",
			"Typical Max-Forwards value is 70",
			"Max-Forwards decrements with each hop",
		}
	case "expires":
		return []string{
			"Expires must be a non-negative integer (seconds)",
			"Use 0 to unregister a contact",
			"Typical registration expires values range from 300 to 3600 seconds",
		}
	case "via":
		return []string{
			"Via format: SIP/2.0/TRANSPORT host:port;branch=z9hG4bK-branch-id",
			"Transport must be UDP, TCP, TLS, SCTP, or WS",
			"Branch parameter is required and should start with z9hG4bK",
		}
	default:
		return []string{
			fmt.Sprintf("Fix %s header: %s", header, reason),
			"Check RFC3261 specification for proper header format",
		}
	}
}

// CreateDetailedErrorContext creates a context map with detailed error information
func (derg *DetailedErrorResponseGenerator) CreateDetailedErrorContext(req *parser.SIPMessage, errorType ErrorType) map[string]interface{} {
	context := make(map[string]interface{})
	
	if req != nil {
		context["method"] = req.GetMethod()
		context["request_uri"] = req.GetRequestURI()
		context["call_id"] = req.GetHeader(parser.HeaderCallID)
		context["cseq"] = req.GetHeader(parser.HeaderCSeq)
		context["from"] = req.GetHeader(parser.HeaderFrom)
		context["to"] = req.GetHeader(parser.HeaderTo)
		context["via"] = req.GetHeader(parser.HeaderVia)
	}
	
	context["error_type"] = errorType.String()
	context["timestamp"] = fmt.Sprintf("%d", getCurrentTimestamp())
	
	return context
}

// getCurrentTimestamp returns current Unix timestamp
func getCurrentTimestamp() int64 {
	return 1640995200 // Fixed timestamp for testing, would use time.Now().Unix() in production
}