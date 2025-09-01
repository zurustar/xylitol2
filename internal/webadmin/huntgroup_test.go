package webadmin

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/zurustar/xylitol2/internal/database"
	"github.com/zurustar/xylitol2/internal/huntgroup"
	"github.com/zurustar/xylitol2/internal/logging"
	"github.com/zurustar/xylitol2/internal/parser"
)

// Simple mock implementations for testing
type SimpleHuntGroupManager struct {
	groups map[int]*huntgroup.HuntGroup
	nextID int
}

func NewSimpleHuntGroupManager() *SimpleHuntGroupManager {
	return &SimpleHuntGroupManager{
		groups: make(map[int]*huntgroup.HuntGroup),
		nextID: 1,
	}
}

func (m *SimpleHuntGroupManager) CreateGroup(group *huntgroup.HuntGroup) error {
	group.ID = m.nextID
	m.nextID++
	group.CreatedAt = time.Now().UTC()
	group.UpdatedAt = time.Now().UTC()
	m.groups[group.ID] = group
	return nil
}

func (m *SimpleHuntGroupManager) GetGroup(id int) (*huntgroup.HuntGroup, error) {
	group, exists := m.groups[id]
	if !exists {
		return nil, database.ErrNotFound
	}
	return group, nil
}

func (m *SimpleHuntGroupManager) GetGroupByExtension(extension string) (*huntgroup.HuntGroup, error) {
	for _, group := range m.groups {
		if group.Extension == extension {
			return group, nil
		}
	}
	return nil, database.ErrNotFound
}

func (m *SimpleHuntGroupManager) UpdateGroup(group *huntgroup.HuntGroup) error {
	_, exists := m.groups[group.ID]
	if !exists {
		return database.ErrNotFound
	}
	group.UpdatedAt = time.Now().UTC()
	m.groups[group.ID] = group
	return nil
}

func (m *SimpleHuntGroupManager) DeleteGroup(id int) error {
	_, exists := m.groups[id]
	if !exists {
		return database.ErrNotFound
	}
	delete(m.groups, id)
	return nil
}

func (m *SimpleHuntGroupManager) ListGroups() ([]*huntgroup.HuntGroup, error) {
	var groups []*huntgroup.HuntGroup
	for _, group := range m.groups {
		groups = append(groups, group)
	}
	return groups, nil
}

func (m *SimpleHuntGroupManager) EnableGroup(groupID int) error { return nil }
func (m *SimpleHuntGroupManager) DisableGroup(groupID int) error { return nil }
func (m *SimpleHuntGroupManager) AddMember(groupID int, member *huntgroup.HuntGroupMember) error { return nil }
func (m *SimpleHuntGroupManager) RemoveMember(groupID int, memberID int) error { return nil }
func (m *SimpleHuntGroupManager) UpdateMember(member *huntgroup.HuntGroupMember) error { return nil }
func (m *SimpleHuntGroupManager) GetGroupMembers(groupID int) ([]*huntgroup.HuntGroupMember, error) { return nil, nil }
func (m *SimpleHuntGroupManager) EnableMember(groupID int, memberID int) error { return nil }
func (m *SimpleHuntGroupManager) DisableMember(groupID int, memberID int) error { return nil }
func (m *SimpleHuntGroupManager) CreateSession(session *huntgroup.CallSession) error { return nil }
func (m *SimpleHuntGroupManager) GetSession(sessionID string) (*huntgroup.CallSession, error) { return nil, nil }
func (m *SimpleHuntGroupManager) UpdateSession(session *huntgroup.CallSession) error { return nil }
func (m *SimpleHuntGroupManager) EndSession(sessionID string) error { return nil }
func (m *SimpleHuntGroupManager) GetActiveSessions() ([]*huntgroup.CallSession, error) { return nil, nil }
func (m *SimpleHuntGroupManager) GetCallStatistics(groupID int) (*huntgroup.CallStatistics, error) {
	return &huntgroup.CallStatistics{
		GroupID:       groupID,
		TotalCalls:    5,
		AnsweredCalls: 4,
		MissedCalls:   1,
	}, nil
}

type SimpleHuntGroupEngine struct{}

func (e *SimpleHuntGroupEngine) ProcessIncomingCall(invite *parser.SIPMessage, group *huntgroup.HuntGroup) (*huntgroup.CallSession, error) { return nil, nil }
func (e *SimpleHuntGroupEngine) HandleMemberResponse(sessionID string, memberExtension string, response *parser.SIPMessage) error { return nil }
func (e *SimpleHuntGroupEngine) CancelSession(sessionID string) error { return nil }
func (e *SimpleHuntGroupEngine) GetCallStatistics(groupID int) (*huntgroup.CallStatistics, error) {
	return &huntgroup.CallStatistics{
		GroupID:       groupID,
		TotalCalls:    5,
		AnsweredCalls: 4,
		MissedCalls:   1,
	}, nil
}

type SimpleUserManager struct{}

func (m *SimpleUserManager) CreateUser(username, realm, password string) error { return nil }
func (m *SimpleUserManager) AuthenticateUser(username, realm, password string) bool { return true }
func (m *SimpleUserManager) UpdatePassword(username, realm, newPassword string) error { return nil }
func (m *SimpleUserManager) DeleteUser(username, realm string) error { return nil }
func (m *SimpleUserManager) ListUsers() ([]*database.User, error) { return nil, nil }
func (m *SimpleUserManager) GeneratePasswordHash(username, realm, password string) string { return "" }
func (m *SimpleUserManager) GetUser(username, realm string) (*database.User, error) { return nil, nil }

type SimpleLogger struct{}

func (l *SimpleLogger) Debug(msg string, fields ...logging.Field) {}
func (l *SimpleLogger) Info(msg string, fields ...logging.Field) {}
func (l *SimpleLogger) Warn(msg string, fields ...logging.Field) {}
func (l *SimpleLogger) Error(msg string, fields ...logging.Field) {}

func setupSimpleTestServer() (*Server, *SimpleHuntGroupManager) {
	mockUserManager := &SimpleUserManager{}
	mockHuntGroupManager := NewSimpleHuntGroupManager()
	mockHuntGroupEngine := &SimpleHuntGroupEngine{}
	mockLogger := &SimpleLogger{}
	
	server := NewServer(mockUserManager, mockHuntGroupManager, mockHuntGroupEngine, mockLogger)
	return server, mockHuntGroupManager
}

func TestHuntGroupHandler_ListHuntGroups(t *testing.T) {
	server, manager := setupSimpleTestServer()
	
	// Create test hunt groups
	group1 := &huntgroup.HuntGroup{
		Name:        "Sales Team",
		Extension:   "100",
		Strategy:    huntgroup.StrategySimultaneous,
		RingTimeout: 30,
		Enabled:     true,
		Description: "Sales team hunt group",
	}
	
	manager.CreateGroup(group1)
	
	req := httptest.NewRequest("GET", "/admin/huntgroups", nil)
	w := httptest.NewRecorder()
	
	server.huntGroupHandler.HandleHuntGroups(w, req)
	
	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
	
	body := w.Body.String()
	if !strings.Contains(body, "Sales Team") {
		t.Error("Expected response to contain 'Sales Team'")
	}
}

func TestHuntGroupHandler_CreateHuntGroup(t *testing.T) {
	server, manager := setupSimpleTestServer()
	
	formData := url.Values{
		"name":         {"Test Group"},
		"extension":    {"300"},
		"strategy":     {"simultaneous"},
		"ring_timeout": {"30"},
		"enabled":      {"on"},
		"description":  {"Test hunt group"},
	}
	
	req := httptest.NewRequest("POST", "/admin/huntgroups", strings.NewReader(formData.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	
	server.huntGroupHandler.HandleHuntGroups(w, req)
	
	if w.Code != http.StatusSeeOther {
		t.Errorf("Expected status 303, got %d", w.Code)
	}
	
	// Verify the group was created
	groups, err := manager.ListGroups()
	if err != nil {
		t.Fatalf("Failed to list groups: %v", err)
	}
	
	if len(groups) != 1 {
		t.Errorf("Expected 1 group, got %d", len(groups))
	}
	
	group := groups[0]
	if group.Name != "Test Group" {
		t.Errorf("Expected name 'Test Group', got '%s'", group.Name)
	}
}

func TestHuntGroupHandler_GetHuntGroup(t *testing.T) {
	server, manager := setupSimpleTestServer()
	
	// Create test hunt group
	group := &huntgroup.HuntGroup{
		Name:        "Test Group",
		Extension:   "100",
		Strategy:    huntgroup.StrategySimultaneous,
		RingTimeout: 30,
		Enabled:     true,
		Description: "Test description",
	}
	
	manager.CreateGroup(group)
	
	req := httptest.NewRequest("GET", "/admin/huntgroups/1", nil)
	w := httptest.NewRecorder()
	
	server.huntGroupHandler.HandleHuntGroupByID(w, req)
	
	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
	
	var responseGroup huntgroup.HuntGroup
	err := json.NewDecoder(w.Body).Decode(&responseGroup)
	if err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}
	
	if responseGroup.Name != "Test Group" {
		t.Errorf("Expected name 'Test Group', got '%s'", responseGroup.Name)
	}
}

func TestHuntGroupHandler_DeleteHuntGroup(t *testing.T) {
	server, manager := setupSimpleTestServer()
	
	// Create test hunt group
	group := &huntgroup.HuntGroup{
		Name:        "Test Group",
		Extension:   "100",
		Strategy:    huntgroup.StrategySimultaneous,
		RingTimeout: 30,
		Enabled:     true,
	}
	
	manager.CreateGroup(group)
	
	req := httptest.NewRequest("DELETE", "/admin/huntgroups/1", nil)
	w := httptest.NewRecorder()
	
	server.huntGroupHandler.HandleHuntGroupByID(w, req)
	
	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
	
	// Verify the group was deleted
	_, err := manager.GetGroup(1)
	if err == nil {
		t.Error("Expected error when getting deleted group, got nil")
	}
}

func TestHuntGroupHandler_NewHuntGroupPage(t *testing.T) {
	server, _ := setupSimpleTestServer()
	
	req := httptest.NewRequest("GET", "/admin/huntgroups/new", nil)
	w := httptest.NewRecorder()
	
	server.huntGroupHandler.HandleNewHuntGroupPage(w, req)
	
	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
	
	body := w.Body.String()
	if !strings.Contains(body, "Create New Hunt Group") {
		t.Error("Expected response to contain 'Create New Hunt Group'")
	}
	
	if !strings.Contains(body, `name="name"`) {
		t.Error("Expected response to contain name field")
	}
}

func TestHuntGroupHandler_GetStatistics(t *testing.T) {
	server, manager := setupSimpleTestServer()
	
	// Create test hunt group
	group := &huntgroup.HuntGroup{
		Name:        "Test Group",
		Extension:   "100",
		Strategy:    huntgroup.StrategySimultaneous,
		RingTimeout: 30,
		Enabled:     true,
	}
	
	manager.CreateGroup(group)
	
	req := httptest.NewRequest("GET", "/admin/huntgroups/1/statistics", nil)
	w := httptest.NewRecorder()
	
	server.huntGroupHandler.HandleHuntGroupStatistics(w, req)
	
	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
	
	var stats huntgroup.CallStatistics
	err := json.NewDecoder(w.Body).Decode(&stats)
	if err != nil {
		t.Fatalf("Failed to decode statistics response: %v", err)
	}
	
	if stats.GroupID != 1 {
		t.Errorf("Expected group ID 1, got %d", stats.GroupID)
	}
	
	if stats.TotalCalls != 5 {
		t.Errorf("Expected 5 total calls, got %d", stats.TotalCalls)
	}
}