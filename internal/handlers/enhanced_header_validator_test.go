package handlers

import (
	"strings"
	"testing"

	"github.com/zurustar/xylitol2/internal/parser"
)

func TestNewEnhancedHeaderValidator(t *testing.T) {
	errorHandler := NewDefaultErrorHandler()
	errorGenerator := NewDetailedErrorResponseGenerator(errorHandler)
	validator := NewEnhancedHeaderValidator(errorGenerator)
	
	if validator == nil {
		t.Fatal("Expected non-nil enhanced header validator")
	}
	
	if validator.errorGenerator == nil {
		t.Error("Expected error generator to be set")
	}
}

func TestValidateRequiredHeaders_INVITE(t *testing.T) {
	errorHandler := NewDefaultErrorHandler()
	errorGenerator := NewDetailedErrorResponseGenerator(errorHandler)
	validator := NewEnhancedHeaderValidator(errorGenerator)
	
	// Create a valid INVITE request
	req := parser.NewRequestMessage(parser.MethodINVITE, "sip:test@example.com")
	req.SetHeader(parser.HeaderVia, "SIP/2.0/UDP host:5060;branch=z9hG4bK123")
	req.SetHeader(parser.HeaderFrom, "sip:caller@example.com;tag=123")
	req.SetHeader(parser.HeaderTo, "sip:test@example.com")
	req.SetHeader(parser.HeaderCallID, "call123@example.com")
	req.SetHeader(parser.HeaderCSeq, "1 INVITE")
	req.SetHeader(parser.HeaderMaxForwards, "70")
	req.SetHeader(parser.HeaderContact, "sip:caller@192.168.1.1:5060")
	req.SetHeader(parser.HeaderContentLength, "0")
	
	result := validator.ValidateRequiredHeaders(req, parser.MethodINVITE)
	
	if !result.Valid {
		t.Errorf("Expected valid result, got invalid with missing: %v, invalid: %v", 
			result.MissingHeaders, result.InvalidHeaders)
	}
	
	if len(result.MissingHeaders) > 0 {
		t.Errorf("Expected no missing headers, got: %v", result.MissingHeaders)
	}
	
	if len(result.InvalidHeaders) > 0 {
		t.Errorf("Expected no invalid headers, got: %v", result.InvalidHeaders)
	}
}

func TestValidateRequiredHeaders_MissingHeaders(t *testing.T) {
	errorHandler := NewDefaultErrorHandler()
	errorGenerator := NewDetailedErrorResponseGenerator(errorHandler)
	validator := NewEnhancedHeaderValidator(errorGenerator)
	
	// Create an incomplete INVITE request
	req := parser.NewRequestMessage(parser.MethodINVITE, "sip:test@example.com")
	req.SetHeader(parser.HeaderVia, "SIP/2.0/UDP host:5060;branch=z9hG4bK123")
	req.SetHeader(parser.HeaderFrom, "sip:caller@example.com;tag=123")
	// Missing To, Call-ID, CSeq, Max-Forwards, Contact, Content-Length
	
	result := validator.ValidateRequiredHeaders(req, parser.MethodINVITE)
	
	if result.Valid {
		t.Error("Expected invalid result due to missing headers")
	}
	
	expectedMissing := []string{
		parser.HeaderTo,
		parser.HeaderCallID,
		parser.HeaderCSeq,
		parser.HeaderMaxForwards,
		parser.HeaderContact,
		parser.HeaderContentLength,
	}
	
	if len(result.MissingHeaders) != len(expectedMissing) {
		t.Errorf("Expected %d missing headers, got %d: %v", 
			len(expectedMissing), len(result.MissingHeaders), result.MissingHeaders)
	}
}

func TestValidateViaHeader(t *testing.T) {
	errorHandler := NewDefaultErrorHandler()
	errorGenerator := NewDetailedErrorResponseGenerator(errorHandler)
	validator := NewEnhancedHeaderValidator(errorGenerator)
	
	tests := []struct {
		name        string
		value       string
		expectError bool
		errorText   string
	}{
		{
			name:        "Valid Via header",
			value:       "SIP/2.0/UDP host:5060;branch=z9hG4bK123",
			expectError: false,
		},
		{
			name:        "Valid Via with TCP",
			value:       "SIP/2.0/TCP host:5060;branch=z9hG4bK123",
			expectError: false,
		},
		{
			name:        "Missing SIP version",
			value:       "UDP host:5060;branch=z9hG4bK123",
			expectError: true,
			errorText:   "SIP/2.0 protocol version",
		},
		{
			name:        "Invalid transport",
			value:       "SIP/2.0/INVALID host:5060;branch=z9hG4bK123",
			expectError: true,
			errorText:   "valid transport",
		},
		{
			name:        "Missing branch parameter",
			value:       "SIP/2.0/UDP host:5060",
			expectError: true,
			errorText:   "branch parameter",
		},
		{
			name:        "Invalid branch format",
			value:       "SIP/2.0/UDP host:5060;branch=invalid123",
			expectError: true,
			errorText:   "z9hG4bK",
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := validator.validateViaHeader(tt.value)
			
			if tt.expectError {
				if result == "" {
					t.Error("Expected validation error, got none")
				} else if !strings.Contains(result, tt.errorText) {
					t.Errorf("Expected error containing '%s', got: %s", tt.errorText, result)
				}
			} else {
				if result != "" {
					t.Errorf("Expected no validation error, got: %s", result)
				}
			}
		})
	}
}

func TestValidateFromToHeader(t *testing.T) {
	errorHandler := NewDefaultErrorHandler()
	errorGenerator := NewDetailedErrorResponseGenerator(errorHandler)
	validator := NewEnhancedHeaderValidator(errorGenerator)
	
	tests := []struct {
		name        string
		value       string
		headerType  string
		expectError bool
		errorText   string
	}{
		{
			name:        "Valid From header",
			value:       "sip:user@example.com;tag=123",
			headerType:  "From",
			expectError: false,
		},
		{
			name:        "Valid To header without tag",
			value:       "sip:user@example.com",
			headerType:  "To",
			expectError: false,
		},
		{
			name:        "From header without tag",
			value:       "sip:user@example.com",
			headerType:  "From",
			expectError: true,
			errorText:   "tag parameter",
		},
		{
			name:        "Invalid URI scheme",
			value:       "http://user@example.com;tag=123",
			headerType:  "From",
			expectError: true,
			errorText:   "SIP or SIPS URI",
		},
		{
			name:        "Invalid URI format",
			value:       "sip:invalid-uri;tag=123",
			headerType:  "From",
			expectError: true,
			errorText:   "invalid SIP URI format",
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := validator.validateFromToHeader(tt.value, tt.headerType)
			
			if tt.expectError {
				if result == "" {
					t.Error("Expected validation error, got none")
				} else if !strings.Contains(result, tt.errorText) {
					t.Errorf("Expected error containing '%s', got: %s", tt.errorText, result)
				}
			} else {
				if result != "" {
					t.Errorf("Expected no validation error, got: %s", result)
				}
			}
		})
	}
}

func TestValidateCSeqHeader(t *testing.T) {
	errorHandler := NewDefaultErrorHandler()
	errorGenerator := NewDetailedErrorResponseGenerator(errorHandler)
	validator := NewEnhancedHeaderValidator(errorGenerator)
	
	// Create a test request
	req := parser.NewRequestMessage(parser.MethodINVITE, "sip:test@example.com")
	
	tests := []struct {
		name        string
		value       string
		expectError bool
		errorText   string
	}{
		{
			name:        "Valid CSeq",
			value:       "1 INVITE",
			expectError: false,
		},
		{
			name:        "Valid CSeq with large number",
			value:       "12345 INVITE",
			expectError: false,
		},
		{
			name:        "Invalid format - missing method",
			value:       "1",
			expectError: true,
			errorText:   "sequence number and method",
		},
		{
			name:        "Invalid format - too many parts",
			value:       "1 INVITE extra",
			expectError: true,
			errorText:   "sequence number and method",
		},
		{
			name:        "Invalid sequence number",
			value:       "abc INVITE",
			expectError: true,
			errorText:   "valid positive integer",
		},
		{
			name:        "Zero sequence number",
			value:       "0 INVITE",
			expectError: true,
			errorText:   "greater than 0",
		},
		{
			name:        "Method mismatch",
			value:       "1 REGISTER",
			expectError: true,
			errorText:   "must match request method",
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := validator.validateCSeqHeader(tt.value, req)
			
			if tt.expectError {
				if result == "" {
					t.Error("Expected validation error, got none")
				} else if !strings.Contains(result, tt.errorText) {
					t.Errorf("Expected error containing '%s', got: %s", tt.errorText, result)
				}
			} else {
				if result != "" {
					t.Errorf("Expected no validation error, got: %s", result)
				}
			}
		})
	}
}

func TestValidateContentLengthHeader(t *testing.T) {
	errorHandler := NewDefaultErrorHandler()
	errorGenerator := NewDetailedErrorResponseGenerator(errorHandler)
	validator := NewEnhancedHeaderValidator(errorGenerator)
	
	// Create a test request with body
	req := parser.NewRequestMessage(parser.MethodINVITE, "sip:test@example.com")
	req.Body = []byte("test body")
	
	tests := []struct {
		name        string
		value       string
		expectError bool
		errorText   string
	}{
		{
			name:        "Valid Content-Length matching body",
			value:       "9", // "test body" is 9 bytes
			expectError: false,
		},
		{
			name:        "Valid Content-Length zero",
			value:       "0",
			expectError: true, // Body is 9 bytes but Content-Length is 0
			errorText:   "does not match actual body length",
		},
		{
			name:        "Invalid Content-Length format",
			value:       "abc",
			expectError: true,
			errorText:   "non-negative integer",
		},
		{
			name:        "Negative Content-Length",
			value:       "-1",
			expectError: true,
			errorText:   "cannot be negative",
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := validator.validateContentLengthHeader(tt.value, req)
			
			if tt.expectError {
				if result == "" {
					t.Error("Expected validation error, got none")
				} else if !strings.Contains(result, tt.errorText) {
					t.Errorf("Expected error containing '%s', got: %s", tt.errorText, result)
				}
			} else {
				if result != "" {
					t.Errorf("Expected no validation error, got: %s", result)
				}
			}
		})
	}
}

func TestValidateSessionExpiresHeader(t *testing.T) {
	errorHandler := NewDefaultErrorHandler()
	errorGenerator := NewDetailedErrorResponseGenerator(errorHandler)
	validator := NewEnhancedHeaderValidator(errorGenerator)
	
	tests := []struct {
		name        string
		value       string
		expectError bool
		errorText   string
	}{
		{
			name:        "Valid Session-Expires",
			value:       "1800",
			expectError: false,
		},
		{
			name:        "Valid Session-Expires with parameters",
			value:       "1800;refresher=uac",
			expectError: false,
		},
		{
			name:        "Empty Session-Expires",
			value:       "",
			expectError: true,
			errorText:   "cannot be empty",
		},
		{
			name:        "Invalid format",
			value:       "abc",
			expectError: true,
			errorText:   "positive integer",
		},
		{
			name:        "Zero value",
			value:       "0",
			expectError: true,
			errorText:   "greater than 0",
		},
		{
			name:        "Too small value",
			value:       "30",
			expectError: true,
			errorText:   "at least 90 seconds",
		},
		{
			name:        "Too large value",
			value:       "100000",
			expectError: true,
			errorText:   "24 hours",
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := validator.validateSessionExpiresHeader(tt.value)
			
			if tt.expectError {
				if result == "" {
					t.Error("Expected validation error, got none")
				} else if !strings.Contains(result, tt.errorText) {
					t.Errorf("Expected error containing '%s', got: %s", tt.errorText, result)
				}
			} else {
				if result != "" {
					t.Errorf("Expected no validation error, got: %s", result)
				}
			}
		})
	}
}

func TestValidateAuthorizationHeader(t *testing.T) {
	errorHandler := NewDefaultErrorHandler()
	errorGenerator := NewDetailedErrorResponseGenerator(errorHandler)
	validator := NewEnhancedHeaderValidator(errorGenerator)
	
	tests := []struct {
		name        string
		value       string
		expectError bool
		errorText   string
	}{
		{
			name:        "Valid Authorization header",
			value:       "Digest username=\"alice\", realm=\"example.com\", nonce=\"abc123\", uri=\"sip:bob@example.com\", response=\"def456\"",
			expectError: false,
		},
		{
			name:        "Invalid scheme",
			value:       "Basic dXNlcjpwYXNz",
			expectError: true,
			errorText:   "Digest authentication scheme",
		},
		{
			name:        "Missing username",
			value:       "Digest realm=\"example.com\", nonce=\"abc123\", uri=\"sip:bob@example.com\", response=\"def456\"",
			expectError: true,
			errorText:   "username",
		},
		{
			name:        "Missing realm",
			value:       "Digest username=\"alice\", nonce=\"abc123\", uri=\"sip:bob@example.com\", response=\"def456\"",
			expectError: true,
			errorText:   "realm",
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := validator.validateAuthorizationHeader(tt.value)
			
			if tt.expectError {
				if result == "" {
					t.Error("Expected validation error, got none")
				} else if !strings.Contains(result, tt.errorText) {
					t.Errorf("Expected error containing '%s', got: %s", tt.errorText, result)
				}
			} else {
				if result != "" {
					t.Errorf("Expected no validation error, got: %s", result)
				}
			}
		})
	}
}

func TestValidateHostPort(t *testing.T) {
	errorHandler := NewDefaultErrorHandler()
	errorGenerator := NewDetailedErrorResponseGenerator(errorHandler)
	validator := NewEnhancedHeaderValidator(errorGenerator)
	
	tests := []struct {
		name        string
		hostPort    string
		expectError bool
		errorText   string
	}{
		{
			name:        "Valid host:port",
			hostPort:    "example.com:5060",
			expectError: false,
		},
		{
			name:        "Valid IP:port",
			hostPort:    "192.168.1.1:5060",
			expectError: false,
		},
		{
			name:        "Valid host without port",
			hostPort:    "example.com",
			expectError: false,
		},
		{
			name:        "Empty host:port",
			hostPort:    "",
			expectError: true,
			errorText:   "cannot be empty",
		},
		{
			name:        "Invalid port",
			hostPort:    "example.com:abc",
			expectError: true,
			errorText:   "port must be a number",
		},
		{
			name:        "Port out of range",
			hostPort:    "example.com:99999",
			expectError: true,
			errorText:   "between 1 and 65535",
		},
		{
			name:        "Invalid hostname",
			hostPort:    "invalid..hostname:5060",
			expectError: true,
			errorText:   "invalid hostname",
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := validator.ValidateHostPort(tt.hostPort)
			
			if tt.expectError {
				if result == "" {
					t.Error("Expected validation error, got none")
				} else if !strings.Contains(result, tt.errorText) {
					t.Errorf("Expected error containing '%s', got: %s", tt.errorText, result)
				}
			} else {
				if result != "" {
					t.Errorf("Expected no validation error, got: %s", result)
				}
			}
		})
	}
}

func TestGenerateHeaderValidationResponse(t *testing.T) {
	errorHandler := NewDefaultErrorHandler()
	errorGenerator := NewDetailedErrorResponseGenerator(errorHandler)
	validator := NewEnhancedHeaderValidator(errorGenerator)
	
	req := parser.NewRequestMessage(parser.MethodINVITE, "sip:test@example.com")
	
	// Test with valid result
	validResult := HeaderValidationResult{Valid: true}
	response := validator.GenerateHeaderValidationResponse(req, validResult)
	if response != nil {
		t.Error("Expected nil response for valid result")
	}
	
	// Test with missing headers
	missingResult := HeaderValidationResult{
		Valid:          false,
		MissingHeaders: []string{"Via", "From"},
		InvalidHeaders: make(map[string]string),
	}
	response = validator.GenerateHeaderValidationResponse(req, missingResult)
	if response == nil {
		t.Error("Expected non-nil response for missing headers")
	}
	if response.GetStatusCode() != parser.StatusBadRequest {
		t.Errorf("Expected status code %d, got %d", parser.StatusBadRequest, response.GetStatusCode())
	}
	
	// Test with invalid headers
	invalidResult := HeaderValidationResult{
		Valid:          false,
		MissingHeaders: []string{},
		InvalidHeaders: map[string]string{"CSeq": "invalid format"},
	}
	response = validator.GenerateHeaderValidationResponse(req, invalidResult)
	if response == nil {
		t.Error("Expected non-nil response for invalid headers")
	}
	if response.GetStatusCode() != parser.StatusBadRequest {
		t.Errorf("Expected status code %d, got %d", parser.StatusBadRequest, response.GetStatusCode())
	}
}