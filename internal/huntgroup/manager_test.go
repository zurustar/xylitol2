package huntgroup

import (
	"testing"
	"time"

	"github.com/zurustar/xylitol2/internal/database"
)

// MockDatabaseManager implements the database.DatabaseManager interface for testing
type MockDatabaseManager struct {
	huntGroups map[string]*database.HuntGroup
	calls      map[string]*database.HuntGroupCall
	nextID     int
}

func NewMockDatabaseManager() *MockDatabaseManager {
	return &MockDatabaseManager{
		huntGroups: make(map[string]*database.HuntGroup),
		calls:      make(map[string]*database.HuntGroupCall),
		nextID:     1,
	}
}

// Hunt Group operations
func (m *MockDatabaseManager) CreateHuntGroup(huntGroup *database.HuntGroup) error {
	// Check for duplicate extension
	for _, hg := range m.huntGroups {
		if hg.Extension == huntGroup.Extension {
			return database.ErrDuplicateExtension
		}
	}
	
	// Set timestamps if not set
	if huntGroup.CreatedAt.IsZero() {
		huntGroup.CreatedAt = time.Now().UTC()
	}
	if huntGroup.UpdatedAt.IsZero() {
		huntGroup.UpdatedAt = time.Now().UTC()
	}
	
	m.huntGroups[huntGroup.ID] = huntGroup
	return nil
}

func (m *MockDatabaseManager) GetHuntGroup(id string) (*database.HuntGroup, error) {
	hg, exists := m.huntGroups[id]
	if !exists {
		return nil, database.ErrNotFound
	}
	return hg, nil
}

func (m *MockDatabaseManager) GetHuntGroupByExtension(extension string) (*database.HuntGroup, error) {
	for _, hg := range m.huntGroups {
		if hg.Extension == extension {
			return hg, nil
		}
	}
	return nil, database.ErrNotFound
}

func (m *MockDatabaseManager) UpdateHuntGroup(huntGroup *database.HuntGroup) error {
	_, exists := m.huntGroups[huntGroup.ID]
	if !exists {
		return database.ErrNotFound
	}
	
	// Check for duplicate extension (excluding current group)
	for id, hg := range m.huntGroups {
		if id != huntGroup.ID && hg.Extension == huntGroup.Extension {
			return database.ErrDuplicateExtension
		}
	}
	
	huntGroup.UpdatedAt = time.Now().UTC()
	m.huntGroups[huntGroup.ID] = huntGroup
	return nil
}

func (m *MockDatabaseManager) DeleteHuntGroup(id string) error {
	_, exists := m.huntGroups[id]
	if !exists {
		return database.ErrNotFound
	}
	delete(m.huntGroups, id)
	return nil
}

func (m *MockDatabaseManager) ListHuntGroups() ([]*database.HuntGroup, error) {
	var huntGroups []*database.HuntGroup
	for _, hg := range m.huntGroups {
		huntGroups = append(huntGroups, hg)
	}
	return huntGroups, nil
}

// Hunt Group Call operations
func (m *MockDatabaseManager) CreateHuntGroupCall(call *database.HuntGroupCall) error {
	// Check if hunt group exists
	_, exists := m.huntGroups[call.HuntGroupID]
	if !exists {
		return database.ErrNotFound
	}
	
	if call.CreatedAt.IsZero() {
		call.CreatedAt = time.Now().UTC()
	}
	if call.UpdatedAt.IsZero() {
		call.UpdatedAt = time.Now().UTC()
	}
	
	m.calls[call.ID] = call
	return nil
}

func (m *MockDatabaseManager) GetHuntGroupCall(id string) (*database.HuntGroupCall, error) {
	call, exists := m.calls[id]
	if !exists {
		return nil, database.ErrNotFound
	}
	return call, nil
}

func (m *MockDatabaseManager) UpdateHuntGroupCall(call *database.HuntGroupCall) error {
	_, exists := m.calls[call.ID]
	if !exists {
		return database.ErrNotFound
	}
	
	call.UpdatedAt = time.Now().UTC()
	m.calls[call.ID] = call
	return nil
}

func (m *MockDatabaseManager) ListHuntGroupCalls(huntGroupID string) ([]*database.HuntGroupCall, error) {
	var calls []*database.HuntGroupCall
	for _, call := range m.calls {
		if call.HuntGroupID == huntGroupID {
			calls = append(calls, call)
		}
	}
	return calls, nil
}

// Unused methods for interface compliance
func (m *MockDatabaseManager) Initialize() error                                                    { return nil }
func (m *MockDatabaseManager) Close() error                                                        { return nil }
func (m *MockDatabaseManager) CreateUser(user *database.User) error                                { return nil }
func (m *MockDatabaseManager) GetUser(username, realm string) (*database.User, error)             { return nil, nil }
func (m *MockDatabaseManager) UpdateUser(user *database.User) error                                { return nil }
func (m *MockDatabaseManager) DeleteUser(username, realm string) error                            { return nil }
func (m *MockDatabaseManager) ListUsers() ([]*database.User, error)                               { return nil, nil }
func (m *MockDatabaseManager) StoreContact(contact *database.Contact) error                       { return nil }
func (m *MockDatabaseManager) RetrieveContacts(aor string) ([]*database.Contact, error)          { return nil, nil }
func (m *MockDatabaseManager) DeleteContact(aor string, contactURI string) error                  { return nil }
func (m *MockDatabaseManager) CleanupExpiredContacts() error                                       { return nil }
func (m *MockDatabaseManager) Exec(query string, args ...interface{}) error                       { return nil }
func (m *MockDatabaseManager) ExecWithResult(query string, args ...interface{}) (database.Result, error) { return nil, nil }
func (m *MockDatabaseManager) Query(query string, args ...interface{}) (database.Rows, error)    { return nil, nil }
func (m *MockDatabaseManager) QueryRow(query string, dest []interface{}, args ...interface{}) error { return nil }

func TestDatabaseManager_CreateGroup(t *testing.T) {
	mockDB := NewMockDatabaseManager()
	manager := NewDatabaseManager(mockDB)

	group := &HuntGroup{
		ID:          1,
		Name:        "Sales Team",
		Extension:   "100",
		Strategy:    StrategySimultaneous,
		RingTimeout: 30,
		Enabled:     true,
		Description: "Sales team hunt group",
		Members: []*HuntGroupMember{
			{
				ID:        1,
				Extension: "101",
				Priority:  1,
				Enabled:   true,
				Timeout:   20,
			},
			{
				ID:        2,
				Extension: "102",
				Priority:  2,
				Enabled:   true,
				Timeout:   20,
			},
		},
	}

	err := manager.CreateGroup(group)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	// Verify the group was created
	retrieved, err := manager.GetGroup(1)
	if err != nil {
		t.Fatalf("Expected no error retrieving group, got %v", err)
	}

	if retrieved.Name != group.Name {
		t.Errorf("Expected name %s, got %s", group.Name, retrieved.Name)
	}

	if retrieved.Extension != group.Extension {
		t.Errorf("Expected extension %s, got %s", group.Extension, retrieved.Extension)
	}

	if len(retrieved.Members) != 2 {
		t.Errorf("Expected 2 members, got %d", len(retrieved.Members))
	}
}

func TestDatabaseManager_CreateGroup_Validation(t *testing.T) {
	mockDB := NewMockDatabaseManager()
	manager := NewDatabaseManager(mockDB)

	tests := []struct {
		name        string
		group       *HuntGroup
		expectError bool
	}{
		{
			name: "empty name",
			group: &HuntGroup{
				ID:          1,
				Name:        "",
				Extension:   "100",
				Strategy:    StrategySimultaneous,
				RingTimeout: 30,
				Enabled:     true,
			},
			expectError: true,
		},
		{
			name: "empty extension",
			group: &HuntGroup{
				ID:          1,
				Name:        "Test Group",
				Extension:   "",
				Strategy:    StrategySimultaneous,
				RingTimeout: 30,
				Enabled:     true,
			},
			expectError: true,
		},
		{
			name: "invalid timeout - zero",
			group: &HuntGroup{
				ID:          1,
				Name:        "Test Group",
				Extension:   "100",
				Strategy:    StrategySimultaneous,
				RingTimeout: 0,
				Enabled:     true,
			},
			expectError: true,
		},
		{
			name: "invalid timeout - too large",
			group: &HuntGroup{
				ID:          1,
				Name:        "Test Group",
				Extension:   "100",
				Strategy:    StrategySimultaneous,
				RingTimeout: 400,
				Enabled:     true,
			},
			expectError: true,
		},
		{
			name: "valid group",
			group: &HuntGroup{
				ID:          1,
				Name:        "Test Group",
				Extension:   "100",
				Strategy:    StrategySimultaneous,
				RingTimeout: 30,
				Enabled:     true,
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := manager.CreateGroup(tt.group)
			if tt.expectError && err == nil {
				t.Errorf("Expected error for %s, got nil", tt.name)
			}
			if !tt.expectError && err != nil {
				t.Errorf("Expected no error for %s, got %v", tt.name, err)
			}
		})
	}
}

func TestDatabaseManager_GetGroupByExtension(t *testing.T) {
	mockDB := NewMockDatabaseManager()
	manager := NewDatabaseManager(mockDB)

	group := &HuntGroup{
		ID:          1,
		Name:        "Sales Team",
		Extension:   "100",
		Strategy:    StrategySimultaneous,
		RingTimeout: 30,
		Enabled:     true,
		Description: "Sales team hunt group",
	}

	err := manager.CreateGroup(group)
	if err != nil {
		t.Fatalf("Failed to create group: %v", err)
	}

	retrieved, err := manager.GetGroupByExtension("100")
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if retrieved.ID != group.ID {
		t.Errorf("Expected ID %d, got %d", group.ID, retrieved.ID)
	}

	// Test non-existent extension
	_, err = manager.GetGroupByExtension("999")
	if err == nil {
		t.Error("Expected error for non-existent extension, got nil")
	}
}

func TestDatabaseManager_UpdateGroup(t *testing.T) {
	mockDB := NewMockDatabaseManager()
	manager := NewDatabaseManager(mockDB)

	group := &HuntGroup{
		ID:          1,
		Name:        "Sales Team",
		Extension:   "100",
		Strategy:    StrategySimultaneous,
		RingTimeout: 30,
		Enabled:     true,
		Description: "Sales team hunt group",
	}

	err := manager.CreateGroup(group)
	if err != nil {
		t.Fatalf("Failed to create group: %v", err)
	}

	// Update the group
	group.Name = "Updated Sales Team"
	group.RingTimeout = 45
	group.Description = "Updated description"

	err = manager.UpdateGroup(group)
	if err != nil {
		t.Fatalf("Expected no error updating group, got %v", err)
	}

	// Verify the update
	retrieved, err := manager.GetGroup(1)
	if err != nil {
		t.Fatalf("Failed to retrieve updated group: %v", err)
	}

	if retrieved.Name != "Updated Sales Team" {
		t.Errorf("Expected name 'Updated Sales Team', got %s", retrieved.Name)
	}

	if retrieved.RingTimeout != 45 {
		t.Errorf("Expected timeout 45, got %d", retrieved.RingTimeout)
	}
}

func TestDatabaseManager_DeleteGroup(t *testing.T) {
	mockDB := NewMockDatabaseManager()
	manager := NewDatabaseManager(mockDB)

	group := &HuntGroup{
		ID:          1,
		Name:        "Sales Team",
		Extension:   "100",
		Strategy:    StrategySimultaneous,
		RingTimeout: 30,
		Enabled:     true,
	}

	err := manager.CreateGroup(group)
	if err != nil {
		t.Fatalf("Failed to create group: %v", err)
	}

	err = manager.DeleteGroup(1)
	if err != nil {
		t.Fatalf("Expected no error deleting group, got %v", err)
	}

	// Verify deletion
	_, err = manager.GetGroup(1)
	if err == nil {
		t.Error("Expected error retrieving deleted group, got nil")
	}
}

func TestDatabaseManager_ListGroups(t *testing.T) {
	mockDB := NewMockDatabaseManager()
	manager := NewDatabaseManager(mockDB)

	groups := []*HuntGroup{
		{
			ID:          1,
			Name:        "Sales Team",
			Extension:   "100",
			Strategy:    StrategySimultaneous,
			RingTimeout: 30,
			Enabled:     true,
		},
		{
			ID:          2,
			Name:        "Support Team",
			Extension:   "200",
			Strategy:    StrategySequential,
			RingTimeout: 45,
			Enabled:     true,
		},
	}

	for _, group := range groups {
		err := manager.CreateGroup(group)
		if err != nil {
			t.Fatalf("Failed to create group %d: %v", group.ID, err)
		}
	}

	retrieved, err := manager.ListGroups()
	if err != nil {
		t.Fatalf("Expected no error listing groups, got %v", err)
	}

	if len(retrieved) != 2 {
		t.Errorf("Expected 2 groups, got %d", len(retrieved))
	}
}

func TestDatabaseManager_AddMember(t *testing.T) {
	mockDB := NewMockDatabaseManager()
	manager := NewDatabaseManager(mockDB)

	group := &HuntGroup{
		ID:          1,
		Name:        "Sales Team",
		Extension:   "100",
		Strategy:    StrategySimultaneous,
		RingTimeout: 30,
		Enabled:     true,
		Members:     []*HuntGroupMember{},
	}

	err := manager.CreateGroup(group)
	if err != nil {
		t.Fatalf("Failed to create group: %v", err)
	}

	member := &HuntGroupMember{
		ID:        1,
		Extension: "101",
		Priority:  1,
		Enabled:   true,
		Timeout:   20,
	}

	err = manager.AddMember(1, member)
	if err != nil {
		t.Fatalf("Expected no error adding member, got %v", err)
	}

	// Verify member was added
	members, err := manager.GetGroupMembers(1)
	if err != nil {
		t.Fatalf("Failed to get group members: %v", err)
	}

	if len(members) != 1 {
		t.Errorf("Expected 1 member, got %d", len(members))
	}

	if members[0].Extension != "101" {
		t.Errorf("Expected extension 101, got %s", members[0].Extension)
	}
}

func TestDatabaseManager_AddMember_Validation(t *testing.T) {
	mockDB := NewMockDatabaseManager()
	manager := NewDatabaseManager(mockDB)

	group := &HuntGroup{
		ID:          1,
		Name:        "Sales Team",
		Extension:   "100",
		Strategy:    StrategySimultaneous,
		RingTimeout: 30,
		Enabled:     true,
		Members:     []*HuntGroupMember{},
	}

	err := manager.CreateGroup(group)
	if err != nil {
		t.Fatalf("Failed to create group: %v", err)
	}

	tests := []struct {
		name        string
		member      *HuntGroupMember
		expectError bool
	}{
		{
			name: "empty extension",
			member: &HuntGroupMember{
				ID:       1,
				Priority: 1,
				Enabled:  true,
				Timeout:  20,
			},
			expectError: true,
		},
		{
			name: "negative priority",
			member: &HuntGroupMember{
				ID:        1,
				Extension: "101",
				Priority:  -1,
				Enabled:   true,
				Timeout:   20,
			},
			expectError: true,
		},
		{
			name: "invalid timeout - zero",
			member: &HuntGroupMember{
				ID:        1,
				Extension: "101",
				Priority:  1,
				Enabled:   true,
				Timeout:   0,
			},
			expectError: true,
		},
		{
			name: "invalid timeout - too large",
			member: &HuntGroupMember{
				ID:        1,
				Extension: "101",
				Priority:  1,
				Enabled:   true,
				Timeout:   150,
			},
			expectError: true,
		},
		{
			name: "valid member",
			member: &HuntGroupMember{
				ID:        1,
				Extension: "101",
				Priority:  1,
				Enabled:   true,
				Timeout:   20,
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := manager.AddMember(1, tt.member)
			if tt.expectError && err == nil {
				t.Errorf("Expected error for %s, got nil", tt.name)
			}
			if !tt.expectError && err != nil {
				t.Errorf("Expected no error for %s, got %v", tt.name, err)
			}
		})
	}
}

func TestDatabaseManager_RemoveMember(t *testing.T) {
	mockDB := NewMockDatabaseManager()
	manager := NewDatabaseManager(mockDB)

	group := &HuntGroup{
		ID:          1,
		Name:        "Sales Team",
		Extension:   "100",
		Strategy:    StrategySimultaneous,
		RingTimeout: 30,
		Enabled:     true,
		Members: []*HuntGroupMember{
			{
				ID:        1,
				Extension: "101",
				Priority:  1,
				Enabled:   true,
				Timeout:   20,
			},
			{
				ID:        2,
				Extension: "102",
				Priority:  2,
				Enabled:   true,
				Timeout:   20,
			},
		},
	}

	err := manager.CreateGroup(group)
	if err != nil {
		t.Fatalf("Failed to create group: %v", err)
	}

	err = manager.RemoveMember(1, 1)
	if err != nil {
		t.Fatalf("Expected no error removing member, got %v", err)
	}

	// Verify member was removed
	members, err := manager.GetGroupMembers(1)
	if err != nil {
		t.Fatalf("Failed to get group members: %v", err)
	}

	if len(members) != 1 {
		t.Errorf("Expected 1 member after removal, got %d", len(members))
	}

	if members[0].ID != 2 {
		t.Errorf("Expected remaining member ID 2, got %d", members[0].ID)
	}
}

func TestDatabaseManager_UpdateMember(t *testing.T) {
	mockDB := NewMockDatabaseManager()
	manager := NewDatabaseManager(mockDB)

	group := &HuntGroup{
		ID:          1,
		Name:        "Sales Team",
		Extension:   "100",
		Strategy:    StrategySimultaneous,
		RingTimeout: 30,
		Enabled:     true,
		Members: []*HuntGroupMember{
			{
				ID:        1,
				GroupID:   1,
				Extension: "101",
				Priority:  1,
				Enabled:   true,
				Timeout:   20,
			},
		},
	}

	err := manager.CreateGroup(group)
	if err != nil {
		t.Fatalf("Failed to create group: %v", err)
	}

	// Update the member
	updatedMember := &HuntGroupMember{
		ID:        1,
		GroupID:   1,
		Extension: "103",
		Priority:  5,
		Enabled:   false,
		Timeout:   30,
	}

	err = manager.UpdateMember(updatedMember)
	if err != nil {
		t.Fatalf("Expected no error updating member, got %v", err)
	}

	// Verify the update
	members, err := manager.GetGroupMembers(1)
	if err != nil {
		t.Fatalf("Failed to get group members: %v", err)
	}

	if len(members) != 1 {
		t.Fatalf("Expected 1 member, got %d", len(members))
	}

	member := members[0]
	if member.Extension != "103" {
		t.Errorf("Expected extension 103, got %s", member.Extension)
	}

	if member.Priority != 5 {
		t.Errorf("Expected priority 5, got %d", member.Priority)
	}

	if member.Enabled != false {
		t.Errorf("Expected enabled false, got %t", member.Enabled)
	}

	if member.Timeout != 30 {
		t.Errorf("Expected timeout 30, got %d", member.Timeout)
	}
}

func TestDatabaseManager_CallStatistics(t *testing.T) {
	mockDB := NewMockDatabaseManager()
	manager := NewDatabaseManager(mockDB)

	group := &HuntGroup{
		ID:          1,
		Name:        "Sales Team",
		Extension:   "100",
		Strategy:    StrategySimultaneous,
		RingTimeout: 30,
		Enabled:     true,
	}

	err := manager.CreateGroup(group)
	if err != nil {
		t.Fatalf("Failed to create group: %v", err)
	}

	// Create some call sessions
	sessions := []*CallSession{
		{
			ID:        "session1",
			GroupID:   1,
			CallerURI: "sip:caller1@example.com",
			Status:    SessionStatusAnswered,
			StartTime: time.Now().Add(-time.Hour),
		},
		{
			ID:        "session2",
			GroupID:   1,
			CallerURI: "sip:caller2@example.com",
			Status:    SessionStatusCancelled,
			StartTime: time.Now().Add(-30 * time.Minute),
		},
	}

	for _, session := range sessions {
		err := manager.CreateSession(session)
		if err != nil {
			t.Fatalf("Failed to create session %s: %v", session.ID, err)
		}
	}

	stats, err := manager.GetCallStatistics(1)
	if err != nil {
		t.Fatalf("Expected no error getting statistics, got %v", err)
	}

	if stats.GroupID != 1 {
		t.Errorf("Expected group ID 1, got %d", stats.GroupID)
	}

	if stats.TotalCalls != 2 {
		t.Errorf("Expected 2 total calls, got %d", stats.TotalCalls)
	}

	if stats.AnsweredCalls != 1 {
		t.Errorf("Expected 1 answered call, got %d", stats.AnsweredCalls)
	}

	if stats.MissedCalls != 1 {
		t.Errorf("Expected 1 missed call, got %d", stats.MissedCalls)
	}
}

func TestDatabaseManager_EnableDisableGroup(t *testing.T) {
	mockDB := NewMockDatabaseManager()
	manager := NewDatabaseManager(mockDB)

	group := &HuntGroup{
		ID:          1,
		Name:        "Sales Team",
		Extension:   "100",
		Strategy:    StrategySimultaneous,
		RingTimeout: 30,
		Enabled:     true,
	}

	err := manager.CreateGroup(group)
	if err != nil {
		t.Fatalf("Failed to create group: %v", err)
	}

	// Test disable
	err = manager.DisableGroup(1)
	if err != nil {
		t.Fatalf("Expected no error disabling group, got %v", err)
	}

	retrieved, err := manager.GetGroup(1)
	if err != nil {
		t.Fatalf("Failed to retrieve group: %v", err)
	}

	if retrieved.Enabled {
		t.Error("Expected group to be disabled")
	}

	// Test enable
	err = manager.EnableGroup(1)
	if err != nil {
		t.Fatalf("Expected no error enabling group, got %v", err)
	}

	retrieved, err = manager.GetGroup(1)
	if err != nil {
		t.Fatalf("Failed to retrieve group: %v", err)
	}

	if !retrieved.Enabled {
		t.Error("Expected group to be enabled")
	}
}

func TestDatabaseManager_EnableDisableMember(t *testing.T) {
	mockDB := NewMockDatabaseManager()
	manager := NewDatabaseManager(mockDB)

	group := &HuntGroup{
		ID:          1,
		Name:        "Sales Team",
		Extension:   "100",
		Strategy:    StrategySimultaneous,
		RingTimeout: 30,
		Enabled:     true,
		Members: []*HuntGroupMember{
			{
				ID:        1,
				Extension: "101",
				Priority:  1,
				Enabled:   true,
				Timeout:   20,
			},
		},
	}

	err := manager.CreateGroup(group)
	if err != nil {
		t.Fatalf("Failed to create group: %v", err)
	}

	// Test disable member
	err = manager.DisableMember(1, 1)
	if err != nil {
		t.Fatalf("Expected no error disabling member, got %v", err)
	}

	members, err := manager.GetGroupMembers(1)
	if err != nil {
		t.Fatalf("Failed to get group members: %v", err)
	}

	if len(members) != 1 {
		t.Fatalf("Expected 1 member, got %d", len(members))
	}

	if members[0].Enabled {
		t.Error("Expected member to be disabled")
	}

	// Test enable member
	err = manager.EnableMember(1, 1)
	if err != nil {
		t.Fatalf("Expected no error enabling member, got %v", err)
	}

	members, err = manager.GetGroupMembers(1)
	if err != nil {
		t.Fatalf("Failed to get group members: %v", err)
	}

	if !members[0].Enabled {
		t.Error("Expected member to be enabled")
	}
}

func TestDatabaseManager_AddMember_DuplicateExtension(t *testing.T) {
	mockDB := NewMockDatabaseManager()
	manager := NewDatabaseManager(mockDB)

	group := &HuntGroup{
		ID:          1,
		Name:        "Sales Team",
		Extension:   "100",
		Strategy:    StrategySimultaneous,
		RingTimeout: 30,
		Enabled:     true,
		Members: []*HuntGroupMember{
			{
				ID:        1,
				Extension: "101",
				Priority:  1,
				Enabled:   true,
				Timeout:   20,
			},
		},
	}

	err := manager.CreateGroup(group)
	if err != nil {
		t.Fatalf("Failed to create group: %v", err)
	}

	// Try to add member with duplicate extension
	duplicateMember := &HuntGroupMember{
		ID:        2,
		Extension: "101", // Same as existing member
		Priority:  2,
		Enabled:   true,
		Timeout:   20,
	}

	err = manager.AddMember(1, duplicateMember)
	if err == nil {
		t.Error("Expected error when adding member with duplicate extension, got nil")
	}
}

func TestDatabaseManager_AutoGenerateMemberID(t *testing.T) {
	mockDB := NewMockDatabaseManager()
	manager := NewDatabaseManager(mockDB)

	group := &HuntGroup{
		ID:          1,
		Name:        "Sales Team",
		Extension:   "100",
		Strategy:    StrategySimultaneous,
		RingTimeout: 30,
		Enabled:     true,
		Members: []*HuntGroupMember{
			{ID: 1, Extension: "101", Priority: 1, Enabled: true, Timeout: 20},
			{ID: 3, Extension: "102", Priority: 2, Enabled: true, Timeout: 20},
		},
	}

	err := manager.CreateGroup(group)
	if err != nil {
		t.Fatalf("Failed to create group: %v", err)
	}

	// Add member without ID (should auto-generate)
	newMember := &HuntGroupMember{
		Extension: "103",
		Priority:  3,
		Enabled:   true,
		Timeout:   20,
	}

	err = manager.AddMember(1, newMember)
	if err != nil {
		t.Fatalf("Expected no error adding member, got %v", err)
	}

	members, err := manager.GetGroupMembers(1)
	if err != nil {
		t.Fatalf("Failed to get group members: %v", err)
	}

	if len(members) != 3 {
		t.Errorf("Expected 3 members, got %d", len(members))
	}

	// Find the new member and check its ID
	var foundMember *HuntGroupMember
	for _, member := range members {
		if member.Extension == "103" {
			foundMember = member
			break
		}
	}

	if foundMember == nil {
		t.Fatal("New member not found")
	}

	if foundMember.ID != 4 {
		t.Errorf("Expected auto-generated ID to be 4, got %d", foundMember.ID)
	}
}

func TestDatabaseManager_CallStatistics_Enhanced(t *testing.T) {
	mockDB := NewMockDatabaseManager()
	manager := NewDatabaseManager(mockDB)

	group := &HuntGroup{
		ID:          1,
		Name:        "Sales Team",
		Extension:   "100",
		Strategy:    StrategySimultaneous,
		RingTimeout: 30,
		Enabled:     true,
	}

	err := manager.CreateGroup(group)
	if err != nil {
		t.Fatalf("Failed to create group: %v", err)
	}

	// Create call sessions with more detailed data
	now := time.Now()
	answeredTime := now.Add(10 * time.Second)
	
	sessions := []*CallSession{
		{
			ID:         "session1",
			GroupID:    1,
			CallerURI:  "sip:caller1@example.com",
			Status:     SessionStatusAnswered,
			StartTime:  now.Add(-time.Hour),
			AnsweredBy: "101",
			AnsweredAt: &answeredTime,
		},
		{
			ID:        "session2",
			GroupID:   1,
			CallerURI: "sip:caller2@example.com",
			Status:    SessionStatusCancelled,
			StartTime: now.Add(-30 * time.Minute),
		},
		{
			ID:         "session3",
			GroupID:    1,
			CallerURI:  "sip:caller3@example.com",
			Status:     SessionStatusAnswered,
			StartTime:  now.Add(-15 * time.Minute),
			AnsweredBy: "101",
			AnsweredAt: &answeredTime,
		},
	}

	for _, session := range sessions {
		err := manager.CreateSession(session)
		if err != nil {
			t.Fatalf("Failed to create session %s: %v", session.ID, err)
		}
	}

	stats, err := manager.GetCallStatistics(1)
	if err != nil {
		t.Fatalf("Expected no error getting statistics, got %v", err)
	}

	if stats.GroupID != 1 {
		t.Errorf("Expected group ID 1, got %d", stats.GroupID)
	}

	if stats.TotalCalls != 3 {
		t.Errorf("Expected 3 total calls, got %d", stats.TotalCalls)
	}

	if stats.AnsweredCalls != 2 {
		t.Errorf("Expected 2 answered calls, got %d", stats.AnsweredCalls)
	}

	if stats.MissedCalls != 1 {
		t.Errorf("Expected 1 missed call, got %d", stats.MissedCalls)
	}

	if stats.BusiestMember != "101" {
		t.Errorf("Expected busiest member to be '101', got '%s'", stats.BusiestMember)
	}
}