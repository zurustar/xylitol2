package webadmin

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

// Integration tests for the web admin interface
func TestWebAdminIntegration(t *testing.T) {
	userManager := NewMockUserManager()
	huntGroupManager := NewSimpleHuntGroupManager()
	huntGroupEngine := &SimpleHuntGroupEngine{}
	logger := &SimpleLogger{}
	server := NewServer(userManager, huntGroupManager, huntGroupEngine, logger)
	
	t.Run("Dashboard Access", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/admin", nil)
		w := httptest.NewRecorder()
		
		server.userHandler.HandleDashboard(w, req)
		
		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", w.Code)
		}
		
		body := w.Body.String()
		if !strings.Contains(body, "SIP Server Administration") {
			t.Error("Expected response to contain dashboard title")
		}
	})
	
	t.Run("User Management Page", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/admin/users", nil)
		w := httptest.NewRecorder()
		
		server.userHandler.HandleUsers(w, req)
		
		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", w.Code)
		}
		
		body := w.Body.String()
		if !strings.Contains(body, "SIP Users") {
			t.Error("Expected response to contain users page title")
		}
	})
	
	t.Run("Hunt Group Management Page", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/admin/huntgroups", nil)
		w := httptest.NewRecorder()
		
		server.huntGroupHandler.HandleHuntGroups(w, req)
		
		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", w.Code)
		}
		
		body := w.Body.String()
		if !strings.Contains(body, "Hunt Groups") {
			t.Error("Expected response to contain hunt groups page title")
		}
	})
	
	t.Run("New User Page", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/admin/users/new", nil)
		w := httptest.NewRecorder()
		
		server.userHandler.HandleNewUserPage(w, req)
		
		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", w.Code)
		}
		
		body := w.Body.String()
		if !strings.Contains(body, "Create New User") {
			t.Error("Expected response to contain new user page title")
		}
	})
	
	t.Run("New Hunt Group Page", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/admin/huntgroups/new", nil)
		w := httptest.NewRecorder()
		
		server.huntGroupHandler.HandleNewHuntGroupPage(w, req)
		
		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", w.Code)
		}
		
		body := w.Body.String()
		if !strings.Contains(body, "Create New Hunt Group") {
			t.Error("Expected response to contain new hunt group page title")
		}
	})
	
	t.Run("Create User Form Submission", func(t *testing.T) {
		form := url.Values{}
		form.Add("username", "testuser")
		form.Add("realm", "example.com")
		form.Add("password", "TestPass123")
		
		req := httptest.NewRequest("POST", "/admin/users", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		w := httptest.NewRecorder()
		
		server.userHandler.HandleUsers(w, req)
		
		if w.Code != http.StatusSeeOther {
			t.Errorf("Expected status 303 (redirect), got %d", w.Code)
		}
	})
	
	t.Run("Create Hunt Group Form Submission", func(t *testing.T) {
		form := url.Values{}
		form.Add("name", "Test Group")
		form.Add("extension", "100")
		form.Add("strategy", "simultaneous")
		form.Add("ring_timeout", "30")
		form.Add("enabled", "on")
		form.Add("description", "Test hunt group")
		
		req := httptest.NewRequest("POST", "/admin/huntgroups", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		w := httptest.NewRecorder()
		
		server.huntGroupHandler.HandleHuntGroups(w, req)
		
		if w.Code != http.StatusSeeOther {
			t.Errorf("Expected status 303 (redirect), got %d", w.Code)
		}
	})
	
	t.Run("Invalid Method Handling", func(t *testing.T) {
		req := httptest.NewRequest("PATCH", "/admin/users", nil)
		w := httptest.NewRecorder()
		
		server.userHandler.HandleUsers(w, req)
		
		if w.Code != http.StatusMethodNotAllowed {
			t.Errorf("Expected status 405, got %d", w.Code)
		}
	})
}