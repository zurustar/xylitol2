package webadmin

import (
	"github.com/zurustar/xylitol2/internal/database"
	"github.com/zurustar/xylitol2/internal/huntgroup"
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

// HuntGroupHandler handles HTTP requests for hunt group management
type HuntGroupHandler struct {
	huntGroupManager huntgroup.HuntGroupManager
	huntGroupEngine  huntgroup.HuntGroupEngine
}

// HTTP endpoints will be:
// GET /admin/users - List all users
// POST /admin/users - Create new user
// PUT /admin/users/{id} - Update user
// DELETE /admin/users/{id} - Delete user
// GET /admin - Admin dashboard
// GET /admin/huntgroups - List all hunt groups
// POST /admin/huntgroups - Create new hunt group
// PUT /admin/huntgroups/{id} - Update hunt group
// DELETE /admin/huntgroups/{id} - Delete hunt group
// GET /admin/huntgroups/{id}/members - List hunt group members
// POST /admin/huntgroups/{id}/members - Add hunt group member
// DELETE /admin/huntgroups/{id}/members/{member_id} - Remove hunt group member
// GET /admin/huntgroups/{id}/statistics - Get hunt group statistics