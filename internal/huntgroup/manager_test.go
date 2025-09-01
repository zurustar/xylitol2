package huntgroup

import (
	"testing"

	"github.com/zurustar/xylitol2/internal/database"
	"github.com/zurustar/xylitol2/internal/logging"
)

func TestHuntGroupManager(t *testing.T) {
	// Create in-memory database for testing
	db := database.NewSQLiteManager(":memory:")
	if err := db.Initialize(); err != nil {
		t.Fatalf("Failed to initialize test database: %v", err)
	}
	defer db.Close()

	// Create logger
	logger := logging.NewConsoleLogger(logging.ErrorLevel)

	// Create hunt group manager
	manager := NewManager(db, logger)

	// Initialize hunt group tables
	if err := manager.InitializeTables(); err != nil {
		t.Fatalf("Failed to initialize hunt group tables: %v", err)
	}

	t.Run("CreateGroup", func(t *testing.T) {
		group := &HuntGroup{
			Name:        "Sales Team",
			Extension:   "100",
			Strategy:    StrategySimultaneous,
			RingTimeout: 30,
			Enabled:     true,
			Description: "Sales department hunt group",
		}

		err := manager.CreateGroup(group)
		if err != nil {
			t.Errorf("Failed to create hunt group: %v", err)
		}

		if group.ID == 0 {
			t.Error("Expected hunt group ID to be set")
		}

		if group.CreatedAt.IsZero() {
			t.Error("Expected CreatedAt to be set")
		}
	})

	t.Run("GetGroup", func(t *testing.T) {
		// Create a group first
		group := &HuntGroup{
			Name:        "Support Team",
			Extension:   "200",
			Strategy:    StrategySequential,
			RingTimeout: 45,
			Enabled:     true,
			Description: "Support department hunt group",
		}

		err := manager.CreateGroup(group)
		if err != nil {
			t.Fatalf("Failed to create hunt group: %v", err)
		}

		// Retrieve the group
		retrieved, err := manager.GetGroup(group.ID)
		if err != nil {
			t.Errorf("Failed to get hunt group: %v", err)
		}

		if retrieved.Name != group.Name {
			t.Errorf("Expected name %s, got %s", group.Name, retrieved.Name)
		}

		if retrieved.Extension != group.Extension {
			t.Errorf("Expected extension %s, got %s", group.Extension, retrieved.Extension)
		}

		if retrieved.Strategy != group.Strategy {
			t.Errorf("Expected strategy %s, got %s", group.Strategy, retrieved.Strategy)
		}
	})

	t.Run("GetGroupByExtension", func(t *testing.T) {
		// Create a group first
		group := &HuntGroup{
			Name:        "Marketing Team",
			Extension:   "300",
			Strategy:    StrategyRoundRobin,
			RingTimeout: 20,
			Enabled:     true,
			Description: "Marketing department hunt group",
		}

		err := manager.CreateGroup(group)
		if err != nil {
			t.Fatalf("Failed to create hunt group: %v", err)
		}

		// Retrieve by extension
		retrieved, err := manager.GetGroupByExtension("300")
		if err != nil {
			t.Errorf("Failed to get hunt group by extension: %v", err)
		}

		if retrieved.ID != group.ID {
			t.Errorf("Expected ID %d, got %d", group.ID, retrieved.ID)
		}
	})

	t.Run("AddMember", func(t *testing.T) {
		// Create a group first
		group := &HuntGroup{
			Name:        "Test Group",
			Extension:   "400",
			Strategy:    StrategySimultaneous,
			RingTimeout: 30,
			Enabled:     true,
		}

		err := manager.CreateGroup(group)
		if err != nil {
			t.Fatalf("Failed to create hunt group: %v", err)
		}

		// Add a member
		member := &HuntGroupMember{
			Extension: "401",
			Priority:  1,
			Enabled:   true,
			Timeout:   25,
		}

		err = manager.AddMember(group.ID, member)
		if err != nil {
			t.Errorf("Failed to add member: %v", err)
		}

		if member.ID == 0 {
			t.Error("Expected member ID to be set")
		}

		// Retrieve members
		members, err := manager.GetGroupMembers(group.ID)
		if err != nil {
			t.Errorf("Failed to get group members: %v", err)
		}

		if len(members) != 1 {
			t.Errorf("Expected 1 member, got %d", len(members))
		}

		if members[0].Extension != "401" {
			t.Errorf("Expected extension 401, got %s", members[0].Extension)
		}
	})

	t.Run("ListGroups", func(t *testing.T) {
		groups, err := manager.ListGroups()
		if err != nil {
			t.Errorf("Failed to list hunt groups: %v", err)
		}

		// Should have at least the groups we created in previous tests
		if len(groups) < 3 {
			t.Errorf("Expected at least 3 groups, got %d", len(groups))
		}

		// Check that members are loaded
		for _, group := range groups {
			if group.Members == nil {
				t.Errorf("Expected members to be loaded for group %s", group.Name)
			}
		}
	})

	t.Run("UpdateGroup", func(t *testing.T) {
		// Get an existing group
		groups, err := manager.ListGroups()
		if err != nil || len(groups) == 0 {
			t.Fatalf("No groups available for update test")
		}

		group := groups[0]
		originalName := group.Name
		group.Name = "Updated " + originalName
		group.RingTimeout = 60

		err = manager.UpdateGroup(group)
		if err != nil {
			t.Errorf("Failed to update hunt group: %v", err)
		}

		// Retrieve and verify
		updated, err := manager.GetGroup(group.ID)
		if err != nil {
			t.Errorf("Failed to get updated hunt group: %v", err)
		}

		if updated.Name != group.Name {
			t.Errorf("Expected updated name %s, got %s", group.Name, updated.Name)
		}

		if updated.RingTimeout != 60 {
			t.Errorf("Expected ring timeout 60, got %d", updated.RingTimeout)
		}
	})

	t.Run("RemoveMember", func(t *testing.T) {
		// Get a group with members
		groups, err := manager.ListGroups()
		if err != nil || len(groups) == 0 {
			t.Fatalf("No groups available for member removal test")
		}

		var groupWithMembers *HuntGroup
		for _, g := range groups {
			if len(g.Members) > 0 {
				groupWithMembers = g
				break
			}
		}

		if groupWithMembers == nil {
			t.Skip("No groups with members found")
		}

		member := groupWithMembers.Members[0]
		err = manager.RemoveMember(groupWithMembers.ID, member.ID)
		if err != nil {
			t.Errorf("Failed to remove member: %v", err)
		}

		// Verify member was removed
		members, err := manager.GetGroupMembers(groupWithMembers.ID)
		if err != nil {
			t.Errorf("Failed to get group members after removal: %v", err)
		}

		for _, m := range members {
			if m.ID == member.ID {
				t.Error("Member was not removed")
			}
		}
	})

	t.Run("DeleteGroup", func(t *testing.T) {
		// Create a group to delete
		group := &HuntGroup{
			Name:        "Temp Group",
			Extension:   "999",
			Strategy:    StrategySimultaneous,
			RingTimeout: 30,
			Enabled:     true,
		}

		err := manager.CreateGroup(group)
		if err != nil {
			t.Fatalf("Failed to create hunt group for deletion: %v", err)
		}

		// Delete the group
		err = manager.DeleteGroup(group.ID)
		if err != nil {
			t.Errorf("Failed to delete hunt group: %v", err)
		}

		// Verify it's deleted
		_, err = manager.GetGroup(group.ID)
		if err == nil {
			t.Error("Expected error when getting deleted group")
		}
	})
}

func TestHuntGroupValidation(t *testing.T) {
	// Create in-memory database for testing
	db := database.NewSQLiteManager(":memory:")
	if err := db.Initialize(); err != nil {
		t.Fatalf("Failed to initialize test database: %v", err)
	}
	defer db.Close()

	logger := logging.NewConsoleLogger(logging.ErrorLevel)
	manager := NewManager(db, logger)

	if err := manager.InitializeTables(); err != nil {
		t.Fatalf("Failed to initialize hunt group tables: %v", err)
	}

	t.Run("CreateGroupWithoutName", func(t *testing.T) {
		group := &HuntGroup{
			Extension:   "100",
			Strategy:    StrategySimultaneous,
			RingTimeout: 30,
			Enabled:     true,
		}

		err := manager.CreateGroup(group)
		if err == nil {
			t.Error("Expected error when creating group without name")
		}
	})

	t.Run("CreateGroupWithoutExtension", func(t *testing.T) {
		group := &HuntGroup{
			Name:        "Test Group",
			Strategy:    StrategySimultaneous,
			RingTimeout: 30,
			Enabled:     true,
		}

		err := manager.CreateGroup(group)
		if err == nil {
			t.Error("Expected error when creating group without extension")
		}
	})

	t.Run("AddMemberWithoutExtension", func(t *testing.T) {
		// Create a valid group first
		group := &HuntGroup{
			Name:        "Valid Group",
			Extension:   "100",
			Strategy:    StrategySimultaneous,
			RingTimeout: 30,
			Enabled:     true,
		}

		err := manager.CreateGroup(group)
		if err != nil {
			t.Fatalf("Failed to create hunt group: %v", err)
		}

		// Try to add member without extension
		member := &HuntGroupMember{
			Priority: 1,
			Enabled:  true,
		}

		err = manager.AddMember(group.ID, member)
		if err == nil {
			t.Error("Expected error when adding member without extension")
		}
	})
}