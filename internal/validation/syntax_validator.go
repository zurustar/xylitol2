package validation

import (
	"strings"

	"github.com/zurustar/xylitol2/internal/parser"
)

// SyntaxValidator validates basic SIP message syntax
type SyntaxValidator struct{}

// NewSyntaxValidator creates a new syntax validator
func NewSyntaxValidator() RequestValidator {
	return &SyntaxValidator{}
}

// Priority returns the priority of this validator (highest priority)
func (sv *SyntaxValidator) Priority() int {
	return 1
}

// Name returns the name of this validator
func (sv *SyntaxValidator) Name() string {
	return "SyntaxValidator"
}

// AppliesTo returns true for all requests
func (sv *SyntaxValidator) AppliesTo(req *parser.SIPMessage) bool {
	return req.IsRequest()
}

// Validate performs basic syntax validation
func (sv *SyntaxValidator) Validate(req *parser.SIPMessage) ValidationResult {
	// Check if method is present and valid
	method := req.GetMethod()
	if method == "" {
		return ValidationResult{
			Valid:       false,
			ErrorCode:   400,
			ErrorReason: "Bad Request",
			Details:     "Missing or empty method",
			ShouldStop:  true,
			Context: map[string]interface{}{
				"validator": "SyntaxValidator",
				"error":     "missing_method",
			},
		}
	}

	// Check if method contains invalid characters
	if strings.ContainsAny(method, " \t\r\n") {
		return ValidationResult{
			Valid:       false,
			ErrorCode:   400,
			ErrorReason: "Bad Request",
			Details:     "Method contains invalid characters",
			ShouldStop:  true,
			Context: map[string]interface{}{
				"validator": "SyntaxValidator",
				"error":     "invalid_method",
				"method":    method,
			},
		}
	}

	// Check if Request-URI is present
	if req.GetRequestURI() == "" {
		return ValidationResult{
			Valid:       false,
			ErrorCode:   400,
			ErrorReason: "Bad Request",
			Details:     "Missing Request-URI",
			ShouldStop:  true,
			Context: map[string]interface{}{
				"validator": "SyntaxValidator",
				"error":     "missing_request_uri",
			},
		}
	}

	// Check for required headers
	requiredHeaders := []string{"Via", "From", "To", "Call-ID", "CSeq"}
	for _, header := range requiredHeaders {
		if req.GetHeader(header) == "" {
			return ValidationResult{
				Valid:       false,
				ErrorCode:   400,
				ErrorReason: "Bad Request",
				Details:     "Missing required header: " + header,
				ShouldStop:  true,
				Context: map[string]interface{}{
					"validator":      "SyntaxValidator",
					"error":          "missing_required_header",
					"missing_header": header,
				},
			}
		}
	}

	// Check CSeq format
	cseq := req.GetHeader("CSeq")
	if cseq != "" {
		parts := strings.Fields(cseq)
		if len(parts) != 2 {
			return ValidationResult{
				Valid:       false,
				ErrorCode:   400,
				ErrorReason: "Bad Request",
				Details:     "Invalid CSeq header format",
				ShouldStop:  true,
				Context: map[string]interface{}{
					"validator": "SyntaxValidator",
					"error":     "invalid_cseq_format",
					"cseq":      cseq,
				},
			}
		}
	}

	// Check Content-Length if present
	contentLength := req.GetHeader("Content-Length")
	if contentLength != "" {
		// Basic validation - should be numeric
		if !isNumeric(contentLength) {
			return ValidationResult{
				Valid:       false,
				ErrorCode:   400,
				ErrorReason: "Bad Request",
				Details:     "Invalid Content-Length header",
				ShouldStop:  true,
				Context: map[string]interface{}{
					"validator":      "SyntaxValidator",
					"error":          "invalid_content_length",
					"content_length": contentLength,
				},
			}
		}
	}

	return ValidationResult{
		Valid: true,
		Context: map[string]interface{}{
			"validator": "SyntaxValidator",
		},
	}
}

// isNumeric checks if a string contains only digits
func isNumeric(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}