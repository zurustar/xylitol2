package parser

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// Parser implements the MessageParser interface
type Parser struct{}

// NewParser creates a new SIP message parser
func NewParser() *Parser {
	return &Parser{}
}

// Parse parses a SIP message from raw bytes
func (p *Parser) Parse(data []byte) (*SIPMessage, error) {
	if len(data) == 0 {
		return nil, errors.New("empty message data")
	}

	reader := bufio.NewReader(bytes.NewReader(data))
	
	// Parse start line
	startLine, err := p.parseStartLine(reader)
	if err != nil {
		return nil, fmt.Errorf("failed to parse start line: %w", err)
	}

	// Parse headers
	headers, err := p.parseHeaders(reader)
	if err != nil {
		return nil, fmt.Errorf("failed to parse headers: %w", err)
	}

	// Parse body
	body, err := p.parseBody(reader, headers)
	if err != nil {
		return nil, fmt.Errorf("failed to parse body: %w", err)
	}

	msg := &SIPMessage{
		StartLine: startLine,
		Headers:   headers,
		Body:      body,
	}

	return msg, nil
}

// parseStartLine parses the first line of a SIP message
func (p *Parser) parseStartLine(reader *bufio.Reader) (StartLine, error) {
	line, err := p.readLine(reader)
	if err != nil {
		return nil, fmt.Errorf("failed to read start line: %w", err)
	}

	parts := strings.Fields(line)
	if len(parts) < 3 {
		return nil, fmt.Errorf("invalid start line format: %s", line)
	}

	// Check if it's a request or response
	if strings.HasPrefix(parts[0], "SIP/") {
		// Response line: SIP/2.0 200 OK
		version := parts[0]
		statusCode, err := strconv.Atoi(parts[1])
		if err != nil {
			return nil, fmt.Errorf("invalid status code: %s", parts[1])
		}
		reasonPhrase := strings.Join(parts[2:], " ")
		
		return &StatusLine{
			Version:      version,
			StatusCode:   statusCode,
			ReasonPhrase: reasonPhrase,
		}, nil
	} else {
		// Request line: INVITE sip:user@example.com SIP/2.0
		method := parts[0]
		requestURI := parts[1]
		version := parts[2]
		
		if !IsValidMethod(method) {
			return nil, fmt.Errorf("invalid method: %s", method)
		}
		
		if version != SIPVersion {
			return nil, fmt.Errorf("unsupported SIP version: %s", version)
		}
		
		return &RequestLine{
			Method:     method,
			RequestURI: requestURI,
			Version:    version,
		}, nil
	}
}

// parseHeaders parses SIP headers
func (p *Parser) parseHeaders(reader *bufio.Reader) (map[string][]string, error) {
	headers := make(map[string][]string)
	var lastHeaderName string
	
	for {
		line, err := p.readLine(reader)
		if err != nil {
			return nil, fmt.Errorf("failed to read header line: %w", err)
		}
		
		// Empty line indicates end of headers
		if line == "" {
			break
		}
		
		// Handle header folding (continuation lines start with space or tab)
		if strings.HasPrefix(line, " ") || strings.HasPrefix(line, "\t") {
			if lastHeaderName == "" {
				return nil, errors.New("header continuation without previous header")
			}
			// Append the continuation to the last header value
			lastIndex := len(headers[lastHeaderName]) - 1
			headers[lastHeaderName][lastIndex] += " " + strings.TrimSpace(line)
			continue
		}
		
		// Parse header name and value
		colonIndex := strings.Index(line, ":")
		if colonIndex == -1 {
			return nil, fmt.Errorf("invalid header format: %s", line)
		}
		
		name := strings.TrimSpace(line[:colonIndex])
		value := strings.TrimSpace(line[colonIndex+1:])
		
		if name == "" {
			return nil, fmt.Errorf("empty header name: %s", line)
		}
		
		// Handle compact header forms
		name = p.expandCompactHeader(name)
		lastHeaderName = name
		
		// Handle multi-value headers (comma-separated)
		if p.isMultiValueHeader(name) {
			values := p.parseMultiValueHeader(value)
			headers[name] = append(headers[name], values...)
		} else {
			headers[name] = append(headers[name], value)
		}
	}
	
	return headers, nil
}

// parseBody parses the message body
func (p *Parser) parseBody(reader *bufio.Reader, headers map[string][]string) ([]byte, error) {
	// Check Content-Length header
	contentLengthStr := ""
	if values, exists := headers[HeaderContentLength]; exists && len(values) > 0 {
		contentLengthStr = values[0]
	}
	
	if contentLengthStr == "" {
		// No Content-Length header, assume no body
		return nil, nil
	}
	
	contentLength, err := strconv.Atoi(contentLengthStr)
	if err != nil {
		return nil, fmt.Errorf("invalid Content-Length: %s", contentLengthStr)
	}
	
	if contentLength < 0 {
		return nil, fmt.Errorf("negative Content-Length: %d", contentLength)
	}
	
	if contentLength == 0 {
		return nil, nil
	}
	
	// Read the exact number of bytes specified by Content-Length
	body := make([]byte, contentLength)
	totalRead := 0
	
	for totalRead < contentLength {
		n, err := reader.Read(body[totalRead:])
		if err != nil {
			if n == 0 {
				return nil, fmt.Errorf("failed to read body: %w", err)
			}
		}
		totalRead += n
		if totalRead >= contentLength {
			break
		}
	}
	
	return body, nil
}

// readLine reads a line from the reader, handling CRLF line endings
func (p *Parser) readLine(reader *bufio.Reader) (string, error) {
	line, err := reader.ReadString('\n')
	if err != nil {
		return "", err
	}
	
	// Remove CRLF or LF
	line = strings.TrimRight(line, "\r\n")
	return line, nil
}

// expandCompactHeader expands compact header forms to full names
func (p *Parser) expandCompactHeader(name string) string {
	switch strings.ToLower(name) {
	case "i":
		return HeaderCallID
	case "m":
		return HeaderContact
	case "l":
		return HeaderContentLength
	case "c":
		return HeaderContentType
	case "f":
		return HeaderFrom
	case "s":
		return HeaderSubject
	case "k":
		return HeaderSupported
	case "t":
		return HeaderTo
	case "v":
		return HeaderVia
	default:
		return name
	}
}

// isMultiValueHeader checks if a header can have multiple comma-separated values
func (p *Parser) isMultiValueHeader(name string) bool {
	switch name {
	case HeaderVia, HeaderContact, HeaderRoute, HeaderRecordRoute, 
		 HeaderAccept, HeaderAcceptEncoding, HeaderAcceptLanguage,
		 HeaderAllow, HeaderSupported, HeaderUnsupported, HeaderRequire,
		 HeaderProxyRequire:
		return true
	default:
		return false
	}
}

// parseMultiValueHeader parses comma-separated header values
func (p *Parser) parseMultiValueHeader(value string) []string {
	var values []string
	var current strings.Builder
	inQuotes := false
	inAngleBrackets := false
	
	for _, char := range value {
		switch char {
		case '"':
			inQuotes = !inQuotes
			current.WriteRune(char)
		case '<':
			inAngleBrackets = true
			current.WriteRune(char)
		case '>':
			inAngleBrackets = false
			current.WriteRune(char)
		case ',':
			if !inQuotes && !inAngleBrackets {
				// End of current value
				val := strings.TrimSpace(current.String())
				if val != "" {
					values = append(values, val)
				}
				current.Reset()
			} else {
				current.WriteRune(char)
			}
		default:
			current.WriteRune(char)
		}
	}
	
	// Add the last value
	val := strings.TrimSpace(current.String())
	if val != "" {
		values = append(values, val)
	}
	
	return values
}

// Validate validates a SIP message according to RFC3261 rules
func (p *Parser) Validate(msg *SIPMessage) error {
	if msg == nil {
		return errors.New("message is nil")
	}
	
	if msg.StartLine == nil {
		return errors.New("start line is missing")
	}
	
	// Validate required headers
	requiredHeaders := []string{HeaderVia, HeaderFrom, HeaderTo, HeaderCallID, HeaderCSeq}
	for _, header := range requiredHeaders {
		if !msg.HasHeader(header) {
			return fmt.Errorf("required header missing: %s", header)
		}
	}
	
	// Validate Max-Forwards for requests
	if msg.IsRequest() {
		if !msg.HasHeader(HeaderMaxForwards) {
			return fmt.Errorf("Max-Forwards header required for requests")
		}
		
		maxForwardsStr := msg.GetHeader(HeaderMaxForwards)
		maxForwards, err := strconv.Atoi(maxForwardsStr)
		if err != nil {
			return fmt.Errorf("invalid Max-Forwards value: %s", maxForwardsStr)
		}
		
		if maxForwards < 0 || maxForwards > 255 {
			return fmt.Errorf("Max-Forwards out of range: %d", maxForwards)
		}
	}
	
	// Validate Content-Length
	if msg.HasHeader(HeaderContentLength) {
		contentLengthStr := msg.GetHeader(HeaderContentLength)
		contentLength, err := strconv.Atoi(contentLengthStr)
		if err != nil {
			return fmt.Errorf("invalid Content-Length: %s", contentLengthStr)
		}
		
		if contentLength < 0 {
			return fmt.Errorf("negative Content-Length: %d", contentLength)
		}
		
		if len(msg.Body) != contentLength {
			return fmt.Errorf("Content-Length mismatch: header says %d, body is %d bytes", 
				contentLength, len(msg.Body))
		}
	}
	
	// Validate CSeq header format
	cseqStr := msg.GetHeader(HeaderCSeq)
	cseqParts := strings.Fields(cseqStr)
	if len(cseqParts) != 2 {
		return fmt.Errorf("invalid CSeq format: %s", cseqStr)
	}
	
	cseqNum, err := strconv.ParseUint(cseqParts[0], 10, 32)
	if err != nil {
		return fmt.Errorf("invalid CSeq number: %s", cseqParts[0])
	}
	
	if cseqNum == 0 {
		return fmt.Errorf("CSeq number cannot be zero")
	}
	
	method := cseqParts[1]
	if !IsValidMethod(method) {
		return fmt.Errorf("invalid method in CSeq: %s", method)
	}
	
	// For requests, CSeq method should match request method
	if msg.IsRequest() {
		requestMethod := msg.GetMethod()
		if method != requestMethod {
			return fmt.Errorf("CSeq method (%s) does not match request method (%s)", 
				method, requestMethod)
		}
	}
	
	return nil
}

// Serialize converts a SIP message back to wire format
func (p *Parser) Serialize(msg *SIPMessage) ([]byte, error) {
	if msg == nil {
		return nil, errors.New("message is nil")
	}

	if msg.StartLine == nil {
		return nil, errors.New("start line is missing")
	}

	var buffer bytes.Buffer

	// Write start line
	buffer.WriteString(msg.StartLine.String())
	buffer.WriteString("\r\n")

	// Write headers in proper order
	headerOrder := []string{
		HeaderVia,
		HeaderMaxForwards,
		HeaderTo,
		HeaderFrom,
		HeaderCallID,
		HeaderCSeq,
		HeaderContact,
		HeaderExpires,
		HeaderSessionExpires,
		HeaderMinSE,
		HeaderAllow,
		HeaderSupported,
		HeaderRequire,
		HeaderProxyRequire,
		HeaderUnsupported,
		HeaderWWWAuthenticate,
		HeaderAuthorization,
		HeaderProxyAuthenticate,
		HeaderProxyAuthorization,
		HeaderUserAgent,
		HeaderServer,
		HeaderContentType,
		HeaderContentLength,
	}

	// Write headers in preferred order
	writtenHeaders := make(map[string]bool)
	for _, headerName := range headerOrder {
		if values, exists := msg.Headers[headerName]; exists {
			for _, value := range values {
				buffer.WriteString(headerName)
				buffer.WriteString(": ")
				buffer.WriteString(value)
				buffer.WriteString("\r\n")
			}
			writtenHeaders[headerName] = true
		}
	}

	// Write remaining headers not in the preferred order
	for headerName, values := range msg.Headers {
		if !writtenHeaders[headerName] {
			for _, value := range values {
				buffer.WriteString(headerName)
				buffer.WriteString(": ")
				buffer.WriteString(value)
				buffer.WriteString("\r\n")
			}
		}
	}

	// Write empty line to separate headers from body
	buffer.WriteString("\r\n")

	// Write body if present
	if len(msg.Body) > 0 {
		buffer.Write(msg.Body)
	}

	return buffer.Bytes(), nil
}

// Additional header constants for parsing
const (
	HeaderSubject      = "Subject"
	HeaderRoute        = "Route"
	HeaderRecordRoute  = "Record-Route"
	HeaderAccept       = "Accept"
	HeaderAcceptEncoding = "Accept-Encoding"
	HeaderAcceptLanguage = "Accept-Language"
)