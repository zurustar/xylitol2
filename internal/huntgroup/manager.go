package huntgroup

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/zurustar/xylitol2/internal/database"
	"github.com/zurustar/xylitol2/internal/logging"
)

// Manager implements the HuntGroupManager interface
type Manager struct {
	db     database.DatabaseManager
	logger logging.Logger
}

// NewManager creates a new hunt group manager
func NewManager(db database.DatabaseManager, logger logging.Logger) *Manager {
	return &Manager{
		db:     db,
		logger: logger,
	}
}

// InitializeTables creates the necessary database tables for hunt groups
func (m *Manager) InitializeTables() error {
	queries := []string{
		`CREATE TABLE IF NOT EXISTS hunt_groups (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL UNIQUE,
			extension TEXT NOT NULL UNIQUE,
			strategy TEXT NOT NULL DEFAULT 'simultaneous',
			ring_timeout INTEGER NOT NULL DEFAULT 30,
			enabled BOOLEAN NOT NULL DEFAULT 1,
			description TEXT,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS hunt_group_members (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			group_id INTEGER NOT NULL,
			extension TEXT NOT NULL,
			priority INTEGER NOT NULL DEFAULT 0,
			enabled BOOLEAN NOT NULL DEFAULT 1,
			timeout INTEGER,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (group_id) REFERENCES hunt_groups(id) ON DELETE CASCADE,
			UNIQUE(group_id, extension)
		)`,
		`CREATE TABLE IF NOT EXISTS hunt_group_call_logs (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			group_id INTEGER NOT NULL,
			session_id TEXT NOT NULL,
			caller_uri TEXT NOT NULL,
			answered_by TEXT,
			start_time DATETIME NOT NULL,
			answer_time DATETIME,
			end_time DATETIME,
			status TEXT NOT NULL,
			ring_duration INTEGER,
			call_duration INTEGER,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (group_id) REFERENCES hunt_groups(id) ON DELETE CASCADE
		)`,
		`CREATE INDEX IF NOT EXISTS idx_hunt_groups_extension ON hunt_groups(extension)`,
		`CREATE INDEX IF NOT EXISTS idx_hunt_group_members_group_id ON hunt_group_members(group_id)`,
		`CREATE INDEX IF NOT EXISTS idx_hunt_group_members_priority ON hunt_group_members(group_id, priority)`,
		`CREATE INDEX IF NOT EXISTS idx_hunt_group_call_logs_group_id ON hunt_group_call_logs(group_id)`,
		`CREATE INDEX IF NOT EXISTS idx_hunt_group_call_logs_start_time ON hunt_group_call_logs(start_time)`,
	}

	for _, query := range queries {
		if err := m.db.Exec(query); err != nil {
			return fmt.Errorf("failed to create hunt group table: %w", err)
		}
	}

	m.logger.Info("Hunt group database tables initialized")
	return nil
}

// CreateGroup creates a new hunt group
func (m *Manager) CreateGroup(group *HuntGroup) error {
	if group == nil {
		return fmt.Errorf("group cannot be nil")
	}

	if group.Name == "" || group.Extension == "" {
		return fmt.Errorf("group name and extension are required")
	}

	now := time.Now().UTC()
	group.CreatedAt = now
	group.UpdatedAt = now

	query := `INSERT INTO hunt_groups (name, extension, strategy, ring_timeout, enabled, description, created_at, updated_at)
			  VALUES (?, ?, ?, ?, ?, ?, ?, ?)`

	result, err := m.db.ExecWithResult(query, 
		group.Name, group.Extension, string(group.Strategy), group.RingTimeout, 
		group.Enabled, group.Description, group.CreatedAt, group.UpdatedAt)
	if err != nil {
		return fmt.Errorf("failed to create hunt group: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("failed to get hunt group ID: %w", err)
	}

	group.ID = int(id)

	m.logger.Info("Hunt group created", 
		logging.Field{Key: "id", Value: group.ID},
		logging.Field{Key: "name", Value: group.Name},
		logging.Field{Key: "extension", Value: group.Extension})

	return nil
}

// GetGroup retrieves a hunt group by ID
func (m *Manager) GetGroup(id int) (*HuntGroup, error) {
	query := `SELECT id, name, extension, strategy, ring_timeout, enabled, description, created_at, updated_at
			  FROM hunt_groups WHERE id = ?`

	var group HuntGroup
	dest := []interface{}{&group.ID, &group.Name, &group.Extension, 
		&group.Strategy, &group.RingTimeout, &group.Enabled, &group.Description,
		&group.CreatedAt, &group.UpdatedAt}
	err := m.db.QueryRow(query, dest, id)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("hunt group not found")
		}
		return nil, fmt.Errorf("failed to get hunt group: %w", err)
	}

	// Load members
	members, err := m.GetGroupMembers(id)
	if err != nil {
		return nil, fmt.Errorf("failed to load group members: %w", err)
	}
	group.Members = members

	return &group, nil
}

// GetGroupByExtension retrieves a hunt group by extension
func (m *Manager) GetGroupByExtension(extension string) (*HuntGroup, error) {
	query := `SELECT id, name, extension, strategy, ring_timeout, enabled, description, created_at, updated_at
			  FROM hunt_groups WHERE extension = ? AND enabled = 1`

	var group HuntGroup
	dest := []interface{}{&group.ID, &group.Name, &group.Extension,
		&group.Strategy, &group.RingTimeout, &group.Enabled, &group.Description,
		&group.CreatedAt, &group.UpdatedAt}
	err := m.db.QueryRow(query, dest, extension)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("hunt group not found")
		}
		return nil, fmt.Errorf("failed to get hunt group: %w", err)
	}

	// Load members
	members, err := m.GetGroupMembers(group.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to load group members: %w", err)
	}
	group.Members = members

	return &group, nil
}

// UpdateGroup updates an existing hunt group
func (m *Manager) UpdateGroup(group *HuntGroup) error {
	if group == nil {
		return fmt.Errorf("group cannot be nil")
	}

	group.UpdatedAt = time.Now().UTC()

	query := `UPDATE hunt_groups 
			  SET name = ?, extension = ?, strategy = ?, ring_timeout = ?, enabled = ?, description = ?, updated_at = ?
			  WHERE id = ?`

	err := m.db.Exec(query, group.Name, group.Extension, string(group.Strategy), 
		group.RingTimeout, group.Enabled, group.Description, group.UpdatedAt, group.ID)
	if err != nil {
		return fmt.Errorf("failed to update hunt group: %w", err)
	}

	m.logger.Info("Hunt group updated",
		logging.Field{Key: "id", Value: group.ID},
		logging.Field{Key: "name", Value: group.Name})

	return nil
}

// DeleteGroup deletes a hunt group
func (m *Manager) DeleteGroup(id int) error {
	query := `DELETE FROM hunt_groups WHERE id = ?`

	err := m.db.Exec(query, id)
	if err != nil {
		return fmt.Errorf("failed to delete hunt group: %w", err)
	}

	m.logger.Info("Hunt group deleted", logging.Field{Key: "id", Value: id})
	return nil
}

// ListGroups retrieves all hunt groups
func (m *Manager) ListGroups() ([]*HuntGroup, error) {
	query := `SELECT id, name, extension, strategy, ring_timeout, enabled, description, created_at, updated_at
			  FROM hunt_groups ORDER BY name`

	rows, err := m.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to list hunt groups: %w", err)
	}
	defer rows.Close()

	var groups []*HuntGroup
	for rows.Next() {
		var group HuntGroup
		err := rows.Scan(&group.ID, &group.Name, &group.Extension, &group.Strategy,
			&group.RingTimeout, &group.Enabled, &group.Description,
			&group.CreatedAt, &group.UpdatedAt)
		if err != nil {
			return nil, fmt.Errorf("failed to scan hunt group: %w", err)
		}

		// Load members for each group
		members, err := m.GetGroupMembers(group.ID)
		if err != nil {
			m.logger.Warn("Failed to load members for group",
				logging.Field{Key: "group_id", Value: group.ID},
				logging.Field{Key: "error", Value: err})
			members = []*HuntGroupMember{} // Empty slice instead of nil
		}
		group.Members = members

		groups = append(groups, &group)
	}

	return groups, nil
}

// AddMember adds a member to a hunt group
func (m *Manager) AddMember(groupID int, member *HuntGroupMember) error {
	if member == nil {
		return fmt.Errorf("member cannot be nil")
	}

	if member.Extension == "" {
		return fmt.Errorf("member extension is required")
	}

	now := time.Now().UTC()
	member.GroupID = groupID
	member.CreatedAt = now
	member.UpdatedAt = now

	query := `INSERT INTO hunt_group_members (group_id, extension, priority, enabled, timeout, created_at, updated_at)
			  VALUES (?, ?, ?, ?, ?, ?, ?)`

	result, err := m.db.ExecWithResult(query, member.GroupID, member.Extension, 
		member.Priority, member.Enabled, member.Timeout, member.CreatedAt, member.UpdatedAt)
	if err != nil {
		return fmt.Errorf("failed to add hunt group member: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("failed to get member ID: %w", err)
	}

	member.ID = int(id)

	m.logger.Info("Hunt group member added",
		logging.Field{Key: "group_id", Value: groupID},
		logging.Field{Key: "member_id", Value: member.ID},
		logging.Field{Key: "extension", Value: member.Extension})

	return nil
}

// RemoveMember removes a member from a hunt group
func (m *Manager) RemoveMember(groupID int, memberID int) error {
	query := `DELETE FROM hunt_group_members WHERE id = ? AND group_id = ?`

	err := m.db.Exec(query, memberID, groupID)
	if err != nil {
		return fmt.Errorf("failed to remove hunt group member: %w", err)
	}

	m.logger.Info("Hunt group member removed",
		logging.Field{Key: "group_id", Value: groupID},
		logging.Field{Key: "member_id", Value: memberID})

	return nil
}

// UpdateMember updates a hunt group member
func (m *Manager) UpdateMember(member *HuntGroupMember) error {
	if member == nil {
		return fmt.Errorf("member cannot be nil")
	}

	member.UpdatedAt = time.Now().UTC()

	query := `UPDATE hunt_group_members 
			  SET extension = ?, priority = ?, enabled = ?, timeout = ?, updated_at = ?
			  WHERE id = ?`

	err := m.db.Exec(query, member.Extension, member.Priority, member.Enabled, 
		member.Timeout, member.UpdatedAt, member.ID)
	if err != nil {
		return fmt.Errorf("failed to update hunt group member: %w", err)
	}

	m.logger.Info("Hunt group member updated",
		logging.Field{Key: "member_id", Value: member.ID},
		logging.Field{Key: "extension", Value: member.Extension})

	return nil
}

// GetGroupMembers retrieves all members of a hunt group
func (m *Manager) GetGroupMembers(groupID int) ([]*HuntGroupMember, error) {
	query := `SELECT id, group_id, extension, priority, enabled, timeout, created_at, updated_at
			  FROM hunt_group_members WHERE group_id = ? ORDER BY priority ASC, extension ASC`

	rows, err := m.db.Query(query, groupID)
	if err != nil {
		return nil, fmt.Errorf("failed to get hunt group members: %w", err)
	}
	defer rows.Close()

	var members []*HuntGroupMember
	for rows.Next() {
		var member HuntGroupMember
		var timeout sql.NullInt64

		err := rows.Scan(&member.ID, &member.GroupID, &member.Extension, &member.Priority,
			&member.Enabled, &timeout, &member.CreatedAt, &member.UpdatedAt)
		if err != nil {
			return nil, fmt.Errorf("failed to scan hunt group member: %w", err)
		}

		if timeout.Valid {
			member.Timeout = int(timeout.Int64)
		}

		members = append(members, &member)
	}

	return members, nil
}

// CreateSession creates a new call session (for logging)
func (m *Manager) CreateSession(session *CallSession) error {
	if session == nil {
		return fmt.Errorf("session cannot be nil")
	}

	query := `INSERT INTO hunt_group_call_logs (group_id, session_id, caller_uri, start_time, status)
			  VALUES (?, ?, ?, ?, ?)`

	err := m.db.Exec(query, session.GroupID, session.ID, session.CallerURI, 
		session.StartTime, string(session.Status))
	if err != nil {
		return fmt.Errorf("failed to create call session log: %w", err)
	}

	return nil
}

// GetSession retrieves a call session (stub implementation)
func (m *Manager) GetSession(sessionID string) (*CallSession, error) {
	// This would typically be stored in memory or a cache
	// For now, return not found
	return nil, fmt.Errorf("session not found")
}

// UpdateSession updates a call session (for logging)
func (m *Manager) UpdateSession(session *CallSession) error {
	if session == nil {
		return fmt.Errorf("session cannot be nil")
	}

	query := `UPDATE hunt_group_call_logs 
			  SET answered_by = ?, answer_time = ?, end_time = ?, status = ?, 
			      ring_duration = ?, call_duration = ?
			  WHERE session_id = ?`

	var ringDuration, callDuration *int
	if session.AnsweredAt != nil {
		rd := int(session.AnsweredAt.Sub(session.StartTime).Seconds())
		ringDuration = &rd

		if session.Status == SessionStatusCompleted {
			cd := int(time.Now().Sub(*session.AnsweredAt).Seconds())
			callDuration = &cd
		}
	}

	err := m.db.Exec(query, session.AnsweredBy, session.AnsweredAt, 
		time.Now().UTC(), string(session.Status), ringDuration, callDuration, session.ID)
	if err != nil {
		return fmt.Errorf("failed to update call session log: %w", err)
	}

	return nil
}

// EndSession ends a call session
func (m *Manager) EndSession(sessionID string) error {
	query := `UPDATE hunt_group_call_logs 
			  SET end_time = ?, status = ?
			  WHERE session_id = ? AND end_time IS NULL`

	err := m.db.Exec(query, time.Now().UTC(), string(SessionStatusCompleted), sessionID)
	if err != nil {
		return fmt.Errorf("failed to end call session: %w", err)
	}

	return nil
}

// GetActiveSessions retrieves active call sessions (stub implementation)
func (m *Manager) GetActiveSessions() ([]*CallSession, error) {
	// This would typically be stored in memory or a cache
	// For now, return empty slice
	return []*CallSession{}, nil
}

// GetCallStatistics retrieves call statistics for a hunt group
func (m *Manager) GetCallStatistics(groupID int) (*CallStatistics, error) {
	stats := &CallStatistics{
		GroupID: groupID,
	}

	// Get total calls
	query := `SELECT COUNT(*) FROM hunt_group_call_logs WHERE group_id = ?`
	dest := []interface{}{&stats.TotalCalls}
	err := m.db.QueryRow(query, dest, groupID)
	if err != nil {
		return nil, fmt.Errorf("failed to get total calls: %w", err)
	}

	// Get answered calls
	query = `SELECT COUNT(*) FROM hunt_group_call_logs WHERE group_id = ? AND answered_by IS NOT NULL`
	dest = []interface{}{&stats.AnsweredCalls}
	err = m.db.QueryRow(query, dest, groupID)
	if err != nil {
		return nil, fmt.Errorf("failed to get answered calls: %w", err)
	}

	stats.MissedCalls = stats.TotalCalls - stats.AnsweredCalls

	// Get average ring time
	query = `SELECT AVG(ring_duration) FROM hunt_group_call_logs WHERE group_id = ? AND ring_duration IS NOT NULL`
	var avgRingSeconds sql.NullFloat64
	dest = []interface{}{&avgRingSeconds}
	err = m.db.QueryRow(query, dest, groupID)
	if err != nil {
		return nil, fmt.Errorf("failed to get average ring time: %w", err)
	}
	if avgRingSeconds.Valid {
		stats.AverageRingTime = time.Duration(avgRingSeconds.Float64) * time.Second
	}

	// Get average call length
	query = `SELECT AVG(call_duration) FROM hunt_group_call_logs WHERE group_id = ? AND call_duration IS NOT NULL`
	var avgCallSeconds sql.NullFloat64
	dest = []interface{}{&avgCallSeconds}
	err = m.db.QueryRow(query, dest, groupID)
	if err != nil {
		return nil, fmt.Errorf("failed to get average call length: %w", err)
	}
	if avgCallSeconds.Valid {
		stats.AverageCallLength = time.Duration(avgCallSeconds.Float64) * time.Second
	}

	// Get busiest member
	query = `SELECT answered_by, COUNT(*) as call_count 
			 FROM hunt_group_call_logs 
			 WHERE group_id = ? AND answered_by IS NOT NULL 
			 GROUP BY answered_by 
			 ORDER BY call_count DESC 
			 LIMIT 1`
	var callCount int
	dest = []interface{}{&stats.BusiestMember, &callCount}
	err = m.db.QueryRow(query, dest, groupID)
	if err != nil && err != sql.ErrNoRows {
		return nil, fmt.Errorf("failed to get busiest member: %w", err)
	}

	// Get last call time
	query = `SELECT MAX(start_time) FROM hunt_group_call_logs WHERE group_id = ?`
	var lastCallTime sql.NullTime
	dest = []interface{}{&lastCallTime}
	err = m.db.QueryRow(query, dest, groupID)
	if err != nil {
		return nil, fmt.Errorf("failed to get last call time: %w", err)
	}
	if lastCallTime.Valid {
		stats.LastCallTime = &lastCallTime.Time
	}

	return stats, nil
}