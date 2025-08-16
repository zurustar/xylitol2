package webadmin

import (
	"github.com/zurustar/xylitol2/internal/database"
)

// WebAdminServer defines the interface for the web administration interface
type WebAdminServer interface {
	Start(port int) error
	Stop() error
	RegisterRoutes()
}

// UserHandler handles HTTP requests for user management
type UserHandler struct {
	userManager database.UserManager
}

// HTTP endpoints will be:
// GET /admin/users - List all users
// POST /admin/users - Create new user
// PUT /admin/users/{id} - Update user
// DELETE /admin/users/{id} - Delete user
// GET /admin - Admin dashboard