package webadmin

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/zurustar/xylitol2/internal/database"
	"github.com/zurustar/xylitol2/internal/huntgroup"
	"github.com/zurustar/xylitol2/internal/logging"
)

// Server implements the WebAdminServer interface
type Server struct {
	userManager      database.UserManager
	huntGroupManager huntgroup.HuntGroupManager
	huntGroupEngine  huntgroup.HuntGroupEngine
	logger           logging.Logger
	server           *http.Server
	userHandler      *WebUserHandler
	huntGroupHandler *WebHuntGroupHandler
}

// NewServer creates a new web admin server
func NewServer(userManager database.UserManager, huntGroupManager huntgroup.HuntGroupManager, huntGroupEngine huntgroup.HuntGroupEngine, logger logging.Logger) *Server {
	userHandler := &WebUserHandler{
		userManager: userManager,
	}

	huntGroupHandler := &WebHuntGroupHandler{
		huntGroupManager: huntGroupManager,
		huntGroupEngine:  huntGroupEngine,
	}

	return &Server{
		userManager:      userManager,
		huntGroupManager: huntGroupManager,
		huntGroupEngine:  huntGroupEngine,
		logger:           logger,
		userHandler:      userHandler,
		huntGroupHandler: huntGroupHandler,
	}
}

// Start starts the web admin server on the specified port
func (s *Server) Start(port int) error {
	mux := http.NewServeMux()
	s.registerRoutesOnMux(mux)

	s.server = &http.Server{
		Addr:         fmt.Sprintf(":%d", port),
		Handler:      mux,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	s.logger.Info("Starting web admin server", logging.Field{Key: "port", Value: port})

	go func() {
		if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			s.logger.Error("Web admin server error", logging.Field{Key: "error", Value: err})
		}
	}()

	return nil
}

// Stop stops the web admin server
func (s *Server) Stop() error {
	if s.server == nil {
		return nil
	}

	s.logger.Info("Stopping web admin server")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	return s.server.Shutdown(ctx)
}

// RegisterRoutes registers HTTP routes for the web admin interface
func (s *Server) RegisterRoutes() {
	// This method is called by the interface but routes are registered in Start()
}

// registerRoutesOnMux registers HTTP routes on the provided mux
func (s *Server) registerRoutesOnMux(mux *http.ServeMux) {
	// Serve static files
	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("web/static/"))))

	// Admin dashboard
	mux.HandleFunc("/", s.userHandler.HandleDashboard)
	mux.HandleFunc("/admin", s.userHandler.HandleDashboard)
	mux.HandleFunc("/admin/", s.userHandler.HandleDashboard)

	// User management API endpoints
	mux.HandleFunc("/admin/users", s.userHandler.HandleUsers)
	mux.HandleFunc("/admin/users/", s.userHandler.HandleUserByID)

	// User management pages
	mux.HandleFunc("/admin/users/new", s.userHandler.HandleNewUserPage)
	mux.HandleFunc("/admin/users/edit/", s.userHandler.HandleEditUserPage)

	// Hunt Group management API endpoints
	mux.HandleFunc("/admin/huntgroups", s.huntGroupHandler.HandleHuntGroups)
	mux.HandleFunc("/admin/huntgroups/", s.huntGroupHandler.HandleHuntGroupByID)

	// Hunt Group management pages
	mux.HandleFunc("/admin/huntgroups/new", s.huntGroupHandler.HandleNewHuntGroupPage)
	mux.HandleFunc("/admin/huntgroups/edit/", s.huntGroupHandler.HandleEditHuntGroupPage)
	mux.HandleFunc("/admin/huntgroups/members/", s.huntGroupHandler.HandleHuntGroupMembers)
	mux.HandleFunc("/admin/huntgroups/statistics/", s.huntGroupHandler.HandleHuntGroupStatistics)
}

// WebUserHandler handles HTTP requests for user management
type WebUserHandler struct {
	userManager database.UserManager
}

// HandleDashboard handles the admin dashboard page
func (h *WebUserHandler) HandleDashboard(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Simple HTML response for now
	html := `<!DOCTYPE html>
<html>
<head>
    <title>SIP Server Admin</title>
    <link rel="stylesheet" href="/static/css/admin.css">
</head>
<body>
    <div class="container">
        <h1>SIP Server Administration</h1>
        <nav>
            <ul>
                <li><a href="/admin/users">Manage Users</a></li>
                <li><a href="/admin/huntgroups">Manage Hunt Groups</a></li>
            </ul>
        </nav>
        <div class="content">
            <h2>Dashboard</h2>
            <p>Welcome to the SIP Server administration interface.</p>
        </div>
    </div>
    <script src="/static/js/admin.js"></script>
</body>
</html>`

	w.Header().Set("Content-Type", "text/html")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(html))
}

// HandleUsers handles user listing and creation
func (h *WebUserHandler) HandleUsers(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.handleListUsers(w, r)
	case http.MethodPost:
		h.handleCreateUser(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// HandleUserByID handles individual user operations
func (h *WebUserHandler) HandleUserByID(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.handleGetUser(w, r)
	case http.MethodPut:
		h.handleUpdateUser(w, r)
	case http.MethodDelete:
		h.handleDeleteUser(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// HandleNewUserPage handles the new user form page
func (h *WebUserHandler) HandleNewUserPage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	html := `<!DOCTYPE html>
<html>
<head>
    <title>New User - SIP Server Admin</title>
    <link rel="stylesheet" href="/static/css/admin.css">
</head>
<body>
    <div class="container">
        <h1>Create New User</h1>
        <form method="POST" action="/admin/users">
            <div class="form-group">
                <label for="username">Username:</label>
                <input type="text" id="username" name="username" required>
            </div>
            <div class="form-group">
                <label for="realm">Realm:</label>
                <input type="text" id="realm" name="realm" required>
            </div>
            <div class="form-group">
                <label for="password">Password:</label>
                <input type="password" id="password" name="password" required>
            </div>
            <div class="form-group">
                <button type="submit">Create User</button>
                <a href="/admin/users">Cancel</a>
            </div>
        </form>
    </div>
</body>
</html>`

	w.Header().Set("Content-Type", "text/html")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(html))
}

// HandleEditUserPage handles the edit user form page
func (h *WebUserHandler) HandleEditUserPage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Simple edit form - in a real implementation, this would load user data
	html := `<!DOCTYPE html>
<html>
<head>
    <title>Edit User - SIP Server Admin</title>
    <link rel="stylesheet" href="/static/css/admin.css">
</head>
<body>
    <div class="container">
        <h1>Edit User</h1>
        <form method="POST" action="/admin/users/">
            <input type="hidden" name="_method" value="PUT">
            <div class="form-group">
                <label for="password">New Password:</label>
                <input type="password" id="password" name="password">
            </div>
            <div class="form-group">
                <label for="enabled">Enabled:</label>
                <input type="checkbox" id="enabled" name="enabled" checked>
            </div>
            <div class="form-group">
                <button type="submit">Update User</button>
                <a href="/admin/users">Cancel</a>
            </div>
        </form>
    </div>
</body>
</html>`

	w.Header().Set("Content-Type", "text/html")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(html))
}

// handleListUsers lists all users
func (h *WebUserHandler) handleListUsers(w http.ResponseWriter, r *http.Request) {
	users, err := h.userManager.ListUsers()
	if err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Simple HTML table for now
	html := `<!DOCTYPE html>
<html>
<head>
    <title>Users - SIP Server Admin</title>
    <link rel="stylesheet" href="/static/css/admin.css">
</head>
<body>
    <div class="container">
        <h1>SIP Users</h1>
        <div class="actions">
            <a href="/admin/users/new" class="button">Add New User</a>
        </div>
        <table>
            <thead>
                <tr>
                    <th>Username</th>
                    <th>Realm</th>
                    <th>Enabled</th>
                    <th>Created</th>
                    <th>Actions</th>
                </tr>
            </thead>
            <tbody>`

	for _, user := range users {
		enabled := "No"
		if user.Enabled {
			enabled = "Yes"
		}
		html += fmt.Sprintf(`
                <tr>
                    <td>%s</td>
                    <td>%s</td>
                    <td>%s</td>
                    <td>%s</td>
                    <td>
                        <a href="/admin/users/edit/%d">Edit</a>
                        <a href="#" onclick="deleteUser(%d)">Delete</a>
                    </td>
                </tr>`, user.Username, user.Realm, enabled, user.CreatedAt.Format("2006-01-02 15:04"), user.ID, user.ID)
	}

	html += `
            </tbody>
        </table>
    </div>
    <script src="/static/js/admin.js"></script>
</body>
</html>`

	w.Header().Set("Content-Type", "text/html")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(html))
}

// handleCreateUser creates a new user
func (h *WebUserHandler) handleCreateUser(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid form data", http.StatusBadRequest)
		return
	}

	username := r.FormValue("username")
	realm := r.FormValue("realm")
	password := r.FormValue("password")

	if username == "" || realm == "" || password == "" {
		http.Error(w, "Missing required fields", http.StatusBadRequest)
		return
	}

	err := h.userManager.CreateUser(username, realm, password)
	if err != nil {
		http.Error(w, "Failed to create user", http.StatusInternalServerError)
		return
	}

	// Redirect to users list
	http.Redirect(w, r, "/admin/users", http.StatusSeeOther)
}

// handleGetUser gets a specific user
func (h *WebUserHandler) handleGetUser(w http.ResponseWriter, r *http.Request) {
	// Simple implementation - would extract user ID from URL in real implementation
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"message": "Get user not implemented"}`))
}

// handleUpdateUser updates a user
func (h *WebUserHandler) handleUpdateUser(w http.ResponseWriter, r *http.Request) {
	// Simple implementation - would extract user ID and update in real implementation
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"message": "Update user not implemented"}`))
}

// handleDeleteUser deletes a user
func (h *WebUserHandler) handleDeleteUser(w http.ResponseWriter, r *http.Request) {
	// Simple implementation - would extract user ID and delete in real implementation
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"message": "Delete user not implemented"}`))
}