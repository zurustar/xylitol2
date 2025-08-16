package webadmin

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

// Integration tests for the web admin interface
func TestWebAdminAPIIntegration(t *testing.T) {
	userManager := NewMockUserManager()
	server := NewServer(userManager)
	
	// Test complete user management workflow via API endpoints only
	t.Run("Complete User Management API Workflow", func(t *testing.T) {
		// 1. List users (should be empty initially)
		users, err := userManager.ListUsers()
		if err != nil {
			t.Fatalf("Failed to get users: %v", err)
		}
		
		if len(users) != 0 {
			t.Errorf("Expected 0 users initially, got %d", len(users))
		}
		
		// 2. Create a new user via handler
		form := url.Values{}
		form.Add("username", "testuser")
		form.Add("realm", "example.com")
		form.Add("password", "TestPass123")
		
		req := httptest.NewRequest("POST", "/admin/users", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		w := httptest.NewRecorder()
		
		server.handleCreateUser(w, req)
		
		if w.Code != http.StatusSeeOther && w.Code != http.StatusOK {
			t.Errorf("Expected status 200 or 303, got %d: %s", w.Code, w.Body.String())
		}
		
		// 3. Verify user was created
		users, err = userManager.ListUsers()
		if err != nil {
			t.Fatalf("Failed to get users: %v", err)
		}
		
		if len(users) != 1 {
			t.Errorf("Expected 1 user after creation, got %d", len(users))
		}
		
		if users[0].Username != "testuser" || users[0].Realm != "example.com" {
			t.Errorf("User not created correctly: %+v", users[0])
		}
		
		// 4. Update user password
		form = url.Values{}
		form.Add("username", "testuser")
		form.Add("realm", "example.com")
		form.Add("password", "NewPass456")
		
		req = httptest.NewRequest("PUT", "/admin/users/1", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		w = httptest.NewRecorder()
		
		server.handleUpdateUser(w, req, 1)
		
		if w.Code != http.StatusSeeOther && w.Code != http.StatusOK {
			t.Errorf("Expected status 200 or 303, got %d: %s", w.Code, w.Body.String())
		}
		
		// 5. Verify password was updated
		if !userManager.AuthenticateUser("testuser", "example.com", "NewPass456") {
			t.Error("Password was not updated correctly")
		}
		
		// 6. Delete user
		req = httptest.NewRequest("DELETE", "/admin/users/1?username=testuser&realm=example.com", nil)
		w = httptest.NewRecorder()
		
		server.handleDeleteUser(w, req, 1)
		
		if w.Code != http.StatusSeeOther && w.Code != http.StatusOK {
			t.Errorf("Expected status 200 or 303, got %d: %s", w.Code, w.Body.String())
		}
		
		// 7. Verify user was deleted
		users, err = userManager.ListUsers()
		if err != nil {
			t.Fatalf("Failed to get users: %v", err)
		}
		
		if len(users) != 0 {
			t.Errorf("Expected 0 users after deletion, got %d", len(users))
		}
	})
}

func TestWebAdminErrorHandling(t *testing.T) {
	userManager := NewMockUserManager()
	server := NewServer(userManager)
	
	t.Run("Create User Validation Errors", func(t *testing.T) {
		testCases := []struct {
			name     string
			username string
			realm    string
			password string
			expectStatus int
		}{
			{"Empty username", "", "example.com", "password", http.StatusBadRequest},
			{"Empty realm", "user", "", "password", http.StatusBadRequest},
			{"Empty password", "user", "example.com", "", http.StatusBadRequest},
			{"Whitespace only username", "   ", "example.com", "password", http.StatusBadRequest},
			{"Whitespace only realm", "user", "   ", "password", http.StatusBadRequest},
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
	})
	
	t.Run("Duplicate User Creation", func(t *testing.T) {
		// Create first user
		form := url.Values{}
		form.Add("username", "duplicate")
		form.Add("realm", "example.com")
		form.Add("password", "password123")
		
		req := httptest.NewRequest("POST", "/admin/users", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		w := httptest.NewRecorder()
		
		server.handleCreateUser(w, req)
		
		if w.Code != http.StatusSeeOther && w.Code != http.StatusOK {
			t.Errorf("Expected status 200 or 303 for first user, got %d", w.Code)
		}
		
		// Try to create duplicate user
		req = httptest.NewRequest("POST", "/admin/users", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		w = httptest.NewRecorder()
		
		server.handleCreateUser(w, req)
		
		if w.Code != http.StatusInternalServerError {
			t.Errorf("Expected status 500 for duplicate user, got %d", w.Code)
		}
	})
	
	t.Run("Update Non-existent User", func(t *testing.T) {
		form := url.Values{}
		form.Add("username", "nonexistent")
		form.Add("realm", "example.com")
		form.Add("password", "newpassword")
		
		req := httptest.NewRequest("PUT", "/admin/users/999", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		w := httptest.NewRecorder()
		
		server.handleUpdateUser(w, req, 999)
		
		if w.Code != http.StatusInternalServerError {
			t.Errorf("Expected status 500 for non-existent user, got %d", w.Code)
		}
	})
	
	t.Run("Delete Non-existent User", func(t *testing.T) {
		req := httptest.NewRequest("DELETE", "/admin/users/999?username=nonexistent&realm=example.com", nil)
		w := httptest.NewRecorder()
		
		server.handleDeleteUser(w, req, 999)
		
		if w.Code != http.StatusInternalServerError {
			t.Errorf("Expected status 500 for non-existent user, got %d", w.Code)
		}
	})
}

func TestWebAdminConcurrency(t *testing.T) {
	userManager := NewMockUserManager()
	server := NewServer(userManager)
	
	t.Run("Concurrent User Creation", func(t *testing.T) {
		const numUsers = 10
		done := make(chan bool, numUsers)
		errors := make(chan error, numUsers)
		
		// Create users concurrently using direct handler calls
		for i := 0; i < numUsers; i++ {
			go func(id int) {
				form := url.Values{}
				form.Add("username", fmt.Sprintf("user%d", id))
				form.Add("realm", "example.com")
				form.Add("password", "password123")
				
				req := httptest.NewRequest("POST", "/admin/users", strings.NewReader(form.Encode()))
				req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
				w := httptest.NewRecorder()
				
				server.handleCreateUser(w, req)
				
				if w.Code != http.StatusSeeOther && w.Code != http.StatusOK {
					errors <- fmt.Errorf("unexpected status code: %d", w.Code)
					return
				}
				
				done <- true
			}(i)
		}
		
		// Wait for all goroutines to complete
		successCount := 0
		errorCount := 0
		
		for i := 0; i < numUsers; i++ {
			select {
			case <-done:
				successCount++
			case err := <-errors:
				t.Logf("Error creating user: %v", err)
				errorCount++
			case <-time.After(5 * time.Second):
				t.Fatal("Timeout waiting for concurrent operations")
			}
		}
		
		if successCount != numUsers {
			t.Errorf("Expected %d successful creations, got %d (errors: %d)", numUsers, successCount, errorCount)
		}
		
		// Verify all users were created
		users, err := userManager.ListUsers()
		if err != nil {
			t.Fatalf("Failed to get users: %v", err)
		}
		
		if len(users) != numUsers {
			t.Errorf("Expected %d users, got %d", numUsers, len(users))
		}
	})
}

func TestWebAdminFormValidation(t *testing.T) {
	userManager := NewMockUserManager()
	server := NewServer(userManager)
	
	t.Run("Invalid Form Data", func(t *testing.T) {
		// Test with invalid content type
		req := httptest.NewRequest("POST", "/admin/users", strings.NewReader("invalid data"))
		req.Header.Set("Content-Type", "text/plain")
		w := httptest.NewRecorder()
		
		server.handleCreateUser(w, req)
		
		if w.Code != http.StatusBadRequest {
			t.Errorf("Expected status 400 for invalid form data, got %d", w.Code)
		}
	})
	
	t.Run("Malformed Form Data", func(t *testing.T) {
		// Test with malformed form data
		req := httptest.NewRequest("POST", "/admin/users", strings.NewReader("%invalid%form%data%"))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		w := httptest.NewRecorder()
		
		server.handleCreateUser(w, req)
		
		// Should still parse but with empty values, leading to validation error
		if w.Code != http.StatusBadRequest {
			t.Errorf("Expected status 400 for malformed form data, got %d", w.Code)
		}
	})
}