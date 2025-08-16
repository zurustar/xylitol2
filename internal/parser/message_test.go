package parser

import (
	"net"
	"testing"
)

func TestNewSIPMessage(t *testing.T) {
	msg := NewSIPMessage()
	if msg == nil {
		t.Fatal("NewSIPMessage() returned nil")
	}
	if msg.Headers == nil {
		t.Error("Headers map should be initialized")
	}
	if len(msg.Headers) != 0 {
		t.Error("Headers map should be empty initially")
	}
}

func TestNewRequestMessage(t *testing.T) {
	method := MethodINVITE
	requestURI := "sip:user@example.com"
	
	msg := NewRequestMessage(method, requestURI)
	if msg == nil {
		t.Fatal("NewRequestMessage() returned nil")
	}
	
	if !msg.IsRequest() {
		t.Error("Message should be a request")
	}
	
	if msg.GetMethod() != method {
		t.Errorf("Expected method %s, got %s", method, msg.GetMethod())
	}
	
	if msg.GetRequestURI() != requestURI {
		t.Errorf("Expected request URI %s, got %s", requestURI, msg.GetRequestURI())
	}
	
	reqLine, ok := msg.StartLine.(*RequestLine)
	if !ok {
		t.Fatal("StartLine should be a RequestLine")
	}
	
	if reqLine.Version != SIPVersion {
		t.Errorf("Expected version %s, got %s", SIPVersion, reqLine.Version)
	}
}

func TestNewResponseMessage(t *testing.T) {
	statusCode := StatusOK
	reasonPhrase := "OK"
	
	msg := NewResponseMessage(statusCode, reasonPhrase)
	if msg == nil {
		t.Fatal("NewResponseMessage() returned nil")
	}
	
	if !msg.IsResponse() {
		t.Error("Message should be a response")
	}
	
	if msg.GetStatusCode() != statusCode {
		t.Errorf("Expected status code %d, got %d", statusCode, msg.GetStatusCode())
	}
	
	if msg.GetReasonPhrase() != reasonPhrase {
		t.Errorf("Expected reason phrase %s, got %s", reasonPhrase, msg.GetReasonPhrase())
	}
	
	statusLine, ok := msg.StartLine.(*StatusLine)
	if !ok {
		t.Fatal("StartLine should be a StatusLine")
	}
	
	if statusLine.Version != SIPVersion {
		t.Errorf("Expected version %s, got %s", SIPVersion, statusLine.Version)
	}
}

func TestRequestLine(t *testing.T) {
	reqLine := &RequestLine{
		Method:     MethodINVITE,
		RequestURI: "sip:user@example.com",
		Version:    SIPVersion,
	}
	
	expected := "INVITE sip:user@example.com SIP/2.0"
	if reqLine.String() != expected {
		t.Errorf("Expected %s, got %s", expected, reqLine.String())
	}
	
	if !reqLine.IsRequest() {
		t.Error("RequestLine should return true for IsRequest()")
	}
}

func TestStatusLine(t *testing.T) {
	statusLine := &StatusLine{
		Version:      SIPVersion,
		StatusCode:   StatusOK,
		ReasonPhrase: "OK",
	}
	
	expected := "SIP/2.0 200 OK"
	if statusLine.String() != expected {
		t.Errorf("Expected %s, got %s", expected, statusLine.String())
	}
	
	if statusLine.IsRequest() {
		t.Error("StatusLine should return false for IsRequest()")
	}
}

func TestSIPMessageHeaders(t *testing.T) {
	msg := NewSIPMessage()
	
	// Test AddHeader
	msg.AddHeader(HeaderFrom, "sip:alice@example.com")
	msg.AddHeader(HeaderTo, "sip:bob@example.com")
	msg.AddHeader(HeaderVia, "SIP/2.0/UDP 192.168.1.1:5060")
	msg.AddHeader(HeaderVia, "SIP/2.0/TCP 192.168.1.2:5060")
	
	// Test GetHeader
	if msg.GetHeader(HeaderFrom) != "sip:alice@example.com" {
		t.Error("GetHeader failed for From header")
	}
	
	// Test GetHeaders for multi-value header
	viaHeaders := msg.GetHeaders(HeaderVia)
	if len(viaHeaders) != 2 {
		t.Errorf("Expected 2 Via headers, got %d", len(viaHeaders))
	}
	
	// Test HasHeader
	if !msg.HasHeader(HeaderFrom) {
		t.Error("HasHeader should return true for existing header")
	}
	
	if msg.HasHeader("NonExistent") {
		t.Error("HasHeader should return false for non-existent header")
	}
	
	// Test SetHeader (should replace existing values)
	msg.SetHeader(HeaderFrom, "sip:charlie@example.com")
	if msg.GetHeader(HeaderFrom) != "sip:charlie@example.com" {
		t.Error("SetHeader failed to replace existing header")
	}
	
	fromHeaders := msg.GetHeaders(HeaderFrom)
	if len(fromHeaders) != 1 {
		t.Errorf("SetHeader should result in single value, got %d", len(fromHeaders))
	}
	
	// Test RemoveHeader
	msg.RemoveHeader(HeaderTo)
	if msg.HasHeader(HeaderTo) {
		t.Error("RemoveHeader failed to remove header")
	}
}

func TestSIPMessageClone(t *testing.T) {
	original := NewRequestMessage(MethodINVITE, "sip:user@example.com")
	original.AddHeader(HeaderFrom, "sip:alice@example.com")
	original.AddHeader(HeaderTo, "sip:bob@example.com")
	original.AddHeader(HeaderVia, "SIP/2.0/UDP 192.168.1.1:5060")
	original.Body = []byte("test body")
	original.Transport = "UDP"
	
	clone := original.Clone()
	
	// Test that clone is not the same object
	if original == clone {
		t.Error("Clone should return a different object")
	}
	
	// Test that start line is cloned
	if original.StartLine == clone.StartLine {
		t.Error("StartLine should be cloned, not shared")
	}
	
	// Test that headers are cloned
	if &original.Headers == &clone.Headers {
		t.Error("Headers map should be cloned, not shared")
	}
	
	// Test that body is cloned
	if &original.Body[0] == &clone.Body[0] {
		t.Error("Body should be cloned, not shared")
	}
	
	// Test that values are equal
	if clone.GetMethod() != original.GetMethod() {
		t.Error("Cloned method should match original")
	}
	
	if clone.GetRequestURI() != original.GetRequestURI() {
		t.Error("Cloned request URI should match original")
	}
	
	if clone.GetHeader(HeaderFrom) != original.GetHeader(HeaderFrom) {
		t.Error("Cloned headers should match original")
	}
	
	if string(clone.Body) != string(original.Body) {
		t.Error("Cloned body should match original")
	}
	
	if clone.Transport != original.Transport {
		t.Error("Cloned transport should match original")
	}
	
	// Test that modifying clone doesn't affect original
	clone.SetHeader(HeaderFrom, "sip:modified@example.com")
	if original.GetHeader(HeaderFrom) == clone.GetHeader(HeaderFrom) {
		t.Error("Modifying clone should not affect original")
	}
}

func TestGetReasonPhraseForCode(t *testing.T) {
	tests := []struct {
		code     int
		expected string
	}{
		{StatusTrying, "Trying"},
		{StatusRinging, "Ringing"},
		{StatusOK, "OK"},
		{StatusBadRequest, "Bad Request"},
		{StatusUnauthorized, "Unauthorized"},
		{StatusNotFound, "Not Found"},
		{StatusMethodNotAllowed, "Method Not Allowed"},
		{StatusExtensionRequired, "Extension Required"},
		{StatusServerInternalError, "Server Internal Error"},
		{StatusServiceUnavailable, "Service Unavailable"},
		{StatusBusyEverywhere, "Busy Everywhere"},
		{999, "Unknown Status Code 999"},
	}
	
	for _, test := range tests {
		result := GetReasonPhraseForCode(test.code)
		if result != test.expected {
			t.Errorf("GetReasonPhraseForCode(%d) = %s, expected %s", 
				test.code, result, test.expected)
		}
	}
}

func TestIsValidMethod(t *testing.T) {
	validMethods := []string{
		MethodINVITE, MethodACK, MethodBYE, MethodCANCEL, MethodREGISTER,
		MethodOPTIONS, MethodINFO, MethodPRACK, MethodUPDATE, MethodSUBSCRIBE,
		MethodNOTIFY, MethodREFER, MethodMESSAGE,
	}
	
	for _, method := range validMethods {
		if !IsValidMethod(method) {
			t.Errorf("IsValidMethod(%s) should return true", method)
		}
	}
	
	invalidMethods := []string{"INVALID", "TEST", "", "invite"}
	for _, method := range invalidMethods {
		if IsValidMethod(method) {
			t.Errorf("IsValidMethod(%s) should return false", method)
		}
	}
}

func TestIsValidStatusCode(t *testing.T) {
	validCodes := []int{100, 200, 300, 400, 500, 600, 699}
	for _, code := range validCodes {
		if !IsValidStatusCode(code) {
			t.Errorf("IsValidStatusCode(%d) should return true", code)
		}
	}
	
	invalidCodes := []int{99, 700, 0, -1, 1000}
	for _, code := range invalidCodes {
		if IsValidStatusCode(code) {
			t.Errorf("IsValidStatusCode(%d) should return false", code)
		}
	}
}

func TestHeader(t *testing.T) {
	header := &Header{
		Name:   HeaderFrom,
		Values: []string{"sip:alice@example.com", "sip:bob@example.com"},
	}
	
	expected := "From: sip:alice@example.com,sip:bob@example.com"
	if header.String() != expected {
		t.Errorf("Header.String() = %s, expected %s", header.String(), expected)
	}
}

func TestSIPMessageWithNetAddr(t *testing.T) {
	msg := NewSIPMessage()
	
	// Create mock addresses
	sourceAddr, _ := net.ResolveUDPAddr("udp", "192.168.1.1:5060")
	destAddr, _ := net.ResolveUDPAddr("udp", "192.168.1.2:5060")
	
	msg.Source = sourceAddr
	msg.Destination = destAddr
	msg.Transport = "UDP"
	
	if msg.Source != sourceAddr {
		t.Error("Source address not set correctly")
	}
	
	if msg.Destination != destAddr {
		t.Error("Destination address not set correctly")
	}
	
	if msg.Transport != "UDP" {
		t.Error("Transport not set correctly")
	}
}

func TestSIPMessageConstants(t *testing.T) {
	// Test that constants are defined correctly
	if SIPVersion != "SIP/2.0" {
		t.Errorf("SIPVersion should be 'SIP/2.0', got %s", SIPVersion)
	}
	
	// Test some method constants
	if MethodINVITE != "INVITE" {
		t.Errorf("MethodINVITE should be 'INVITE', got %s", MethodINVITE)
	}
	
	if MethodREGISTER != "REGISTER" {
		t.Errorf("MethodREGISTER should be 'REGISTER', got %s", MethodREGISTER)
	}
	
	// Test some status code constants
	if StatusOK != 200 {
		t.Errorf("StatusOK should be 200, got %d", StatusOK)
	}
	
	if StatusBadRequest != 400 {
		t.Errorf("StatusBadRequest should be 400, got %d", StatusBadRequest)
	}
	
	// Test some header constants
	if HeaderFrom != "From" {
		t.Errorf("HeaderFrom should be 'From', got %s", HeaderFrom)
	}
	
	if HeaderTo != "To" {
		t.Errorf("HeaderTo should be 'To', got %s", HeaderTo)
	}
}

func TestSIPMessageEmptyHeaders(t *testing.T) {
	msg := NewSIPMessage()
	
	// Test getting non-existent header
	if msg.GetHeader("NonExistent") != "" {
		t.Error("GetHeader should return empty string for non-existent header")
	}
	
	// Test getting headers for non-existent header
	headers := msg.GetHeaders("NonExistent")
	if headers != nil {
		t.Error("GetHeaders should return nil for non-existent header")
	}
}

func TestSIPMessageMethodsForWrongType(t *testing.T) {
	// Test request methods on response message
	respMsg := NewResponseMessage(StatusOK, "OK")
	
	if respMsg.GetMethod() != "" {
		t.Error("GetMethod should return empty string for response message")
	}
	
	if respMsg.GetRequestURI() != "" {
		t.Error("GetRequestURI should return empty string for response message")
	}
	
	// Test response methods on request message
	reqMsg := NewRequestMessage(MethodINVITE, "sip:user@example.com")
	
	if reqMsg.GetStatusCode() != 0 {
		t.Error("GetStatusCode should return 0 for request message")
	}
	
	if reqMsg.GetReasonPhrase() != "" {
		t.Error("GetReasonPhrase should return empty string for request message")
	}
}