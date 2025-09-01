package huntgroup

import (
	"fmt"
	"strconv"
	"time"

	"github.com/zurustar/xylitol2/internal/database"
)

// DatabaseManager implements the HuntGroupManager interface using a database backend
type DatabaseManager struct {
	db database.DatabaseManager
}

// NewDatabaseManager creates a new hunt group database manager
func NewDatabaseManager(db database.DatabaseManager) HuntGroupManager {
	return &DatabaseManager{
		db: db,
	}
}

// CreateGroup creates a new hunt group
func (m *DatabaseManager) CreateGroup(group *HuntGroup) error {
	if err := m.validateHuntGroup(group); err != nil {
		return fmt.Errorf("hunt group validation failed: %w", err)
	}

	// Set timestamps
	now := time.Now().UTC()
	group.CreatedAt = now
	group.UpdatedAt = now

	// Convert to database format and store
	dbHG := m.toDatabase(group)
	if err := m.db.CreateHuntGroup(dbHG); err != nil {
		return fmt.Errorf("failed to create hunt group in database: %w", err)
	}

	return nil
}

// GetGroup retrieves a hunt group by ID
func (m *DatabaseManager) GetGroup(id int) (*HuntGroup, error) {
	if id <= 0 {
		return nil, fmt.Errorf("hunt group ID must be positive")
	}

	dbHG, err := m.db.GetHuntGroup(strconv.Itoa(id))
	if err != nil {
		return nil, fmt.Errorf("failed to get hunt group from database: %w", err)
	}

	hg, err := m.fromDatabase(dbHG)
	if err != nil {
		return nil, fmt.Errorf("failed to convert hunt group from database format: %w", err)
	}

	return hg, nil
}

// GetGroupByExtension retrieves a hunt group by extension
func (m *DatabaseManager) GetGroupByExtension(extension string) (*HuntGroup, error) {
	if extension == "" {
		return nil, fmt.Errorf("hunt group extension cannot be empty")
	}

	dbHG, err := m.db.GetHuntGroupByExtension(extension)
	if err != nil {
		return nil, fmt.Errorf("failed to get hunt group by extension from database: %w", err)
	}

	hg, err := m.fromDatabase(dbHG)
	if err != nil {
		return nil, fmt.Errorf("failed to convert hunt group from database format: %w", err)
	}

	return hg, nil
}

// UpdateGroup updates an existing hunt group
func (m *DatabaseManager) UpdateGroup(group *HuntGroup) error {
	if err := m.validateHuntGroup(group); err != nil {
		return fmt.Errorf("hunt group validation failed: %w", err)
	}

	// Update timestamp
	group.UpdatedAt = time.Now().UTC()

	// Convert to database format and update
	dbHG := m.toDatabase(group)
	if err := m.db.UpdateHuntGroup(dbHG); err != nil {
		return fmt.Errorf("failed to update hunt group in database: %w", err)
	}

	return nil
}

// DeleteGroup deletes a hunt group
func (m *DatabaseManager) DeleteGroup(id int) error {
	if id <= 0 {
		return fmt.Errorf("hunt group ID must be positive")
	}

	if err := m.db.DeleteHuntGroup(strconv.Itoa(id)); err != nil {
		return fmt.Errorf("failed to delete hunt group from database: %w", err)
	}

	return nil
}

// ListGroups returns all hunt groups
func (m *DatabaseManager) ListGroups() ([]*HuntGroup, error) {
	dbHGs, err := m.db.ListHuntGroups()
	if err != nil {
		return nil, fmt.Errorf("failed to list hunt groups from database: %w", err)
	}

	huntGroups := make([]*HuntGroup, len(dbHGs))
	for i, dbHG := range dbHGs {
		hg, err := m.fromDatabase(dbHG)
		if err != nil {
			return nil, fmt.Errorf("failed to convert hunt group %d from database format: %w", i, err)
		}
		huntGroups[i] = hg
	}

	return huntGroups, nil
}

// AddMember adds a member to a hunt group
func (m *DatabaseManager) AddMember(groupID int, member *HuntGroupMember) error {
	if err := m.validateHuntGroupMember(member); err != nil {
		return fmt.Errorf("hunt group member validation failed: %w", err)
	}

	// Get the hunt group
	group, err := m.GetGroup(groupID)
	if err != nil {
		return fmt.Errorf("failed to get hunt group: %w", err)
	}

	// Check for duplicate extension within the group
	for _, existingMember := range group.Members {
		if existingMember.Extension == member.Extension {
			return fmt.Errorf("member with extension %s already exists in hunt group", member.Extension)
		}
	}

	// Set member group ID and timestamps
	member.GroupID = groupID
	now := time.Now().UTC()
	member.CreatedAt = now
	member.UpdatedAt = now

	// Generate ID if not set
	if member.ID == 0 {
		member.ID = m.generateMemberID(group.Members)
	}

	// Add member to the group
	group.Members = append(group.Members, member)

	// Update the group
	if err := m.UpdateGroup(group); err != nil {
		return fmt.Errorf("failed to update hunt group with new member: %w", err)
	}

	return nil
}

// RemoveMember removes a member from a hunt group
func (m *DatabaseManager) RemoveMember(groupID int, memberID int) error {
	// Get the hunt group
	group, err := m.GetGroup(groupID)
	if err != nil {
		return fmt.Errorf("failed to get hunt group: %w", err)
	}

	// Find and remove the member
	for i, member := range group.Members {
		if member.ID == memberID {
			group.Members = append(group.Members[:i], group.Members[i+1:]...)
			break
		}
	}

	// Update the group
	if err := m.UpdateGroup(group); err != nil {
		return fmt.Errorf("failed to update hunt group after removing member: %w", err)
	}

	return nil
}

// UpdateMember updates a hunt group member
func (m *DatabaseManager) UpdateMember(member *HuntGroupMember) error {
	if err := m.validateHuntGroupMember(member); err != nil {
		return fmt.Errorf("hunt group member validation failed: %w", err)
	}

	// Get the hunt group
	group, err := m.GetGroup(member.GroupID)
	if err != nil {
		return fmt.Errorf("failed to get hunt group: %w", err)
	}

	// Find and update the member
	found := false
	for i, existingMember := range group.Members {
		if existingMember.ID == member.ID {
			member.UpdatedAt = time.Now().UTC()
			group.Members[i] = member
			found = true
			break
		}
	}

	if !found {
		return fmt.Errorf("member not found in hunt group")
	}

	// Update the group
	if err := m.UpdateGroup(group); err != nil {
		return fmt.Errorf("failed to update hunt group with updated member: %w", err)
	}

	return nil
}

// GetGroupMembers returns all members of a hunt group
func (m *DatabaseManager) GetGroupMembers(groupID int) ([]*HuntGroupMember, error) {
	group, err := m.GetGroup(groupID)
	if err != nil {
		return nil, fmt.Errorf("failed to get hunt group: %w", err)
	}

	return group.Members, nil
}

// CreateSession creates a new call session
func (m *DatabaseManager) CreateSession(session *CallSession) error {
	if err := m.validateCallSession(session); err != nil {
		return fmt.Errorf("call session validation failed: %w", err)
	}

	// For now, we'll store call sessions as hunt group calls
	// This is a simplified implementation
	call := &database.HuntGroupCall{
		ID:          session.ID,
		HuntGroupID: strconv.Itoa(session.GroupID),
		SessionID:   session.ID,
		CallerURI:   session.CallerURI,
		CallerName:  "", // Extract from SIP message if needed
		Status:      string(session.Status),
		AnsweredBy:  session.AnsweredBy,
		AnsweredAt:  session.AnsweredAt,
		Duration:    0, // Will be calculated later
		CreatedAt:   session.StartTime,
		UpdatedAt:   session.StartTime,
	}

	if err := m.db.CreateHuntGroupCall(call); err != nil {
		return fmt.Errorf("failed to create call session in database: %w", err)
	}

	return nil
}

// GetSession retrieves a call session by ID
func (m *DatabaseManager) GetSession(sessionID string) (*CallSession, error) {
	if sessionID == "" {
		return nil, fmt.Errorf("session ID cannot be empty")
	}

	dbCall, err := m.db.GetHuntGroupCall(sessionID)
	if err != nil {
		return nil, fmt.Errorf("failed to get call session from database: %w", err)
	}

	groupID, err := strconv.Atoi(dbCall.HuntGroupID)
	if err != nil {
		return nil, fmt.Errorf("invalid group ID in database: %w", err)
	}

	session := &CallSession{
		ID:         dbCall.ID,
		GroupID:    groupID,
		CallerURI:  dbCall.CallerURI,
		StartTime:  dbCall.CreatedAt,
		Status:     CallSessionStatus(dbCall.Status),
		AnsweredBy: dbCall.AnsweredBy,
		AnsweredAt: dbCall.AnsweredAt,
	}

	return session, nil
}

// UpdateSession updates a call session
func (m *DatabaseManager) UpdateSession(session *CallSession) error {
	if err := m.validateCallSession(session); err != nil {
		return fmt.Errorf("call session validation failed: %w", err)
	}

	call := &database.HuntGroupCall{
		ID:          session.ID,
		HuntGroupID: strconv.Itoa(session.GroupID),
		SessionID:   session.ID,
		CallerURI:   session.CallerURI,
		Status:      string(session.Status),
		AnsweredBy:  session.AnsweredBy,
		AnsweredAt:  session.AnsweredAt,
		UpdatedAt:   time.Now().UTC(),
	}

	if err := m.db.UpdateHuntGroupCall(call); err != nil {
		return fmt.Errorf("failed to update call session in database: %w", err)
	}

	return nil
}

// EndSession ends a call session
func (m *DatabaseManager) EndSession(sessionID string) error {
	session, err := m.GetSession(sessionID)
	if err != nil {
		return fmt.Errorf("failed to get session for ending: %w", err)
	}

	session.Status = SessionStatusCompleted
	if err := m.UpdateSession(session); err != nil {
		return fmt.Errorf("failed to end session: %w", err)
	}

	return nil
}

// GetActiveSessions returns all active call sessions
func (m *DatabaseManager) GetActiveSessions() ([]*CallSession, error) {
	// This is a simplified implementation
	// In a real implementation, you'd query for active sessions
	return []*CallSession{}, nil
}

// GetCallStatistics returns call statistics for a hunt group
func (m *DatabaseManager) GetCallStatistics(groupID int) (*CallStatistics, error) {
	calls, err := m.db.ListHuntGroupCalls(strconv.Itoa(groupID))
	if err != nil {
		return nil, fmt.Errorf("failed to get hunt group calls: %w", err)
	}

	stats := &CallStatistics{
		GroupID:    groupID,
		TotalCalls: len(calls),
	}

	var totalRingTime time.Duration
	var totalCallLength time.Duration
	memberCallCounts := make(map[string]int)

	for _, call := range calls {
		if call.Status == "answered" {
			stats.AnsweredCalls++
			if call.AnsweredAt != nil {
				ringTime := call.AnsweredAt.Sub(call.CreatedAt)
				totalRingTime += ringTime
			}
			if call.AnsweredBy != "" {
				memberCallCounts[call.AnsweredBy]++
			}
		} else {
			stats.MissedCalls++
		}

		if stats.LastCallTime == nil || call.CreatedAt.After(*stats.LastCallTime) {
			stats.LastCallTime = &call.CreatedAt
		}

		if call.Duration > 0 {
			totalCallLength += time.Duration(call.Duration) * time.Second
		}
	}

	// Calculate averages
	if stats.AnsweredCalls > 0 {
		stats.AverageRingTime = totalRingTime / time.Duration(stats.AnsweredCalls)
		stats.AverageCallLength = totalCallLength / time.Duration(stats.AnsweredCalls)
	}

	// Find busiest member
	maxCalls := 0
	for member, count := range memberCallCounts {
		if count > maxCalls {
			maxCalls = count
			stats.BusiestMember = member
		}
	}

	return stats, nil
}

// EnableGroup enables a hunt group
func (m *DatabaseManager) EnableGroup(groupID int) error {
	group, err := m.GetGroup(groupID)
	if err != nil {
		return fmt.Errorf("failed to get hunt group: %w", err)
	}

	group.Enabled = true
	if err := m.UpdateGroup(group); err != nil {
		return fmt.Errorf("failed to enable hunt group: %w", err)
	}

	return nil
}

// DisableGroup disables a hunt group
func (m *DatabaseManager) DisableGroup(groupID int) error {
	group, err := m.GetGroup(groupID)
	if err != nil {
		return fmt.Errorf("failed to get hunt group: %w", err)
	}

	group.Enabled = false
	if err := m.UpdateGroup(group); err != nil {
		return fmt.Errorf("failed to disable hunt group: %w", err)
	}

	return nil
}

// EnableMember enables a hunt group member
func (m *DatabaseManager) EnableMember(groupID int, memberID int) error {
	group, err := m.GetGroup(groupID)
	if err != nil {
		return fmt.Errorf("failed to get hunt group: %w", err)
	}

	for _, member := range group.Members {
		if member.ID == memberID {
			member.Enabled = true
			member.UpdatedAt = time.Now().UTC()
			break
		}
	}

	if err := m.UpdateGroup(group); err != nil {
		return fmt.Errorf("failed to enable hunt group member: %w", err)
	}

	return nil
}

// DisableMember disables a hunt group member
func (m *DatabaseManager) DisableMember(groupID int, memberID int) error {
	group, err := m.GetGroup(groupID)
	if err != nil {
		return fmt.Errorf("failed to get hunt group: %w", err)
	}

	for _, member := range group.Members {
		if member.ID == memberID {
			member.Enabled = false
			member.UpdatedAt = time.Now().UTC()
			break
		}
	}

	if err := m.UpdateGroup(group); err != nil {
		return fmt.Errorf("failed to disable hunt group member: %w", err)
	}

	return nil
}

// Helper methods for validation and conversion

func (m *DatabaseManager) validateHuntGroup(group *HuntGroup) error {
	if group.Name == "" {
		return fmt.Errorf("hunt group name cannot be empty")
	}

	if group.Extension == "" {
		return fmt.Errorf("hunt group extension cannot be empty")
	}

	if group.RingTimeout <= 0 {
		return fmt.Errorf("hunt group ring timeout must be positive")
	}

	if group.RingTimeout > 300 {
		return fmt.Errorf("hunt group ring timeout cannot exceed 300 seconds")
	}

	return nil
}

func (m *DatabaseManager) validateHuntGroupMember(member *HuntGroupMember) error {
	if member.Extension == "" {
		return fmt.Errorf("hunt group member extension cannot be empty")
	}

	if member.Priority < 0 {
		return fmt.Errorf("hunt group member priority cannot be negative")
	}

	if member.Timeout <= 0 {
		return fmt.Errorf("hunt group member timeout must be positive")
	}

	if member.Timeout > 120 {
		return fmt.Errorf("hunt group member timeout cannot exceed 120 seconds")
	}

	return nil
}

func (m *DatabaseManager) validateCallSession(session *CallSession) error {
	if session.ID == "" {
		return fmt.Errorf("call session ID cannot be empty")
	}

	if session.GroupID <= 0 {
		return fmt.Errorf("call session group ID must be positive")
	}

	if session.CallerURI == "" {
		return fmt.Errorf("call session caller URI cannot be empty")
	}

	return nil
}

func (m *DatabaseManager) toDatabase(group *HuntGroup) *database.HuntGroup {
	// Convert members to database format
	dbMembers := make([]database.HuntGroupMember, len(group.Members))
	for i, member := range group.Members {
		dbMembers[i] = database.HuntGroupMember{
			ID:          strconv.Itoa(member.ID),
			URI:         member.Extension,
			DisplayName: "", // Not available in current structure
			Priority:    member.Priority,
			Timeout:     member.Timeout,
			Enabled:     member.Enabled,
		}
	}

	return &database.HuntGroup{
		ID:          strconv.Itoa(group.ID),
		Name:        group.Name,
		Extension:   group.Extension,
		Description: group.Description,
		Strategy:    string(group.Strategy),
		Timeout:     group.RingTimeout,
		Enabled:     group.Enabled,
		Members:     dbMembers,
		CreatedAt:   group.CreatedAt,
		UpdatedAt:   group.UpdatedAt,
	}
}

func (m *DatabaseManager) fromDatabase(dbHG *database.HuntGroup) (*HuntGroup, error) {
	id, err := strconv.Atoi(dbHG.ID)
	if err != nil {
		return nil, fmt.Errorf("invalid hunt group ID: %w", err)
	}

	// Convert members from database format
	members := make([]*HuntGroupMember, len(dbHG.Members))
	for i, dbMember := range dbHG.Members {
		memberID, err := strconv.Atoi(dbMember.ID)
		if err != nil {
			return nil, fmt.Errorf("invalid member ID: %w", err)
		}

		members[i] = &HuntGroupMember{
			ID:        memberID,
			GroupID:   id,
			Extension: dbMember.URI, // Using URI as extension
			Priority:  dbMember.Priority,
			Enabled:   dbMember.Enabled,
			Timeout:   dbMember.Timeout,
			CreatedAt: time.Now().UTC(), // Default timestamp
			UpdatedAt: time.Now().UTC(), // Default timestamp
		}
	}

	return &HuntGroup{
		ID:          id,
		Name:        dbHG.Name,
		Extension:   dbHG.Extension,
		Strategy:    HuntGroupStrategy(dbHG.Strategy),
		RingTimeout: dbHG.Timeout,
		Enabled:     dbHG.Enabled,
		Description: dbHG.Description,
		CreatedAt:   dbHG.CreatedAt,
		UpdatedAt:   dbHG.UpdatedAt,
		Members:     members,
	}, nil
}

// generateMemberID generates a unique ID for a new member within a hunt group
func (m *DatabaseManager) generateMemberID(existingMembers []*HuntGroupMember) int {
	maxID := 0
	for _, member := range existingMembers {
		if member.ID > maxID {
			maxID = member.ID
		}
	}
	return maxID + 1
}