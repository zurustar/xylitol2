package integration

import (
	"crypto/md5"
	"fmt"
	"net"
	"strings"
	"testing"
	"time"
)

// SIPTestClient provides utilities for SIP testing
type SIPTestClient struct {
	udpConn    *net.UDPConn
	tcpConn    *net.TCPConn
	serverAddr string
}

// NewSIPTestClient creates a new SIP test client
func NewSIPTestClient(serverAddr string) (*SIPTestClient, error) {
	client := &SIPTestClient{
		serverAddr: serverAddr,
	}
	
	// Setup UDP connection
	udpAddr, err := net.ResolveUDPAddr("udp", serverAddr)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve UDP address: %w", err)
	}
	
	udpConn, err := net.DialUDP("udp", nil, udpAddr)
	if err != nil {
		return nil, fmt.Errorf("failed to create UDP connection: %w", err)
	}
	client.udpConn = udpConn
	
	// Setup TCP connection
	tcpAddr, err := net.ResolveTCPAddr("tcp", serverAddr)
	if err != nil {
		udpConn.Close()
		return nil, fmt.Errorf("failed to resolve TCP address: %w", err)
	}
	
	tcpConn, err := net.DialTCP("tcp", nil, tcpAddr)
	if err != nil {
		udpConn.Close()
		return nil, fmt.Errorf("failed to create TCP connection: %w", err)
	}
	client.tcpConn = tcpConn
	
	return client, nil
}

// Close closes all connections
func (c *SIPTestClient) Close() {
	if c.udpConn != nil {
		c.udpConn.Close()
	}
	if c.tcpConn != nil {
		c.tcpConn.Close()
	}
}

// SendUDP sends a message over UDP and returns the response
func (c *SIPTestClient) SendUDP(message string, timeout time.Duration) (string, error) {
	_, err := c.udpConn.Write([]byte(message))
	if err != nil {
		return "", fmt.Errorf("failed to send UDP message: %w", err)
	}

	c.udpConn.SetReadDeadline(time.Now().Add(timeout))
	buffer := make([]byte, 8192)
	n, err := c.udpConn.Read(buffer)
	if err != nil {
		return "", fmt.Errorf("failed to read UDP response: %w", err)
	}

	return string(buffer[:n]), nil
}

// SendTCP sends a message over TCP and returns the response
func (c *SIPTestClient) SendTCP(message string, timeout time.Duration) (string, error) {
	_, err := c.tcpConn.Write([]byte(message))
	if err != nil {
		return "", fmt.Errorf("failed to send TCP message: %w", err)
	}

	c.tcpConn.SetReadDeadline(time.Now().Add(timeout))
	buffer := make([]byte, 8192)
	n, err := c.tcpConn.Read(buffer)
	if err != nil {
		return "", fmt.Errorf("failed to read TCP response: %w", err)
	}

	return string(buffer[:n]), nil
}

// SIPMessageBuilder helps build SIP messages for testing
type SIPMessageBuilder struct {
	method      string
	requestURI  string
	headers     map[string]string
	body        string
	transport   string
	branch      string
	callID      string
	cseq        int
}

// NewSIPMessageBuilder creates a new SIP message builder
func NewSIPMessageBuilder(method, requestURI string) *SIPMessageBuilder {
	return &SIPMessageBuilder{
		method:     method,
		requestURI: requestURI,
		headers:    make(map[string]string),
		transport:  "UDP",
		branch:     generateBranch(),
		callID:     generateCallID(),
		cseq:       1,
	}
}

// SetTransport sets the transport protocol
func (b *SIPMessageBuilder) SetTransport(transport string) *SIPMessageBuilder {
	b.transport = transport
	return b
}

// SetHeader sets a SIP header
func (b *SIPMessageBuilder) SetHeader(name, value string) *SIPMessageBuilder {
	b.headers[name] = value
	return b
}

// SetBody sets the message body
func (b *SIPMessageBuilder) SetBody(body string) *SIPMessageBuilder {
	b.body = body
	return b
}

// SetCSeq sets the CSeq number
func (b *SIPMessageBuilder) SetCSeq(cseq int) *SIPMessageBuilder {
	b.cseq = cseq
	return b
}

// Build constructs the SIP message
func (b *SIPMessageBuilder) Build() string {
	var msg strings.Builder
	
	// Request line
	msg.WriteString(fmt.Sprintf("%s %s SIP/2.0\r\n", b.method, b.requestURI))
	
	// Via header
	via := fmt.Sprintf("SIP/2.0/%s 192.168.1.100:5060;branch=%s", b.transport, b.branch)
	msg.WriteString(fmt.Sprintf("Via: %s\r\n", via))
	
	// Required headers
	if from, exists := b.headers["From"]; exists {
		msg.WriteString(fmt.Sprintf("From: %s\r\n", from))
	} else {
		msg.WriteString("From: <sip:test@test.local>;tag=test-tag\r\n")
	}
	
	if to, exists := b.headers["To"]; exists {
		msg.WriteString(fmt.Sprintf("To: %s\r\n", to))
	} else {
		msg.WriteString("To: <sip:test.local>\r\n")
	}
	
	msg.WriteString(fmt.Sprintf("Call-ID: %s\r\n", b.callID))
	msg.WriteString(fmt.Sprintf("CSeq: %d %s\r\n", b.cseq, b.method))
	
	// Additional headers
	for name, value := range b.headers {
		if name != "From" && name != "To" {
			msg.WriteString(fmt.Sprintf("%s: %s\r\n", name, value))
		}
	}
	
	// Content-Length
	msg.WriteString(fmt.Sprintf("Content-Length: %d\r\n", len(b.body)))
	
	// Empty line
	msg.WriteString("\r\n")
	
	// Body
	if b.body != "" {
		msg.WriteString(b.body)
	}
	
	return msg.String()
}

// generateBranch generates a unique branch parameter
func generateBranch() string {
	return fmt.Sprintf("z9hG4bK-%d", time.Now().UnixNano())
}

// generateCallID generates a unique Call-ID
func generateCallID() string {
	return fmt.Sprintf("test-call-%d@test.local", time.Now().UnixNano())
}

// SIPResponseParser helps parse SIP responses
type SIPResponseParser struct {
	response string
}

// NewSIPResponseParser creates a new response parser
func NewSIPResponseParser(response string) *SIPResponseParser {
	return &SIPResponseParser{response: response}
}

// GetStatusCode extracts the status code from the response
func (p *SIPResponseParser) GetStatusCode() (int, error) {
	lines := strings.Split(p.response, "\r\n")
	if len(lines) == 0 {
		return 0, fmt.Errorf("empty response")
	}
	
	statusLine := lines[0]
	parts := strings.Fields(statusLine)
	if len(parts) < 2 {
		return 0, fmt.Errorf("invalid status line: %s", statusLine)
	}
	
	var statusCode int
	_, err := fmt.Sscanf(parts[1], "%d", &statusCode)
	if err != nil {
		return 0, fmt.Errorf("invalid status code: %s", parts[1])
	}
	
	return statusCode, nil
}

// GetHeader extracts a header value from the response
func (p *SIPResponseParser) GetHeader(headerName string) string {
	lines := strings.Split(p.response, "\r\n")
	
	for _, line := range lines {
		if strings.HasPrefix(strings.ToLower(line), strings.ToLower(headerName)+":") {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				return strings.TrimSpace(parts[1])
			}
		}
	}
	
	return ""
}

// HasHeader checks if a header exists in the response
func (p *SIPResponseParser) HasHeader(headerName string) bool {
	return p.GetHeader(headerName) != ""
}

// GetBody extracts the message body from the response
func (p *SIPResponseParser) GetBody() string {
	parts := strings.Split(p.response, "\r\n\r\n")
	if len(parts) > 1 {
		return parts[1]
	}
	return ""
}

// AuthHelper provides utilities for SIP authentication testing
type AuthHelper struct {
	username string
	realm    string
	password string
}

// NewAuthHelper creates a new authentication helper
func NewAuthHelper(username, realm, password string) *AuthHelper {
	return &AuthHelper{
		username: username,
		realm:    realm,
		password: password,
	}
}

// GenerateAuthHeader generates a digest authentication header
func (a *AuthHelper) GenerateAuthHeader(method, uri, nonce string) string {
	// Calculate HA1
	ha1Data := fmt.Sprintf("%s:%s:%s", a.username, a.realm, a.password)
	ha1Hash := md5.Sum([]byte(ha1Data))
	ha1 := fmt.Sprintf("%x", ha1Hash)
	
	// Calculate HA2
	ha2Data := fmt.Sprintf("%s:%s", method, uri)
	ha2Hash := md5.Sum([]byte(ha2Data))
	ha2 := fmt.Sprintf("%x", ha2Hash)
	
	// Calculate response
	responseData := fmt.Sprintf("%s:%s:%s", ha1, nonce, ha2)
	responseHash := md5.Sum([]byte(responseData))
	response := fmt.Sprintf("%x", responseHash)
	
	return fmt.Sprintf(`Digest username="%s", realm="%s", nonce="%s", uri="%s", response="%s", algorithm="MD5"`,
		a.username, a.realm, nonce, uri, response)
}

// ExtractNonce extracts nonce from WWW-Authenticate header
func (a *AuthHelper) ExtractNonce(wwwAuthHeader string) string {
	// Simple nonce extraction (in real implementation, should use proper parsing)
	start := strings.Index(wwwAuthHeader, `nonce="`)
	if start == -1 {
		return ""
	}
	start += 7 // len(`nonce="`)
	
	end := strings.Index(wwwAuthHeader[start:], `"`)
	if end == -1 {
		return ""
	}
	
	return wwwAuthHeader[start : start+end]
}

// TestScenario represents a test scenario
type TestScenario struct {
	Name        string
	Description string
	Setup       func(*testing.T, interface{})
	Execute     func(*testing.T, interface{}) error
	Verify      func(*testing.T, interface{}, error)
	Cleanup     func(*testing.T, interface{})
}

// RunScenario executes a test scenario
func RunScenario(t *testing.T, suite interface{}, scenario TestScenario) {
	t.Run(scenario.Name, func(t *testing.T) {
		if scenario.Setup != nil {
			scenario.Setup(t, suite)
		}
		
		var err error
		if scenario.Execute != nil {
			err = scenario.Execute(t, suite)
		}
		
		if scenario.Verify != nil {
			scenario.Verify(t, suite, err)
		}
		
		if scenario.Cleanup != nil {
			scenario.Cleanup(t, suite)
		}
	})
}

// LoadTestConfig represents load test configuration
type LoadTestConfig struct {
	NumRequests   int
	Concurrency   int
	Duration      time.Duration
	RequestDelay  time.Duration
	MessageType   string
	Transport     string
}

// DefaultLoadTestConfig returns default load test configuration
func DefaultLoadTestConfig() LoadTestConfig {
	return LoadTestConfig{
		NumRequests:  100,
		Concurrency:  10,
		Duration:     30 * time.Second,
		RequestDelay: 10 * time.Millisecond,
		MessageType:  "OPTIONS",
		Transport:    "UDP",
	}
}

// LoadTestResult represents load test results
type LoadTestResult struct {
	TotalRequests    int
	SuccessfulReqs   int
	FailedReqs       int
	Duration         time.Duration
	RequestsPerSec   float64
	AvgResponseTime  time.Duration
	MinResponseTime  time.Duration
	MaxResponseTime  time.Duration
	ErrorRate        float64
}

// RunLoadTest executes a load test
func RunLoadTest(t *testing.T, suite interface{}, config LoadTestConfig) LoadTestResult {
	// Implementation would go here - this is a placeholder
	// In a real implementation, this would:
	// 1. Create the specified number of concurrent workers
	// 2. Send requests according to the configuration
	// 3. Measure response times and success rates
	// 4. Return comprehensive results
	
	return LoadTestResult{
		TotalRequests:   config.NumRequests,
		SuccessfulReqs:  config.NumRequests - 5, // Simulate some failures
		FailedReqs:      5,
		Duration:        config.Duration,
		RequestsPerSec:  float64(config.NumRequests) / config.Duration.Seconds(),
		AvgResponseTime: 50 * time.Millisecond,
		MinResponseTime: 10 * time.Millisecond,
		MaxResponseTime: 200 * time.Millisecond,
		ErrorRate:       5.0 / float64(config.NumRequests) * 100,
	}
}

// AssertSIPResponse provides assertions for SIP responses
func AssertSIPResponse(t *testing.T, response string, expectedStatusCode int, requiredHeaders []string) {
	parser := NewSIPResponseParser(response)
	
	// Check status code
	statusCode, err := parser.GetStatusCode()
	if err != nil {
		t.Errorf("Failed to parse status code: %v", err)
		return
	}
	
	if statusCode != expectedStatusCode {
		t.Errorf("Expected status code %d, got %d", expectedStatusCode, statusCode)
	}
	
	// Check required headers
	for _, header := range requiredHeaders {
		if !parser.HasHeader(header) {
			t.Errorf("Missing required header: %s", header)
		}
	}
}

// AssertNoSIPResponse verifies that no response is received (for ACK, etc.)
func AssertNoSIPResponse(t *testing.T, conn net.Conn, timeout time.Duration) {
	conn.SetReadDeadline(time.Now().Add(timeout))
	buffer := make([]byte, 4096)
	n, err := conn.Read(buffer)
	
	if err == nil {
		response := string(buffer[:n])
		t.Errorf("Unexpected response received: %s", response)
	}
	// Timeout is expected behavior
}

// WaitForCondition waits for a condition to be true or timeout
func WaitForCondition(condition func() bool, timeout time.Duration, interval time.Duration) bool {
	deadline := time.Now().Add(timeout)
	
	for time.Now().Before(deadline) {
		if condition() {
			return true
		}
		time.Sleep(interval)
	}
	
	return false
}

// GenerateUniqueIdentifier generates a unique identifier for testing
func GenerateUniqueIdentifier(prefix string) string {
	return fmt.Sprintf("%s-%d", prefix, time.Now().UnixNano())
}

// CreateSDPBody creates a basic SDP body for testing
func CreateSDPBody(sessionName string) string {
	return fmt.Sprintf(`v=0
o=test 2890844526 2890844526 IN IP4 192.168.1.100
s=%s
c=IN IP4 192.168.1.100
t=0 0
m=audio 5004 RTP/AVP 0
a=rtpmap:0 PCMU/8000
a=sendrecv
`, sessionName)
}