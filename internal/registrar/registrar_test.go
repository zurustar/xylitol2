package registrar

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/zurustar/xylitol2/internal/auth"
	"github.com/zurustar/xylitol2/internal/database"
	"github.com/zurustar/xylitol2/internal/parser"
)

// Mock implementations for testing

type mockRegistrationDB struct {
	contacts map[string][]*database.RegistrarContact
}

func newMockRegistrationDB() *mockRegistrationDB {
	return &mockRegistrationDB{
		contacts: make(map[string][]*database.RegistrarContact),
	}
}

func (m *mockRegistrationDB) Store(contact *database.RegistrarContact) error {
	// Remove existing contact with same URI
	if contacts, exists := m.contacts[contact.AOR]; exists {
		for i, c := range contacts {
			if c.URI == contact.URI {
				m.contacts[contact.AOR] = append(contacts[:i], contacts[i+1:]...)
				break
			}
		}
	}
	
	// Add new contact
	m.contacts[contact.AOR] = append(m.contacts[contact.AOR], contact)
	return nil
}

func (m *mockRegistrationDB) Retrieve(aor string) ([]*database.RegistrarContact, error) {
	contacts := m.contacts[aor]
	var validContacts []*database.RegistrarContact
	
	// Filter out expired contacts
	now := time.Now().UTC()
	for _, contact := range contacts {
		if contact.Expires.After(now) {
			validContacts = append(validContacts, contact)
		}
	}
	
	return validContacts, nil
}

func (m *mockRegistrationDB) Delete(aor string, contactURI string) error {
	if contacts, exists := m.contacts[aor]; exists {
		for i, contact := range contacts {
			if contact.URI == contactURI {
				m.contacts[aor] = append(contacts[:i], contacts[i+1:]...)
				return nil
			}
		}
	}
	return fmt.Errorf("contact not found")
}

func (m *mockRegistrationDB) CleanupExpired() error {
	now := time.Now().UTC()
	for aor, contacts := range m.contacts {
		var validContacts []*database.RegistrarContact
		for _, contact := range contacts {
			if contact.Expires.After(now) {
				validContacts = append(validContacts, contact)
			}
		}
		m.contacts[aor] = validContacts
	}
	return nil
}

type mockMessageAuthenticator struct {
	authenticated bool
	requiresAuth  bool
	user          *database.User
}

func newMockMessageAuthenticator(authenticated, requiresAuth bool) *mockMessageAuthenticator {
	return &mockMessageAuthenticator{
		authenticated: authenticated,
		requiresAuth:  requiresAuth,
		user: &database.User{
			Username: "testuser",
			Realm:    "example.com",
		},
	}
}

func (m *mockMessageAuthenticator) AuthenticateRequest(msg *parser.SIPMessage, userManager database.UserManager) (*auth.AuthResult, error) {
	return &auth.AuthResult{
		Authenticated: m.authenticated,
		RequiresAuth:  m.requiresAuth,
		User:          m.user,
	}, nil
}

func (m *mockMessageAuthenticator) CreateAuthChallenge(request *parser.SIPMessage, realm string) (*parser.SIPMessage, error) {
	response := parser.NewResponseMessage(parser.StatusUnauthorized, "Unauthorized")
	response.SetHeader(parser.HeaderWWWAuthenticate, fmt.Sprintf(`Digest realm="%s", nonce="test-nonce"`, realm))
	return response, nil
}

func (m *mockMessageAuthenticator) CreateAuthFailureResponse(request *parser.SIPMessage) (*parser.SIPMessage, error) {
	return parser.NewResponseMessage(parser.StatusForbidden, "Forbidden"), nil
}

func (m *mockMessageAuthenticator) SetRealm(realm string) {}
func (m *mockMessageAuthenticator) GetRealm() string { return "example.com" }

type mockUserManager struct{}

func (m *mockUserManager) CreateUser(username, realm, password string) error { return nil }
func (m *mockUserManager) AuthenticateUser(username, realm, password string) bool { return true }
func (m *mockUserManager) UpdatePassword(username, realm, newPassword string) error { return nil }
func (m *mockUserManager) DeleteUser(username, realm string) error { return nil }
func (m *mockUserManager) ListUsers() ([]*database.User, error) { return nil, nil }
func (m *mockUserManager) GeneratePasswordHash(username, realm, password string) string { return "hash" }
func (m *mockUserManager) GetUser(username, realm string) (*database.User, error) { return nil, nil }

func createTestRegisterRequest(aor, contactURI string, expires int) *parser.SIPMessage {
	request := parser.NewRequestMessage(parser.MethodREGISTER, aor)
	
	// Add required headers
	request.SetHeader(parser.HeaderVia, "SIP/2.0/UDP 192.168.1.100:5060;branch=z9hG4bK-test")
	request.SetHeader(parser.HeaderFrom, fmt.Sprintf("<%s>;tag=test-from-tag", aor))
	request.SetHeader(parser.HeaderTo, aor)
	request.SetHeader(parser.HeaderCallID, "test-call-id@192.168.1.100")
	request.SetHeader(parser.HeaderCSeq, "1 REGISTER")
	request.SetHeader(parser.HeaderMaxForwards, "70")
	request.SetHeader(parser.HeaderUserAgent, "Test-Client/1.0")
	request.SetHeader(parser.HeaderContentLength, "0")
	
	// Add Contact header if provided
	if contactURI != "" {
		if expires >= 0 {
			request.SetHeader(parser.HeaderContact, fmt.Sprintf("<%s>;expires=%d", contactURI, expires))
		} else {
			request.SetHeader(parser.HeaderContact, fmt.Sprintf("<%s>", contactURI))
		}
	}
	
	// Add Expires header if specified and no contact expires
	if expires >= 0 && !strings.Contains(contactURI, "expires=") {
		request.SetHeader(parser.HeaderExpires, fmt.Sprintf("%d", expires))
	}
	
	return request
}

func TestSIPRegistrar_ProcessRegisterRequest(t *testing.T) {
	storage := newMockRegistrationDB()
	authenticator := newMockMessageAuthenticator(true, false) // Authenticated
	userManager := &mockUserManager{}
	registrar := NewSIPRegistrar(storage, authenticator, userManager, "example.com")
	
	t.Run("Successful Registration", func(t *testing.T) {
		request := createTestRegisterRequest("sip:alice@example.com", "sip:alice@192.168.1.100:5060", 3600)
		
		response, err := registrar.ProcessRegisterRequest(request)
		if err != nil {
			t.Fatalf("Failed to process REGISTER request: %v", err)
		}
		
		if response.GetStatusCode() != parser.StatusOK {
			t.Errorf("Expected status code %d, got %d", parser.StatusOK, response.GetStatusCode())
		}
		
		// Check that contact was stored
		contacts, err := registrar.FindContacts("sip:alice@example.com")
		if err != nil {
			t.Fatalf("Failed to find contacts: %v", err)
		}
		
		if len(contacts) != 1 {
			t.Errorf("Expected 1 contact, got %d", len(contacts))
		}
		
		if contacts[0].URI != "sip:alice@192.168.1.100:5060" {
			t.Errorf("Expected contact URI sip:alice@192.168.1.100:5060, got %s", contacts[0].URI)
		}
	})
	
	t.Run("Registration Query", func(t *testing.T) {
		// First register a contact
		request1 := createTestRegisterRequest("sip:bob@example.com", "sip:bob@192.168.1.101:5060", 3600)
		_, err := registrar.ProcessRegisterRequest(request1)
		if err != nil {
			t.Fatalf("Failed to register contact: %v", err)
		}
		
		// Now query without Contact header
		request2 := createTestRegisterRequest("sip:bob@example.com", "", -1)
		response, err := registrar.ProcessRegisterRequest(request2)
		if err != nil {
			t.Fatalf("Failed to process query request: %v", err)
		}
		
		if response.GetStatusCode() != parser.StatusOK {
			t.Errorf("Expected status code %d, got %d", parser.StatusOK, response.GetStatusCode())
		}
		
		// Check that response contains Contact header
		contacts := response.GetHeaders(parser.HeaderContact)
		if len(contacts) != 1 {
			t.Errorf("Expected 1 contact in response, got %d", len(contacts))
		}
		
		if !strings.Contains(contacts[0], "sip:bob@192.168.1.101:5060") {
			t.Errorf("Expected contact URI in response, got %s", contacts[0])
		}
	})
	
	t.Run("Deregistration with expires=0", func(t *testing.T) {
		// First register a contact
		request1 := createTestRegisterRequest("sip:charlie@example.com", "sip:charlie@192.168.1.102:5060", 3600)
		_, err := registrar.ProcessRegisterRequest(request1)
		if err != nil {
			t.Fatalf("Failed to register contact: %v", err)
		}
		
		// Verify contact is registered
		contacts, err := registrar.FindContacts("sip:charlie@example.com")
		if err != nil || len(contacts) != 1 {
			t.Fatalf("Contact not registered properly")
		}
		
		// Now deregister with expires=0
		request2 := createTestRegisterRequest("sip:charlie@example.com", "sip:charlie@192.168.1.102:5060", 0)
		response, err := registrar.ProcessRegisterRequest(request2)
		if err != nil {
			t.Fatalf("Failed to process deregistration: %v", err)
		}
		
		if response.GetStatusCode() != parser.StatusOK {
			t.Errorf("Expected status code %d, got %d", parser.StatusOK, response.GetStatusCode())
		}
		
		// Verify contact is removed
		contacts, err = registrar.FindContacts("sip:charlie@example.com")
		if err != nil {
			t.Fatalf("Failed to find contacts: %v", err)
		}
		
		if len(contacts) != 0 {
			t.Errorf("Expected 0 contacts after deregistration, got %d", len(contacts))
		}
	})
	
	t.Run("Wildcard Deregistration", func(t *testing.T) {
		aor := "sip:dave@example.com"
		
		// Register multiple contacts
		request1 := createTestRegisterRequest(aor, "sip:dave@192.168.1.103:5060", 3600)
		_, err := registrar.ProcessRegisterRequest(request1)
		if err != nil {
			t.Fatalf("Failed to register first contact: %v", err)
		}
		
		request2 := createTestRegisterRequest(aor, "sip:dave@192.168.1.104:5060", 3600)
		_, err = registrar.ProcessRegisterRequest(request2)
		if err != nil {
			t.Fatalf("Failed to register second contact: %v", err)
		}
		
		// Verify both contacts are registered
		contacts, err := registrar.FindContacts(aor)
		if err != nil || len(contacts) != 2 {
			t.Fatalf("Contacts not registered properly, got %d contacts", len(contacts))
		}
		
		// Deregister all with wildcard
		request3 := createTestRegisterRequest(aor, "*", 0)
		response, err := registrar.ProcessRegisterRequest(request3)
		if err != nil {
			t.Fatalf("Failed to process wildcard deregistration: %v", err)
		}
		
		if response.GetStatusCode() != parser.StatusOK {
			t.Errorf("Expected status code %d, got %d", parser.StatusOK, response.GetStatusCode())
		}
		
		// Verify all contacts are removed
		contacts, err = registrar.FindContacts(aor)
		if err != nil {
			t.Fatalf("Failed to find contacts: %v", err)
		}
		
		if len(contacts) != 0 {
			t.Errorf("Expected 0 contacts after wildcard deregistration, got %d", len(contacts))
		}
	})
	
	t.Run("Authentication Required", func(t *testing.T) {
		// Create registrar with authentication required
		authRequired := newMockMessageAuthenticator(false, true) // Not authenticated, requires auth
		registrarAuth := NewSIPRegistrar(storage, authRequired, userManager, "example.com")
		
		request := createTestRegisterRequest("sip:eve@example.com", "sip:eve@192.168.1.105:5060", 3600)
		
		response, err := registrarAuth.ProcessRegisterRequest(request)
		if err != nil {
			t.Fatalf("Failed to process REGISTER request: %v", err)
		}
		
		if response.GetStatusCode() != parser.StatusUnauthorized {
			t.Errorf("Expected status code %d, got %d", parser.StatusUnauthorized, response.GetStatusCode())
		}
		
		// Check for WWW-Authenticate header
		authHeader := response.GetHeader(parser.HeaderWWWAuthenticate)
		if authHeader == "" {
			t.Error("Expected WWW-Authenticate header in 401 response")
		}
		
		if !strings.Contains(authHeader, "Digest") {
			t.Errorf("Expected Digest authentication, got %s", authHeader)
		}
	})
	
	t.Run("Authentication Forbidden", func(t *testing.T) {
		// Create registrar with authentication forbidden
		authForbidden := newMockMessageAuthenticator(false, false) // Not authenticated, no auth required
		registrarAuth := NewSIPRegistrar(storage, authForbidden, userManager, "example.com")
		
		request := createTestRegisterRequest("sip:frank@example.com", "sip:frank@192.168.1.106:5060", 3600)
		
		response, err := registrarAuth.ProcessRegisterRequest(request)
		if err != nil {
			t.Fatalf("Failed to process REGISTER request: %v", err)
		}
		
		if response.GetStatusCode() != parser.StatusForbidden {
			t.Errorf("Expected status code %d, got %d", parser.StatusForbidden, response.GetStatusCode())
		}
	})
	
	t.Run("Invalid Request Method", func(t *testing.T) {
		request := parser.NewRequestMessage(parser.MethodINVITE, "sip:test@example.com")
		
		_, err := registrar.ProcessRegisterRequest(request)
		if err == nil {
			t.Error("Expected error for non-REGISTER request")
		}
		
		if !strings.Contains(err.Error(), "not a REGISTER request") {
			t.Errorf("Expected 'not a REGISTER request' error, got %v", err)
		}
	})
	
	t.Run("Missing To Header", func(t *testing.T) {
		request := parser.NewRequestMessage(parser.MethodREGISTER, "sip:test@example.com")
		// Don't add To header
		
		response, err := registrar.ProcessRegisterRequest(request)
		if err != nil {
			t.Fatalf("Failed to process request: %v", err)
		}
		
		if response.GetStatusCode() != parser.StatusBadRequest {
			t.Errorf("Expected status code %d, got %d", parser.StatusBadRequest, response.GetStatusCode())
		}
	})
	
	t.Run("Missing Call-ID Header", func(t *testing.T) {
		request := createTestRegisterRequest("sip:test@example.com", "sip:test@192.168.1.100:5060", 3600)
		request.RemoveHeader(parser.HeaderCallID) // Remove Call-ID
		
		response, err := registrar.ProcessRegisterRequest(request)
		if err != nil {
			t.Fatalf("Failed to process request: %v", err)
		}
		
		if response.GetStatusCode() != parser.StatusBadRequest {
			t.Errorf("Expected status code %d, got %d", parser.StatusBadRequest, response.GetStatusCode())
		}
	})
	
	t.Run("Missing CSeq Header", func(t *testing.T) {
		request := createTestRegisterRequest("sip:test@example.com", "sip:test@192.168.1.100:5060", 3600)
		request.RemoveHeader(parser.HeaderCSeq) // Remove CSeq
		
		response, err := registrar.ProcessRegisterRequest(request)
		if err != nil {
			t.Fatalf("Failed to process request: %v", err)
		}
		
		if response.GetStatusCode() != parser.StatusBadRequest {
			t.Errorf("Expected status code %d, got %d", parser.StatusBadRequest, response.GetStatusCode())
		}
	})
}

func TestSIPRegistrar_Register(t *testing.T) {
	storage := newMockRegistrationDB()
	authenticator := newMockMessageAuthenticator(true, false)
	userManager := &mockUserManager{}
	registrar := NewSIPRegistrar(storage, authenticator, userManager, "example.com")
	
	t.Run("Register Contact", func(t *testing.T) {
		contact := &database.RegistrarContact{
			AOR:    "sip:alice@example.com",
			URI:    "sip:alice@192.168.1.100:5060",
			CallID: "test-call-id",
			CSeq:   1,
		}
		
		err := registrar.Register(contact, 3600)
		if err != nil {
			t.Fatalf("Failed to register contact: %v", err)
		}
		
		// Verify contact was stored
		contacts, err := registrar.FindContacts("sip:alice@example.com")
		if err != nil {
			t.Fatalf("Failed to find contacts: %v", err)
		}
		
		if len(contacts) != 1 {
			t.Errorf("Expected 1 contact, got %d", len(contacts))
		}
		
		if contacts[0].URI != contact.URI {
			t.Errorf("Expected URI %s, got %s", contact.URI, contacts[0].URI)
		}
	})
	
	t.Run("Deregister Contact with expires=0", func(t *testing.T) {
		contact := &database.RegistrarContact{
			AOR:    "sip:bob@example.com",
			URI:    "sip:bob@192.168.1.101:5060",
			CallID: "test-call-id",
			CSeq:   1,
		}
		
		// First register
		err := registrar.Register(contact, 3600)
		if err != nil {
			t.Fatalf("Failed to register contact: %v", err)
		}
		
		// Then deregister
		err = registrar.Register(contact, 0)
		if err != nil {
			t.Fatalf("Failed to deregister contact: %v", err)
		}
		
		// Verify contact was removed
		contacts, err := registrar.FindContacts("sip:bob@example.com")
		if err != nil {
			t.Fatalf("Failed to find contacts: %v", err)
		}
		
		if len(contacts) != 0 {
			t.Errorf("Expected 0 contacts after deregistration, got %d", len(contacts))
		}
	})
	
	t.Run("Invalid expires value", func(t *testing.T) {
		contact := &database.RegistrarContact{
			AOR:    "sip:charlie@example.com",
			URI:    "sip:charlie@192.168.1.102:5060",
			CallID: "test-call-id",
			CSeq:   1,
		}
		
		err := registrar.Register(contact, -1)
		if err == nil {
			t.Error("Expected error for negative expires value")
		}
	})
	
	t.Run("Expires limits", func(t *testing.T) {
		contact := &database.RegistrarContact{
			AOR:    "sip:dave@example.com",
			URI:    "sip:dave@192.168.1.103:5060",
			CallID: "test-call-id",
			CSeq:   1,
		}
		
		// Test max expires limit
		err := registrar.Register(contact, 10000) // Above max
		if err != nil {
			t.Fatalf("Failed to register contact: %v", err)
		}
		
		contacts, err := registrar.FindContacts("sip:dave@example.com")
		if err != nil {
			t.Fatalf("Failed to find contacts: %v", err)
		}
		
		// Should be limited to max expires (7200 seconds)
		expectedExpiry := time.Now().UTC().Add(time.Duration(registrar.GetMaxExpires()) * time.Second)
		actualExpiry := contacts[0].Expires
		
		// Allow some tolerance for test execution time
		if actualExpiry.After(expectedExpiry.Add(5*time.Second)) || actualExpiry.Before(expectedExpiry.Add(-5*time.Second)) {
			t.Errorf("Expected expiry around %v, got %v", expectedExpiry, actualExpiry)
		}
	})
}

func TestSIPRegistrar_Unregister(t *testing.T) {
	storage := newMockRegistrationDB()
	authenticator := newMockMessageAuthenticator(true, false)
	userManager := &mockUserManager{}
	registrar := NewSIPRegistrar(storage, authenticator, userManager, "example.com")
	
	aor := "sip:alice@example.com"
	
	// Register multiple contacts
	contact1 := &database.RegistrarContact{AOR: aor, URI: "sip:alice@192.168.1.100:5060", CallID: "call1", CSeq: 1}
	contact2 := &database.RegistrarContact{AOR: aor, URI: "sip:alice@192.168.1.101:5060", CallID: "call2", CSeq: 1}
	
	err := registrar.Register(contact1, 3600)
	if err != nil {
		t.Fatalf("Failed to register contact1: %v", err)
	}
	
	err = registrar.Register(contact2, 3600)
	if err != nil {
		t.Fatalf("Failed to register contact2: %v", err)
	}
	
	// Verify both contacts are registered
	contacts, err := registrar.FindContacts(aor)
	if err != nil || len(contacts) != 2 {
		t.Fatalf("Expected 2 contacts, got %d", len(contacts))
	}
	
	// Unregister all contacts for AOR
	err = registrar.Unregister(aor)
	if err != nil {
		t.Fatalf("Failed to unregister contacts: %v", err)
	}
	
	// Verify all contacts are removed
	contacts, err = registrar.FindContacts(aor)
	if err != nil {
		t.Fatalf("Failed to find contacts: %v", err)
	}
	
	if len(contacts) != 0 {
		t.Errorf("Expected 0 contacts after unregistration, got %d", len(contacts))
	}
}

func TestSIPRegistrar_CleanupExpired(t *testing.T) {
	storage := newMockRegistrationDB()
	authenticator := newMockMessageAuthenticator(true, false)
	userManager := &mockUserManager{}
	registrar := NewSIPRegistrar(storage, authenticator, userManager, "example.com")
	
	// Register contact with very short expiry
	contact := &database.RegistrarContact{
		AOR:     "sip:alice@example.com",
		URI:     "sip:alice@192.168.1.100:5060",
		CallID:  "test-call-id",
		CSeq:    1,
		Expires: time.Now().UTC().Add(-1 * time.Hour), // Already expired
	}
	
	// Store directly to bypass expiry validation in Register method
	err := storage.Store(contact)
	if err != nil {
		t.Fatalf("Failed to store expired contact: %v", err)
	}
	
	// Cleanup expired contacts
	registrar.CleanupExpired()
	
	// Verify expired contact is removed (should return empty since it's expired)
	contacts, err := registrar.FindContacts("sip:alice@example.com")
	if err != nil {
		t.Fatalf("Failed to find contacts: %v", err)
	}
	
	if len(contacts) != 0 {
		t.Errorf("Expected 0 contacts after cleanup, got %d", len(contacts))
	}
}

func TestSIPRegistrar_HeaderParsing(t *testing.T) {
	storage := newMockRegistrationDB()
	authenticator := newMockMessageAuthenticator(true, false)
	userManager := &mockUserManager{}
	registrar := NewSIPRegistrar(storage, authenticator, userManager, "example.com")
	
	t.Run("Parse Contact Header with Angle Brackets", func(t *testing.T) {
		uri, expires, err := registrar.parseContactHeader("<sip:alice@192.168.1.100:5060>;expires=7200", 3600)
		if err != nil {
			t.Fatalf("Failed to parse contact header: %v", err)
		}
		
		if uri != "sip:alice@192.168.1.100:5060" {
			t.Errorf("Expected URI sip:alice@192.168.1.100:5060, got %s", uri)
		}
		
		if expires != 7200 {
			t.Errorf("Expected expires 7200, got %d", expires)
		}
	})
	
	t.Run("Parse Contact Header without Angle Brackets", func(t *testing.T) {
		uri, expires, err := registrar.parseContactHeader("sip:bob@192.168.1.101:5060;expires=1800", 3600)
		if err != nil {
			t.Fatalf("Failed to parse contact header: %v", err)
		}
		
		if uri != "sip:bob@192.168.1.101:5060" {
			t.Errorf("Expected URI sip:bob@192.168.1.101:5060, got %s", uri)
		}
		
		if expires != 1800 {
			t.Errorf("Expected expires 1800, got %d", expires)
		}
	})
	
	t.Run("Parse Contact Header with Default Expires", func(t *testing.T) {
		uri, expires, err := registrar.parseContactHeader("sip:charlie@192.168.1.102:5060", 3600)
		if err != nil {
			t.Fatalf("Failed to parse contact header: %v", err)
		}
		
		if uri != "sip:charlie@192.168.1.102:5060" {
			t.Errorf("Expected URI sip:charlie@192.168.1.102:5060, got %s", uri)
		}
		
		if expires != 3600 {
			t.Errorf("Expected expires 3600, got %d", expires)
		}
	})
	
	t.Run("Parse Wildcard Contact", func(t *testing.T) {
		uri, expires, err := registrar.parseContactHeader("*", 3600)
		if err != nil {
			t.Fatalf("Failed to parse wildcard contact: %v", err)
		}
		
		if uri != "*" {
			t.Errorf("Expected URI *, got %s", uri)
		}
		
		if expires != 0 {
			t.Errorf("Expected expires 0 for wildcard, got %d", expires)
		}
	})
}