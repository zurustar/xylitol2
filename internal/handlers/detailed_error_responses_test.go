package handlers

import (
	"errors"
	"strings"
	"testing"

	"github.com/zurustar/xylitol2/internal/parser"
)

func TestNewDetailedErrorResponseGenerator(t *testing.T) {
	errorHandler := NewDefaultErrorHandler()
	generator := NewDetailedErrorResponseGenerator(errorHandler)
	
	if generator == nil {
		t.Fatal("NewDetailedErrorResponseGenerator should not return nil")
	}
	
	if generator.errorHandler != errorHandler {
		t.Error("Generator should store the provided error handler")
	}
}

func TestDetailedErrorResponseGenerator_GenerateBadRequestResponse(t *testing.T) {
	errorHandler := NewDefaultErrorHandler()
	generator := NewDetailedErrorResponseGenerator(errorHandler)
	
	req := parser.NewRequestMessage(parser.MethodINVITE, "sip:test@example.com")
	req.SetHeader(parser.HeaderCallID, "test-call-id")
	
	missingHeaders := []string{"Via", "From"}
	invalidHeaders := map[string]string{
		"CSeq": "invalid format",
	}
	
	response := generator.GenerateBadRequestResponse(req, "Test error details", missingHeaders, invalidHeaders)
	
	if response.GetStatusCode() != parser.StatusBadRequest {
		t.Errorf("Response should be 400 Bad Request, got %d", response.GetStatusCode())
	}
	
	// Should have suggestions
	if len(response.Body) == 0 {
		t.Error("Response should have body with suggestions")
	}
}

func TestDetailedErrorResponseGenerator_GenerateMethodNotAllowedResponse(t *testing.T) {
	errorHandler := NewDefaultErrorHandler()
	generator := NewDetailedErrorResponseGenerator(errorHandler)
	
	req := parser.NewRequestMessage("UNKNOWN", "sip:test@example.com")
	req.SetHeader(parser.HeaderCallID, "test-call-id")
	
	allowedMethods := []string{"INVITE", "REGISTER", "OPTIONS"}
	
	response := generator.GenerateMethodNotAllowedResponse(req, allowedMethods)
	
	if response.GetStatusCode() != parser.StatusMethodNotAllowed {
		t.Errorf("Response should be 405 Method Not Allowed, got %d", response.GetStatusCode())
	}
	
	// Should have Allow header
	allowHeader := response.GetHeader(parser.HeaderAllow)
	if !strings.Contains(allowHeader, "INVITE") {
		t.Error("Response should have Allow header with supported methods")
	}
}

func TestDetailedErrorResponseGenerator_GenerateMethodNotAllowedResponse_NilRequest(t *testing.T) {
	errorHandler := NewDefaultErrorHandler()
	generator := NewDetailedErrorResponseGenerator(errorHandler)
	
	allowedMethods := []string{"INVITE", "REGISTER", "OPTIONS"}
	
	response := generator.GenerateMethodNotAllowedResponse(nil, allowedMethods)
	
	if response.GetStatusCode() != parser.StatusMethodNotAllowed {
		t.Errorf("Response should be 405 Method Not Allowed, got %d", response.GetStatusCode())
	}
}

func TestDetailedErrorResponseGenerator_GenerateExtensionRequiredResponse(t *testing.T) {
	errorHandler := NewDefaultErrorHandler()
	generator := NewDetailedErrorResponseGenerator(errorHandler)
	
	req := parser.NewRequestMessage(parser.MethodINVITE, "sip:test@example.com")
	req.SetHeader(parser.HeaderCallID, "test-call-id")
	
	response := generator.GenerateExtensionRequiredResponse(req, "timer")
	
	if response.GetStatusCode() != parser.StatusExtensionRequired {
		t.Errorf("Response should be 421 Extension Required, got %d", response.GetStatusCode())
	}
	
	// Should have Require and Supported headers
	if response.GetHeader(parser.HeaderRequire) != "timer" {
		t.Error("Response should have Require: timer header")
	}
	if response.GetHeader(parser.HeaderSupported) != "timer" {
		t.Error("Response should have Supported: timer header")
	}
}

func TestDetailedErrorResponseGenerator_GenerateIntervalTooBriefResponse(t *testing.T) {
	errorHandler := NewDefaultErrorHandler()
	generator := NewDetailedErrorResponseGenerator(errorHandler)
	
	req := parser.NewRequestMessage(parser.MethodINVITE, "sip:test@example.com")
	req.SetHeader(parser.HeaderCallID, "test-call-id")
	
	response := generator.GenerateIntervalTooBriefResponse(req, 90, 30)
	
	if response.GetStatusCode() != parser.StatusIntervalTooBrief {
		t.Errorf("Response should be 422 Session Interval Too Small, got %d", response.GetStatusCode())
	}
	
	// Should have Min-SE header
	if response.GetHeader(parser.HeaderMinSE) != "90" {
		t.Errorf("Response should have Min-SE: 90 header, got %s", response.GetHeader(parser.HeaderMinSE))
	}
}

func TestDetailedErrorResponseGenerator_GenerateMissingHeaderResponse(t *testing.T) {
	errorHandler := NewDefaultErrorHandler()
	generator := NewDetailedErrorResponseGenerator(errorHandler)
	
	req := parser.NewRequestMessage(parser.MethodINVITE, "sip:test@example.com")
	missingHeaders := []string{"Via", "From", "Session-Expires"}
	
	response := generator.GenerateMissingHeaderResponse(req, missingHeaders, "TestValidator")
	
	if response.GetStatusCode() != parser.StatusBadRequest {
		t.Errorf("Response should be 400 Bad Request, got %d", response.GetStatusCode())
	}
	
	// Should have detailed error information
	if len(response.Body) == 0 {
		t.Error("Response should have body with error details")
	}
	
	bodyText := string(response.Body)
	if !strings.Contains(bodyText, "Via") || !strings.Contains(bodyText, "From") {
		t.Error("Response body should contain information about missing headers")
	}
}

func TestDetailedErrorResponseGenerator_GenerateInvalidHeaderResponse(t *testing.T) {
	errorHandler := NewDefaultErrorHandler()
	generator := NewDetailedErrorResponseGenerator(errorHandler)
	
	req := parser.NewRequestMessage(parser.MethodINVITE, "sip:test@example.com")
	invalidHeaders := map[string]string{
		"CSeq":           "invalid format",
		"Content-Length": "not a number",
	}
	
	response := generator.GenerateInvalidHeaderResponse(req, invalidHeaders, "TestValidator")
	
	if response.GetStatusCode() != parser.StatusBadRequest {
		t.Errorf("Response should be 400 Bad Request, got %d", response.GetStatusCode())
	}
	
	// Should have detailed error information
	if len(response.Body) == 0 {
		t.Error("Response should have body with error details")
	}
	
	bodyText := string(response.Body)
	if !strings.Contains(bodyText, "CSeq") || !strings.Contains(bodyText, "Content-Length") {
		t.Error("Response body should contain information about invalid headers")
	}
}

func TestDetailedErrorResponseGenerator_GenerateParseErrorResponse(t *testing.T) {
	errorHandler := NewDefaultErrorHandler()
	generator := NewDetailedErrorResponseGenerator(errorHandler)
	
	parseError := errors.New("invalid request line")
	rawMessage := []byte("INVALID SIP MESSAGE")
	
	response := generator.GenerateParseErrorResponse(parseError, rawMessage)
	
	if response.GetStatusCode() != parser.StatusBadRequest {
		t.Errorf("Response should be 400 Bad Request, got %d", response.GetStatusCode())
	}
}

func TestDetailedErrorResponseGenerator_getSuggestionsForMissingHeader(t *testing.T) {
	errorHandler := NewDefaultErrorHandler()
	generator := NewDetailedErrorResponseGenerator(errorHandler)
	
	tests := []struct {
		header            string
		expectedKeywords  []string
	}{
		{"Via", []string{"Via:", "SIP/2.0", "branch"}},
		{"From", []string{"From:", "tag"}},
		{"To", []string{"To:", "sip:"}},
		{"Call-ID", []string{"Call-ID:", "unique"}},
		{"CSeq", []string{"CSeq:", "sequence"}},
		{"Session-Expires", []string{"Session-Expires:", "duration"}},
		{"Contact", []string{"Contact:", "sip:"}},
		{"Content-Length", []string{"Content-Length:", "body-size"}},
		{"Unknown-Header", []string{"Unknown-Header"}},
	}
	
	for _, test := range tests {
		suggestions := generator.getSuggestionsForMissingHeader(test.header)
		
		if len(suggestions) == 0 {
			t.Errorf("Should have suggestions for missing header %s", test.header)
			continue
		}
		
		suggestionText := strings.Join(suggestions, " ")
		for _, keyword := range test.expectedKeywords {
			if !strings.Contains(suggestionText, keyword) {
				t.Errorf("Suggestions for %s should contain keyword '%s', got: %s", 
					test.header, keyword, suggestionText)
			}
		}
	}
}

func TestDetailedErrorResponseGenerator_getSuggestionsForInvalidHeader(t *testing.T) {
	errorHandler := NewDefaultErrorHandler()
	generator := NewDetailedErrorResponseGenerator(errorHandler)
	
	tests := []struct {
		header           string
		reason           string
		expectedKeywords []string
	}{
		{"CSeq", "invalid format", []string{"CSeq:", "sequence-number", "METHOD"}},
		{"Content-Length", "not a number", []string{"Content-Length", "integer", "body"}},
		{"Session-Expires", "negative value", []string{"Session-Expires", "positive", "seconds"}},
		{"Min-SE", "invalid", []string{"Min-SE", "positive", "integer"}},
		{"Max-Forwards", "invalid", []string{"Max-Forwards", "integer", "70"}},
		{"Expires", "invalid", []string{"Expires", "integer", "seconds"}},
		{"Via", "malformed", []string{"Via", "SIP/2.0", "TRANSPORT", "branch"}},
		{"Unknown-Header", "some reason", []string{"Unknown-Header", "some reason"}},
	}
	
	for _, test := range tests {
		suggestions := generator.getSuggestionsForInvalidHeader(test.header, test.reason)
		
		if len(suggestions) == 0 {
			t.Errorf("Should have suggestions for invalid header %s", test.header)
			continue
		}
		
		suggestionText := strings.Join(suggestions, " ")
		for _, keyword := range test.expectedKeywords {
			if !strings.Contains(suggestionText, keyword) {
				t.Errorf("Suggestions for %s should contain keyword '%s', got: %s", 
					test.header, keyword, suggestionText)
			}
		}
	}
}

func TestDetailedErrorResponseGenerator_CreateDetailedErrorContext(t *testing.T) {
	errorHandler := NewDefaultErrorHandler()
	generator := NewDetailedErrorResponseGenerator(errorHandler)
	
	req := parser.NewRequestMessage(parser.MethodINVITE, "sip:test@example.com")
	req.SetHeader(parser.HeaderCallID, "test-call-id")
	req.SetHeader(parser.HeaderFrom, "sip:alice@example.com")
	req.SetHeader(parser.HeaderTo, "sip:bob@example.com")
	req.SetHeader(parser.HeaderVia, "SIP/2.0/UDP client.example.com:5060")
	req.SetHeader(parser.HeaderCSeq, "1 INVITE")
	
	context := generator.CreateDetailedErrorContext(req, ErrorTypeValidationError)
	
	// Check that all expected fields are present
	expectedFields := []string{"method", "request_uri", "call_id", "cseq", "from", "to", "via", "error_type", "timestamp"}
	for _, field := range expectedFields {
		if _, exists := context[field]; !exists {
			t.Errorf("Context should contain field '%s'", field)
		}
	}
	
	// Check specific values
	if context["method"] != parser.MethodINVITE {
		t.Errorf("Context method should be %s, got %v", parser.MethodINVITE, context["method"])
	}
	if context["error_type"] != ErrorTypeValidationError.String() {
		t.Errorf("Context error_type should be %s, got %v", ErrorTypeValidationError.String(), context["error_type"])
	}
}

func TestDetailedErrorResponseGenerator_CreateDetailedErrorContext_NilRequest(t *testing.T) {
	errorHandler := NewDefaultErrorHandler()
	generator := NewDetailedErrorResponseGenerator(errorHandler)
	
	context := generator.CreateDetailedErrorContext(nil, ErrorTypeParseError)
	
	// Should still have error_type and timestamp
	if _, exists := context["error_type"]; !exists {
		t.Error("Context should contain error_type even with nil request")
	}
	if _, exists := context["timestamp"]; !exists {
		t.Error("Context should contain timestamp even with nil request")
	}
	
	// Should not have request-specific fields
	if _, exists := context["method"]; exists {
		t.Error("Context should not contain method with nil request")
	}
}

func TestDetailedErrorResponseGenerator_addBadRequestSuggestions(t *testing.T) {
	errorHandler := NewDefaultErrorHandler()
	generator := NewDetailedErrorResponseGenerator(errorHandler)
	
	req := parser.NewRequestMessage(parser.MethodINVITE, "sip:test@example.com")
	
	details := &DetailedValidationError{
		ValidationError: &ValidationError{
			ValidatorName: "TestValidator",
			Code:          parser.StatusBadRequest,
			Reason:        "Bad Request",
		},
		MissingHeaders: []string{"Via", "From"},
		InvalidHeaders: map[string]string{
			"CSeq": "invalid format",
		},
	}
	
	generator.addBadRequestSuggestions(details, req)
	
	if len(details.Suggestions) == 0 {
		t.Error("Should have added suggestions for bad request")
	}
	
	suggestionText := strings.Join(details.Suggestions, " ")
	if !strings.Contains(suggestionText, "Via") {
		t.Error("Should have suggestions for missing Via header")
	}
	if !strings.Contains(suggestionText, "CSeq") {
		t.Error("Should have suggestions for invalid CSeq header")
	}
}

func TestDetailedErrorResponseGenerator_addBadRequestSuggestions_NoSpecificErrors(t *testing.T) {
	errorHandler := NewDefaultErrorHandler()
	generator := NewDetailedErrorResponseGenerator(errorHandler)
	
	req := parser.NewRequestMessage(parser.MethodINVITE, "sip:test@example.com")
	
	details := &DetailedValidationError{
		ValidationError: &ValidationError{
			ValidatorName: "TestValidator",
			Code:          parser.StatusBadRequest,
			Reason:        "Bad Request",
		},
		// No missing or invalid headers
	}
	
	generator.addBadRequestSuggestions(details, req)
	
	if len(details.Suggestions) == 0 {
		t.Error("Should have added general suggestions when no specific errors")
	}
	
	suggestionText := strings.Join(details.Suggestions, " ")
	if !strings.Contains(suggestionText, "RFC3261") {
		t.Error("Should have general RFC3261 suggestion")
	}
}