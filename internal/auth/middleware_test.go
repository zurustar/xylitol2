package auth

import (
	"strings"
	"testing"

	"github.com/zurustar/xylitol2/internal/parser"
)

func TestAuthenticationMiddleware_SetGetRealm(t *testing.T) {
	userManager := NewMockUserManager()
	realm := "example.com"
	middleware := NewAuthenticationMiddleware(userManager, realm)

	// Test initial realm
	if middleware.GetRealm() != realm {
		t.Errorf("Expected realm %s, got %s", realm, middleware.GetRealm())
	}

	// Test setting new realm
	newRealm := "test.com"
	middleware.SetRealm(newRealm)

	if middleware.GetRealm() != newRealm {
		t.Errorf("Expected realm %s, got %s", newRealm, middleware.GetRealm())
	}
}

func TestAuthenticationMiddleware_RequiresAuthentication(t *testing.T) {
	userManager := NewMockUserManager()
	realm := "example.com"
	middleware := NewAuthenticationMiddleware(userManager, realm)

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
			result := middleware.RequiresAuthentication(tt.method)
			if result != tt.expected {
				t.Errorf("Expected RequiresAuthentication(%s)=%v, got %v", tt.method, tt.expected, result)
			}
		})
	}
}

func TestAuthenticationMiddleware_ValidateAuthorizationHeader(t *testing.T) {
	userManager := NewMockUserManager()
	realm := "example.com"
	middleware := NewAuthenticationMiddleware(userManager, realm)

	// Create test user
	username := "alice"
	password := "secret123"
	userManager.CreateUser(username, realm, password)

	// Create test request
	request := parser.NewRequestMessage(parser.MethodREGISTER, "sip:example.com")
	request.SetHeader(parser.HeaderFrom, "sip:alice@example.com")
	request.SetHeader(parser.HeaderTo, "sip:example.com")
	request.SetHeader(parser.HeaderCallID, "test-call-id")
	request.SetHeader(parser.HeaderCSeq, "1 REGISTER")
	request.SetHeader(parser.HeaderVia, "SIP/2.0/UDP 192.168.1.100:5060")

	// Test with valid authorization header
	validAuthHeader := createTestAuthHeader(t, middleware, username, realm, password, parser.MethodREGISTER, "sip:example.com", true, userManager)
	
	user, err := middleware.ValidateAuthorizationHeader(request, validAuthHeader)
	if err != nil {
		t.Fatalf("Unexpected error with valid auth header: %v", err)
	}
	if user == nil {
		t.Fatal("Expected user but got none")
	}
	if user.Username != username {
		t.Errorf("Expected username %s, got %s", username, user.Username)
	}
	if user.Realm != realm {
		t.Errorf("Expected realm %s, got %s", realm, user.Realm)
	}

	// Test with invalid authorization header
	invalidAuthHeader := createTestAuthHeader(t, middleware, username, realm, password, parser.MethodREGISTER, "sip:example.com", false, userManager)
	
	user, err = middleware.ValidateAuthorizationHeader(request, invalidAuthHeader)
	if err == nil {
		t.Error("Expected error with invalid auth header")
	}
	if user != nil {
		t.Error("Expected no user with invalid auth header")
	}

	// Test with malformed authorization header
	malformedAuthHeader := "Digest invalid"
	
	user, err = middleware.ValidateAuthorizationHeader(request, malformedAuthHeader)
	if err == nil {
		t.Error("Expected error with malformed auth header")
	}
	if user != nil {
		t.Error("Expected no user with malformed auth header")
	}

	// Test with non-existent user
	nonExistentAuthHeader := createTestAuthHeader(t, middleware, "nonexistent", realm, password, parser.MethodREGISTER, "sip:example.com", true, userManager)
	
	user, err = middleware.ValidateAuthorizationHeader(request, nonExistentAuthHeader)
	if err == nil {
		t.Error("Expected error with non-existent user")
	}
	if user != nil {
		t.Error("Expected no user with non-existent user")
	}

	// Test with disabled user
	userManager.DisableUser(username, realm)
	
	user, err = middleware.ValidateAuthorizationHeader(request, validAuthHeader)
	if err == nil {
		t.Error("Expected error with disabled user")
	}
	if user != nil {
		t.Error("Expected no user with disabled user")
	}
}

func TestAuthenticationMiddleware_ProcessRequest_ErrorHandling(t *testing.T) {
	userManager := NewMockUserManager()
	realm := "example.com"
	middleware := NewAuthenticationMiddleware(userManager, realm)

	// Create test user
	username := "alice"
	password := "secret123"
	userManager.CreateUser(username, realm, password)

	// Test with malformed authorization header
	request := parser.NewRequestMessage(parser.MethodREGISTER, "sip:example.com")
	request.SetHeader(parser.HeaderFrom, "sip:alice@example.com")
	request.SetHeader(parser.HeaderTo, "sip:example.com")
	request.SetHeader(parser.HeaderCallID, "test-call-id")
	request.SetHeader(parser.HeaderCSeq, "1 REGISTER")
	request.SetHeader(parser.HeaderVia, "SIP/2.0/UDP 192.168.1.100:5060")
	request.SetHeader(parser.HeaderAuthorization, "Digest invalid")

	response, authenticated, err := middleware.ProcessRequest(request)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Should return 403 Forbidden for malformed auth header
	if response == nil {
		t.Fatal("Expected response but got none")
	}
	if response.GetStatusCode() != parser.StatusForbidden {
		t.Errorf("Expected status code %d, got %d", parser.StatusForbidden, response.GetStatusCode())
	}
	if authenticated {
		t.Error("Expected authenticated=false")
	}
}

func TestAuthenticationMiddleware_ProcessRequest_AuthChallenge(t *testing.T) {
	userManager := NewMockUserManager()
	realm := "example.com"
	middleware := NewAuthenticationMiddleware(userManager, realm)

	// Create REGISTER request without authorization header
	request := parser.NewRequestMessage(parser.MethodREGISTER, "sip:example.com")
	request.SetHeader(parser.HeaderFrom, "sip:alice@example.com")
	request.SetHeader(parser.HeaderTo, "sip:example.com")
	request.SetHeader(parser.HeaderCallID, "test-call-id")
	request.SetHeader(parser.HeaderCSeq, "1 REGISTER")
	request.SetHeader(parser.HeaderVia, "SIP/2.0/UDP 192.168.1.100:5060")

	response, authenticated, err := middleware.ProcessRequest(request)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Should return 401 Unauthorized with WWW-Authenticate header
	if response == nil {
		t.Fatal("Expected response but got none")
	}
	if response.GetStatusCode() != parser.StatusUnauthorized {
		t.Errorf("Expected status code %d, got %d", parser.StatusUnauthorized, response.GetStatusCode())
	}
	if authenticated {
		t.Error("Expected authenticated=false")
	}

	// Verify WWW-Authenticate header
	wwwAuth := response.GetHeader(parser.HeaderWWWAuthenticate)
	if wwwAuth == "" {
		t.Error("Expected WWW-Authenticate header")
	}
	if !strings.HasPrefix(wwwAuth, "Digest ") {
		t.Error("WWW-Authenticate header should start with 'Digest '")
	}
	if !strings.Contains(wwwAuth, `realm="`+realm+`"`) {
		t.Error("WWW-Authenticate header should contain the correct realm")
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
}