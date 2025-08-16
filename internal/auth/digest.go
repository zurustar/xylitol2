package auth

import (
	"crypto/md5"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/zurustar/xylitol2/internal/database"
)

// SIPDigestAuthenticator implements RFC2617 digest authentication for SIP
type SIPDigestAuthenticator struct {
	nonceStore NonceStore
}

// NewSIPDigestAuthenticator creates a new digest authenticator
func NewSIPDigestAuthenticator() *SIPDigestAuthenticator {
	return &SIPDigestAuthenticator{
		nonceStore: NewMemoryNonceStore(),
	}
}

// NewSIPDigestAuthenticatorWithStore creates a new digest authenticator with custom nonce store
func NewSIPDigestAuthenticatorWithStore(store NonceStore) *SIPDigestAuthenticator {
	return &SIPDigestAuthenticator{
		nonceStore: store,
	}
}

// GenerateChallenge creates a WWW-Authenticate header value for digest authentication
func (d *SIPDigestAuthenticator) GenerateChallenge(realm string) (string, error) {
	nonce, err := d.GenerateNonce()
	if err != nil {
		return "", fmt.Errorf("failed to generate nonce: %w", err)
	}

	// Store the nonce for later validation
	if err := d.nonceStore.StoreNonce(nonce); err != nil {
		return "", fmt.Errorf("failed to store nonce: %w", err)
	}

	// Generate opaque value
	opaque, err := d.generateOpaque()
	if err != nil {
		return "", fmt.Errorf("failed to generate opaque: %w", err)
	}

	// Create digest challenge according to RFC2617
	challenge := fmt.Sprintf(`Digest realm="%s", nonce="%s", opaque="%s", algorithm=MD5, qop="auth"`,
		realm, nonce, opaque)

	return challenge, nil
}

// ValidateCredentials validates Authorization header credentials against user database
func (d *SIPDigestAuthenticator) ValidateCredentials(authHeader string, method string, user *database.User) (bool, error) {
	// Parse the authorization header
	creds, err := d.ParseAuthorizationHeader(authHeader)
	if err != nil {
		return false, fmt.Errorf("failed to parse authorization header: %w", err)
	}

	// Validate nonce
	if !d.ValidateNonce(creds.Nonce) {
		return false, fmt.Errorf("invalid or expired nonce")
	}

	// Validate user credentials
	if creds.Username != user.Username || creds.Realm != user.Realm {
		return false, fmt.Errorf("username or realm mismatch")
	}

	// Calculate expected response
	expectedResponse, err := d.calculateDigestResponse(creds, method, user.PasswordHash)
	if err != nil {
		return false, fmt.Errorf("failed to calculate digest response: %w", err)
	}

	// Compare responses
	if creds.Response != expectedResponse {
		return false, fmt.Errorf("invalid credentials")
	}

	return true, nil
}

// ParseAuthorizationHeader parses an Authorization header and returns digest parameters
func (d *SIPDigestAuthenticator) ParseAuthorizationHeader(authHeader string) (*DigestCredentials, error) {
	// Remove "Digest " prefix
	if !strings.HasPrefix(authHeader, "Digest ") {
		return nil, fmt.Errorf("not a digest authorization header")
	}

	digestPart := strings.TrimPrefix(authHeader, "Digest ")
	
	// Parse digest parameters using regex
	creds := &DigestCredentials{}
	
	// Define regex patterns for each parameter
	patterns := map[string]*regexp.Regexp{
		"username":  regexp.MustCompile(`username="([^"]*)"` + `|username=([^,\s]*)`),
		"realm":     regexp.MustCompile(`realm="([^"]*)"` + `|realm=([^,\s]*)`),
		"nonce":     regexp.MustCompile(`nonce="([^"]*)"` + `|nonce=([^,\s]*)`),
		"uri":       regexp.MustCompile(`uri="([^"]*)"` + `|uri=([^,\s]*)`),
		"response":  regexp.MustCompile(`response="([^"]*)"` + `|response=([^,\s]*)`),
		"algorithm": regexp.MustCompile(`algorithm="([^"]*)"` + `|algorithm=([^,\s]*)`),
		"opaque":    regexp.MustCompile(`opaque="([^"]*)"` + `|opaque=([^,\s]*)`),
		"qop":       regexp.MustCompile(`qop="([^"]*)"` + `|qop=([^,\s]*)`),
		"nc":        regexp.MustCompile(`nc="([^"]*)"` + `|nc=([^,\s]*)`),
		"cnonce":    regexp.MustCompile(`cnonce="([^"]*)"` + `|cnonce=([^,\s]*)`),
	}

	// Extract each parameter
	for param, pattern := range patterns {
		matches := pattern.FindStringSubmatch(digestPart)
		if len(matches) > 1 {
			// Use the first non-empty match (quoted or unquoted)
			value := matches[1]
			if value == "" && len(matches) > 2 {
				value = matches[2]
			}
			
			switch param {
			case "username":
				creds.Username = value
			case "realm":
				creds.Realm = value
			case "nonce":
				creds.Nonce = value
			case "uri":
				creds.URI = value
			case "response":
				creds.Response = value
			case "algorithm":
				creds.Algorithm = value
			case "opaque":
				creds.Opaque = value
			case "qop":
				creds.QOP = value
			case "nc":
				creds.NC = value
			case "cnonce":
				creds.CNonce = value
			}
		}
	}

	// Validate required fields
	if creds.Username == "" {
		return nil, fmt.Errorf("missing username in authorization header")
	}
	if creds.Realm == "" {
		return nil, fmt.Errorf("missing realm in authorization header")
	}
	if creds.Nonce == "" {
		return nil, fmt.Errorf("missing nonce in authorization header")
	}
	if creds.URI == "" {
		return nil, fmt.Errorf("missing uri in authorization header")
	}
	if creds.Response == "" {
		return nil, fmt.Errorf("missing response in authorization header")
	}

	// Set default algorithm if not specified
	if creds.Algorithm == "" {
		creds.Algorithm = "MD5"
	}

	return creds, nil
}

// GenerateNonce creates a new nonce value for authentication challenges
func (d *SIPDigestAuthenticator) GenerateNonce() (string, error) {
	// Generate 16 random bytes
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("failed to generate random bytes: %w", err)
	}

	// Add timestamp to make nonce unique and time-based
	timestamp := time.Now().Unix()
	nonceData := fmt.Sprintf("%x:%d", bytes, timestamp)
	
	// Hash the nonce data
	hash := md5.Sum([]byte(nonceData))
	return hex.EncodeToString(hash[:]), nil
}

// ValidateNonce checks if a nonce is valid and not expired
func (d *SIPDigestAuthenticator) ValidateNonce(nonce string) bool {
	return d.nonceStore.ValidateNonce(nonce)
}

// calculateDigestResponse calculates the expected digest response
func (d *SIPDigestAuthenticator) calculateDigestResponse(creds *DigestCredentials, method string, passwordHash string) (string, error) {
	// For SIP, the password hash is already MD5(username:realm:password)
	// So we use it directly as HA1
	ha1 := passwordHash

	// Calculate HA2 = MD5(method:uri)
	ha2Data := fmt.Sprintf("%s:%s", method, creds.URI)
	ha2Hash := md5.Sum([]byte(ha2Data))
	ha2 := hex.EncodeToString(ha2Hash[:])

	var response string
	
	// Calculate response based on qop
	if creds.QOP == "auth" || creds.QOP == "auth-int" {
		// With qop: MD5(HA1:nonce:nc:cnonce:qop:HA2)
		responseData := fmt.Sprintf("%s:%s:%s:%s:%s:%s",
			ha1, creds.Nonce, creds.NC, creds.CNonce, creds.QOP, ha2)
		responseHash := md5.Sum([]byte(responseData))
		response = hex.EncodeToString(responseHash[:])
	} else {
		// Without qop: MD5(HA1:nonce:HA2)
		responseData := fmt.Sprintf("%s:%s:%s", ha1, creds.Nonce, ha2)
		responseHash := md5.Sum([]byte(responseData))
		response = hex.EncodeToString(responseHash[:])
	}

	return response, nil
}

// generateOpaque generates an opaque value for the challenge
func (d *SIPDigestAuthenticator) generateOpaque() (string, error) {
	bytes := make([]byte, 8)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("failed to generate random bytes: %w", err)
	}
	return hex.EncodeToString(bytes), nil
}

// MemoryNonceStore implements NonceStore using in-memory storage
type MemoryNonceStore struct {
	nonces map[string]time.Time
	mutex  sync.RWMutex
	ttl    time.Duration
}

// NewMemoryNonceStore creates a new memory-based nonce store
func NewMemoryNonceStore() *MemoryNonceStore {
	store := &MemoryNonceStore{
		nonces: make(map[string]time.Time),
		ttl:    5 * time.Minute, // 5 minute TTL for nonces
	}
	
	// Start cleanup goroutine
	go store.cleanupLoop()
	
	return store
}

// StoreNonce stores a nonce with expiration time
func (s *MemoryNonceStore) StoreNonce(nonce string) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	
	s.nonces[nonce] = time.Now().Add(s.ttl)
	return nil
}

// ValidateNonce checks if a nonce exists and is not expired
func (s *MemoryNonceStore) ValidateNonce(nonce string) bool {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	
	expiry, exists := s.nonces[nonce]
	if !exists {
		return false
	}
	
	return time.Now().Before(expiry)
}

// CleanupExpired removes expired nonces
func (s *MemoryNonceStore) CleanupExpired() {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	
	now := time.Now()
	for nonce, expiry := range s.nonces {
		if now.After(expiry) {
			delete(s.nonces, nonce)
		}
	}
}

// cleanupLoop runs periodic cleanup of expired nonces
func (s *MemoryNonceStore) cleanupLoop() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()
	
	for range ticker.C {
		s.CleanupExpired()
	}
}