package auth

import (
	"crypto/md5"
	"fmt"
	"strings"
	"testing"

	"github.com/zurustar/xylitol2/internal/database"
	"github.com/zurustar/xylitol2/internal/parser"
)

// MockUserManager implements database.UserManager for testing
type MockUserManager struct {
	users map[string]*database.User
}

func NewMockUserManager() *MockUserManager {
	return &MockUserManager{
		users: make(map[string]*database.User),
	}
}

func (m *MockUserManager) CreateUser(username, realm, password string) error {
	key := fmt.Sprintf("%s@%s", username, realm)
	passwordHash := fmt.Sprintf("%x", md5.Sum([]byte(fmt.Sprintf("%s:%s:%s", username, realm, password))))
	m.users[key] = &database.User{
		Username:     username,
		Realm:        realm,
		PasswordHash: passwordHash,
		Enabled:      true,
	}
	return nil
}

func (m *MockUserManager) AuthenticateUser(username, realm, password string) bool {
	user, err := m.GetUser(username, realm)
	if err != nil {
		return false
	}
	expectedHash := fmt.Sprintf("%x", md5.Sum([]byte(fmt.Sprintf("%s:%s:%s", username, realm, password))))
	return user.PasswordHash == expectedHash && user.Enabled
}

func (m *MockUserManager) UpdatePassword(username, realm, newPassword string) error {
	key := fmt.Sprintf("%s@%s", username, realm)
	if user, exists := m.users[key]; exists {
		user.PasswordHash = fmt.Sprintf("%x", md5.Sum([]byte(fmt.Sprintf("%s:%s:%s", username, realm, newPassword))))
		return nil
	}
	return fmt.Errorf("user not found")
}

func (m *MockUserManager) DeleteUser(username, realm string) error {
	key := fmt.Sprintf("%s@%s", username, realm)
	delete(m.users, key)
	return nil
}

func (m *MockUserManager) ListUsers() ([]*database.User, error) {
	users := make([]*database.User, 0, len(m.users))
	for _, user := range m.users {
		users = append(users, user)
	}
	return users, nil
}

func (m *MockUserManager) GeneratePasswordHash(username, realm, password string) string {
	return fmt.Sprintf("%x", md5.Sum([]byte(fmt.Sprintf("%s:%s:%s", username, realm, password))))
}

func (m *MockUserManager) GetUser(username, realm string) (*database.User, error) {
	key := fmt.Sprintf("%s@%s", username, realm)
	if user, exists := m.users[key]; exists {
		return user, nil
	}
	return nil, fmt.Errorf("user not found")
}

func (m *MockUserManager) DisableUser(username, realm string) {
	key := fmt.Sprintf("%s@%s", username, realm)
	if user, exists := m.users[key]; exists {
		user.Enabled = false
	}
}

func TestSIPMessageAuthenticator_AuthenticateRequest(t *testing.T) {
	realm := "example.com"
	auth := NewSIPMessageAuthenticator(realm)
	userManager := NewMockUserManager()

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
		expectedAuth        bool
		expectedRequiresAuth bool
	}{
		{
			name:                "OPTIONS method - no auth required",
			method:              parser.MethodOPTIONS,
			hasAuthHeader:       false,
			validCredentials:    false,
			userEnabled:         true,
			expectedAuth:        true,
			expectedRequiresAuth: false,
		},
		{
			name:                "REGISTER method - no auth header",
			method:              parser.MethodREGISTER,
			hasAuthHeader:       false,
			validCredentials:    false,
			userEnabled:         true,
			expectedAuth:        false,
			expectedRequiresAuth: true,
		},
		{
			name:                "REGISTER method - valid credentials",
			method:              parser.MethodREGISTER,
			hasAuthHeader:       true,
			validCredentials:    true,
			userEnabled:         true,
			expectedAuth:        true,
			expectedRequiresAuth: true,
		},
		{
			name:                "REGISTER method - invalid credentials",
			method:              parser.MethodREGISTER,
			hasAuthHeader:       true,
			validCredentials:    false,
			userEnabled:         true,
			expectedAuth:        false,
			expectedRequiresAuth: true,
		},
		{
			name:                "REGISTER method - disabled user",
			method:              parser.MethodREGISTER,
			hasAuthHeader:       true,
			validCredentials:    true,
			userEnabled:         false,
			expectedAuth:        false,
			expectedRequiresAuth: true,
		},
		{
			name:                "INVITE method - valid credentials",
			method:              parser.MethodINVITE,
			hasAuthHeader:       true,
			validCredentials:    true,
			userEnabled:         true,
			expectedAuth:        true,
			expectedRequiresAuth: true,
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
				// Re-enable user if it was disabled in previous test
				if user, err := userManager.GetUser(username, realm); err == nil {
					user.Enabled = true
				}
			}

			// Add authorization header if needed
			if tt.hasAuthHeader {
				// Generate nonce and store it
				nonce, err := auth.digestAuth.GenerateNonce()
				if err != nil {
					t.Fatalf("Failed to generate nonce: %v", err)
				}
				auth.digestAuth.(*SIPDigestAuthenticator).nonceStore.StoreNonce(nonce)

				// Calculate response
				uri := "sip:example.com"
				user, _ := userManager.GetUser(username, realm)
				
				var response string
				if tt.validCredentials && user != nil {
					// Calculate correct response
					ha2Data := fmt.Sprintf("%s:%s", tt.method, uri)
					ha2Hash := md5.Sum([]byte(ha2Data))
					ha2 := fmt.Sprintf("%x", ha2Hash)
					
					responseData := fmt.Sprintf("%s:%s:%s", user.PasswordHash, nonce, ha2)
					responseHash := md5.Sum([]byte(responseData))
					response = fmt.Sprintf("%x", responseHash)
				} else {
					response = "invalidresponse"
				}

				authHeader := fmt.Sprintf(`Digest username="%s", realm="%s", nonce="%s", uri="%s", response="%s", algorithm="MD5"`,
					username, realm, nonce, uri, response)
				msg.SetHeader(parser.HeaderAuthorization, authHeader)
			}

			// Test authentication
			result, err := auth.AuthenticateRequest(msg, userManager)
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			if result.Authenticated != tt.expectedAuth {
				t.Errorf("Expected authenticated=%v, got %v", tt.expectedAuth, result.Authenticated)
			}

			if result.RequiresAuth != tt.expectedRequiresAuth {
				t.Errorf("Expected requiresAuth=%v, got %v", tt.expectedRequiresAuth, result.RequiresAuth)
			}

			// Check user is set for successful authentication
			if result.Authenticated && result.RequiresAuth && result.User == nil {
				t.Error("Expected user to be set for successful authentication")
			}
		})
	}
}

func TestSIPMessageAuthenticator_CreateAuthChallenge(t *testing.T) {
	realm := "example.com"
	auth := NewSIPMessageAuthenticator(realm)

	// Create test request
	request := parser.NewRequestMessage(parser.MethodREGISTER, "sip:example.com")
	request.SetHeader(parser.HeaderFrom, "sip:alice@example.com")
	request.SetHeader(parser.HeaderTo, "sip:example.com")
	request.SetHeader(parser.HeaderCallID, "test-call-id")
	request.SetHeader(parser.HeaderCSeq, "1 REGISTER")
	request.SetHeader(parser.HeaderVia, "SIP/2.0/UDP 192.168.1.100:5060")

	// Create auth challenge
	response, err := auth.CreateAuthChallenge(request, realm)
	if err != nil {
		t.Fatalf("Failed to create auth challenge: %v", err)
	}

	// Verify response is 401 Unauthorized
	if response.GetStatusCode() != parser.StatusUnauthorized {
		t.Errorf("Expected status code %d, got %d", parser.StatusUnauthorized, response.GetStatusCode())
	}

	// Verify WWW-Authenticate header is present
	wwwAuth := response.GetHeader(parser.HeaderWWWAuthenticate)
	if wwwAuth == "" {
		t.Error("WWW-Authenticate header should be present")
	}

	// Verify WWW-Authenticate header format
	if !strings.HasPrefix(wwwAuth, "Digest ") {
		t.Error("WWW-Authenticate header should start with 'Digest '")
	}

	if !strings.Contains(wwwAuth, fmt.Sprintf(`realm="%s"`, realm)) {
		t.Error("WWW-Authenticate header should contain the specified realm")
	}

	// Verify required headers are copied
	if response.GetHeader(parser.HeaderVia) != request.GetHeader(parser.HeaderVia) {
		t.Error("Via header should be copied from request")
	}

	if response.GetHeader(parser.HeaderFrom) != request.GetHeader(parser.HeaderFrom) {
		t.Error("From header should be copied from request")
	}

	if response.GetHeader(parser.HeaderCallID) != request.GetHeader(parser.HeaderCallID) {
		t.Error("Call-ID header should be copied from request")
	}

	if response.GetHeader(parser.HeaderCSeq) != request.GetHeader(parser.HeaderCSeq) {
		t.Error("CSeq header should be copied from request")
	}

	// Verify To header has tag added
	toHeader := response.GetHeader(parser.HeaderTo)
	if !strings.Contains(toHeader, "tag=") {
		t.Error("To header should have tag added")
	}

	// Verify Content-Length is 0
	if response.GetHeader(parser.HeaderContentLength) != "0" {
		t.Error("Content-Length should be 0")
	}

	// Verify Server header is present
	if response.GetHeader(parser.HeaderServer) == "" {
		t.Error("Server header should be present")
	}
}

func TestSIPMessageAuthenticator_CreateAuthFailureResponse(t *testing.T) {
	realm := "example.com"
	auth := NewSIPMessageAuthenticator(realm)

	// Create test request
	request := parser.NewRequestMessage(parser.MethodREGISTER, "sip:example.com")
	request.SetHeader(parser.HeaderFrom, "sip:alice@example.com")
	request.SetHeader(parser.HeaderTo, "sip:example.com")
	request.SetHeader(parser.HeaderCallID, "test-call-id")
	request.SetHeader(parser.HeaderCSeq, "1 REGISTER")
	request.SetHeader(parser.HeaderVia, "SIP/2.0/UDP 192.168.1.100:5060")

	// Create auth failure response
	response, err := auth.CreateAuthFailureResponse(request)
	if err != nil {
		t.Fatalf("Failed to create auth failure response: %v", err)
	}

	// Verify response is 403 Forbidden
	if response.GetStatusCode() != parser.StatusForbidden {
		t.Errorf("Expected status code %d, got %d", parser.StatusForbidden, response.GetStatusCode())
	}

	// Verify required headers are copied
	if response.GetHeader(parser.HeaderVia) != request.GetHeader(parser.HeaderVia) {
		t.Error("Via header should be copied from request")
	}

	if response.GetHeader(parser.HeaderFrom) != request.GetHeader(parser.HeaderFrom) {
		t.Error("From header should be copied from request")
	}

	if response.GetHeader(parser.HeaderCallID) != request.GetHeader(parser.HeaderCallID) {
		t.Error("Call-ID header should be copied from request")
	}

	if response.GetHeader(parser.HeaderCSeq) != request.GetHeader(parser.HeaderCSeq) {
		t.Error("CSeq header should be copied from request")
	}

	// Verify Content-Length is 0
	if response.GetHeader(parser.HeaderContentLength) != "0" {
		t.Error("Content-Length should be 0")
	}

	// Verify Server header is present
	if response.GetHeader(parser.HeaderServer) == "" {
		t.Error("Server header should be present")
	}
}

func TestSIPMessageAuthenticator_RequiresAuthentication(t *testing.T) {
	realm := "example.com"
	auth := NewSIPMessageAuthenticator(realm)

	tests := []struct {
		method   string
		expected bool
	}{
		{parser.MethodREGISTER, true},
		{parser.MethodINVITE, true},
		{parser.MethodOPTIONS, false},
		{parser.MethodACK, false},
		{parser.MethodBYE, false},
		{parser.MethodCANCEL, false},
		{parser.MethodINFO, false},
	}

	for _, tt := range tests {
		t.Run(tt.method, func(t *testing.T) {
			result := auth.requiresAuthentication(tt.method)
			if result != tt.expected {
				t.Errorf("Expected requiresAuthentication(%s)=%v, got %v", tt.method, tt.expected, result)
			}
		})
	}
}

func TestSIPMessageAuthenticator_SetGetRealm(t *testing.T) {
	realm := "example.com"
	auth := NewSIPMessageAuthenticator(realm)

	// Test initial realm
	if auth.GetRealm() != realm {
		t.Errorf("Expected realm %s, got %s", realm, auth.GetRealm())
	}

	// Test setting new realm
	newRealm := "test.com"
	auth.SetRealm(newRealm)
	
	if auth.GetRealm() != newRealm {
		t.Errorf("Expected realm %s, got %s", newRealm, auth.GetRealm())
	}
}