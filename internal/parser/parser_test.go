package parser

import (
	"fmt"
	"strings"
	"testing"
)

func TestParseINVITERequest(t *testing.T) {
	sipMessage := `INVITE sip:bob@example.com SIP/2.0
Via: SIP/2.0/UDP 192.168.1.1:5060;branch=z9hG4bK776asdhds
Max-Forwards: 70
To: Bob <sip:bob@example.com>
From: Alice <sip:alice@example.com>;tag=1928301774
Call-ID: a84b4c76e66710@pc33.example.com
CSeq: 314159 INVITE
Contact: <sip:alice@192.168.1.1:5060>
Content-Type: application/sdp
Content-Length: 128

v=0
o=alice 2890844526 2890844526 IN IP4 192.168.1.1
s=-
c=IN IP4 192.168.1.1
t=0 0
m=audio 49170 RTP/AVP 0
a=rtpmap:0 PCMU/8000`

	parser := NewParser()
	msg, err := parser.Parse([]byte(sipMessage))
	if err != nil {
		t.Fatalf("Failed to parse INVITE request: %v", err)
	}

	// Test start line
	if !msg.IsRequest() {
		t.Error("Message should be a request")
	}

	if msg.GetMethod() != MethodINVITE {
		t.Errorf("Expected method %s, got %s", MethodINVITE, msg.GetMethod())
	}

	if msg.GetRequestURI() != "sip:bob@example.com" {
		t.Errorf("Expected request URI sip:bob@example.com, got %s", msg.GetRequestURI())
	}

	// Test headers
	expectedHeaders := map[string]string{
		HeaderVia:           "SIP/2.0/UDP 192.168.1.1:5060;branch=z9hG4bK776asdhds",
		HeaderMaxForwards:   "70",
		HeaderTo:            "Bob <sip:bob@example.com>",
		HeaderFrom:          "Alice <sip:alice@example.com>;tag=1928301774",
		HeaderCallID:        "a84b4c76e66710@pc33.example.com",
		HeaderCSeq:          "314159 INVITE",
		HeaderContact:       "<sip:alice@192.168.1.1:5060>",
		HeaderContentType:   "application/sdp",
		HeaderContentLength: "128",
	}

	for header, expectedValue := range expectedHeaders {
		actualValue := msg.GetHeader(header)
		if actualValue != expectedValue {
			t.Errorf("Header %s: expected %s, got %s", header, expectedValue, actualValue)
		}
	}

	// Test body
	expectedBodyLength := 128
	if len(msg.Body) != expectedBodyLength {
		t.Errorf("Expected body length %d, got %d", expectedBodyLength, len(msg.Body))
	}

	bodyStr := string(msg.Body)
	if !strings.Contains(bodyStr, "v=0") {
		t.Error("Body should contain SDP content")
	}
}

func TestParseOKResponse(t *testing.T) {
	sipMessage := `SIP/2.0 200 OK
Via: SIP/2.0/UDP 192.168.1.1:5060;branch=z9hG4bK776asdhds
To: Bob <sip:bob@example.com>;tag=a6c85cf
From: Alice <sip:alice@example.com>;tag=1928301774
Call-ID: a84b4c76e66710@pc33.example.com
CSeq: 314159 INVITE
Contact: <sip:bob@192.168.1.2:5060>
Content-Type: application/sdp
Content-Length: 126

v=0
o=bob 2890844527 2890844527 IN IP4 192.168.1.2
s=-
c=IN IP4 192.168.1.2
t=0 0
m=audio 49172 RTP/AVP 0
a=rtpmap:0 PCMU/8000`

	parser := NewParser()
	msg, err := parser.Parse([]byte(sipMessage))
	if err != nil {
		t.Fatalf("Failed to parse 200 OK response: %v", err)
	}

	// Test start line
	if !msg.IsResponse() {
		t.Error("Message should be a response")
	}

	if msg.GetStatusCode() != StatusOK {
		t.Errorf("Expected status code %d, got %d", StatusOK, msg.GetStatusCode())
	}

	if msg.GetReasonPhrase() != "OK" {
		t.Errorf("Expected reason phrase OK, got %s", msg.GetReasonPhrase())
	}

	// Test that response methods return empty for requests
	if msg.GetMethod() != "" {
		t.Error("GetMethod should return empty for response")
	}

	if msg.GetRequestURI() != "" {
		t.Error("GetRequestURI should return empty for response")
	}
}

func TestParseREGISTERRequest(t *testing.T) {
	sipMessage := `REGISTER sip:example.com SIP/2.0
Via: SIP/2.0/UDP 192.168.1.1:5060;branch=z9hG4bK623asdhds
Max-Forwards: 70
To: Alice <sip:alice@example.com>
From: Alice <sip:alice@example.com>;tag=456248
Call-ID: 843817637684230@998sdasdh09
CSeq: 1826 REGISTER
Contact: <sip:alice@192.168.1.1:5060>
Expires: 7200
Content-Length: 0

`

	parser := NewParser()
	msg, err := parser.Parse([]byte(sipMessage))
	if err != nil {
		t.Fatalf("Failed to parse REGISTER request: %v", err)
	}

	if msg.GetMethod() != MethodREGISTER {
		t.Errorf("Expected method %s, got %s", MethodREGISTER, msg.GetMethod())
	}

	if msg.GetHeader(HeaderExpires) != "7200" {
		t.Errorf("Expected Expires 7200, got %s", msg.GetHeader(HeaderExpires))
	}

	if len(msg.Body) != 0 {
		t.Errorf("Expected empty body, got %d bytes", len(msg.Body))
	}
}

func TestParseMultiValueHeaders(t *testing.T) {
	sipMessage := `INVITE sip:bob@example.com SIP/2.0
Via: SIP/2.0/UDP 192.168.1.1:5060;branch=z9hG4bK776asdhds, SIP/2.0/TCP 192.168.1.2:5060;branch=z9hG4bK123456
Max-Forwards: 70
To: Bob <sip:bob@example.com>
From: Alice <sip:alice@example.com>;tag=1928301774
Call-ID: a84b4c76e66710@pc33.example.com
CSeq: 314159 INVITE
Contact: <sip:alice@192.168.1.1:5060>, <sip:alice@192.168.1.3:5060>
Allow: INVITE, ACK, BYE, CANCEL, OPTIONS
Content-Length: 0

`

	parser := NewParser()
	msg, err := parser.Parse([]byte(sipMessage))
	if err != nil {
		t.Fatalf("Failed to parse message with multi-value headers: %v", err)
	}

	// Test Via headers
	viaHeaders := msg.GetHeaders(HeaderVia)
	if len(viaHeaders) != 2 {
		t.Errorf("Expected 2 Via headers, got %d", len(viaHeaders))
	}

	expectedVia1 := "SIP/2.0/UDP 192.168.1.1:5060;branch=z9hG4bK776asdhds"
	expectedVia2 := "SIP/2.0/TCP 192.168.1.2:5060;branch=z9hG4bK123456"
	if viaHeaders[0] != expectedVia1 {
		t.Errorf("Expected first Via %s, got %s", expectedVia1, viaHeaders[0])
	}
	if viaHeaders[1] != expectedVia2 {
		t.Errorf("Expected second Via %s, got %s", expectedVia2, viaHeaders[1])
	}

	// Test Contact headers
	contactHeaders := msg.GetHeaders(HeaderContact)
	if len(contactHeaders) != 2 {
		t.Errorf("Expected 2 Contact headers, got %d", len(contactHeaders))
	}

	// Test Allow header (should be parsed as multiple values)
	allowHeaders := msg.GetHeaders(HeaderAllow)
	if len(allowHeaders) != 5 {
		t.Errorf("Expected 5 Allow values, got %d", len(allowHeaders))
	}
}

func TestParseCompactHeaders(t *testing.T) {
	sipMessage := `INVITE sip:bob@example.com SIP/2.0
v: SIP/2.0/UDP 192.168.1.1:5060;branch=z9hG4bK776asdhds
Max-Forwards: 70
t: Bob <sip:bob@example.com>
f: Alice <sip:alice@example.com>;tag=1928301774
i: a84b4c76e66710@pc33.example.com
CSeq: 314159 INVITE
m: <sip:alice@192.168.1.1:5060>
c: application/sdp
l: 0

`

	parser := NewParser()
	msg, err := parser.Parse([]byte(sipMessage))
	if err != nil {
		t.Fatalf("Failed to parse message with compact headers: %v", err)
	}

	// Test that compact headers are expanded
	if msg.GetHeader(HeaderVia) == "" {
		t.Error("Compact Via header (v) should be expanded to Via")
	}

	if msg.GetHeader(HeaderTo) == "" {
		t.Error("Compact To header (t) should be expanded to To")
	}

	if msg.GetHeader(HeaderFrom) == "" {
		t.Error("Compact From header (f) should be expanded to From")
	}

	if msg.GetHeader(HeaderCallID) == "" {
		t.Error("Compact Call-ID header (i) should be expanded to Call-ID")
	}

	if msg.GetHeader(HeaderContact) == "" {
		t.Error("Compact Contact header (m) should be expanded to Contact")
	}

	if msg.GetHeader(HeaderContentType) == "" {
		t.Error("Compact Content-Type header (c) should be expanded to Content-Type")
	}

	if msg.GetHeader(HeaderContentLength) == "" {
		t.Error("Compact Content-Length header (l) should be expanded to Content-Length")
	}
}

func TestParseHeaderFolding(t *testing.T) {
	sipMessage := `INVITE sip:bob@example.com SIP/2.0
Via: SIP/2.0/UDP 192.168.1.1:5060;branch=z9hG4bK776asdhds
Max-Forwards: 70
To: Bob <sip:bob@example.com>
From: Alice <sip:alice@example.com>;tag=1928301774
Call-ID: a84b4c76e66710@pc33.example.com
CSeq: 314159 INVITE
Contact: <sip:alice@192.168.1.1:5060>
Subject: This is a very long subject line
 that continues on the next line
 and even on a third line
Content-Length: 0

`

	parser := NewParser()
	msg, err := parser.Parse([]byte(sipMessage))
	if err != nil {
		t.Fatalf("Failed to parse message with header folding: %v", err)
	}

	subject := msg.GetHeader(HeaderSubject)
	expectedSubject := "This is a very long subject line that continues on the next line and even on a third line"
	if subject != expectedSubject {
		t.Errorf("Expected subject %s, got %s", expectedSubject, subject)
	}
}

func TestParseErrors(t *testing.T) {
	testCases := []struct {
		name    string
		message string
		wantErr bool
	}{
		{
			name:    "Empty message",
			message: "",
			wantErr: true,
		},
		{
			name:    "Invalid start line",
			message: "INVALID\r\n\r\n",
			wantErr: true,
		},
		{
			name: "Invalid method",
			message: `INVALID sip:bob@example.com SIP/2.0
Via: SIP/2.0/UDP 192.168.1.1:5060
From: Alice <sip:alice@example.com>
To: Bob <sip:bob@example.com>
Call-ID: test
CSeq: 1 INVALID
Content-Length: 0

`,
			wantErr: true,
		},
		{
			name: "Invalid status code",
			message: `SIP/2.0 ABC OK
Via: SIP/2.0/UDP 192.168.1.1:5060
From: Alice <sip:alice@example.com>
To: Bob <sip:bob@example.com>
Call-ID: test
CSeq: 1 INVITE
Content-Length: 0

`,
			wantErr: true,
		},
		{
			name: "Header without colon",
			message: `INVITE sip:bob@example.com SIP/2.0
Via: SIP/2.0/UDP 192.168.1.1:5060
InvalidHeader
From: Alice <sip:alice@example.com>
To: Bob <sip:bob@example.com>
Call-ID: test
CSeq: 1 INVITE
Content-Length: 0

`,
			wantErr: true,
		},
		{
			name: "Invalid Content-Length",
			message: `INVITE sip:bob@example.com SIP/2.0
Via: SIP/2.0/UDP 192.168.1.1:5060
From: Alice <sip:alice@example.com>
To: Bob <sip:bob@example.com>
Call-ID: test
CSeq: 1 INVITE
Content-Length: ABC

`,
			wantErr: true,
		},
		{
			name: "Negative Content-Length",
			message: `INVITE sip:bob@example.com SIP/2.0
Via: SIP/2.0/UDP 192.168.1.1:5060
From: Alice <sip:alice@example.com>
To: Bob <sip:bob@example.com>
Call-ID: test
CSeq: 1 INVITE
Content-Length: -1

`,
			wantErr: true,
		},
	}

	parser := NewParser()
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := parser.Parse([]byte(tc.message))
			if tc.wantErr && err == nil {
				t.Error("Expected error but got none")
			}
			if !tc.wantErr && err != nil {
				t.Errorf("Expected no error but got: %v", err)
			}
		})
	}
}

func TestValidateMessage(t *testing.T) {
	parser := NewParser()

	// Test valid message
	validMessage := `INVITE sip:bob@example.com SIP/2.0
Via: SIP/2.0/UDP 192.168.1.1:5060;branch=z9hG4bK776asdhds
Max-Forwards: 70
To: Bob <sip:bob@example.com>
From: Alice <sip:alice@example.com>;tag=1928301774
Call-ID: a84b4c76e66710@pc33.example.com
CSeq: 314159 INVITE
Content-Length: 0

`

	msg, err := parser.Parse([]byte(validMessage))
	if err != nil {
		t.Fatalf("Failed to parse valid message: %v", err)
	}

	err = parser.Validate(msg)
	if err != nil {
		t.Errorf("Valid message should pass validation: %v", err)
	}

	// Test message missing required headers
	invalidMessage := `INVITE sip:bob@example.com SIP/2.0
Via: SIP/2.0/UDP 192.168.1.1:5060;branch=z9hG4bK776asdhds
Max-Forwards: 70
Content-Length: 0

`

	msg2, err := parser.Parse([]byte(invalidMessage))
	if err != nil {
		t.Fatalf("Failed to parse message: %v", err)
	}

	err = parser.Validate(msg2)
	if err == nil {
		t.Error("Message missing required headers should fail validation")
	}
}

func TestValidateCSeq(t *testing.T) {
	parser := NewParser()

	testCases := []struct {
		name    string
		cseq    string
		method  string
		wantErr bool
	}{
		{
			name:    "Valid CSeq",
			cseq:    "314159 INVITE",
			method:  MethodINVITE,
			wantErr: false,
		},
		{
			name:    "Invalid CSeq format",
			cseq:    "314159",
			method:  MethodINVITE,
			wantErr: true,
		},
		{
			name:    "Invalid CSeq number",
			cseq:    "ABC INVITE",
			method:  MethodINVITE,
			wantErr: true,
		},
		{
			name:    "Zero CSeq number",
			cseq:    "0 INVITE",
			method:  MethodINVITE,
			wantErr: true,
		},
		{
			name:    "Method mismatch",
			cseq:    "314159 BYE",
			method:  MethodINVITE,
			wantErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			msg := NewRequestMessage(tc.method, "sip:test@example.com")
			msg.SetHeader(HeaderVia, "SIP/2.0/UDP 192.168.1.1:5060")
			msg.SetHeader(HeaderMaxForwards, "70")
			msg.SetHeader(HeaderTo, "sip:test@example.com")
			msg.SetHeader(HeaderFrom, "sip:test@example.com")
			msg.SetHeader(HeaderCallID, "test")
			msg.SetHeader(HeaderCSeq, tc.cseq)
			msg.SetHeader(HeaderContentLength, "0")

			err := parser.Validate(msg)
			if tc.wantErr && err == nil {
				t.Error("Expected validation error but got none")
			}
			if !tc.wantErr && err != nil {
				t.Errorf("Expected no validation error but got: %v", err)
			}
		})
	}
}

func TestValidateMaxForwards(t *testing.T) {
	parser := NewParser()

	testCases := []struct {
		name        string
		maxForwards string
		wantErr     bool
	}{
		{"Valid Max-Forwards", "70", false},
		{"Zero Max-Forwards", "0", false},
		{"Max Max-Forwards", "255", false},
		{"Invalid Max-Forwards", "ABC", true},
		{"Negative Max-Forwards", "-1", true},
		{"Too large Max-Forwards", "256", true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			msg := NewRequestMessage(MethodINVITE, "sip:test@example.com")
			msg.SetHeader(HeaderVia, "SIP/2.0/UDP 192.168.1.1:5060")
			msg.SetHeader(HeaderMaxForwards, tc.maxForwards)
			msg.SetHeader(HeaderTo, "sip:test@example.com")
			msg.SetHeader(HeaderFrom, "sip:test@example.com")
			msg.SetHeader(HeaderCallID, "test")
			msg.SetHeader(HeaderCSeq, "1 INVITE")
			msg.SetHeader(HeaderContentLength, "0")

			err := parser.Validate(msg)
			if tc.wantErr && err == nil {
				t.Error("Expected validation error but got none")
			}
			if !tc.wantErr && err != nil {
				t.Errorf("Expected no validation error but got: %v", err)
			}
		})
	}
}
func TestSerializeINVITERequest(t *testing.T) {
	// Create a SIP INVITE message
	msg := NewRequestMessage(MethodINVITE, "sip:bob@example.com")
	msg.AddHeader(HeaderVia, "SIP/2.0/UDP 192.168.1.1:5060;branch=z9hG4bK776asdhds")
	msg.AddHeader(HeaderMaxForwards, "70")
	msg.AddHeader(HeaderTo, "Bob <sip:bob@example.com>")
	msg.AddHeader(HeaderFrom, "Alice <sip:alice@example.com>;tag=1928301774")
	msg.AddHeader(HeaderCallID, "a84b4c76e66710@pc33.example.com")
	msg.AddHeader(HeaderCSeq, "314159 INVITE")
	msg.AddHeader(HeaderContact, "<sip:alice@192.168.1.1:5060>")
	msg.AddHeader(HeaderContentType, "application/sdp")
	msg.SetHeader(HeaderContentLength, "128")
	
	sdpBody := `v=0
o=alice 2890844526 2890844526 IN IP4 192.168.1.1
s=-
c=IN IP4 192.168.1.1
t=0 0
m=audio 49170 RTP/AVP 0
a=rtpmap:0 PCMU/8000`
	msg.Body = []byte(sdpBody)

	parser := NewParser()
	serialized, err := parser.Serialize(msg)
	if err != nil {
		t.Fatalf("Failed to serialize INVITE request: %v", err)
	}

	// Parse it back to verify round-trip
	parsed, err := parser.Parse(serialized)
	if err != nil {
		t.Fatalf("Failed to parse serialized message: %v", err)
	}

	// Verify start line
	if parsed.GetMethod() != msg.GetMethod() {
		t.Errorf("Method mismatch: expected %s, got %s", msg.GetMethod(), parsed.GetMethod())
	}

	if parsed.GetRequestURI() != msg.GetRequestURI() {
		t.Errorf("Request URI mismatch: expected %s, got %s", msg.GetRequestURI(), parsed.GetRequestURI())
	}

	// Verify headers
	expectedHeaders := []string{HeaderVia, HeaderMaxForwards, HeaderTo, HeaderFrom, 
		HeaderCallID, HeaderCSeq, HeaderContact, HeaderContentType, HeaderContentLength}
	
	for _, header := range expectedHeaders {
		if parsed.GetHeader(header) != msg.GetHeader(header) {
			t.Errorf("Header %s mismatch: expected %s, got %s", 
				header, msg.GetHeader(header), parsed.GetHeader(header))
		}
	}

	// Verify body
	if string(parsed.Body) != string(msg.Body) {
		t.Errorf("Body mismatch: expected %s, got %s", string(msg.Body), string(parsed.Body))
	}
}

func TestSerializeOKResponse(t *testing.T) {
	// Create a SIP 200 OK response
	msg := NewResponseMessage(StatusOK, "OK")
	msg.AddHeader(HeaderVia, "SIP/2.0/UDP 192.168.1.1:5060;branch=z9hG4bK776asdhds")
	msg.AddHeader(HeaderTo, "Bob <sip:bob@example.com>;tag=a6c85cf")
	msg.AddHeader(HeaderFrom, "Alice <sip:alice@example.com>;tag=1928301774")
	msg.AddHeader(HeaderCallID, "a84b4c76e66710@pc33.example.com")
	msg.AddHeader(HeaderCSeq, "314159 INVITE")
	msg.AddHeader(HeaderContact, "<sip:bob@192.168.1.2:5060>")
	msg.AddHeader(HeaderContentType, "application/sdp")
	msg.SetHeader(HeaderContentLength, "126")
	
	sdpBody := `v=0
o=bob 2890844527 2890844527 IN IP4 192.168.1.2
s=-
c=IN IP4 192.168.1.2
t=0 0
m=audio 49172 RTP/AVP 0
a=rtpmap:0 PCMU/8000`
	msg.Body = []byte(sdpBody)

	parser := NewParser()
	serialized, err := parser.Serialize(msg)
	if err != nil {
		t.Fatalf("Failed to serialize 200 OK response: %v", err)
	}

	// Parse it back to verify round-trip
	parsed, err := parser.Parse(serialized)
	if err != nil {
		t.Fatalf("Failed to parse serialized message: %v", err)
	}

	// Verify start line
	if parsed.GetStatusCode() != msg.GetStatusCode() {
		t.Errorf("Status code mismatch: expected %d, got %d", msg.GetStatusCode(), parsed.GetStatusCode())
	}

	if parsed.GetReasonPhrase() != msg.GetReasonPhrase() {
		t.Errorf("Reason phrase mismatch: expected %s, got %s", msg.GetReasonPhrase(), parsed.GetReasonPhrase())
	}

	// Verify body
	if string(parsed.Body) != string(msg.Body) {
		t.Errorf("Body mismatch: expected %s, got %s", string(msg.Body), string(parsed.Body))
	}
}

func TestSerializeREGISTERRequest(t *testing.T) {
	// Create a SIP REGISTER message
	msg := NewRequestMessage(MethodREGISTER, "sip:example.com")
	msg.AddHeader(HeaderVia, "SIP/2.0/UDP 192.168.1.1:5060;branch=z9hG4bK623asdhds")
	msg.AddHeader(HeaderMaxForwards, "70")
	msg.AddHeader(HeaderTo, "Alice <sip:alice@example.com>")
	msg.AddHeader(HeaderFrom, "Alice <sip:alice@example.com>;tag=456248")
	msg.AddHeader(HeaderCallID, "843817637684230@998sdasdh09")
	msg.AddHeader(HeaderCSeq, "1826 REGISTER")
	msg.AddHeader(HeaderContact, "<sip:alice@192.168.1.1:5060>")
	msg.AddHeader(HeaderExpires, "7200")
	msg.SetHeader(HeaderContentLength, "0")

	parser := NewParser()
	serialized, err := parser.Serialize(msg)
	if err != nil {
		t.Fatalf("Failed to serialize REGISTER request: %v", err)
	}

	// Parse it back to verify round-trip
	parsed, err := parser.Parse(serialized)
	if err != nil {
		t.Fatalf("Failed to parse serialized message: %v", err)
	}

	// Verify start line
	if parsed.GetMethod() != MethodREGISTER {
		t.Errorf("Method mismatch: expected %s, got %s", MethodREGISTER, parsed.GetMethod())
	}

	if parsed.GetRequestURI() != "sip:example.com" {
		t.Errorf("Request URI mismatch: expected sip:example.com, got %s", parsed.GetRequestURI())
	}

	// Verify Expires header
	if parsed.GetHeader(HeaderExpires) != "7200" {
		t.Errorf("Expires header mismatch: expected 7200, got %s", parsed.GetHeader(HeaderExpires))
	}

	// Verify no body
	if len(parsed.Body) != 0 {
		t.Errorf("Expected empty body, got %d bytes", len(parsed.Body))
	}
}

func TestSerializeMultiValueHeaders(t *testing.T) {
	// Create a message with multi-value headers
	msg := NewRequestMessage(MethodINVITE, "sip:bob@example.com")
	msg.AddHeader(HeaderVia, "SIP/2.0/UDP 192.168.1.1:5060;branch=z9hG4bK776asdhds")
	msg.AddHeader(HeaderVia, "SIP/2.0/TCP 192.168.1.2:5060;branch=z9hG4bK123456")
	msg.AddHeader(HeaderMaxForwards, "70")
	msg.AddHeader(HeaderTo, "Bob <sip:bob@example.com>")
	msg.AddHeader(HeaderFrom, "Alice <sip:alice@example.com>;tag=1928301774")
	msg.AddHeader(HeaderCallID, "a84b4c76e66710@pc33.example.com")
	msg.AddHeader(HeaderCSeq, "314159 INVITE")
	msg.AddHeader(HeaderContact, "<sip:alice@192.168.1.1:5060>")
	msg.AddHeader(HeaderContact, "<sip:alice@192.168.1.3:5060>")
	msg.AddHeader(HeaderAllow, "INVITE")
	msg.AddHeader(HeaderAllow, "ACK")
	msg.AddHeader(HeaderAllow, "BYE")
	msg.SetHeader(HeaderContentLength, "0")

	parser := NewParser()
	serialized, err := parser.Serialize(msg)
	if err != nil {
		t.Fatalf("Failed to serialize message with multi-value headers: %v", err)
	}

	// Parse it back to verify round-trip
	parsed, err := parser.Parse(serialized)
	if err != nil {
		t.Fatalf("Failed to parse serialized message: %v", err)
	}

	// Verify Via headers
	viaHeaders := parsed.GetHeaders(HeaderVia)
	if len(viaHeaders) != 2 {
		t.Errorf("Expected 2 Via headers, got %d", len(viaHeaders))
	}

	// Verify Contact headers
	contactHeaders := parsed.GetHeaders(HeaderContact)
	if len(contactHeaders) != 2 {
		t.Errorf("Expected 2 Contact headers, got %d", len(contactHeaders))
	}

	// Verify Allow headers
	allowHeaders := parsed.GetHeaders(HeaderAllow)
	if len(allowHeaders) != 3 {
		t.Errorf("Expected 3 Allow headers, got %d", len(allowHeaders))
	}
}

func TestSerializeEmptyMessage(t *testing.T) {
	parser := NewParser()

	// Test nil message
	_, err := parser.Serialize(nil)
	if err == nil {
		t.Error("Expected error for nil message")
	}

	// Test message with nil start line
	msg := &SIPMessage{
		Headers: make(map[string][]string),
	}
	_, err = parser.Serialize(msg)
	if err == nil {
		t.Error("Expected error for message with nil start line")
	}
}

func TestSerializeHeaderOrdering(t *testing.T) {
	// Create a message with headers in random order
	msg := NewRequestMessage(MethodINVITE, "sip:bob@example.com")
	msg.AddHeader(HeaderContentLength, "0")
	msg.AddHeader(HeaderFrom, "Alice <sip:alice@example.com>;tag=1928301774")
	msg.AddHeader(HeaderVia, "SIP/2.0/UDP 192.168.1.1:5060;branch=z9hG4bK776asdhds")
	msg.AddHeader(HeaderCSeq, "314159 INVITE")
	msg.AddHeader(HeaderTo, "Bob <sip:bob@example.com>")
	msg.AddHeader(HeaderCallID, "a84b4c76e66710@pc33.example.com")
	msg.AddHeader(HeaderMaxForwards, "70")

	parser := NewParser()
	serialized, err := parser.Serialize(msg)
	if err != nil {
		t.Fatalf("Failed to serialize message: %v", err)
	}

	serializedStr := string(serialized)
	
	// Check that Via comes before other headers (should be first in order)
	viaIndex := strings.Index(serializedStr, "Via:")
	fromIndex := strings.Index(serializedStr, "From:")
	
	if viaIndex == -1 {
		t.Error("Via header not found in serialized message")
	}
	
	if fromIndex == -1 {
		t.Error("From header not found in serialized message")
	}
	
	if viaIndex > fromIndex {
		t.Error("Via header should come before From header in serialized message")
	}

	// Verify the message can be parsed back correctly
	parsed, err := parser.Parse(serialized)
	if err != nil {
		t.Fatalf("Failed to parse serialized message: %v", err)
	}

	// Verify all headers are present
	expectedHeaders := []string{HeaderVia, HeaderMaxForwards, HeaderTo, HeaderFrom, 
		HeaderCallID, HeaderCSeq, HeaderContentLength}
	
	for _, header := range expectedHeaders {
		if !parsed.HasHeader(header) {
			t.Errorf("Header %s missing in parsed message", header)
		}
	}
}

func TestRoundTripSerialization(t *testing.T) {
	// Test various message types for round-trip serialization
	testMessages := []string{
		// INVITE request
		`INVITE sip:bob@example.com SIP/2.0
Via: SIP/2.0/UDP 192.168.1.1:5060;branch=z9hG4bK776asdhds
Max-Forwards: 70
To: Bob <sip:bob@example.com>
From: Alice <sip:alice@example.com>;tag=1928301774
Call-ID: a84b4c76e66710@pc33.example.com
CSeq: 314159 INVITE
Contact: <sip:alice@192.168.1.1:5060>
Content-Length: 0

`,
		// 200 OK response
		`SIP/2.0 200 OK
Via: SIP/2.0/UDP 192.168.1.1:5060;branch=z9hG4bK776asdhds
To: Bob <sip:bob@example.com>;tag=a6c85cf
From: Alice <sip:alice@example.com>;tag=1928301774
Call-ID: a84b4c76e66710@pc33.example.com
CSeq: 314159 INVITE
Contact: <sip:bob@192.168.1.2:5060>
Content-Length: 0

`,
		// REGISTER request
		`REGISTER sip:example.com SIP/2.0
Via: SIP/2.0/UDP 192.168.1.1:5060;branch=z9hG4bK623asdhds
Max-Forwards: 70
To: Alice <sip:alice@example.com>
From: Alice <sip:alice@example.com>;tag=456248
Call-ID: 843817637684230@998sdasdh09
CSeq: 1826 REGISTER
Contact: <sip:alice@192.168.1.1:5060>
Expires: 7200
Content-Length: 0

`,
	}

	parser := NewParser()
	
	for i, originalMessage := range testMessages {
		t.Run(fmt.Sprintf("Message_%d", i), func(t *testing.T) {
			// Parse original message
			parsed, err := parser.Parse([]byte(originalMessage))
			if err != nil {
				t.Fatalf("Failed to parse original message: %v", err)
			}

			// Serialize parsed message
			serialized, err := parser.Serialize(parsed)
			if err != nil {
				t.Fatalf("Failed to serialize parsed message: %v", err)
			}

			// Parse serialized message
			reparsed, err := parser.Parse(serialized)
			if err != nil {
				t.Fatalf("Failed to parse serialized message: %v", err)
			}

			// Compare key fields
			if parsed.IsRequest() != reparsed.IsRequest() {
				t.Error("Request/Response type mismatch")
			}

			if parsed.IsRequest() {
				if parsed.GetMethod() != reparsed.GetMethod() {
					t.Errorf("Method mismatch: expected %s, got %s", 
						parsed.GetMethod(), reparsed.GetMethod())
				}
				if parsed.GetRequestURI() != reparsed.GetRequestURI() {
					t.Errorf("Request URI mismatch: expected %s, got %s", 
						parsed.GetRequestURI(), reparsed.GetRequestURI())
				}
			} else {
				if parsed.GetStatusCode() != reparsed.GetStatusCode() {
					t.Errorf("Status code mismatch: expected %d, got %d", 
						parsed.GetStatusCode(), reparsed.GetStatusCode())
				}
			}

			// Compare essential headers
			essentialHeaders := []string{HeaderVia, HeaderFrom, HeaderTo, HeaderCallID, HeaderCSeq}
			for _, header := range essentialHeaders {
				if parsed.GetHeader(header) != reparsed.GetHeader(header) {
					t.Errorf("Header %s mismatch: expected %s, got %s", 
						header, parsed.GetHeader(header), reparsed.GetHeader(header))
				}
			}

			// Compare body
			if string(parsed.Body) != string(reparsed.Body) {
				t.Errorf("Body mismatch: expected %s, got %s", 
					string(parsed.Body), string(reparsed.Body))
			}
		})
	}
}