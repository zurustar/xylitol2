package webadmin

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/zurustar/xylitol2/internal/database"
)

// MockUserManager implements database.UserManager for testing
type MockUserManager struct {
	users map[string]*database.User
	idCounter int64
}

func NewMockUserManager() *MockUserManager {
	return &MockUserManager{
		users: make(map[string]*database.User),
		idCounter: 1,
	}
}

func (m *MockUserManager) CreateUser(username, realm, password string) error {
	key := fmt.Sprintf("%s@%s", username, realm)
	if _, exists := m.users[key]; exists {
		return fmt.Errorf("user already exists")
	}
	
	m.users[key] = &database.User{
		ID:           m.idCounter,
		Username:     username,
		Realm:        realm,
		PasswordHash: m.GeneratePasswordHash(username, realm, password),
		Enabled:      true,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}
	m.idCounter++
	return nil
}

func (m *MockUserManager) AuthenticateUser(username, realm, password string) bool {
	key := fmt.Sprintf("%s@%s", username, realm)
	user, exists := m.users[key]
	if !exists {
		return false
	}
	expectedHash := m.GeneratePasswordHash(username, realm, password)
	return user.PasswordHash == expectedHash
}

func (m *MockUserManager) UpdatePassword(username, realm, newPassword string) error {
	key := fmt.Sprintf("%s@%s", username, realm)
	user, exists := m.users[key]
	if !exists {
		return fmt.Errorf("user not found")
	}
	user.PasswordHash = m.GeneratePasswordHash(username, realm, newPassword)
	user.UpdatedAt = time.Now()
	return nil
}

func (m *MockUserManager) DeleteUser(username, realm string) error {
	key := fmt.Sprintf("%s@%s", username, realm)
	if _, exists := m.users[key]; !exists {
		return fmt.Errorf("user not found")
	}
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
	return fmt.Sprintf("hash_%s_%s_%s", username, realm, password)
}

func (m *MockUserManager) GetUser(username, realm string) (*database.User, error) {
	key := fmt.Sprintf("%s@%s", username, realm)
	user, exists := m.users[key]
	if !exists {
		return nil, fmt.Errorf("user not found")
	}
	return user, nil
}

func TestNewServer(t *testing.T) {
	userManager := NewMockUserManager()
	server := NewServer(userManager)
	
	if server == nil {
		t.Fatal("NewServer returned nil")
	}
	
	if server.userManager != userManager {
		t.Error("Server userManager not set correctly")
	}
}

func TestServer_RegisterRoutes(t *testing.T) {
	userManager := NewMockUserManager()
	server := NewServer(userManager)
	
	mux := http.NewServeMux()
	server.RegisterRoutes(mux)
	
	// Test that routes are registered by checking for 404 vs other errors
	testCases := []struct {
		method string
		path   string
	}{
		{"GET", "/admin/users"}, // This should work without templates
		{"POST", "/admin/users"}, // This should give 400 for missing form data
	}
	
	for _, tc := range testCases {
		req := httptest.NewRequest(tc.method, tc.path, nil)
		w := httptest.NewRecorder()
		
		mux.ServeHTTP(w, req)
		
		// Routes should be registered (not 404)
		if w.Code == http.StatusNotFound {
			t.Errorf("Route %s %s not registered", tc.method, tc.path)
		}
	}
}

func TestServer_handleListUsers(t *testing.T) {
	userManager := NewMockUserManager()
	server := NewServer(userManager)
	
	// Add test users
	userManager.CreateUser("alice", "example.com", "password123")
	userManager.CreateUser("bob", "example.com", "password456")
	
	req := httptest.NewRequest("GET", "/admin/users", nil)
	w := httptest.NewRecorder()
	
	server.handleListUsers(w, req)
	
	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
	
	var users []*database.User
	if err := json.Unmarshal(w.Body.Bytes(), &users); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}
	
	if len(users) != 2 {
		t.Errorf("Expected 2 users, got %d", len(users))
	}
}

func TestServer_handleCreateUser(t *testing.T) {
	userManager := NewMockUserManager()
	server := NewServer(userManager)
	
	// Test successful user creation
	form := url.Values{}
	form.Add("username", "testuser")
	form.Add("realm", "example.com")
	form.Add("password", "password123")
	
	req := httptest.NewRequest("POST", "/admin/users", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	
	w := httptest.NewRecorder()
	server.handleCreateUser(w, req)
	
	if w.Code != http.StatusSeeOther && w.Code != http.StatusOK {
		t.Errorf("Expected status 200 or 303, got %d: %s", w.Code, w.Body.String())
	}
	
	// Verify user was created
	user, err := userManager.GetUser("testuser", "example.com")
	if err != nil {
		t.Errorf("User was not created: %v", err)
		return
	}
	if user.Username != "testuser" {
		t.Errorf("Expected username 'testuser', got '%s'", user.Username)
	}
}

func TestServer_handleCreateUser_ValidationErrors(t *testing.T) {
	userManager := NewMockUserManager()
	server := NewServer(userManager)
	
	testCases := []struct {
		name     string
		username string
		realm    string
		password string
		expectStatus int
	}{
		{"missing username", "", "example.com", "password", http.StatusBadRequest},
		{"missing realm", "user", "", "password", http.StatusBadRequest},
		{"missing password", "user", "example.com", "", http.StatusBadRequest},
		{"all missing", "", "", "", http.StatusBadRequest},
	}
	
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			form := url.Values{}
			form.Add("username", tc.username)
			form.Add("realm", tc.realm)
			form.Add("password", tc.password)
			
			req := httptest.NewRequest("POST", "/admin/users", strings.NewReader(form.Encode()))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			
			w := httptest.NewRecorder()
			server.handleCreateUser(w, req)
			
			if w.Code != tc.expectStatus {
				t.Errorf("Expected status %d, got %d", tc.expectStatus, w.Code)
			}
		})
	}
}

func TestServer_handleUpdateUser(t *testing.T) {
	userManager := NewMockUserManager()
	server := NewServer(userManager)
	
	// Create a test user first
	userManager.CreateUser("testuser", "example.com", "oldpassword")
	
	// Test password update
	form := url.Values{}
	form.Add("username", "testuser")
	form.Add("realm", "example.com")
	form.Add("password", "newpassword")
	
	req := httptest.NewRequest("PUT", "/admin/users/1", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	
	w := httptest.NewRecorder()
	server.handleUpdateUser(w, req, 1)
	
	if w.Code != http.StatusSeeOther && w.Code != http.StatusOK {
		t.Errorf("Expected status 200 or 303, got %d: %s", w.Code, w.Body.String())
	}
	
	// Verify password was updated
	if !userManager.AuthenticateUser("testuser", "example.com", "newpassword") {
		t.Error("Password was not updated")
	}
	if userManager.AuthenticateUser("testuser", "example.com", "oldpassword") {
		t.Error("Old password still works")
	}
}

func TestServer_handleDeleteUser(t *testing.T) {
	userManager := NewMockUserManager()
	server := NewServer(userManager)
	
	// Create a test user first
	userManager.CreateUser("testuser", "example.com", "password")
	
	// Test user deletion using query parameters (since DELETE doesn't parse body by default)
	req := httptest.NewRequest("DELETE", "/admin/users/1?username=testuser&realm=example.com", nil)
	
	w := httptest.NewRecorder()
	server.handleDeleteUser(w, req, 1)
	
	if w.Code != http.StatusSeeOther && w.Code != http.StatusOK {
		t.Errorf("Expected status 200 or 303, got %d: %s", w.Code, w.Body.String())
	}
	
	// Verify user was deleted
	_, err := userManager.GetUser("testuser", "example.com")
	if err == nil {
		t.Error("User was not deleted")
	}
}

func TestServer_handleUserByID(t *testing.T) {
	userManager := NewMockUserManager()
	server := NewServer(userManager)
	
	// Test invalid user ID
	req := httptest.NewRequest("PUT", "/admin/users/invalid", nil)
	w := httptest.NewRecorder()
	
	server.handleUserByID(w, req)
	
	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400 for invalid ID, got %d", w.Code)
	}
	
	// Test unsupported method
	req = httptest.NewRequest("GET", "/admin/users/1", nil)
	w = httptest.NewRecorder()
	
	server.handleUserByID(w, req)
	
	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405 for unsupported method, got %d", w.Code)
	}
}

func TestServer_handleUsers_MethodNotAllowed(t *testing.T) {
	userManager := NewMockUserManager()
	server := NewServer(userManager)
	
	// Test unsupported method
	req := httptest.NewRequest("DELETE", "/admin/users", nil)
	w := httptest.NewRecorder()
	
	server.handleUsers(w, req)
	
	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d", w.Code)
	}
}