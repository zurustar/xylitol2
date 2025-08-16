package parser

import (
	"fmt"
	"net"
	"strconv"
	"strings"
)

// SIP Methods
const (
	MethodINVITE   = "INVITE"
	MethodACK      = "ACK"
	MethodBYE      = "BYE"
	MethodCANCEL   = "CANCEL"
	MethodREGISTER = "REGISTER"
	MethodOPTIONS  = "OPTIONS"
	MethodINFO     = "INFO"
	MethodPRACK    = "PRACK"
	MethodUPDATE   = "UPDATE"
	MethodSUBSCRIBE = "SUBSCRIBE"
	MethodNOTIFY   = "NOTIFY"
	MethodREFER    = "REFER"
	MethodMESSAGE  = "MESSAGE"
)

// SIP Response Codes
const (
	// 1xx Provisional Responses
	StatusTrying               = 100
	StatusRinging              = 180
	StatusCallIsBeingForwarded = 181
	StatusQueued               = 182
	StatusSessionProgress      = 183

	// 2xx Success Responses
	StatusOK = 200

	// 3xx Redirection Responses
	StatusMultipleChoices    = 300
	StatusMovedPermanently   = 301
	StatusMovedTemporarily   = 302
	StatusUseProxy          = 305
	StatusAlternativeService = 380

	// 4xx Client Error Responses
	StatusBadRequest                = 400
	StatusUnauthorized              = 401
	StatusPaymentRequired           = 402
	StatusForbidden                 = 403
	StatusNotFound                  = 404
	StatusMethodNotAllowed          = 405
	StatusNotAcceptable             = 406
	StatusProxyAuthenticationRequired = 407
	StatusRequestTimeout            = 408
	StatusGone                      = 410
	StatusRequestEntityTooLarge     = 413
	StatusRequestURITooLong         = 414
	StatusUnsupportedMediaType      = 415
	StatusUnsupportedURIScheme      = 416
	StatusBadExtension              = 420
	StatusExtensionRequired         = 421
	StatusIntervalTooBrief          = 423
	StatusTemporarilyUnavailable    = 480
	StatusCallTransactionDoesNotExist = 481
	StatusLoopDetected              = 482
	StatusTooManyHops               = 483
	StatusAddressIncomplete         = 484
	StatusAmbiguous                 = 485
	StatusBusyHere                  = 486
	StatusRequestTerminated         = 487
	StatusNotAcceptableHere         = 488
	StatusRequestPending            = 491
	StatusUndecipherable            = 493

	// 5xx Server Error Responses
	StatusServerInternalError = 500
	StatusNotImplemented      = 501
	StatusBadGateway          = 502
	StatusServiceUnavailable  = 503
	StatusServerTimeout       = 504
	StatusVersionNotSupported = 505
	StatusMessageTooLarge     = 513

	// 6xx Global Failure Responses
	StatusBusyEverywhere     = 600
	StatusDecline            = 603
	StatusDoesNotExistAnywhere = 604
	StatusNotAcceptableGlobal  = 606
)

// SIP Version
const SIPVersion = "SIP/2.0"

// Common SIP Headers
const (
	HeaderVia             = "Via"
	HeaderFrom            = "From"
	HeaderTo              = "To"
	HeaderCallID          = "Call-ID"
	HeaderCSeq            = "CSeq"
	HeaderMaxForwards     = "Max-Forwards"
	HeaderContact         = "Contact"
	HeaderExpires         = "Expires"
	HeaderContentType     = "Content-Type"
	HeaderContentLength   = "Content-Length"
	HeaderUserAgent       = "User-Agent"
	HeaderServer          = "Server"
	HeaderAllow           = "Allow"
	HeaderSupported       = "Supported"
	HeaderRequire         = "Require"
	HeaderProxyRequire    = "Proxy-Require"
	HeaderUnsupported     = "Unsupported"
	HeaderWWWAuthenticate = "WWW-Authenticate"
	HeaderAuthorization   = "Authorization"
	HeaderProxyAuthenticate = "Proxy-Authenticate"
	HeaderProxyAuthorization = "Proxy-Authorization"
	HeaderSessionExpires  = "Session-Expires"
	HeaderMinSE           = "Min-SE"
)

// SIPMessage represents a complete SIP message
type SIPMessage struct {
	StartLine   StartLine
	Headers     map[string][]string
	Body        []byte
	Transport   string
	Source      net.Addr
	Destination net.Addr
}

// StartLine interface for request and status lines
type StartLine interface {
	String() string
	IsRequest() bool
}

// RequestLine represents a SIP request line
type RequestLine struct {
	Method     string
	RequestURI string
	Version    string
}

// String returns the string representation of the request line
func (r *RequestLine) String() string {
	return r.Method + " " + r.RequestURI + " " + r.Version
}

// IsRequest returns true for request lines
func (r *RequestLine) IsRequest() bool {
	return true
}

// StatusLine represents a SIP status line
type StatusLine struct {
	Version      string
	StatusCode   int
	ReasonPhrase string
}

// String returns the string representation of the status line
func (s *StatusLine) String() string {
	return s.Version + " " + strconv.Itoa(s.StatusCode) + " " + s.ReasonPhrase
}

// IsRequest returns false for status lines
func (s *StatusLine) IsRequest() bool {
	return false
}

// Header represents a SIP header with name and values
type Header struct {
	Name   string
	Values []string
}

// String returns the string representation of the header
func (h *Header) String() string {
	return h.Name + ": " + strings.Join(h.Values, ",")
}

// NewSIPMessage creates a new SIP message
func NewSIPMessage() *SIPMessage {
	return &SIPMessage{
		Headers: make(map[string][]string),
	}
}

// NewRequestMessage creates a new SIP request message
func NewRequestMessage(method, requestURI string) *SIPMessage {
	msg := NewSIPMessage()
	msg.StartLine = &RequestLine{
		Method:     method,
		RequestURI: requestURI,
		Version:    SIPVersion,
	}
	return msg
}

// NewResponseMessage creates a new SIP response message
func NewResponseMessage(statusCode int, reasonPhrase string) *SIPMessage {
	msg := NewSIPMessage()
	msg.StartLine = &StatusLine{
		Version:      SIPVersion,
		StatusCode:   statusCode,
		ReasonPhrase: reasonPhrase,
	}
	return msg
}

// AddHeader adds a header to the message
func (m *SIPMessage) AddHeader(name, value string) {
	if m.Headers == nil {
		m.Headers = make(map[string][]string)
	}
	m.Headers[name] = append(m.Headers[name], value)
}

// SetHeader sets a header value, replacing any existing values
func (m *SIPMessage) SetHeader(name, value string) {
	if m.Headers == nil {
		m.Headers = make(map[string][]string)
	}
	m.Headers[name] = []string{value}
}

// GetHeader returns the first value of a header
func (m *SIPMessage) GetHeader(name string) string {
	if values, exists := m.Headers[name]; exists && len(values) > 0 {
		return values[0]
	}
	return ""
}

// GetHeaders returns all values of a header
func (m *SIPMessage) GetHeaders(name string) []string {
	if values, exists := m.Headers[name]; exists {
		return values
	}
	return nil
}

// HasHeader checks if a header exists
func (m *SIPMessage) HasHeader(name string) bool {
	_, exists := m.Headers[name]
	return exists
}

// RemoveHeader removes a header from the message
func (m *SIPMessage) RemoveHeader(name string) {
	delete(m.Headers, name)
}

// IsRequest returns true if the message is a request
func (m *SIPMessage) IsRequest() bool {
	return m.StartLine != nil && m.StartLine.IsRequest()
}

// IsResponse returns true if the message is a response
func (m *SIPMessage) IsResponse() bool {
	return m.StartLine != nil && !m.StartLine.IsRequest()
}

// GetMethod returns the method for request messages
func (m *SIPMessage) GetMethod() string {
	if req, ok := m.StartLine.(*RequestLine); ok {
		return req.Method
	}
	return ""
}

// GetStatusCode returns the status code for response messages
func (m *SIPMessage) GetStatusCode() int {
	if resp, ok := m.StartLine.(*StatusLine); ok {
		return resp.StatusCode
	}
	return 0
}

// GetReasonPhrase returns the reason phrase for response messages
func (m *SIPMessage) GetReasonPhrase() string {
	if resp, ok := m.StartLine.(*StatusLine); ok {
		return resp.ReasonPhrase
	}
	return ""
}

// GetRequestURI returns the request URI for request messages
func (m *SIPMessage) GetRequestURI() string {
	if req, ok := m.StartLine.(*RequestLine); ok {
		return req.RequestURI
	}
	return ""
}

// Clone creates a deep copy of the SIP message
func (m *SIPMessage) Clone() *SIPMessage {
	clone := &SIPMessage{
		Headers:     make(map[string][]string),
		Body:        make([]byte, len(m.Body)),
		Transport:   m.Transport,
		Source:      m.Source,
		Destination: m.Destination,
	}

	// Copy body
	copy(clone.Body, m.Body)

	// Copy headers
	for name, values := range m.Headers {
		clone.Headers[name] = make([]string, len(values))
		copy(clone.Headers[name], values)
	}

	// Copy start line
	if req, ok := m.StartLine.(*RequestLine); ok {
		clone.StartLine = &RequestLine{
			Method:     req.Method,
			RequestURI: req.RequestURI,
			Version:    req.Version,
		}
	} else if resp, ok := m.StartLine.(*StatusLine); ok {
		clone.StartLine = &StatusLine{
			Version:      resp.Version,
			StatusCode:   resp.StatusCode,
			ReasonPhrase: resp.ReasonPhrase,
		}
	}

	return clone
}

// GetReasonPhraseForCode returns the standard reason phrase for a status code
func GetReasonPhraseForCode(code int) string {
	switch code {
	case StatusTrying:
		return "Trying"
	case StatusRinging:
		return "Ringing"
	case StatusCallIsBeingForwarded:
		return "Call Is Being Forwarded"
	case StatusQueued:
		return "Queued"
	case StatusSessionProgress:
		return "Session Progress"
	case StatusOK:
		return "OK"
	case StatusMultipleChoices:
		return "Multiple Choices"
	case StatusMovedPermanently:
		return "Moved Permanently"
	case StatusMovedTemporarily:
		return "Moved Temporarily"
	case StatusUseProxy:
		return "Use Proxy"
	case StatusAlternativeService:
		return "Alternative Service"
	case StatusBadRequest:
		return "Bad Request"
	case StatusUnauthorized:
		return "Unauthorized"
	case StatusPaymentRequired:
		return "Payment Required"
	case StatusForbidden:
		return "Forbidden"
	case StatusNotFound:
		return "Not Found"
	case StatusMethodNotAllowed:
		return "Method Not Allowed"
	case StatusNotAcceptable:
		return "Not Acceptable"
	case StatusProxyAuthenticationRequired:
		return "Proxy Authentication Required"
	case StatusRequestTimeout:
		return "Request Timeout"
	case StatusGone:
		return "Gone"
	case StatusRequestEntityTooLarge:
		return "Request Entity Too Large"
	case StatusRequestURITooLong:
		return "Request-URI Too Long"
	case StatusUnsupportedMediaType:
		return "Unsupported Media Type"
	case StatusUnsupportedURIScheme:
		return "Unsupported URI Scheme"
	case StatusBadExtension:
		return "Bad Extension"
	case StatusExtensionRequired:
		return "Extension Required"
	case StatusIntervalTooBrief:
		return "Interval Too Brief"
	case StatusTemporarilyUnavailable:
		return "Temporarily Unavailable"
	case StatusCallTransactionDoesNotExist:
		return "Call/Transaction Does Not Exist"
	case StatusLoopDetected:
		return "Loop Detected"
	case StatusTooManyHops:
		return "Too Many Hops"
	case StatusAddressIncomplete:
		return "Address Incomplete"
	case StatusAmbiguous:
		return "Ambiguous"
	case StatusBusyHere:
		return "Busy Here"
	case StatusRequestTerminated:
		return "Request Terminated"
	case StatusNotAcceptableHere:
		return "Not Acceptable Here"
	case StatusRequestPending:
		return "Request Pending"
	case StatusUndecipherable:
		return "Undecipherable"
	case StatusServerInternalError:
		return "Server Internal Error"
	case StatusNotImplemented:
		return "Not Implemented"
	case StatusBadGateway:
		return "Bad Gateway"
	case StatusServiceUnavailable:
		return "Service Unavailable"
	case StatusServerTimeout:
		return "Server Time-out"
	case StatusVersionNotSupported:
		return "Version Not Supported"
	case StatusMessageTooLarge:
		return "Message Too Large"
	case StatusBusyEverywhere:
		return "Busy Everywhere"
	case StatusDecline:
		return "Decline"
	case StatusDoesNotExistAnywhere:
		return "Does Not Exist Anywhere"
	case StatusNotAcceptableGlobal:
		return "Not Acceptable"
	default:
		return fmt.Sprintf("Unknown Status Code %d", code)
	}
}

// IsValidMethod checks if a method is valid
func IsValidMethod(method string) bool {
	switch method {
	case MethodINVITE, MethodACK, MethodBYE, MethodCANCEL, MethodREGISTER,
		MethodOPTIONS, MethodINFO, MethodPRACK, MethodUPDATE, MethodSUBSCRIBE,
		MethodNOTIFY, MethodREFER, MethodMESSAGE:
		return true
	default:
		return false
	}
}

// IsValidStatusCode checks if a status code is valid
func IsValidStatusCode(code int) bool {
	return code >= 100 && code <= 699
}