package handlers

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/zurustar/xylitol2/internal/parser"
)

// MalformedMessageDetector detects specific types of malformed SIP messages
type MalformedMessageDetector struct {
	errorGenerator *DetailedErrorResponseGenerator
}

// NewMalformedMessageDetector creates a new malformed message detector
func NewMalformedMessageDetector(errorGenerator *DetailedErrorResponseGenerator) *MalformedMessageDetector {
	return &MalformedMessageDetector{
		errorGenerator: errorGenerator,
	}
}

// MalformedMessageError represents a specific type of malformed message error
type MalformedMessageError struct {
	Type        MalformedType
	Description string
	Location    string
	Suggestion  string
	Context     map[string]interface{}
}

// MalformedType represents different types of malformed message errors
type MalformedType int

const (
	MalformedStartLine MalformedType = iota
	MalformedHeader
	MalformedHeaderValue
	MalformedBody
	MalformedLineEnding
	MalformedEncoding
)

// String returns the string representation of MalformedType
func (mt MalformedType) String() string {
	switch mt {
	case MalformedStartLine:
		return "MalformedStartLine"
	case MalformedHeader:
		return "MalformedHeader"
	case MalformedHeaderValue:
		return "MalformedHeaderValue"
	case MalformedBody:
		return "MalformedBody"
	case MalformedLineEnding:
		return "MalformedLineEnding"
	case MalformedEncoding:
		return "MalformedEncoding"
	default:
		return "UnknownMalformed"
	}
}

// DetectMalformedMessage analyzes raw message data to detect specific malformation issues
func (mmd *MalformedMessageDetector) DetectMalformedMessage(rawMessage []byte) []MalformedMessageError {
	var errors []MalformedMessageError
	
	messageStr := string(rawMessage)
	lines := strings.Split(messageStr, "\n")
	
	// Check for proper line endings
	if malformedLineErrors := mmd.checkLineEndings(messageStr); len(malformedLineErrors) > 0 {
		errors = append(errors, malformedLineErrors...)
	}
	
	// Check start line
	if len(lines) > 0 {
		if startLineErrors := mmd.checkStartLine(lines[0]); len(startLineErrors) > 0 {
			errors = append(errors, startLineErrors...)
		}
	}
	
	// Check headers
	headerErrors := mmd.checkHeaders(lines)
	errors = append(errors, headerErrors...)
	
	// Check for encoding issues
	if encodingErrors := mmd.checkEncoding(rawMessage); len(encodingErrors) > 0 {
		errors = append(errors, encodingErrors...)
	}
	
	return errors
}

// checkLineEndings verifies proper CRLF line endings
func (mmd *MalformedMessageDetector) checkLineEndings(message string) []MalformedMessageError {
	var errors []MalformedMessageError
	
	// Check for missing CRLF
	if strings.Contains(message, "\n") && !strings.Contains(message, "\r\n") {
		errors = append(errors, MalformedMessageError{
			Type:        MalformedLineEnding,
			Description: "SIP messages must use CRLF (\\r\\n) line endings",
			Location:    "message line endings",
			Suggestion:  "Replace LF (\\n) with CRLF (\\r\\n) line endings",
			Context: map[string]interface{}{
				"found_lf":   true,
				"found_crlf": false,
			},
		})
	}
	
	// Check for mixed line endings
	if strings.Contains(message, "\r\n") && strings.Contains(message, "\n") {
		lfCount := strings.Count(message, "\n")
		crlfCount := strings.Count(message, "\r\n")
		if lfCount != crlfCount {
			errors = append(errors, MalformedMessageError{
				Type:        MalformedLineEnding,
				Description: "Mixed line endings detected",
				Location:    "message line endings",
				Suggestion:  "Use consistent CRLF (\\r\\n) line endings throughout the message",
				Context: map[string]interface{}{
					"lf_count":   lfCount - crlfCount,
					"crlf_count": crlfCount,
				},
			})
		}
	}
	
	return errors
}

// checkStartLine validates the SIP start line format
func (mmd *MalformedMessageDetector) checkStartLine(startLine string) []MalformedMessageError {
	var errors []MalformedMessageError
	
	startLine = strings.TrimSpace(startLine)
	if startLine == "" {
		errors = append(errors, MalformedMessageError{
			Type:        MalformedStartLine,
			Description: "Empty start line",
			Location:    "first line",
			Suggestion:  "Add proper SIP request line (METHOD sip:uri SIP/2.0) or status line (SIP/2.0 code phrase)",
		})
		return errors
	}
	
	parts := strings.Fields(startLine)
	if len(parts) < 3 {
		errors = append(errors, MalformedMessageError{
			Type:        MalformedStartLine,
			Description: "Start line must have at least 3 parts",
			Location:    "first line",
			Suggestion:  "Format: METHOD sip:uri SIP/2.0 or SIP/2.0 code phrase",
			Context: map[string]interface{}{
				"parts_count": len(parts),
				"start_line":  startLine,
			},
		})
		return errors
	}
	
	// Check if it's a request or response
	if strings.HasPrefix(startLine, "SIP/2.0") {
		// Response line validation
		errors = append(errors, mmd.validateResponseLine(parts)...)
	} else {
		// Request line validation
		errors = append(errors, mmd.validateRequestLine(parts)...)
	}
	
	return errors
}

// validateRequestLine validates SIP request line format
func (mmd *MalformedMessageDetector) validateRequestLine(parts []string) []MalformedMessageError {
	var errors []MalformedMessageError
	
	// Check method
	method := parts[0]
	validMethods := []string{"INVITE", "ACK", "BYE", "CANCEL", "REGISTER", "OPTIONS", "INFO", "PRACK", "UPDATE", "REFER", "NOTIFY", "SUBSCRIBE"}
	isValidMethod := false
	for _, validMethod := range validMethods {
		if method == validMethod {
			isValidMethod = true
			break
		}
	}
	
	if !isValidMethod {
		errors = append(errors, MalformedMessageError{
			Type:        MalformedStartLine,
			Description: fmt.Sprintf("Invalid or unknown SIP method: %s", method),
			Location:    "request method",
			Suggestion:  fmt.Sprintf("Use a valid SIP method: %s", strings.Join(validMethods, ", ")),
			Context: map[string]interface{}{
				"invalid_method": method,
				"valid_methods":  validMethods,
			},
		})
	}
	
	// Check Request-URI
	requestURI := parts[1]
	if !strings.HasPrefix(requestURI, "sip:") && !strings.HasPrefix(requestURI, "sips:") {
		errors = append(errors, MalformedMessageError{
			Type:        MalformedStartLine,
			Description: "Request-URI must be a SIP or SIPS URI",
			Location:    "request URI",
			Suggestion:  "Use format: sip:user@domain or sips:user@domain",
			Context: map[string]interface{}{
				"invalid_uri": requestURI,
			},
		})
	}
	
	// Check SIP version
	if len(parts) >= 3 && parts[2] != "SIP/2.0" {
		errors = append(errors, MalformedMessageError{
			Type:        MalformedStartLine,
			Description: fmt.Sprintf("Invalid SIP version: %s", parts[2]),
			Location:    "SIP version",
			Suggestion:  "Use SIP/2.0 as the protocol version",
			Context: map[string]interface{}{
				"invalid_version": parts[2],
			},
		})
	}
	
	return errors
}

// validateResponseLine validates SIP response line format
func (mmd *MalformedMessageDetector) validateResponseLine(parts []string) []MalformedMessageError {
	var errors []MalformedMessageError
	
	// Check SIP version
	if parts[0] != "SIP/2.0" {
		errors = append(errors, MalformedMessageError{
			Type:        MalformedStartLine,
			Description: fmt.Sprintf("Invalid SIP version in response: %s", parts[0]),
			Location:    "SIP version",
			Suggestion:  "Use SIP/2.0 as the protocol version",
			Context: map[string]interface{}{
				"invalid_version": parts[0],
			},
		})
	}
	
	// Check status code
	if len(parts) >= 2 {
		statusCode, err := strconv.Atoi(parts[1])
		if err != nil || statusCode < 100 || statusCode > 699 {
			errors = append(errors, MalformedMessageError{
				Type:        MalformedStartLine,
				Description: fmt.Sprintf("Invalid status code: %s", parts[1]),
				Location:    "status code",
				Suggestion:  "Use a valid 3-digit status code (100-699)",
				Context: map[string]interface{}{
					"invalid_status_code": parts[1],
				},
			})
		}
	}
	
	return errors
}

// checkHeaders validates SIP header format
func (mmd *MalformedMessageDetector) checkHeaders(lines []string) []MalformedMessageError {
	var errors []MalformedMessageError
	
	headerRegex := regexp.MustCompile(`^([a-zA-Z0-9\-]+):\s*(.*)$`)
	
	for i, line := range lines {
		if i == 0 {
			continue // Skip start line
		}
		
		line = strings.TrimRight(line, "\r")
		if line == "" {
			break // End of headers
		}
		
		// Check for header continuation (folding)
		if strings.HasPrefix(line, " ") || strings.HasPrefix(line, "\t") {
			continue // Header folding is valid
		}
		
		// Check header format
		if !headerRegex.MatchString(line) {
			errors = append(errors, MalformedMessageError{
				Type:        MalformedHeader,
				Description: "Invalid header format",
				Location:    fmt.Sprintf("line %d", i+1),
				Suggestion:  "Use format: Header-Name: header-value",
				Context: map[string]interface{}{
					"line_number":   i + 1,
					"invalid_line":  line,
					"expected_format": "Header-Name: header-value",
				},
			})
			continue
		}
		
		// Extract header name and value
		matches := headerRegex.FindStringSubmatch(line)
		if len(matches) >= 3 {
			headerName := matches[1]
			headerValue := matches[2]
			
			// Validate specific headers
			if headerErrors := mmd.validateSpecificHeader(headerName, headerValue, i+1); len(headerErrors) > 0 {
				errors = append(errors, headerErrors...)
			}
		}
	}
	
	return errors
}

// validateSpecificHeader validates specific header values
func (mmd *MalformedMessageDetector) validateSpecificHeader(name, value string, lineNumber int) []MalformedMessageError {
	var errors []MalformedMessageError
	
	switch strings.ToLower(name) {
	case "content-length":
		if _, err := strconv.Atoi(strings.TrimSpace(value)); err != nil {
			errors = append(errors, MalformedMessageError{
				Type:        MalformedHeaderValue,
				Description: "Content-Length must be a non-negative integer",
				Location:    fmt.Sprintf("line %d", lineNumber),
				Suggestion:  "Use a valid integer value for Content-Length",
				Context: map[string]interface{}{
					"header":       name,
					"invalid_value": value,
				},
			})
		}
		
	case "cseq":
		parts := strings.Fields(value)
		if len(parts) != 2 {
			errors = append(errors, MalformedMessageError{
				Type:        MalformedHeaderValue,
				Description: "CSeq must contain sequence number and method",
				Location:    fmt.Sprintf("line %d", lineNumber),
				Suggestion:  "Format: CSeq: sequence-number METHOD",
				Context: map[string]interface{}{
					"header":       name,
					"invalid_value": value,
				},
			})
		} else {
			if _, err := strconv.Atoi(parts[0]); err != nil {
				errors = append(errors, MalformedMessageError{
					Type:        MalformedHeaderValue,
					Description: "CSeq sequence number must be an integer",
					Location:    fmt.Sprintf("line %d", lineNumber),
					Suggestion:  "Use a valid integer for the sequence number",
					Context: map[string]interface{}{
						"header":           name,
						"invalid_sequence": parts[0],
					},
				})
			}
		}
		
	case "max-forwards":
		if value != "" {
			if _, err := strconv.Atoi(strings.TrimSpace(value)); err != nil {
				errors = append(errors, MalformedMessageError{
					Type:        MalformedHeaderValue,
					Description: "Max-Forwards must be a non-negative integer",
					Location:    fmt.Sprintf("line %d", lineNumber),
					Suggestion:  "Use a valid integer value for Max-Forwards",
					Context: map[string]interface{}{
						"header":       name,
						"invalid_value": value,
					},
				})
			}
		}
		
	case "via":
		if !strings.Contains(value, "SIP/2.0/") {
			errors = append(errors, MalformedMessageError{
				Type:        MalformedHeaderValue,
				Description: "Via header must contain SIP/2.0 protocol version",
				Location:    fmt.Sprintf("line %d", lineNumber),
				Suggestion:  "Format: Via: SIP/2.0/TRANSPORT host:port;branch=z9hG4bK-branch-id",
				Context: map[string]interface{}{
					"header":       name,
					"invalid_value": value,
				},
			})
		}
		
		if !strings.Contains(value, "branch=") {
			errors = append(errors, MalformedMessageError{
				Type:        MalformedHeaderValue,
				Description: "Via header must contain branch parameter",
				Location:    fmt.Sprintf("line %d", lineNumber),
				Suggestion:  "Add branch parameter: ;branch=z9hG4bK-unique-branch-id",
				Context: map[string]interface{}{
					"header":       name,
					"missing_param": "branch",
				},
			})
		}
	}
	
	return errors
}

// checkEncoding checks for encoding issues in the message
func (mmd *MalformedMessageDetector) checkEncoding(rawMessage []byte) []MalformedMessageError {
	var errors []MalformedMessageError
	
	// Check for null bytes
	for i, b := range rawMessage {
		if b == 0 {
			errors = append(errors, MalformedMessageError{
				Type:        MalformedEncoding,
				Description: "Null bytes found in message",
				Location:    fmt.Sprintf("byte position %d", i),
				Suggestion:  "Remove null bytes from the message",
				Context: map[string]interface{}{
					"byte_position": i,
				},
			})
			break
		}
	}
	
	// Check for non-ASCII characters in headers (basic check)
	messageStr := string(rawMessage)
	lines := strings.Split(messageStr, "\n")
	
	for i, line := range lines {
		if line == "" {
			break // End of headers
		}
		
		for j, r := range line {
			if r > 127 {
				errors = append(errors, MalformedMessageError{
					Type:        MalformedEncoding,
					Description: "Non-ASCII characters found in headers",
					Location:    fmt.Sprintf("line %d, position %d", i+1, j+1),
					Suggestion:  "Use only ASCII characters in SIP headers",
					Context: map[string]interface{}{
						"line_number": i + 1,
						"char_position": j + 1,
						"character": string(r),
					},
				})
				break
			}
		}
	}
	
	return errors
}

// GenerateMalformedMessageResponse generates a detailed response for malformed messages
func (mmd *MalformedMessageDetector) GenerateMalformedMessageResponse(malformedErrors []MalformedMessageError, rawMessage []byte) *parser.SIPMessage {
	if len(malformedErrors) == 0 {
		return nil
	}
	
	// Create a comprehensive error description
	var descriptions []string
	var suggestions []string
	
	for _, err := range malformedErrors {
		descriptions = append(descriptions, fmt.Sprintf("%s: %s (at %s)", err.Type.String(), err.Description, err.Location))
		if err.Suggestion != "" {
			suggestions = append(suggestions, err.Suggestion)
		}
	}
	
	errorDetails := fmt.Sprintf("Multiple malformed message issues detected: %s", strings.Join(descriptions, "; "))
	
	details := &DetailedValidationError{
		ValidationError: &ValidationError{
			ValidatorName: "MalformedMessageDetector",
			Code:          parser.StatusBadRequest,
			Reason:        "Bad Request - Malformed Message",
			Details:       errorDetails,
			Suggestions:   suggestions,
			Context: map[string]interface{}{
				"malformed_count": len(malformedErrors),
				"message_length":  len(rawMessage),
			},
		},
		ErrorType: ErrorTypeParseError,
	}
	
	return mmd.errorGenerator.errorHandler.HandleValidationError(details, nil)
}