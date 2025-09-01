package handlers

import (
	"bytes"
	"net"
	"testing"

	"github.com/zurustar/xylitol2/internal/logging"
	"github.com/zurustar/xylitol2/internal/parser"
	"github.com/zurustar/xylitol2/internal/transaction"
	"github.com/zurustar/xylitol2/internal/transport"
	"github.com/zurustar/xylitol2/internal/validation"
)

// TestLogger implements the Logger interface for testing
type TestLoggerIntegration struct {
	messages []string
	buffer   *bytes.Buffer
}

func NewTestLoggerIntegration() *TestLoggerIntegration {
	return &TestLoggerIntegration{
		messages: make([]string, 0),
		buffer:   &bytes.Buffer{},
	}
}

func (tl *TestLoggerIntegration) Debug(msg string, fields ...logging.Field) {
	tl.log("DEBUG", msg, fields...)
}

func (tl *TestLoggerIntegration) Info(msg string, fields ...logging.Field) {
	tl.log("INFO", msg, fields...)
}

func (tl *TestLoggerIntegration) Warn(msg string, fields ...logging.Field) {
	tl.log("WARN", msg, fields...)
}

func (tl *TestLoggerIntegration) Error(msg string, fields ...logging.Field) {
	tl.log("ERROR", msg, fields...)
}

func (tl *TestLoggerIntegration) log(level, msg string, fields ...logging.Field) {
	tl.buffer.WriteString(level + ": " + msg)
	for _, field := range fields {
		tl.buffer.WriteString(" " + field.Key + "=" + field.Value.(string))
	}
	tl.buffer.WriteString("\n")
}

func (tl *TestLoggerIntegration) GetOutput() string {
	return tl.buffer.String()
}

func (tl *TestLoggerIntegration) Reset() {
	tl.buffer.Reset()
}

// Mock method handler for testing
type mockMethodHandler struct {
	canHandleMethods []string
	handledRequests  []*parser.SIPMessage
}

func (m *mockMethodHandler) CanHandle(method string) bool {
	for _, supportedMethod := range m.canHandleMethods {
		if method == supportedMethod {
			return true
		}
	}
	return false
}

func (m *mockMethodHandler) HandleRequest(req *parser.SIPMessage, txn transaction.Transaction) error {
	m.handledRequests = append(m.handledRequests, req)
	
	// Send a 200 OK response for successful handling
	response := parser.NewResponseMessage(200, "OK")
	
	// Copy required headers
	if via := req.GetHeader("Via"); via != "" {
		response.SetHeader("Via", via)
	}
	if from := req.GetHeader("From"); from != "" {
		response.SetHeader("From", from)
	}
	if to := req.GetHeader("To"); to != "" {
		if !containsTagIntegration(to) {
			to += ";tag=mock-tag-123"
		}
		response.SetHeader("To", to)
	}
	if callID := req.GetHeader("Call-ID"); callID != "" {
		response.SetHeader("Call-ID", callID)
	}
	if cseq := req.GetHeader("CSeq"); cseq != "" {
		response.SetHeader("CSeq", cseq)
	}
	response.SetHeader("Content-Length", "0")
	
	return txn.SendResponse(response)
}

// TestValidationChainIntegration tests the complete integration of validation chain with message processing
func TestValidationChainIntegration(t *testing.T) {
	// Create validated manager
	validatedManager := NewValidatedManager()

	// Set up validation chain with default validators
	config := DefaultValidationConfig()
	// Disable authentication for testing
	config.AuthConfig.Enabled = false
	validatedManager.SetupDefaultValidators(config)

	// Set up the underlying manager with mock handlers
	validatedManager.Manager = NewManager()
	mockHandler := &mockMethodHandler{
		canHandleMethods: []string{"INVITE", "REGISTER", "OPTIONS"},
	}
	validatedManager.Manager.RegisterHandler(mockHandler)

	// Create mock transaction
	mockTxn := &mockTransactionIntegration{
		responses: make([]*parser.SIPMessage, 0),
	}

	tests := []struct {
		name           string
		request        *parser.SIPMessage
		expectedValid  bool
		expectedCode   int
		expectedReason string
		description    string
	}{
		{
			name:           "Valid INVITE with Session-Timer",
			request:        createValidInviteWithSessionTimer(),
			expectedValid:  true,
			expectedCode:   0,
			expectedReason: "",
			description:    "INVITE with proper Session-Timer should pass validation",
		},
		{
			name:           "INVITE without Session-Timer",
			request:        createInviteWithoutSessionTimer(),
			expectedValid:  false,
			expectedCode:   421,
			expectedReason: "Extension Required",
			description:    "INVITE without Session-Timer should be rejected with 421",
		},
		{
			name:           "INVITE with invalid Session-Timer value",
			request:        createInviteWithInvalidSessionTimer(),
			expectedValid:  false,
			expectedCode:   422,
			expectedReason: "Session Interval Too Small",
			description:    "INVITE with Session-Timer below minimum should be rejected with 422",
		},
		{
			name:           "INVITE without authorization after Session-Timer validation",
			request:        createInviteWithSessionTimerNoAuth(),
			expectedValid:  true, // Authentication is disabled for this test
			expectedCode:   0,
			expectedReason: "",
			description:    "INVITE should be validated for Session-Timer first, then authentication",
		},
		{
			name:           "Malformed INVITE request",
			request:        createMalformedInvite(),
			expectedValid:  false,
			expectedCode:   400,
			expectedReason: "Bad Request",
			description:    "Malformed INVITE should be rejected with 400",
		},
		{
			name:           "REGISTER request bypasses Session-Timer validation",
			request:        createValidRegister(),
			expectedValid:  true,
			expectedCode:   0,
			expectedReason: "",
			description:    "REGISTER requests should not be subject to Session-Timer validation",
		},
		{
			name:           "OPTIONS request bypasses Session-Timer validation",
			request:        createValidOptions(),
			expectedValid:  true,
			expectedCode:   0,
			expectedReason: "",
			description:    "OPTIONS requests should not be subject to Session-Timer validation",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset mock transaction
			mockTxn.responses = make([]*parser.SIPMessage, 0)



			// Process request through validation chain
			err := validatedManager.HandleRequest(tt.request, mockTxn)

			if tt.expectedValid {
				// Should not have error and no error responses sent
				if err != nil {
					t.Errorf("Expected valid request but got error: %v", err)
				}
				if len(mockTxn.responses) > 0 {
					response := mockTxn.responses[0]
					if response.IsResponse() {
						if statusLine, ok := response.StartLine.(*parser.StatusLine); ok {
							if statusLine.StatusCode >= 400 {
								t.Errorf("Expected no error responses but got %d response with status %d %s", 
									statusLine.StatusCode, statusLine.StatusCode, statusLine.ReasonPhrase)
							}
						}
					} else {
						t.Errorf("Expected no error responses but got %d responses", len(mockTxn.responses))
					}
				}
			} else {
				// Should have sent an error response
				if len(mockTxn.responses) == 0 {
					t.Errorf("Expected error response but none was sent")
					return
				}

				response := mockTxn.responses[0]
				if !response.IsResponse() {
					t.Errorf("Expected response message but got request")
					return
				}

				statusLine, ok := response.StartLine.(*parser.StatusLine)
				if !ok {
					t.Errorf("Expected status line in response")
					return
				}

				if statusLine.StatusCode != tt.expectedCode {
					t.Errorf("Expected status code %d but got %d", tt.expectedCode, statusLine.StatusCode)
				}

				if statusLine.ReasonPhrase != tt.expectedReason {
					t.Errorf("Expected reason phrase '%s' but got '%s'", tt.expectedReason, statusLine.ReasonPhrase)
				}
			}
		})
	}
}

// TestValidationPriorityOrder tests that validators are executed in the correct priority order
func TestValidationPriorityOrder(t *testing.T) {
	// Create validated manager
	validatedManager := NewValidatedManager()

	// Add validators in reverse priority order to test sorting
	authValidator := validation.NewAuthValidator(true, "test.com")
	sessionTimerValidator := validation.NewSessionTimerValidator(90, 1800, true)
	syntaxValidator := validation.NewSyntaxValidator()

	// Add in reverse order
	validatedManager.AddValidator(authValidator)
	validatedManager.AddValidator(sessionTimerValidator)
	validatedManager.AddValidator(syntaxValidator)

	// Get validators and check order
	validators := validatedManager.GetValidators()

	if len(validators) != 3 {
		t.Fatalf("Expected 3 validators but got %d", len(validators))
	}

	// Check priority order (lower number = higher priority)
	expectedOrder := []string{"SyntaxValidator", "SessionTimerValidator", "AuthValidator"}
	for i, validator := range validators {
		if validator.Name() != expectedOrder[i] {
			t.Errorf("Expected validator %d to be %s but got %s", i, expectedOrder[i], validator.Name())
		}
	}

	// Verify priorities are in ascending order
	for i := 1; i < len(validators); i++ {
		if validators[i-1].Priority() > validators[i].Priority() {
			t.Errorf("Validators not sorted by priority: %s (priority %d) should come after %s (priority %d)",
				validators[i-1].Name(), validators[i-1].Priority(),
				validators[i].Name(), validators[i].Priority())
		}
	}
}

// TestValidationChainErrorResponseGeneration tests proper error response generation
func TestValidationChainErrorResponseGeneration(t *testing.T) {
	validatedManager := NewValidatedManager()
	config := DefaultValidationConfig()
	validatedManager.SetupDefaultValidators(config)

	mockTxn := &mockTransactionIntegration{
		responses: make([]*parser.SIPMessage, 0),
	}

	// Test Session-Timer error response
	inviteWithoutTimer := createInviteWithoutSessionTimer()
	err := validatedManager.HandleRequest(inviteWithoutTimer, mockTxn)

	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if len(mockTxn.responses) != 1 {
		t.Fatalf("Expected 1 response but got %d", len(mockTxn.responses))
	}

	response := mockTxn.responses[0]

	// Check mandatory headers are copied
	if response.GetHeader("Via") != inviteWithoutTimer.GetHeader("Via") {
		t.Errorf("Via header not copied correctly")
	}
	if response.GetHeader("From") != inviteWithoutTimer.GetHeader("From") {
		t.Errorf("From header not copied correctly")
	}
	if response.GetHeader("Call-ID") != inviteWithoutTimer.GetHeader("Call-ID") {
		t.Errorf("Call-ID header not copied correctly")
	}
	if response.GetHeader("CSeq") != inviteWithoutTimer.GetHeader("CSeq") {
		t.Errorf("CSeq header not copied correctly")
	}

	// Check To header has tag added
	toHeader := response.GetHeader("To")
	if toHeader == "" || !containsTagIntegration(toHeader) {
		t.Errorf("To header should have tag added, got: %s", toHeader)
	}

	// Check Require header for 421 response
	requireHeader := response.GetHeader("Require")
	if requireHeader != "timer" {
		t.Errorf("Expected Require header 'timer' but got '%s'", requireHeader)
	}
}

// TestValidationChainWithTransportAdapter tests integration with transport adapter
func TestValidationChainWithTransportAdapter(t *testing.T) {
	// Create components
	messageParser := parser.NewParser()
	transactionManager := &mockTransactionManagerIntegration{}
	transportManager := &mockTransportManagerIntegration{}
	
	// Create validated manager
	validatedManager := NewValidatedManager()
	config := DefaultValidationConfig()
	validatedManager.SetupDefaultValidators(config)

	// Create transport adapter
	adapter := NewTransportAdapter(validatedManager, transactionManager, messageParser, transportManager)

	// Test with valid INVITE
	validInvite := createValidInviteWithSessionTimer()
	data, err := messageParser.Serialize(validInvite)
	if err != nil {
		t.Fatalf("Failed to serialize message: %v", err)
	}

	addr, _ := net.ResolveUDPAddr("udp", "127.0.0.1:5060")
	err = adapter.HandleMessage(data, "udp", addr)

	// Should not have error for valid message
	if err != nil {
		t.Errorf("Expected no error for valid message but got: %v", err)
	}

	// Test with invalid INVITE
	invalidInvite := createInviteWithoutSessionTimer()
	data, err = messageParser.Serialize(invalidInvite)
	if err != nil {
		t.Fatalf("Failed to serialize message: %v", err)
	}

	err = adapter.HandleMessage(data, "udp", addr)

	// Should not have error (error response should be sent through transaction)
	if err != nil {
		t.Errorf("Expected no error (response should be sent) but got: %v", err)
	}
}

// Helper functions to create test messages

func createValidInviteWithSessionTimer() *parser.SIPMessage {
	invite := parser.NewRequestMessage(parser.MethodINVITE, "sip:test@example.com")
	invite.SetHeader("Via", "SIP/2.0/UDP client.example.com:5060;branch=z9hG4bK123")
	invite.SetHeader("From", "sip:caller@example.com;tag=caller123")
	invite.SetHeader("To", "sip:test@example.com")
	invite.SetHeader("Call-ID", "test-call-id-123")
	invite.SetHeader("CSeq", "1 INVITE")
	invite.SetHeader("Session-Expires", "1800")
	invite.SetHeader("Supported", "timer")
	invite.SetHeader("Authorization", "Digest username=\"test\", realm=\"example.com\", nonce=\"abc123\", response=\"def456\"")
	invite.SetHeader("Content-Length", "0")
	return invite
}

func createInviteWithoutSessionTimer() *parser.SIPMessage {
	invite := parser.NewRequestMessage(parser.MethodINVITE, "sip:test@example.com")
	invite.SetHeader("Via", "SIP/2.0/UDP client.example.com:5060;branch=z9hG4bK124")
	invite.SetHeader("From", "sip:caller@example.com;tag=caller124")
	invite.SetHeader("To", "sip:test@example.com")
	invite.SetHeader("Call-ID", "test-call-id-124")
	invite.SetHeader("CSeq", "1 INVITE")
	invite.SetHeader("Content-Length", "0")
	return invite
}

func createInviteWithInvalidSessionTimer() *parser.SIPMessage {
	invite := parser.NewRequestMessage(parser.MethodINVITE, "sip:test@example.com")
	invite.SetHeader("Via", "SIP/2.0/UDP client.example.com:5060;branch=z9hG4bK125")
	invite.SetHeader("From", "sip:caller@example.com;tag=caller125")
	invite.SetHeader("To", "sip:test@example.com")
	invite.SetHeader("Call-ID", "test-call-id-125")
	invite.SetHeader("CSeq", "1 INVITE")
	invite.SetHeader("Session-Expires", "30") // Below minimum of 90
	invite.SetHeader("Supported", "timer")
	invite.SetHeader("Content-Length", "0")
	return invite
}

func createInviteWithSessionTimerNoAuth() *parser.SIPMessage {
	invite := parser.NewRequestMessage(parser.MethodINVITE, "sip:test@example.com")
	invite.SetHeader("Via", "SIP/2.0/UDP client.example.com:5060;branch=z9hG4bK126")
	invite.SetHeader("From", "sip:caller@example.com;tag=caller126")
	invite.SetHeader("To", "sip:test@example.com")
	invite.SetHeader("Call-ID", "test-call-id-126")
	invite.SetHeader("CSeq", "1 INVITE")
	invite.SetHeader("Session-Expires", "1800")
	invite.SetHeader("Supported", "timer")
	invite.SetHeader("Content-Length", "0")
	// No Authorization header
	return invite
}

func createMalformedInvite() *parser.SIPMessage {
	invite := parser.NewRequestMessage(parser.MethodINVITE, "sip:test@example.com")
	invite.SetHeader("Via", "SIP/2.0/UDP client.example.com:5060;branch=z9hG4bK127")
	invite.SetHeader("From", "sip:caller@example.com;tag=caller127")
	// Missing To header (required)
	invite.SetHeader("Call-ID", "test-call-id-127")
	invite.SetHeader("CSeq", "1 INVITE")
	invite.SetHeader("Content-Length", "0")
	return invite
}

func createValidRegister() *parser.SIPMessage {
	register := parser.NewRequestMessage(parser.MethodREGISTER, "sip:example.com")
	register.SetHeader("Via", "SIP/2.0/UDP client.example.com:5060;branch=z9hG4bK128")
	register.SetHeader("From", "sip:test@example.com;tag=reg123")
	register.SetHeader("To", "sip:test@example.com")
	register.SetHeader("Call-ID", "reg-call-id-128")
	register.SetHeader("CSeq", "1 REGISTER")
	register.SetHeader("Contact", "sip:test@client.example.com:5060")
	register.SetHeader("Authorization", "Digest username=\"test\", realm=\"example.com\", nonce=\"abc123\", response=\"def456\"")
	register.SetHeader("Content-Length", "0")
	return register
}

func createValidOptions() *parser.SIPMessage {
	options := parser.NewRequestMessage(parser.MethodOPTIONS, "sip:test@example.com")
	options.SetHeader("Via", "SIP/2.0/UDP client.example.com:5060;branch=z9hG4bK129")
	options.SetHeader("From", "sip:caller@example.com;tag=opt123")
	options.SetHeader("To", "sip:test@example.com")
	options.SetHeader("Call-ID", "opt-call-id-129")
	options.SetHeader("CSeq", "1 OPTIONS")
	options.SetHeader("Content-Length", "0")
	return options
}

// Mock implementations

type mockTransactionIntegration struct {
	responses []*parser.SIPMessage
}

func (m *mockTransactionIntegration) SendResponse(response *parser.SIPMessage) error {
	m.responses = append(m.responses, response)
	return nil
}

func (m *mockTransactionIntegration) GetState() transaction.TransactionState {
	return transaction.StateTrying
}

func (m *mockTransactionIntegration) ProcessMessage(msg *parser.SIPMessage) error {
	return nil
}

func (m *mockTransactionIntegration) SetTimer(duration interface{}, callback func()) {
	// Mock implementation
}

func (m *mockTransactionIntegration) GetID() string {
	return "mock-transaction-id"
}

func (m *mockTransactionIntegration) IsClient() bool {
	return false
}

type mockTransactionManagerIntegration struct{}

func (m *mockTransactionManagerIntegration) CreateTransaction(msg *parser.SIPMessage) transaction.Transaction {
	return &mockTransactionIntegration{
		responses: make([]*parser.SIPMessage, 0),
	}
}

func (m *mockTransactionManagerIntegration) FindTransaction(msg *parser.SIPMessage) transaction.Transaction {
	return nil
}

func (m *mockTransactionManagerIntegration) CleanupExpired() {
	// Mock implementation
}

type mockTransportManagerIntegration struct {
	handler transport.MessageHandler
}

func (m *mockTransportManagerIntegration) StartUDP(port int) error {
	return nil
}

func (m *mockTransportManagerIntegration) StartTCP(port int) error {
	return nil
}

func (m *mockTransportManagerIntegration) SendMessage(msg []byte, transport string, addr net.Addr) error {
	return nil
}

func (m *mockTransportManagerIntegration) RegisterHandler(handler transport.MessageHandler) {
	m.handler = handler
}

func (m *mockTransportManagerIntegration) Stop() error {
	return nil
}

// Helper function
func containsTagIntegration(header string) bool {
	return len(header) > 4 && (header[len(header)-4:] == ";tag" || 
		findSubstringIntegration(header, ";tag=") >= 0)
}

func findSubstringIntegration(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}