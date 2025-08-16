package auth

import (
	"crypto/md5"
	"fmt"
	"strings"
	"testing"

	"github.com/zurustar/xylitol2/internal/parser"
	"github.com/zurustar/xylitol2/internal/transaction"
)

// MockTransaction implements transaction.Transaction for testing
type MockTransaction struct {
	id       string
	isClient bool
	state    transaction.TransactionState
}

func NewMockTransaction(id string, isClient bool) *MockTransaction {
	return &MockTransaction{
		id:       id,
		isClient: isClient,
		state:    transaction.StateTrying,
	}
}

func (t *MockTransaction) GetState() transaction.TransactionState {
	return t.state
}

func (t *MockTransaction) ProcessMessage(msg *parser.SIPMessage) error {
	return nil
}

func (t *MockTransaction) SendResponse(response *parser.SIPMessage) error {
	return nil
}

func (t *MockTransaction) GetID() string {
	return t.id
}

func (t *MockTransaction) IsClient() bool {
	return t.isClient
}

func TestAuthenticationMiddleware_ProcessRequest(t *testing.T) {
	userManager := NewMockUserManager()
	realm := "example.com"
	middleware := NewAuthenticationMiddleware(userManager, realm)

	// Create test user
	username := "alice"
	password := "secret123"
	userManager.CreateUser(username, realm, password)

	tests := []struct {
		name                string
		method              string
		hasAuthHeader       bool
		validCredentials    bool
		userEnabled         bool
		expectedResponse    bool
		expectedAuth        bool
		expectedStatusCode  int
	}{
		{
			name:               "OPTIONS method - no auth required",
			method:             parser.MethodOPTIONS,
			hasAuthHeader:      false,
			validCredentials:   false,
			userEnabled:        true,
			expectedResponse:   false,
			expectedAuth:       true,
			expectedStatusCode: 0,
		},
		{
			name:               "REGISTER method - no auth header",
			method:             parser.MethodREGISTER,
			hasAuthHeader:      false,
			validCredentials:   false,
			userEnabled:        true,
			expectedResponse:   true,
			expectedAuth:       false,
			expectedStatusCode: parser.StatusUnauthorized,
		},
		{
			name:               "REGISTER method - valid credentials",
			method:             parser.MethodREGISTER,
			hasAuthHeader:      true,
			validCredentials:   true,
			userEnabled:        true,
			expectedResponse:   false,
			expectedAuth:       true,
			expectedStatusCode: 0,
		},
		{
			name:               "REGISTER method - invalid credentials",
			method:             parser.MethodREGISTER,
			hasAuthHeader:      true,
			validCredentials:   false,
			userEnabled:        true,
			expectedResponse:   true,
			expectedAuth:       false,
			expectedStatusCode: parser.StatusForbidden,
		},
		{
			name:               "REGISTER method - disabled user",
			method:             parser.MethodREGISTER,
			hasAuthHeader:      true,
			validCredentials:   true,
			userEnabled:        false,
			expectedResponse:   true,
			expectedAuth:       false,
			expectedStatusCode: parser.StatusForbidden,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test message
			msg := parser.NewRequestMessage(tt.method, "sip:example.com")
			msg.SetHeader(parser.HeaderFrom, "sip:alice@example.com")
			msg.SetHeader(parser.HeaderTo, "sip:example.com")
			msg.SetHeader(parser.HeaderCallID, "test-call-id")
			msg.SetHeader(parser.HeaderCSeq, "1 "+tt.method)
			msg.SetHeader(parser.HeaderVia, "SIP/2.0/UDP 192.168.1.100:5060")

			// Set user enabled/disabled state
			if !tt.userEnabled {
				userManager.DisableUser(username, realm)
			} else {
				if user, err := userManager.GetUser(username, realm); err == nil {
					user.Enabled = true
				}
			}

			// Add authorization header if needed
			if tt.hasAuthHeader {
				authHeader := createTestAuthHeader(t, middleware, username, realm, password, tt.method, "sip:example.com", tt.validCredentials, userManager)
				msg.SetHeader(parser.HeaderAuthorization, authHeader)
			}

			// Process request
			response, authenticated, err := middleware.ProcessRequest(msg)
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			// Check if response is expected
			if tt.expectedResponse && response == nil {
				t.Error("Expected response but got none")
			}
			if !tt.expectedResponse && response != nil {
				t.Error("Expected no response but got one")
			}

			// Check authentication result
			if authenticated != tt.expectedAuth {
				t.Errorf("Expected authenticated=%v, got %v", tt.expectedAuth, authenticated)
			}

			// Check response status code if response is expected
			if tt.expectedResponse && response != nil {
				if response.GetStatusCode() != tt.expectedStatusCode {
					t.Errorf("Expected status code %d, got %d", tt.expectedStatusCode, response.GetStatusCode())
				}
			}
		})
	}
}

func TestAuthenticatedMessageProcessor_ProcessIncomingRequest(t *testing.T) {
	userManager := NewMockUserManager()
	realm := "example.com"
	processor := NewAuthenticatedMessageProcessor(userManager, realm)

	// Create test user
	username := "alice"
	password := "secret123"
	userManager.CreateUser(username, realm, password)

	// Create mock transaction
	transaction := NewMockTransaction("test-transaction", false)

	tests := []struct {
		name                string
		method              string
		hasAuthHeader       bool
		validCredentials    bool
		expectedResponse    bool
		expectedUser        bool
		expectedStatusCode  int
	}{
		{
			name:               "OPTIONS method - no auth required",
			method:             parser.MethodOPTIONS,
			hasAuthHeader:      false,
			validCredentials:   false,
			expectedResponse:   false,
			expectedUser:       false,
			expectedStatusCode: 0,
		},
		{
			name:               "REGISTER method - valid credentials",
			method:             parser.MethodREGISTER,
			hasAuthHeader:      true,
			validCredentials:   true,
			expectedResponse:   false,
			expectedUser:       true,
			expectedStatusCode: 0,
		},
		{
			name:               "REGISTER method - no auth header",
			method:             parser.MethodREGISTER,
			hasAuthHeader:      false,
			validCredentials:   false,
			expectedResponse:   true,
			expectedUser:       false,
			expectedStatusCode: parser.StatusUnauthorized,
		},
		{
			name:               "INVITE method - valid credentials",
			method:             parser.MethodINVITE,
			hasAuthHeader:      true,
			validCredentials:   true,
			expectedResponse:   false,
			expectedUser:       true,
			expectedStatusCode: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test message
			msg := parser.NewRequestMessage(tt.method, "sip:example.com")
			msg.SetHeader(parser.HeaderFrom, "sip:alice@example.com")
			msg.SetHeader(parser.HeaderTo, "sip:example.com")
			msg.SetHeader(parser.HeaderCallID, "test-call-id")
			msg.SetHeader(parser.HeaderCSeq, "1 "+tt.method)
			msg.SetHeader(parser.HeaderVia, "SIP/2.0/UDP 192.168.1.100:5060")

			// Add authorization header if needed
			if tt.hasAuthHeader {
				authHeader := createTestAuthHeader(t, processor.authHandler.(*AuthenticationMiddleware), username, realm, password, tt.method, "sip:example.com", tt.validCredentials, userManager)
				msg.SetHeader(parser.HeaderAuthorization, authHeader)
			}

			// Process request
			response, user, err := processor.ProcessIncomingRequest(msg, transaction)
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			// Check if response is expected
			if tt.expectedResponse && response == nil {
				t.Error("Expected response but got none")
			}
			if !tt.expectedResponse && response != nil {
				t.Error("Expected no response but got one")
			}

			// Check user result
			if tt.expectedUser && user == nil {
				t.Error("Expected user but got none")
			}
			if !tt.expectedUser && user != nil {
				t.Error("Expected no user but got one")
			}

			// Check response status code if response is expected
			if tt.expectedResponse && response != nil {
				if response.GetStatusCode() != tt.expectedStatusCode {
					t.Errorf("Expected status code %d, got %d", tt.expectedStatusCode, response.GetStatusCode())
				}
			}

			// Verify user details if expected
			if tt.expectedUser && user != nil {
				if user.Username != username {
					t.Errorf("Expected username %s, got %s", username, user.Username)
				}
				if user.Realm != realm {
					t.Errorf("Expected realm %s, got %s", realm, user.Realm)
				}
			}
		})
	}
}

func TestAuthenticatedMessageProcessor_ProcessREGISTERRequest(t *testing.T) {
	userManager := NewMockUserManager()
	realm := "example.com"
	processor := NewAuthenticatedMessageProcessor(userManager, realm)

	// Create test user
	username := "alice"
	password := "secret123"
	userManager.CreateUser(username, realm, password)

	// Create mock transaction
	transaction := NewMockTransaction("test-transaction", false)

	tests := []struct {
		name                string
		hasAuthHeader       bool
		validCredentials    bool
		expectedResponse    bool
		expectedUser        bool
		expectedStatusCode  int
	}{
		{
			name:               "Valid credentials",
			hasAuthHeader:      true,
			validCredentials:   true,
			expectedResponse:   false,
			expectedUser:       true,
			expectedStatusCode: 0,
		},
		{
			name:               "No auth header",
			hasAuthHeader:      false,
			validCredentials:   false,
			expectedResponse:   true,
			expectedUser:       false,
			expectedStatusCode: parser.StatusUnauthorized,
		},
		{
			name:               "Invalid credentials",
			hasAuthHeader:      true,
			validCredentials:   false,
			expectedResponse:   true,
			expectedUser:       false,
			expectedStatusCode: parser.StatusForbidden,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create REGISTER message
			msg := parser.NewRequestMessage(parser.MethodREGISTER, "sip:example.com")
			msg.SetHeader(parser.HeaderFrom, "sip:alice@example.com")
			msg.SetHeader(parser.HeaderTo, "sip:example.com")
			msg.SetHeader(parser.HeaderCallID, "test-call-id")
			msg.SetHeader(parser.HeaderCSeq, "1 REGISTER")
			msg.SetHeader(parser.HeaderVia, "SIP/2.0/UDP 192.168.1.100:5060")
			msg.SetHeader(parser.HeaderContact, "sip:alice@192.168.1.100:5060")

			// Add authorization header if needed
			if tt.hasAuthHeader {
				authHeader := createTestAuthHeader(t, processor.authHandler.(*AuthenticationMiddleware), username, realm, password, parser.MethodREGISTER, "sip:example.com", tt.validCredentials, userManager)
				msg.SetHeader(parser.HeaderAuthorization, authHeader)
			}

			// Process REGISTER request
			response, user, err := processor.ProcessREGISTERRequest(msg, transaction)
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			// Check if response is expected
			if tt.expectedResponse && response == nil {
				t.Error("Expected response but got none")
			}
			if !tt.expectedResponse && response != nil {
				t.Error("Expected no response but got one")
			}

			// Check user result
			if tt.expectedUser && user == nil {
				t.Error("Expected user but got none")
			}
			if !tt.expectedUser && user != nil {
				t.Error("Expected no user but got one")
			}

			// Check response status code if response is expected
			if tt.expectedResponse && response != nil {
				if response.GetStatusCode() != tt.expectedStatusCode {
					t.Errorf("Expected status code %d, got %d", tt.expectedStatusCode, response.GetStatusCode())
				}

				// Verify WWW-Authenticate header for 401 responses
				if tt.expectedStatusCode == parser.StatusUnauthorized {
					wwwAuth := response.GetHeader(parser.HeaderWWWAuthenticate)
					if wwwAuth == "" {
						t.Error("Expected WWW-Authenticate header in 401 response")
					}
					if !strings.Contains(wwwAuth, fmt.Sprintf(`realm="%s"`, realm)) {
						t.Error("WWW-Authenticate header should contain the correct realm")
					}
				}
			}
		})
	}
}

func TestAuthenticatedMessageProcessor_GetAuthenticatedUser(t *testing.T) {
	userManager := NewMockUserManager()
	realm := "example.com"
	processor := NewAuthenticatedMessageProcessor(userManager, realm)

	// Create test user
	username := "alice"
	password := "secret123"
	userManager.CreateUser(username, realm, password)

	// Test with REGISTER method (requires auth)
	msg := parser.NewRequestMessage(parser.MethodREGISTER, "sip:example.com")
	msg.SetHeader(parser.HeaderFrom, "sip:alice@example.com")
	msg.SetHeader(parser.HeaderTo, "sip:example.com")
	msg.SetHeader(parser.HeaderCallID, "test-call-id")
	msg.SetHeader(parser.HeaderCSeq, "1 REGISTER")
	msg.SetHeader(parser.HeaderVia, "SIP/2.0/UDP 192.168.1.100:5060")

	// Test without auth header
	user, err := processor.GetAuthenticatedUser(msg)
	if err == nil {
		t.Error("Expected error for missing auth header")
	}
	if user != nil {
		t.Error("Expected no user for missing auth header")
	}

	// Add valid auth header
	authHeader := createTestAuthHeader(t, processor.authHandler.(*AuthenticationMiddleware), username, realm, password, parser.MethodREGISTER, "sip:example.com", true, userManager)
	msg.SetHeader(parser.HeaderAuthorization, authHeader)

	// Test with valid auth header
	user, err = processor.GetAuthenticatedUser(msg)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if user == nil {
		t.Fatal("Expected user but got none")
	}
	if user.Username != username {
		t.Errorf("Expected username %s, got %s", username, user.Username)
	}

	// Test with OPTIONS method (no auth required)
	optionsMsg := parser.NewRequestMessage(parser.MethodOPTIONS, "sip:example.com")
	user, err = processor.GetAuthenticatedUser(optionsMsg)
	if err != nil {
		t.Errorf("Unexpected error for OPTIONS method: %v", err)
	}
	if user != nil {
		t.Error("Expected no user for OPTIONS method")
	}
}

// Helper function to create test authorization headers
func createTestAuthHeader(t *testing.T, middleware *AuthenticationMiddleware, username, realm, password, method, uri string, valid bool, userManager *MockUserManager) string {
	// Generate nonce and store it
	digestAuth := middleware.messageAuth.(*SIPMessageAuthenticator).digestAuth
	nonce, err := digestAuth.GenerateNonce()
	if err != nil {
		t.Fatalf("Failed to generate nonce: %v", err)
	}
	digestAuth.(*SIPDigestAuthenticator).nonceStore.StoreNonce(nonce)

	// Calculate response
	var response string
	if valid {
		user, err := userManager.GetUser(username, realm)
		if err != nil {
			// If user doesn't exist, we still need to create a header for testing
			// Use a dummy password hash
			passwordHash := fmt.Sprintf("%x", md5.Sum([]byte(fmt.Sprintf("%s:%s:%s", username, realm, password))))
			
			// Calculate correct response with dummy hash
			ha2Data := fmt.Sprintf("%s:%s", method, uri)
			ha2Hash := md5.Sum([]byte(ha2Data))
			ha2 := fmt.Sprintf("%x", ha2Hash)

			responseData := fmt.Sprintf("%s:%s:%s", passwordHash, nonce, ha2)
			responseHash := md5.Sum([]byte(responseData))
			response = fmt.Sprintf("%x", responseHash)
		} else {
			// Calculate correct response
			ha2Data := fmt.Sprintf("%s:%s", method, uri)
			ha2Hash := md5.Sum([]byte(ha2Data))
			ha2 := fmt.Sprintf("%x", ha2Hash)

			responseData := fmt.Sprintf("%s:%s:%s", user.PasswordHash, nonce, ha2)
			responseHash := md5.Sum([]byte(responseData))
			response = fmt.Sprintf("%x", responseHash)
		}
	} else {
		response = "invalidresponse"
	}

	return fmt.Sprintf(`Digest username="%s", realm="%s", nonce="%s", uri="%s", response="%s", algorithm="MD5"`,
		username, realm, nonce, uri, response)
}