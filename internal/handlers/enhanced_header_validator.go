package handlers

import (
	"fmt"
	"net"
	"regexp"
	"strconv"
	"strings"

	"github.com/zurustar/xylitol2/internal/parser"
)

// EnhancedHeaderValidator provides detailed validation for SIP headers
type EnhancedHeaderValidator struct {
	errorGenerator *DetailedErrorResponseGenerator
}

// NewEnhancedHeaderValidator creates a new enhanced header validator
func NewEnhancedHeaderValidator(errorGenerator *DetailedErrorResponseGenerator) *EnhancedHeaderValidator {
	return &EnhancedHeaderValidator{
		errorGenerator: errorGenerator,
	}
}

// HeaderValidationResult contains detailed information about header validation
type HeaderValidationResult struct {
	Valid          bool
	MissingHeaders []string
	InvalidHeaders map[string]string
	Suggestions    []string
	Context        map[string]interface{}
}

// ValidateRequiredHeaders validates that all required headers are present and valid
func (ehv *EnhancedHeaderValidator) ValidateRequiredHeaders(req *parser.SIPMessage, method string) HeaderValidationResult {
	result := HeaderValidationResult{
		Valid:          true,
		MissingHeaders: []string{},
		InvalidHeaders: make(map[string]string),
		Suggestions:    []string{},
		Context:        make(map[string]interface{}),
	}
	
	// Get required headers based on method
	requiredHeaders := ehv.getRequiredHeaders(method)
	
	// Check for missing headers
	for _, header := range requiredHeaders {
		if !req.HasHeader(header) {
			result.MissingHeaders = append(result.MissingHeaders, header)
			result.Valid = false
		}
	}
	
	// Validate existing headers
	for header, values := range req.Headers {
		if len(values) > 0 {
			if validationError := ehv.validateHeaderValue(header, values[0], req); validationError != "" {
				result.InvalidHeaders[header] = validationError
				result.Valid = false
			}
		}
	}
	
	// Add context information
	result.Context["method"] = method
	result.Context["total_headers"] = len(req.Headers)
	result.Context["missing_count"] = len(result.MissingHeaders)
	result.Context["invalid_count"] = len(result.InvalidHeaders)
	
	return result
}

// getRequiredHeaders returns the list of required headers for a given method
func (ehv *EnhancedHeaderValidator) getRequiredHeaders(method string) []string {
	baseHeaders := []string{
		parser.HeaderVia,
		parser.HeaderFrom,
		parser.HeaderTo,
		parser.HeaderCallID,
		parser.HeaderCSeq,
		parser.HeaderMaxForwards,
	}
	
	switch method {
	case parser.MethodINVITE:
		return append(baseHeaders, parser.HeaderContact, parser.HeaderContentLength)
	case parser.MethodREGISTER:
		return append(baseHeaders, parser.HeaderContact, parser.HeaderExpires, parser.HeaderContentLength)
	case parser.MethodACK:
		// ACK doesn't require Max-Forwards
		return []string{
			parser.HeaderVia,
			parser.HeaderFrom,
			parser.HeaderTo,
			parser.HeaderCallID,
			parser.HeaderCSeq,
			parser.HeaderContentLength,
		}
	case parser.MethodBYE, parser.MethodCANCEL:
		return append(baseHeaders, parser.HeaderContentLength)
	case parser.MethodOPTIONS, parser.MethodINFO:
		return append(baseHeaders, parser.HeaderContentLength)
	default:
		return append(baseHeaders, parser.HeaderContentLength)
	}
}

// validateHeaderValue validates the format and content of a specific header value
func (ehv *EnhancedHeaderValidator) validateHeaderValue(headerName, headerValue string, req *parser.SIPMessage) string {
	switch strings.ToLower(headerName) {
	case "via":
		return ehv.validateViaHeader(headerValue)
	case "from":
		return ehv.validateFromToHeader(headerValue, "From")
	case "to":
		return ehv.validateFromToHeader(headerValue, "To")
	case "call-id":
		return ehv.validateCallIDHeader(headerValue)
	case "cseq":
		return ehv.validateCSeqHeader(headerValue, req)
	case "max-forwards":
		return ehv.validateMaxForwardsHeader(headerValue)
	case "content-length":
		return ehv.validateContentLengthHeader(headerValue, req)
	case "contact":
		return ehv.validateContactHeader(headerValue)
	case "expires":
		return ehv.validateExpiresHeader(headerValue)
	case "session-expires":
		return ehv.validateSessionExpiresHeader(headerValue)
	case "min-se":
		return ehv.validateMinSEHeader(headerValue)
	case "authorization":
		return ehv.validateAuthorizationHeader(headerValue)
	case "www-authenticate":
		return ehv.validateWWWAuthenticateHeader(headerValue)
	default:
		// For unknown headers, just check basic format
		return ehv.validateGenericHeader(headerName, headerValue)
	}
}

// validateViaHeader validates Via header format
func (ehv *EnhancedHeaderValidator) validateViaHeader(value string) string {
	// Via: SIP/2.0/transport host:port;branch=z9hG4bK-branch-id
	if !strings.Contains(value, "SIP/2.0/") {
		return "Via header must contain SIP/2.0 protocol version"
	}
	
	// Check for valid transport
	validTransports := []string{"UDP", "TCP", "TLS", "SCTP", "WS", "WSS"}
	hasValidTransport := false
	for _, transport := range validTransports {
		if strings.Contains(value, "SIP/2.0/"+transport) {
			hasValidTransport = true
			break
		}
	}
	
	if !hasValidTransport {
		return fmt.Sprintf("Via header must contain valid transport (one of: %s)", strings.Join(validTransports, ", "))
	}
	
	// Check for branch parameter
	if !strings.Contains(value, "branch=") {
		return "Via header must contain branch parameter"
	}
	
	// Check branch parameter format (should start with z9hG4bK)
	branchRegex := regexp.MustCompile(`branch=([^;,\s]+)`)
	matches := branchRegex.FindStringSubmatch(value)
	if len(matches) > 1 {
		branch := matches[1]
		if !strings.HasPrefix(branch, "z9hG4bK") {
			return "Via branch parameter should start with 'z9hG4bK' for RFC3261 compliance"
		}
	}
	
	return ""
}

// validateFromToHeader validates From/To header format
func (ehv *EnhancedHeaderValidator) validateFromToHeader(value, headerType string) string {
	// From/To: "Display Name" <sip:user@domain>;tag=tag-value
	// or: sip:user@domain;tag=tag-value
	
	if !strings.Contains(value, "sip:") && !strings.Contains(value, "sips:") {
		return fmt.Sprintf("%s header must contain a SIP or SIPS URI", headerType)
	}
	
	// For From header, tag is required in requests
	if headerType == "From" && !strings.Contains(value, "tag=") {
		return "From header must contain a tag parameter"
	}
	
	// Basic URI format validation
	uriRegex := regexp.MustCompile(`sips?:[^@]+@[^;>\s]+`)
	if !uriRegex.MatchString(value) {
		return fmt.Sprintf("%s header contains invalid SIP URI format", headerType)
	}
	
	return ""
}

// validateCallIDHeader validates Call-ID header format
func (ehv *EnhancedHeaderValidator) validateCallIDHeader(value string) string {
	if strings.TrimSpace(value) == "" {
		return "Call-ID header cannot be empty"
	}
	
	// Call-ID should be globally unique, typically contains @ symbol
	if !strings.Contains(value, "@") {
		return "Call-ID should contain '@' symbol for global uniqueness (recommended format: localid@host)"
	}
	
	return ""
}

// validateCSeqHeader validates CSeq header format
func (ehv *EnhancedHeaderValidator) validateCSeqHeader(value string, req *parser.SIPMessage) string {
	parts := strings.Fields(value)
	if len(parts) != 2 {
		return "CSeq header must contain sequence number and method (format: 'number METHOD')"
	}
	
	// Validate sequence number
	seqNum, err := strconv.ParseUint(parts[0], 10, 32)
	if err != nil {
		return "CSeq sequence number must be a valid positive integer"
	}
	
	if seqNum == 0 {
		return "CSeq sequence number must be greater than 0"
	}
	
	// Validate method matches request method
	method := parts[1]
	if req != nil && req.IsRequest() {
		requestMethod := req.GetMethod()
		if method != requestMethod {
			return fmt.Sprintf("CSeq method '%s' must match request method '%s'", method, requestMethod)
		}
	}
	
	return ""
}

// validateMaxForwardsHeader validates Max-Forwards header format
func (ehv *EnhancedHeaderValidator) validateMaxForwardsHeader(value string) string {
	maxForwards, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil {
		return "Max-Forwards must be a non-negative integer"
	}
	
	if maxForwards < 0 {
		return "Max-Forwards cannot be negative"
	}
	
	if maxForwards > 255 {
		return "Max-Forwards should not exceed 255"
	}
	
	return ""
}

// validateContentLengthHeader validates Content-Length header format
func (ehv *EnhancedHeaderValidator) validateContentLengthHeader(value string, req *parser.SIPMessage) string {
	contentLength, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil {
		return "Content-Length must be a non-negative integer"
	}
	
	if contentLength < 0 {
		return "Content-Length cannot be negative"
	}
	
	// If we have the actual message body, validate the length matches
	if req != nil && len(req.Body) != contentLength {
		return fmt.Sprintf("Content-Length (%d) does not match actual body length (%d)", contentLength, len(req.Body))
	}
	
	return ""
}

// validateContactHeader validates Contact header format
func (ehv *EnhancedHeaderValidator) validateContactHeader(value string) string {
	if strings.TrimSpace(value) == "*" {
		return "" // "*" is valid for unregistering all contacts
	}
	
	if !strings.Contains(value, "sip:") && !strings.Contains(value, "sips:") {
		return "Contact header must contain a SIP or SIPS URI"
	}
	
	// Basic URI format validation
	uriRegex := regexp.MustCompile(`sips?:[^@]+@[^;>\s]+`)
	if !uriRegex.MatchString(value) {
		return "Contact header contains invalid SIP URI format"
	}
	
	return ""
}

// validateExpiresHeader validates Expires header format
func (ehv *EnhancedHeaderValidator) validateExpiresHeader(value string) string {
	expires, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil {
		return "Expires must be a non-negative integer (seconds)"
	}
	
	if expires < 0 {
		return "Expires cannot be negative"
	}
	
	// Reasonable upper limit check
	if expires > 86400*365 { // More than 1 year
		return "Expires value seems unreasonably large (more than 1 year)"
	}
	
	return ""
}

// validateSessionExpiresHeader validates Session-Expires header format
func (ehv *EnhancedHeaderValidator) validateSessionExpiresHeader(value string) string {
	// Parse the main value (before any parameters)
	parts := strings.Split(value, ";")
	if len(parts) == 0 {
		return "Session-Expires header cannot be empty"
	}
	
	sessionExpires, err := strconv.Atoi(strings.TrimSpace(parts[0]))
	if err != nil {
		return "Session-Expires must be a positive integer (seconds)"
	}
	
	if sessionExpires <= 0 {
		return "Session-Expires must be greater than 0"
	}
	
	// Reasonable range check
	if sessionExpires < 90 {
		return "Session-Expires should be at least 90 seconds"
	}
	
	if sessionExpires > 86400 { // More than 24 hours
		return "Session-Expires should not exceed 24 hours (86400 seconds)"
	}
	
	return ""
}

// validateMinSEHeader validates Min-SE header format
func (ehv *EnhancedHeaderValidator) validateMinSEHeader(value string) string {
	minSE, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil {
		return "Min-SE must be a positive integer (seconds)"
	}
	
	if minSE <= 0 {
		return "Min-SE must be greater than 0"
	}
	
	// Reasonable range check
	if minSE < 90 {
		return "Min-SE should be at least 90 seconds"
	}
	
	if minSE > 3600 { // More than 1 hour
		return "Min-SE should not exceed 1 hour (3600 seconds)"
	}
	
	return ""
}

// validateAuthorizationHeader validates Authorization header format
func (ehv *EnhancedHeaderValidator) validateAuthorizationHeader(value string) string {
	if !strings.HasPrefix(value, "Digest ") {
		return "Authorization header must use Digest authentication scheme"
	}
	
	// Check for required digest parameters
	requiredParams := []string{"username", "realm", "nonce", "uri", "response"}
	for _, param := range requiredParams {
		if !strings.Contains(value, param+"=") {
			return fmt.Sprintf("Authorization header missing required parameter: %s", param)
		}
	}
	
	return ""
}

// validateWWWAuthenticateHeader validates WWW-Authenticate header format
func (ehv *EnhancedHeaderValidator) validateWWWAuthenticateHeader(value string) string {
	if !strings.HasPrefix(value, "Digest ") {
		return "WWW-Authenticate header must use Digest authentication scheme"
	}
	
	// Check for required digest parameters
	requiredParams := []string{"realm", "nonce"}
	for _, param := range requiredParams {
		if !strings.Contains(value, param+"=") {
			return fmt.Sprintf("WWW-Authenticate header missing required parameter: %s", param)
		}
	}
	
	return ""
}

// validateGenericHeader performs basic validation for unknown headers
func (ehv *EnhancedHeaderValidator) validateGenericHeader(name, value string) string {
	// Check for empty value
	if strings.TrimSpace(value) == "" {
		return fmt.Sprintf("%s header cannot be empty", name)
	}
	
	// Check for control characters (except tab)
	for _, r := range value {
		if r < 32 && r != 9 { // Allow tab (9) but not other control characters
			return fmt.Sprintf("%s header contains invalid control characters", name)
		}
	}
	
	return ""
}

// GenerateHeaderValidationResponse generates a detailed response for header validation failures
func (ehv *EnhancedHeaderValidator) GenerateHeaderValidationResponse(req *parser.SIPMessage, result HeaderValidationResult) *parser.SIPMessage {
	if result.Valid {
		return nil
	}
	
	// Determine the primary error type
	if len(result.MissingHeaders) > 0 {
		return ehv.errorGenerator.GenerateMissingHeaderResponse(req, result.MissingHeaders, "EnhancedHeaderValidator")
	}
	
	if len(result.InvalidHeaders) > 0 {
		return ehv.errorGenerator.GenerateInvalidHeaderResponse(req, result.InvalidHeaders, "EnhancedHeaderValidator")
	}
	
	// Fallback to generic bad request
	return ehv.errorGenerator.GenerateBadRequestResponse(req, "Header validation failed", []string{}, result.InvalidHeaders)
}

// ValidateHostPort validates host:port format
func (ehv *EnhancedHeaderValidator) ValidateHostPort(hostPort string) string {
	if hostPort == "" {
		return "host:port cannot be empty"
	}
	
	host, port, err := net.SplitHostPort(hostPort)
	if err != nil {
		// Try without port
		if net.ParseIP(hostPort) != nil || isValidHostname(hostPort) {
			return "" // Valid host without port
		}
		return "invalid host:port format"
	}
	
	// Validate host
	if net.ParseIP(host) == nil && !isValidHostname(host) {
		return "invalid hostname or IP address"
	}
	
	// Validate port
	portNum, err := strconv.Atoi(port)
	if err != nil {
		return "port must be a number"
	}
	
	if portNum < 1 || portNum > 65535 {
		return "port must be between 1 and 65535"
	}
	
	return ""
}

// isValidHostname checks if a string is a valid hostname
func isValidHostname(hostname string) bool {
	if len(hostname) == 0 || len(hostname) > 253 {
		return false
	}
	
	// Hostname regex pattern
	hostnameRegex := regexp.MustCompile(`^[a-zA-Z0-9]([a-zA-Z0-9\-]{0,61}[a-zA-Z0-9])?(\.[a-zA-Z0-9]([a-zA-Z0-9\-]{0,61}[a-zA-Z0-9])?)*$`)
	return hostnameRegex.MatchString(hostname)
}