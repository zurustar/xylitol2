package registrar

import (
	"github.com/zurustar/xylitol2/internal/database"
	"github.com/zurustar/xylitol2/internal/logging"
)

// SimpleRegistrar implements the Registrar interface
type SimpleRegistrar struct {
	storage database.RegistrationDB
	logger  logging.Logger
}

// NewRegistrar creates a new simple registrar
func NewRegistrar(storage database.RegistrationDB, logger logging.Logger) Registrar {
	return &SimpleRegistrar{
		storage: storage,
		logger:  logger,
	}
}

// Register registers a contact
func (r *SimpleRegistrar) Register(contact *database.RegistrarContact, expires int) error {
	return r.storage.Store(contact)
}

// Unregister removes all contacts for an AOR
func (r *SimpleRegistrar) Unregister(aor string) error {
	// Get all contacts for the AOR
	contacts, err := r.storage.Retrieve(aor)
	if err != nil {
		return err
	}

	// Delete each contact
	for _, contact := range contacts {
		if err := r.storage.Delete(aor, contact.URI); err != nil {
			r.logger.Error("Failed to delete contact during unregister", 
				logging.Field{Key: "aor", Value: aor}, 
				logging.Field{Key: "contact", Value: contact.URI},
				logging.Field{Key: "error", Value: err})
		}
	}

	return nil
}

// FindContacts retrieves all registered contacts for an AOR
func (r *SimpleRegistrar) FindContacts(aor string) ([]*database.RegistrarContact, error) {
	return r.storage.Retrieve(aor)
}

// CleanupExpired removes expired registrations
func (r *SimpleRegistrar) CleanupExpired() {
	if err := r.storage.CleanupExpired(); err != nil {
		r.logger.Error("Failed to cleanup expired contacts", logging.Field{Key: "error", Value: err})
	}
}