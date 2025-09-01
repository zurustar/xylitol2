package webadmin

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/zurustar/xylitol2/internal/huntgroup"
)

// WebHuntGroupHandler handles HTTP requests for hunt group management
type WebHuntGroupHandler struct {
	huntGroupManager huntgroup.HuntGroupManager
	huntGroupEngine  huntgroup.HuntGroupEngine
}

// HandleHuntGroups handles hunt group listing and creation
func (h *WebHuntGroupHandler) HandleHuntGroups(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.handleListHuntGroups(w, r)
	case http.MethodPost:
		h.handleCreateHuntGroup(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// HandleHuntGroupByID handles individual hunt group operations
func (h *WebHuntGroupHandler) HandleHuntGroupByID(w http.ResponseWriter, r *http.Request) {
	// Extract ID from URL path
	path := strings.TrimPrefix(r.URL.Path, "/admin/huntgroups/")
	parts := strings.Split(path, "/")
	if len(parts) == 0 || parts[0] == "" {
		http.Error(w, "Hunt group ID required", http.StatusBadRequest)
		return
	}

	id, err := strconv.Atoi(parts[0])
	if err != nil {
		http.Error(w, "Invalid hunt group ID", http.StatusBadRequest)
		return
	}

	switch r.Method {
	case http.MethodGet:
		h.handleGetHuntGroup(w, r, id)
	case http.MethodPut:
		h.handleUpdateHuntGroup(w, r, id)
	case http.MethodDelete:
		h.handleDeleteHuntGroup(w, r, id)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// HandleNewHuntGroupPage handles the new hunt group form page
func (h *WebHuntGroupHandler) HandleNewHuntGroupPage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	html := `<!DOCTYPE html>
<html>
<head>
    <title>New Hunt Group - SIP Server Admin</title>
    <link rel="stylesheet" href="/static/css/admin.css">
    <style>
        .form-group { margin-bottom: 15px; }
        .form-group label { display: block; margin-bottom: 5px; font-weight: bold; }
        .form-group input, .form-group select, .form-group textarea { 
            width: 100%; padding: 8px; border: 1px solid #ddd; border-radius: 4px; 
        }
        .form-group textarea { height: 80px; resize: vertical; }
        .button { 
            background: #007cba; color: white; padding: 10px 20px; 
            border: none; border-radius: 4px; cursor: pointer; text-decoration: none;
            display: inline-block; margin-right: 10px;
        }
        .button:hover { background: #005a87; }
        .button.secondary { background: #6c757d; }
        .button.secondary:hover { background: #545b62; }
    </style>
</head>
<body>
    <div class="container">
        <h1>Create New Hunt Group</h1>
        <form method="POST" action="/admin/huntgroups">
            <div class="form-group">
                <label for="name">Name:</label>
                <input type="text" id="name" name="name" required placeholder="Sales Team">
            </div>
            <div class="form-group">
                <label for="extension">Extension:</label>
                <input type="text" id="extension" name="extension" required placeholder="100">
            </div>
            <div class="form-group">
                <label for="strategy">Strategy:</label>
                <select id="strategy" name="strategy" required>
                    <option value="simultaneous">Simultaneous (Ring All)</option>
                    <option value="sequential">Sequential (One by One)</option>
                    <option value="round_robin">Round Robin</option>
                    <option value="longest_idle">Longest Idle</option>
                </select>
            </div>
            <div class="form-group">
                <label for="ring_timeout">Ring Timeout (seconds):</label>
                <input type="number" id="ring_timeout" name="ring_timeout" value="30" min="5" max="300" required>
            </div>
            <div class="form-group">
                <label for="enabled">Enabled:</label>
                <input type="checkbox" id="enabled" name="enabled" checked>
            </div>
            <div class="form-group">
                <label for="description">Description:</label>
                <textarea id="description" name="description" placeholder="Optional description"></textarea>
            </div>
            <div class="form-group">
                <button type="submit" class="button">Create Hunt Group</button>
                <a href="/admin/huntgroups" class="button secondary">Cancel</a>
            </div>
        </form>
    </div>
</body>
</html>`

	w.Header().Set("Content-Type", "text/html")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(html))
}

// HandleEditHuntGroupPage handles the edit hunt group form page
func (h *WebHuntGroupHandler) HandleEditHuntGroupPage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract ID from URL
	path := strings.TrimPrefix(r.URL.Path, "/admin/huntgroups/edit/")
	id, err := strconv.Atoi(path)
	if err != nil {
		http.Error(w, "Invalid hunt group ID", http.StatusBadRequest)
		return
	}

	// Get hunt group
	group, err := h.huntGroupManager.GetGroup(id)
	if err != nil {
		http.Error(w, "Hunt group not found", http.StatusNotFound)
		return
	}

	enabledChecked := ""
	if group.Enabled {
		enabledChecked = "checked"
	}

	html := fmt.Sprintf(`<!DOCTYPE html>
<html>
<head>
    <title>Edit Hunt Group - SIP Server Admin</title>
    <link rel="stylesheet" href="/static/css/admin.css">
    <style>
        .form-group { margin-bottom: 15px; }
        .form-group label { display: block; margin-bottom: 5px; font-weight: bold; }
        .form-group input, .form-group select, .form-group textarea { 
            width: 100%%; padding: 8px; border: 1px solid #ddd; border-radius: 4px; 
        }
        .form-group textarea { height: 80px; resize: vertical; }
        .button { 
            background: #007cba; color: white; padding: 10px 20px; 
            border: none; border-radius: 4px; cursor: pointer; text-decoration: none;
            display: inline-block; margin-right: 10px;
        }
        .button:hover { background: #005a87; }
        .button.secondary { background: #6c757d; }
        .button.secondary:hover { background: #545b62; }
        .members-section { margin-top: 30px; padding-top: 20px; border-top: 1px solid #ddd; }
        .member-item { 
            display: flex; justify-content: space-between; align-items: center; 
            padding: 10px; border: 1px solid #ddd; margin-bottom: 5px; border-radius: 4px;
        }
    </style>
</head>
<body>
    <div class="container">
        <h1>Edit Hunt Group: %s</h1>
        <form method="POST" action="/admin/huntgroups/%d">
            <input type="hidden" name="_method" value="PUT">
            <div class="form-group">
                <label for="name">Name:</label>
                <input type="text" id="name" name="name" value="%s" required>
            </div>
            <div class="form-group">
                <label for="extension">Extension:</label>
                <input type="text" id="extension" name="extension" value="%s" required>
            </div>
            <div class="form-group">
                <label for="strategy">Strategy:</label>
                <select id="strategy" name="strategy" required>
                    <option value="simultaneous" %s>Simultaneous (Ring All)</option>
                    <option value="sequential" %s>Sequential (One by One)</option>
                    <option value="round_robin" %s>Round Robin</option>
                    <option value="longest_idle" %s>Longest Idle</option>
                </select>
            </div>
            <div class="form-group">
                <label for="ring_timeout">Ring Timeout (seconds):</label>
                <input type="number" id="ring_timeout" name="ring_timeout" value="%d" min="5" max="300" required>
            </div>
            <div class="form-group">
                <label for="enabled">Enabled:</label>
                <input type="checkbox" id="enabled" name="enabled" %s>
            </div>
            <div class="form-group">
                <label for="description">Description:</label>
                <textarea id="description" name="description">%s</textarea>
            </div>
            <div class="form-group">
                <button type="submit" class="button">Update Hunt Group</button>
                <a href="/admin/huntgroups" class="button secondary">Cancel</a>
            </div>
        </form>

        <div class="members-section">
            <h2>Members</h2>
            <div class="actions">
                <button onclick="showAddMemberForm()" class="button">Add Member</button>
            </div>
            <div id="members-list">`,
		group.Name, group.ID, group.Name, group.Extension,
		h.getSelectedOption(string(group.Strategy), "simultaneous"),
		h.getSelectedOption(string(group.Strategy), "sequential"),
		h.getSelectedOption(string(group.Strategy), "round_robin"),
		h.getSelectedOption(string(group.Strategy), "longest_idle"),
		group.RingTimeout, enabledChecked, group.Description)

	// Add members
	for _, member := range group.Members {
		enabledText := "Disabled"
		if member.Enabled {
			enabledText = "Enabled"
		}
		html += fmt.Sprintf(`
                <div class="member-item">
                    <span>%s (Priority: %d, %s)</span>
                    <button onclick="removeMember(%d)" class="button secondary">Remove</button>
                </div>`, member.Extension, member.Priority, enabledText, member.ID)
	}

	html += `
            </div>
        </div>

        <!-- Add Member Form (hidden by default) -->
        <div id="add-member-form" style="display: none; margin-top: 20px; padding: 20px; border: 1px solid #ddd; border-radius: 4px;">
            <h3>Add Member</h3>
            <form method="POST" action="/admin/huntgroups/` + strconv.Itoa(group.ID) + `/members">
                <div class="form-group">
                    <label for="member_extension">Extension:</label>
                    <input type="text" id="member_extension" name="extension" required>
                </div>
                <div class="form-group">
                    <label for="member_priority">Priority:</label>
                    <input type="number" id="member_priority" name="priority" value="0" min="0">
                </div>
                <div class="form-group">
                    <label for="member_timeout">Timeout (seconds, optional):</label>
                    <input type="number" id="member_timeout" name="timeout" min="5" max="300">
                </div>
                <div class="form-group">
                    <label for="member_enabled">Enabled:</label>
                    <input type="checkbox" id="member_enabled" name="enabled" checked>
                </div>
                <div class="form-group">
                    <button type="submit" class="button">Add Member</button>
                    <button type="button" onclick="hideAddMemberForm()" class="button secondary">Cancel</button>
                </div>
            </form>
        </div>
    </div>

    <script>
        function showAddMemberForm() {
            document.getElementById('add-member-form').style.display = 'block';
        }
        
        function hideAddMemberForm() {
            document.getElementById('add-member-form').style.display = 'none';
        }
        
        function removeMember(memberId) {
            if (confirm('Are you sure you want to remove this member?')) {
                fetch('/admin/huntgroups/` + strconv.Itoa(group.ID) + `/members/' + memberId, {
                    method: 'DELETE'
                }).then(response => {
                    if (response.ok) {
                        location.reload();
                    } else {
                        alert('Failed to remove member');
                    }
                });
            }
        }
    </script>
</body>
</html>`

	w.Header().Set("Content-Type", "text/html")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(html))
}

// HandleHuntGroupMembers handles hunt group member operations
func (h *WebHuntGroupHandler) HandleHuntGroupMembers(w http.ResponseWriter, r *http.Request) {
	// Extract group ID from URL
	path := strings.TrimPrefix(r.URL.Path, "/admin/huntgroups/")
	parts := strings.Split(path, "/")
	if len(parts) < 2 || parts[1] != "members" {
		http.Error(w, "Invalid URL", http.StatusBadRequest)
		return
	}

	groupID, err := strconv.Atoi(parts[0])
	if err != nil {
		http.Error(w, "Invalid hunt group ID", http.StatusBadRequest)
		return
	}

	switch r.Method {
	case http.MethodPost:
		h.handleAddMember(w, r, groupID)
	case http.MethodDelete:
		if len(parts) >= 3 {
			memberID, err := strconv.Atoi(parts[2])
			if err != nil {
				http.Error(w, "Invalid member ID", http.StatusBadRequest)
				return
			}
			h.handleRemoveMember(w, r, groupID, memberID)
		} else {
			http.Error(w, "Member ID required", http.StatusBadRequest)
		}
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// HandleHuntGroupStatistics handles hunt group statistics
func (h *WebHuntGroupHandler) HandleHuntGroupStatistics(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract group ID from URL
	path := strings.TrimPrefix(r.URL.Path, "/admin/huntgroups/")
	parts := strings.Split(path, "/")
	if len(parts) < 2 || parts[1] != "statistics" {
		http.Error(w, "Invalid URL", http.StatusBadRequest)
		return
	}

	groupID, err := strconv.Atoi(parts[0])
	if err != nil {
		http.Error(w, "Invalid hunt group ID", http.StatusBadRequest)
		return
	}

	stats, err := h.huntGroupEngine.GetCallStatistics(groupID)
	if err != nil {
		http.Error(w, "Failed to get statistics", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stats)
}

// Private handler methods

func (h *WebHuntGroupHandler) handleListHuntGroups(w http.ResponseWriter, r *http.Request) {
	groups, err := h.huntGroupManager.ListGroups()
	if err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	html := `<!DOCTYPE html>
<html>
<head>
    <title>Hunt Groups - SIP Server Admin</title>
    <link rel="stylesheet" href="/static/css/admin.css">
    <style>
        .actions { margin-bottom: 20px; }
        .button { 
            background: #007cba; color: white; padding: 10px 20px; 
            border: none; border-radius: 4px; cursor: pointer; text-decoration: none;
            display: inline-block; margin-right: 10px;
        }
        .button:hover { background: #005a87; }
        .button.danger { background: #dc3545; }
        .button.danger:hover { background: #c82333; }
        table { width: 100%; border-collapse: collapse; margin-top: 20px; }
        th, td { padding: 12px; text-align: left; border-bottom: 1px solid #ddd; }
        th { background-color: #f8f9fa; font-weight: bold; }
        tr:hover { background-color: #f5f5f5; }
        .status-enabled { color: #28a745; font-weight: bold; }
        .status-disabled { color: #dc3545; font-weight: bold; }
        .member-count { background: #e9ecef; padding: 2px 8px; border-radius: 12px; font-size: 0.9em; }
    </style>
</head>
<body>
    <div class="container">
        <h1>Hunt Groups</h1>
        <div class="actions">
            <a href="/admin/huntgroups/new" class="button">Add New Hunt Group</a>
            <a href="/admin" class="button" style="background: #6c757d;">Back to Dashboard</a>
        </div>
        <table>
            <thead>
                <tr>
                    <th>Name</th>
                    <th>Extension</th>
                    <th>Strategy</th>
                    <th>Ring Timeout</th>
                    <th>Members</th>
                    <th>Status</th>
                    <th>Created</th>
                    <th>Actions</th>
                </tr>
            </thead>
            <tbody>`

	for _, group := range groups {
		status := `<span class="status-disabled">Disabled</span>`
		if group.Enabled {
			status = `<span class="status-enabled">Enabled</span>`
		}

		memberCount := len(group.Members)
		enabledMembers := 0
		for _, member := range group.Members {
			if member.Enabled {
				enabledMembers++
			}
		}

		html += fmt.Sprintf(`
                <tr>
                    <td><strong>%s</strong><br><small>%s</small></td>
                    <td>%s</td>
                    <td>%s</td>
                    <td>%d sec</td>
                    <td><span class="member-count">%d total (%d enabled)</span></td>
                    <td>%s</td>
                    <td>%s</td>
                    <td>
                        <a href="/admin/huntgroups/edit/%d" class="button" style="padding: 5px 10px; font-size: 0.9em;">Edit</a>
                        <button onclick="deleteHuntGroup(%d)" class="button danger" style="padding: 5px 10px; font-size: 0.9em;">Delete</button>
                        <button onclick="viewStatistics(%d)" class="button" style="padding: 5px 10px; font-size: 0.9em; background: #17a2b8;">Stats</button>
                    </td>
                </tr>`,
			group.Name, group.Description, group.Extension, string(group.Strategy),
			group.RingTimeout, memberCount, enabledMembers, status,
			group.CreatedAt.Format("2006-01-02 15:04"), group.ID, group.ID, group.ID)
	}

	html += `
            </tbody>
        </table>
    </div>

    <script>
        function deleteHuntGroup(id) {
            if (confirm('Are you sure you want to delete this hunt group? This action cannot be undone.')) {
                fetch('/admin/huntgroups/' + id, {
                    method: 'DELETE'
                }).then(response => {
                    if (response.ok) {
                        location.reload();
                    } else {
                        alert('Failed to delete hunt group');
                    }
                });
            }
        }

        function viewStatistics(id) {
            fetch('/admin/huntgroups/' + id + '/statistics')
                .then(response => response.json())
                .then(stats => {
                    let message = 'Hunt Group Statistics:\\n\\n';
                    message += 'Total Calls: ' + stats.total_calls + '\\n';
                    message += 'Answered Calls: ' + stats.answered_calls + '\\n';
                    message += 'Missed Calls: ' + stats.missed_calls + '\\n';
                    if (stats.average_ring_time) {
                        message += 'Average Ring Time: ' + Math.round(stats.average_ring_time / 1000000000) + ' seconds\\n';
                    }
                    if (stats.busiest_member) {
                        message += 'Busiest Member: ' + stats.busiest_member + '\\n';
                    }
                    alert(message);
                })
                .catch(error => {
                    alert('Failed to load statistics');
                });
        }
    </script>
</body>
</html>`

	w.Header().Set("Content-Type", "text/html")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(html))
}

func (h *WebHuntGroupHandler) handleCreateHuntGroup(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid form data", http.StatusBadRequest)
		return
	}

	name := r.FormValue("name")
	extension := r.FormValue("extension")
	strategy := r.FormValue("strategy")
	ringTimeoutStr := r.FormValue("ring_timeout")
	enabled := r.FormValue("enabled") == "on"
	description := r.FormValue("description")

	if name == "" || extension == "" || strategy == "" || ringTimeoutStr == "" {
		http.Error(w, "Missing required fields", http.StatusBadRequest)
		return
	}

	ringTimeout, err := strconv.Atoi(ringTimeoutStr)
	if err != nil {
		http.Error(w, "Invalid ring timeout", http.StatusBadRequest)
		return
	}

	group := &huntgroup.HuntGroup{
		Name:        name,
		Extension:   extension,
		Strategy:    huntgroup.HuntGroupStrategy(strategy),
		RingTimeout: ringTimeout,
		Enabled:     enabled,
		Description: description,
	}

	err = h.huntGroupManager.CreateGroup(group)
	if err != nil {
		http.Error(w, "Failed to create hunt group", http.StatusInternalServerError)
		return
	}

	// Redirect to hunt groups list
	http.Redirect(w, r, "/admin/huntgroups", http.StatusSeeOther)
}

func (h *WebHuntGroupHandler) handleGetHuntGroup(w http.ResponseWriter, r *http.Request, id int) {
	group, err := h.huntGroupManager.GetGroup(id)
	if err != nil {
		http.Error(w, "Hunt group not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(group)
}

func (h *WebHuntGroupHandler) handleUpdateHuntGroup(w http.ResponseWriter, r *http.Request, id int) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid form data", http.StatusBadRequest)
		return
	}

	group, err := h.huntGroupManager.GetGroup(id)
	if err != nil {
		http.Error(w, "Hunt group not found", http.StatusNotFound)
		return
	}

	// Update fields
	if name := r.FormValue("name"); name != "" {
		group.Name = name
	}
	if extension := r.FormValue("extension"); extension != "" {
		group.Extension = extension
	}
	if strategy := r.FormValue("strategy"); strategy != "" {
		group.Strategy = huntgroup.HuntGroupStrategy(strategy)
	}
	if ringTimeoutStr := r.FormValue("ring_timeout"); ringTimeoutStr != "" {
		if ringTimeout, err := strconv.Atoi(ringTimeoutStr); err == nil {
			group.RingTimeout = ringTimeout
		}
	}
	group.Enabled = r.FormValue("enabled") == "on"
	group.Description = r.FormValue("description")

	err = h.huntGroupManager.UpdateGroup(group)
	if err != nil {
		http.Error(w, "Failed to update hunt group", http.StatusInternalServerError)
		return
	}

	// Redirect to hunt groups list
	http.Redirect(w, r, "/admin/huntgroups", http.StatusSeeOther)
}

func (h *WebHuntGroupHandler) handleDeleteHuntGroup(w http.ResponseWriter, r *http.Request, id int) {
	err := h.huntGroupManager.DeleteGroup(id)
	if err != nil {
		http.Error(w, "Failed to delete hunt group", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}

func (h *WebHuntGroupHandler) handleAddMember(w http.ResponseWriter, r *http.Request, groupID int) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid form data", http.StatusBadRequest)
		return
	}

	extension := r.FormValue("extension")
	priorityStr := r.FormValue("priority")
	timeoutStr := r.FormValue("timeout")
	enabled := r.FormValue("enabled") == "on"

	if extension == "" {
		http.Error(w, "Extension is required", http.StatusBadRequest)
		return
	}

	priority := 0
	if priorityStr != "" {
		if p, err := strconv.Atoi(priorityStr); err == nil {
			priority = p
		}
	}

	var timeout int
	if timeoutStr != "" {
		if t, err := strconv.Atoi(timeoutStr); err == nil {
			timeout = t
		}
	}

	member := &huntgroup.HuntGroupMember{
		Extension: extension,
		Priority:  priority,
		Enabled:   enabled,
		Timeout:   timeout,
	}

	err := h.huntGroupManager.AddMember(groupID, member)
	if err != nil {
		http.Error(w, "Failed to add member", http.StatusInternalServerError)
		return
	}

	// Redirect back to edit page
	http.Redirect(w, r, fmt.Sprintf("/admin/huntgroups/edit/%d", groupID), http.StatusSeeOther)
}

func (h *WebHuntGroupHandler) handleRemoveMember(w http.ResponseWriter, r *http.Request, groupID int, memberID int) {
	err := h.huntGroupManager.RemoveMember(groupID, memberID)
	if err != nil {
		http.Error(w, "Failed to remove member", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}

// Helper methods

func (h *WebHuntGroupHandler) getSelectedOption(current, option string) string {
	if current == option {
		return "selected"
	}
	return ""
}