package handlers

import (
	"strings"
	"testing"

	"github.com/zurustar/xylitol2/internal/parser"
)

func TestNewMalformedMessageDetector(t *testing.T) {
	errorHandler := NewDefaultErrorHandler()
	errorGenerator := NewDetailedErrorResponseGenerator(errorHandler)
	detector := NewMalformedMessageDetector(errorGenerator)
	
	if detector == nil {
		t.Fatal("Expected non-nil malformed message detector")
	}
	
	if detector.errorGenerator == nil {
		t.Error("Expected error generator to be set")
	}
}

func TestMalformedType_String(t *testing.T) {
	tests := []struct {
		malformedType MalformedType
		expected      string
	}{
		{MalformedStartLine, "MalformedStartLine"},
		{MalformedHeader, "MalformedHeader"},
		{MalformedHeaderValue, "MalformedHeaderValue"},
		{MalformedBody, "MalformedBody"},
		{MalformedLineEnding, "MalformedLineEnding"},
		{MalformedEncoding, "MalformedEncoding"},
		{MalformedType(999), "UnknownMalformed"},
	}
	
	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := tt.malformedType.String()
			if result != tt.expected {
				t.Errorf("Expected %s, got %s", tt.expected, result)
			}
		})
	}
}

func TestDetectMalformedMessage_LineEndings(t *testing.T) {
	errorHandler := NewDefaultErrorHandler()
	errorGenerator := NewDetailedErrorResponseGenerator(errorHandler)
	detector := NewMalformedMessageDetector(errorGenerator)
	
	tests := []struct {
		name          string
		message       string
		expectErrors  bool
		errorType     MalformedType
	}{
		{
			name:         "Valid CRLF endings",
			message:      "INVITE sip:test@example.com SIP/2.0\r\nVia: SIP/2.0/UDP host:5060;branch=z9hG4bK123\r\n\r\n",
			expectErrors: false,
		},
		{
			name:         "Invalid LF only endings",
			message:      "INVITE sip:test@example.com SIP/2.0\nVia: SIP/2.0/UDP host:5060\n\n",
			expectErrors: true,
			errorType:    MalformedLineEnding,
		},
		{
			name:         "Mixed line endings",
			message:      "INVITE sip:test@example.com SIP/2.0\r\nVia: SIP/2.0/UDP host:5060\n\r\n",
			expectErrors: true,
			errorType:    MalformedLineEnding,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errors := detector.DetectMalformedMessage([]byte(tt.message))
			
			if tt.expectErrors {
				if len(errors) == 0 {
					t.Error("Expected malformed message errors, got none")
				} else {
					found := false
					for _, err := range errors {
						if err.Type == tt.errorType {
							found = true
							break
						}
					}
					if !found {
						t.Errorf("Expected error type %s, but not found", tt.errorType.String())
					}
				}
			} else {
				if len(errors) > 0 {
					t.Errorf("Expected no errors, got %d errors: %v", len(errors), errors)
				}
			}
		})
	}
}

func TestDetectMalformedMessage_StartLine(t *testing.T) {
	errorHandler := NewDefaultErrorHandler()
	errorGenerator := NewDetailedErrorResponseGenerator(errorHandler)
	detector := NewMalformedMessageDetector(errorGenerator)
	
	tests := []struct {
		name         string
		message      string
		expectErrors bool
		description  string
	}{
		{
			name:         "Valid INVITE request",
			message:      "INVITE sip:test@example.com SIP/2.0\r\n\r\n",
			expectErrors: false,
		},
		{
			name:         "Valid response",
			message:      "SIP/2.0 200 OK\r\n\r\n",
			expectErrors: false,
		},
		{
			name:         "Empty start line",
			message:      "\r\n\r\n",
			expectErrors: true,
			description:  "Empty start line",
		},
		{
			name:         "Invalid method",
			message:      "INVALID sip:test@example.com SIP/2.0\r\n\r\n",
			expectErrors: true,
			description:  "Invalid or unknown SIP method",
		},
		{
			name:         "Invalid Request-URI",
			message:      "INVITE http://test.example.com SIP/2.0\r\n\r\n",
			expectErrors: true,
			description:  "Request-URI must be a SIP or SIPS URI",
		},
		{
			name:         "Invalid SIP version",
			message:      "INVITE sip:test@example.com SIP/1.0\r\n\r\n",
			expectErrors: true,
			description:  "Invalid SIP version",
		},
		{
			name:         "Invalid status code",
			message:      "SIP/2.0 999 Invalid\r\n\r\n",
			expectErrors: true,
			description:  "Invalid status code",
		},
		{
			name:         "Too few parts",
			message:      "INVITE sip:test@example.com\r\n\r\n",
			expectErrors: true,
			description:  "Start line must have at least 3 parts",
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errors := detector.DetectMalformedMessage([]byte(tt.message))
			
			if tt.expectErrors {
				if len(errors) == 0 {
					t.Error("Expected malformed message errors, got none")
				} else {
					found := false
					for _, err := range errors {
						if err.Type == MalformedStartLine && strings.Contains(err.Description, tt.description) {
							found = true
							break
						}
					}
					if !found {
						t.Errorf("Expected start line error containing '%s', but not found", tt.description)
					}
				}
			} else {
				startLineErrors := 0
				for _, err := range errors {
					if err.Type == MalformedStartLine {
						startLineErrors++
					}
				}
				if startLineErrors > 0 {
					t.Errorf("Expected no start line errors, got %d", startLineErrors)
				}
			}
		})
	}
}

func TestDetectMalformedMessage_Headers(t *testing.T) {
	errorHandler := NewDefaultErrorHandler()
	errorGenerator := NewDetailedErrorResponseGenerator(errorHandler)
	detector := NewMalformedMessageDetector(errorGenerator)
	
	tests := []struct {
		name         string
		message      string
		expectErrors bool
		errorType    MalformedType
	}{
		{
			name:         "Valid headers",
			message:      "INVITE sip:test@example.com SIP/2.0\r\nVia: SIP/2.0/UDP host:5060;branch=z9hG4bK123\r\nContent-Length: 0\r\n\r\n",
			expectErrors: false,
		},
		{
			name:         "Invalid header format",
			message:      "INVITE sip:test@example.com SIP/2.0\r\nInvalidHeader\r\n\r\n",
			expectErrors: true,
			errorType:    MalformedHeader,
		},
		{
			name:         "Invalid Content-Length",
			message:      "INVITE sip:test@example.com SIP/2.0\r\nContent-Length: invalid\r\n\r\n",
			expectErrors: true,
			errorType:    MalformedHeaderValue,
		},
		{
			name:         "Invalid CSeq format",
			message:      "INVITE sip:test@example.com SIP/2.0\r\nCSeq: invalid\r\n\r\n",
			expectErrors: true,
			errorType:    MalformedHeaderValue,
		},
		{
			name:         "Invalid CSeq sequence number",
			message:      "INVITE sip:test@example.com SIP/2.0\r\nCSeq: abc INVITE\r\n\r\n",
			expectErrors: true,
			errorType:    MalformedHeaderValue,
		},
		{
			name:         "Invalid Max-Forwards",
			message:      "INVITE sip:test@example.com SIP/2.0\r\nMax-Forwards: invalid\r\n\r\n",
			expectErrors: true,
			errorType:    MalformedHeaderValue,
		},
		{
			name:         "Via without SIP version",
			message:      "INVITE sip:test@example.com SIP/2.0\r\nVia: UDP host:5060\r\n\r\n",
			expectErrors: true,
			errorType:    MalformedHeaderValue,
		},
		{
			name:         "Via without branch parameter",
			message:      "INVITE sip:test@example.com SIP/2.0\r\nVia: SIP/2.0/UDP host:5060\r\n\r\n",
			expectErrors: true,
			errorType:    MalformedHeaderValue,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errors := detector.DetectMalformedMessage([]byte(tt.message))
			
			if tt.expectErrors {
				if len(errors) == 0 {
					t.Error("Expected malformed message errors, got none")
				} else {
					found := false
					for _, err := range errors {
						if err.Type == tt.errorType {
							found = true
							break
						}
					}
					if !found {
						t.Errorf("Expected error type %s, but not found", tt.errorType.String())
					}
				}
			} else {
				headerErrors := 0
				for _, err := range errors {
					if err.Type == MalformedHeader || err.Type == MalformedHeaderValue {
						headerErrors++
					}
				}
				if headerErrors > 0 {
					t.Errorf("Expected no header errors, got %d", headerErrors)
				}
			}
		})
	}
}

func TestDetectMalformedMessage_Encoding(t *testing.T) {
	errorHandler := NewDefaultErrorHandler()
	errorGenerator := NewDetailedErrorResponseGenerator(errorHandler)
	detector := NewMalformedMessageDetector(errorGenerator)
	
	tests := []struct {
		name         string
		message      []byte
		expectErrors bool
		description  string
	}{
		{
			name:         "Valid ASCII message",
			message:      []byte("INVITE sip:test@example.com SIP/2.0\r\nVia: SIP/2.0/UDP host:5060\r\n\r\n"),
			expectErrors: false,
		},
		{
			name:         "Message with null bytes",
			message:      []byte("INVITE sip:test@example.com SIP/2.0\r\n\x00Via: SIP/2.0/UDP host:5060\r\n\r\n"),
			expectErrors: true,
			description:  "Null bytes found in message",
		},
		{
			name:         "Message with non-ASCII characters",
			message:      []byte("INVITE sip:test@example.com SIP/2.0\r\nVia: SIP/2.0/UDP hôst:5060\r\n\r\n"),
			expectErrors: true,
			description:  "Non-ASCII characters found in headers",
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errors := detector.DetectMalformedMessage(tt.message)
			
			if tt.expectErrors {
				if len(errors) == 0 {
					t.Error("Expected malformed message errors, got none")
				} else {
					found := false
					for _, err := range errors {
						if err.Type == MalformedEncoding && strings.Contains(err.Description, tt.description) {
							found = true
							break
						}
					}
					if !found {
						t.Errorf("Expected encoding error containing '%s', but not found", tt.description)
					}
				}
			} else {
				encodingErrors := 0
				for _, err := range errors {
					if err.Type == MalformedEncoding {
						encodingErrors++
					}
				}
				if encodingErrors > 0 {
					t.Errorf("Expected no encoding errors, got %d", encodingErrors)
				}
			}
		})
	}
}

func TestGenerateMalformedMessageResponse(t *testing.T) {
	errorHandler := NewDefaultErrorHandler()
	errorGenerator := NewDetailedErrorResponseGenerator(errorHandler)
	detector := NewMalformedMessageDetector(errorGenerator)
	
	malformedErrors := []MalformedMessageError{
		{
			Type:        MalformedStartLine,
			Description: "Invalid SIP method",
			Location:    "line 1",
			Suggestion:  "Use a valid SIP method",
		},
		{
			Type:        MalformedHeader,
			Description: "Invalid header format",
			Location:    "line 2",
			Suggestion:  "Use proper header format",
		},
	}
	
	rawMessage := []byte("INVALID sip:test@example.com SIP/2.0\r\nBadHeader\r\n\r\n")
	
	response := detector.GenerateMalformedMessageResponse(malformedErrors, rawMessage)
	
	if response == nil {
		t.Fatal("Expected non-nil response")
	}
	
	if response.GetStatusCode() != parser.StatusBadRequest {
		t.Errorf("Expected status code %d, got %d", parser.StatusBadRequest, response.GetStatusCode())
	}
	
	// Test with no errors
	response = detector.GenerateMalformedMessageResponse([]MalformedMessageError{}, rawMessage)
	if response != nil {
		t.Error("Expected nil response when no malformed errors")
	}
}

func TestDetectMalformedMessage_ComplexMessage(t *testing.T) {
	errorHandler := NewDefaultErrorHandler()
	errorGenerator := NewDetailedErrorResponseGenerator(errorHandler)
	detector := NewMalformedMessageDetector(errorGenerator)
	
	// Message with multiple issues
	message := []byte("INVALID http://test.com SIP/1.0\nBadHeader\nContent-Length: abc\n\n")
	
	errors := detector.DetectMalformedMessage(message)
	
	if len(errors) == 0 {
		t.Fatal("Expected multiple malformed message errors")
	}
	
	// Check that we detected multiple types of errors
	errorTypes := make(map[MalformedType]bool)
	for _, err := range errors {
		errorTypes[err.Type] = true
	}
	
	expectedTypes := []MalformedType{
		MalformedLineEnding,
		MalformedStartLine,
		MalformedHeader,
		MalformedHeaderValue,
	}
	
	for _, expectedType := range expectedTypes {
		if !errorTypes[expectedType] {
			t.Errorf("Expected to find error type %s", expectedType.String())
		}
	}
}

func TestValidateSpecificHeader_EdgeCases(t *testing.T) {
	errorHandler := NewDefaultErrorHandler()
	errorGenerator := NewDetailedErrorResponseGenerator(errorHandler)
	detector := NewMalformedMessageDetector(errorGenerator)
	
	tests := []struct {
		name        string
		headerName  string
		headerValue string
		expectError bool
	}{
		{
			name:        "Valid Content-Length zero",
			headerName:  "Content-Length",
			headerValue: "0",
			expectError: false,
		},
		{
			name:        "Valid CSeq",
			headerName:  "CSeq",
			headerValue: "1 INVITE",
			expectError: false,
		},
		{
			name:        "Valid Max-Forwards",
			headerName:  "Max-Forwards",
			headerValue: "70",
			expectError: false,
		},
		{
			name:        "Valid Via with branch",
			headerName:  "Via",
			headerValue: "SIP/2.0/UDP host:5060;branch=z9hG4bK123",
			expectError: false,
		},
		{
			name:        "Empty Max-Forwards",
			headerName:  "Max-Forwards",
			headerValue: "",
			expectError: false, // Empty is allowed, will be handled by missing header validation
		},
		{
			name:        "Unknown header",
			headerName:  "Custom-Header",
			headerValue: "any-value",
			expectError: false, // Unknown headers are allowed
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errors := detector.validateSpecificHeader(tt.headerName, tt.headerValue, 1)
			
			if tt.expectError && len(errors) == 0 {
				t.Error("Expected validation error, got none")
			} else if !tt.expectError && len(errors) > 0 {
				t.Errorf("Expected no validation errors, got %d", len(errors))
			}
		})
	}
}