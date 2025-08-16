package registrar

import (
	"github.com/zurustar/xylitol2/internal/database"
)

// Registrar defines the interface for SIP registration services
type Registrar interface {
	Register(contact *database.RegistrarContact, expires int) error
	Unregister(aor string) error
	FindContacts(aor string) ([]*database.RegistrarContact, error)
	CleanupExpired()
}