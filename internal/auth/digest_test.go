package auth

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/zurustar/xylitol2/internal/database"
)

func TestSIPDigestAuthenticator_GenerateChallenge(t *testing.T) {
	auth := NewSIPDigestAuthenticator()
	realm := "example.com"

	challenge, err := auth.GenerateChallenge(realm)
	if err != nil {
		t.Fatalf("Failed to generate challenge: %v", err)
	}

	// Verify challenge format
	if !strings.HasPrefix(challenge, "Digest ") {
		t.Error("Challenge should start with 'Digest '")
	}

	// Verify realm is present
	if !strings.Contains(challenge, fmt.Sprintf(`realm="%s"`, realm)) {
		t.Error("Challenge should contain the specified realm")
	}

	// Verify required parameters are present
	requiredParams := []string{"nonce=", "opaque=", "algorithm=MD5", `qop="auth"`}
	for _, param := range requiredParams {
		if !strings.Contains(challenge, param) {
			t.Errorf("Challenge should contain parameter: %s", param)
		}
	}
}

func TestSIPDigestAuthenticator_GenerateNonce(t *testing.T) {
	auth := NewSIPDigestAuthenticator()

	nonce1, err := auth.GenerateNonce()
	if err != nil {
		t.Fatalf("Failed to generate nonce: %v", err)
	}

	nonce2, err := auth.GenerateNonce()
	if err != nil {
		t.Fatalf("Failed to generate second nonce: %v", err)
	}

	// Verify nonces are different
	if nonce1 == nonce2 {
		t.Error("Generated nonces should be different")
	}

	// Verify nonce format (should be 32 character hex string)
	if len(nonce1) != 32 {
		t.Errorf("Expected nonce length 32, got %d", len(nonce1))
	}

	// Verify nonce is valid hex
	if _, err := hex.DecodeString(nonce1); err != nil {
		t.Errorf("Nonce should be valid hex string: %v", err)
	}
}

func TestSIPDigestAuthenticator_ParseAuthorizationHeader(t *testing.T) {
	auth := NewSIPDigestAuthenticator()

	tests := []struct {
		name        string
		authHeader  string
		expectError bool
		expected    *DigestCredentials
	}{
		{
			name: "Valid authorization header with quotes",
			authHeader: `Digest username="alice", realm="example.com", nonce="abc123", uri="sip:example.com", response="def456", algorithm="MD5", opaque="xyz789", qop="auth", nc="00000001", cnonce="client123"`,
			expectError: false,
			expected: &DigestCredentials{
				Username:  "alice",
				Realm:     "example.com",
				Nonce:     "abc123",
				URI:       "sip:example.com",
				Response:  "def456",
				Algorithm: "MD5",
				Opaque:    "xyz789",
				QOP:       "auth",
				NC:        "00000001",
				CNonce:    "client123",
			},
		},
		{
			name: "Valid authorization header without quotes",
			authHeader: `Digest username=alice, realm=example.com, nonce=abc123, uri=sip:example.com, response=def456`,
			expectError: false,
			expected: &DigestCredentials{
				Username:  "alice",
				Realm:     "example.com",
				Nonce:     "abc123",
				URI:       "sip:example.com",
				Response:  "def456",
				Algorithm: "MD5", // default
			},
		},
		{
			name:        "Invalid header - not digest",
			authHeader:  "Basic dXNlcjpwYXNz",
			expectError: true,
		},
		{
			name:        "Missing username",
			authHeader:  `Digest realm="example.com", nonce="abc123", uri="sip:example.com", response="def456"`,
			expectError: true,
		},
		{
			name:        "Missing realm",
			authHeader:  `Digest username="alice", nonce="abc123", uri="sip:example.com", response="def456"`,
			expectError: true,
		},
		{
			name:        "Missing nonce",
			authHeader:  `Digest username="alice", realm="example.com", uri="sip:example.com", response="def456"`,
			expectError: true,
		},
		{
			name:        "Missing uri",
			authHeader:  `Digest username="alice", realm="example.com", nonce="abc123", response="def456"`,
			expectError: true,
		},
		{
			name:        "Missing response",
			authHeader:  `Digest username="alice", realm="example.com", nonce="abc123", uri="sip:example.com"`,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			creds, err := auth.ParseAuthorizationHeader(tt.authHeader)
			
			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got none")
				}
				return
			}

			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			if creds.Username != tt.expected.Username {
				t.Errorf("Expected username %s, got %s", tt.expected.Username, creds.Username)
			}
			if creds.Realm != tt.expected.Realm {
				t.Errorf("Expected realm %s, got %s", tt.expected.Realm, creds.Realm)
			}
			if creds.Nonce != tt.expected.Nonce {
				t.Errorf("Expected nonce %s, got %s", tt.expected.Nonce, creds.Nonce)
			}
			if creds.URI != tt.expected.URI {
				t.Errorf("Expected URI %s, got %s", tt.expected.URI, creds.URI)
			}
			if creds.Response != tt.expected.Response {
				t.Errorf("Expected response %s, got %s", tt.expected.Response, creds.Response)
			}
			if creds.Algorithm != tt.expected.Algorithm {
				t.Errorf("Expected algorithm %s, got %s", tt.expected.Algorithm, creds.Algorithm)
			}
		})
	}
}

func TestSIPDigestAuthenticator_ValidateCredentials(t *testing.T) {
	auth := NewSIPDigestAuthenticator()

	// Create test user
	username := "alice"
	realm := "example.com"
	password := "secret123"
	
	// Generate password hash like SIPUserManager does
	passwordHash := fmt.Sprintf("%x", md5.Sum([]byte(fmt.Sprintf("%s:%s:%s", username, realm, password))))
	
	user := &database.User{
		Username:     username,
		Realm:        realm,
		PasswordHash: passwordHash,
		Enabled:      true,
	}

	// Generate a valid nonce and store it
	nonce, err := auth.GenerateNonce()
	if err != nil {
		t.Fatalf("Failed to generate nonce: %v", err)
	}
	auth.nonceStore.StoreNonce(nonce)

	// Calculate valid response
	method := "REGISTER"
	uri := "sip:example.com"
	
	// Calculate HA2
	ha2Data := fmt.Sprintf("%s:%s", method, uri)
	ha2Hash := md5.Sum([]byte(ha2Data))
	ha2 := fmt.Sprintf("%x", ha2Hash)
	
	// Calculate response (without qop)
	responseData := fmt.Sprintf("%s:%s:%s", passwordHash, nonce, ha2)
	responseHash := md5.Sum([]byte(responseData))
	expectedResponse := fmt.Sprintf("%x", responseHash)

	// Create valid authorization header
	validAuthHeader := fmt.Sprintf(`Digest username="%s", realm="%s", nonce="%s", uri="%s", response="%s", algorithm="MD5"`,
		username, realm, nonce, uri, expectedResponse)

	// Test valid credentials
	valid, err := auth.ValidateCredentials(validAuthHeader, method, user)
	if err != nil {
		t.Fatalf("Unexpected error validating credentials: %v", err)
	}
	if !valid {
		t.Error("Valid credentials should pass validation")
	}

	// Test invalid response
	invalidAuthHeader := fmt.Sprintf(`Digest username="%s", realm="%s", nonce="%s", uri="%s", response="%s", algorithm="MD5"`,
		username, realm, nonce, uri, "invalidresponse")
	
	valid, err = auth.ValidateCredentials(invalidAuthHeader, method, user)
	if err == nil {
		t.Error("Expected error for invalid response")
	}
	if valid {
		t.Error("Invalid credentials should fail validation")
	}

	// Test invalid nonce
	invalidNonceHeader := fmt.Sprintf(`Digest username="%s", realm="%s", nonce="%s", uri="%s", response="%s", algorithm="MD5"`,
		username, realm, "invalidnonce", uri, expectedResponse)
	
	valid, err = auth.ValidateCredentials(invalidNonceHeader, method, user)
	if err == nil {
		t.Error("Expected error for invalid nonce")
	}
	if valid {
		t.Error("Invalid nonce should fail validation")
	}
}

func TestSIPDigestAuthenticator_ValidateCredentialsWithQOP(t *testing.T) {
	auth := NewSIPDigestAuthenticator()

	// Create test user
	username := "alice"
	realm := "example.com"
	password := "secret123"
	
	passwordHash := fmt.Sprintf("%x", md5.Sum([]byte(fmt.Sprintf("%s:%s:%s", username, realm, password))))
	
	user := &database.User{
		Username:     username,
		Realm:        realm,
		PasswordHash: passwordHash,
		Enabled:      true,
	}

	// Generate a valid nonce and store it
	nonce, err := auth.GenerateNonce()
	if err != nil {
		t.Fatalf("Failed to generate nonce: %v", err)
	}
	auth.nonceStore.StoreNonce(nonce)

	// Calculate valid response with qop=auth
	method := "REGISTER"
	uri := "sip:example.com"
	qop := "auth"
	nc := "00000001"
	cnonce := "clientnonce123"
	
	// Calculate HA2
	ha2Data := fmt.Sprintf("%s:%s", method, uri)
	ha2Hash := md5.Sum([]byte(ha2Data))
	ha2 := fmt.Sprintf("%x", ha2Hash)
	
	// Calculate response with qop
	responseData := fmt.Sprintf("%s:%s:%s:%s:%s:%s", passwordHash, nonce, nc, cnonce, qop, ha2)
	responseHash := md5.Sum([]byte(responseData))
	expectedResponse := fmt.Sprintf("%x", responseHash)

	// Create valid authorization header with qop
	validAuthHeader := fmt.Sprintf(`Digest username="%s", realm="%s", nonce="%s", uri="%s", response="%s", algorithm="MD5", qop="%s", nc="%s", cnonce="%s"`,
		username, realm, nonce, uri, expectedResponse, qop, nc, cnonce)

	// Test valid credentials with qop
	valid, err := auth.ValidateCredentials(validAuthHeader, method, user)
	if err != nil {
		t.Fatalf("Unexpected error validating credentials with qop: %v", err)
	}
	if !valid {
		t.Error("Valid credentials with qop should pass validation")
	}
}

func TestMemoryNonceStore(t *testing.T) {
	store := NewMemoryNonceStore()

	// Test storing and validating nonce
	nonce := "testnonce123"
	err := store.StoreNonce(nonce)
	if err != nil {
		t.Fatalf("Failed to store nonce: %v", err)
	}

	// Nonce should be valid immediately after storing
	if !store.ValidateNonce(nonce) {
		t.Error("Stored nonce should be valid")
	}

	// Invalid nonce should not be valid
	if store.ValidateNonce("invalidnonce") {
		t.Error("Invalid nonce should not be valid")
	}

	// Test cleanup
	store.CleanupExpired()
	
	// Nonce should still be valid after cleanup (not expired yet)
	if !store.ValidateNonce(nonce) {
		t.Error("Non-expired nonce should still be valid after cleanup")
	}
}

func TestMemoryNonceStore_Expiration(t *testing.T) {
	// Create store with very short TTL for testing
	store := &MemoryNonceStore{
		nonces: make(map[string]time.Time),
		ttl:    10 * time.Millisecond,
	}

	nonce := "testnonce123"
	err := store.StoreNonce(nonce)
	if err != nil {
		t.Fatalf("Failed to store nonce: %v", err)
	}

	// Nonce should be valid initially
	if !store.ValidateNonce(nonce) {
		t.Error("Stored nonce should be valid initially")
	}

	// Wait for expiration
	time.Sleep(20 * time.Millisecond)

	// Nonce should be expired now
	if store.ValidateNonce(nonce) {
		t.Error("Expired nonce should not be valid")
	}

	// Cleanup should remove expired nonce
	store.CleanupExpired()
	
	// Verify nonce was removed
	store.mutex.RLock()
	_, exists := store.nonces[nonce]
	store.mutex.RUnlock()
	
	if exists {
		t.Error("Expired nonce should be removed after cleanup")
	}
}