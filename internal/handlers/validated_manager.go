package handlers

import (
	"fmt"

	"github.com/zurustar/xylitol2/internal/parser"
	"github.com/zurustar/xylitol2/internal/transaction"
	"github.com/zurustar/xylitol2/internal/validation"
)

// ValidatedManager implements HandlerManager with validation chain integration
type ValidatedManager struct {
	*Manager
	processor *validation.MessageProcessor
}

// NewValidatedManager creates a new validated handler manager
func NewValidatedManager() *ValidatedManager {
	return &ValidatedManager{
		Manager:   NewManager(),
		processor: validation.NewMessageProcessor(),
	}
}

// AddValidator adds a validator to the validation chain
func (vm *ValidatedManager) AddValidator(validator validation.RequestValidator) {
	vm.processor.AddValidator(validator)
}

// RemoveValidator removes a validator from the validation chain
func (vm *ValidatedManager) RemoveValidator(name string) bool {
	return vm.processor.RemoveValidator(name)
}

// GetValidators returns all validators in the chain
func (vm *ValidatedManager) GetValidators() []validation.RequestValidator {
	return vm.processor.GetValidators()
}

// HandleRequest processes the request through validation chain first, then routes to handlers
func (vm *ValidatedManager) HandleRequest(req *parser.SIPMessage, txn transaction.Transaction) error {
	// Only validate request messages
	if !req.IsRequest() {
		return fmt.Errorf("message is not a request")
	}

	// Run validation chain
	errorResp, err := vm.processor.ProcessRequest(req)
	if err != nil {
		return fmt.Errorf("validation processing failed: %w", err)
	}

	// If validation failed, send error response
	if errorResp != nil {
		return txn.SendResponse(errorResp)
	}

	// Validation passed, proceed with normal handler processing
	return vm.Manager.HandleRequest(req, txn)
}

// SetupDefaultValidators sets up the default validation chain with proper priorities
func (vm *ValidatedManager) SetupDefaultValidators(config ValidationConfig) {
	// Add validators in any order - they will be sorted by priority
	
	// Syntax validator (priority 1 - highest)
	vm.AddValidator(validation.NewSyntaxValidator())
	
	// Session-Timer validator (priority 10 - before auth)
	if config.SessionTimerConfig.Enabled {
		sessionTimerValidator := validation.NewSessionTimerValidator(
			config.SessionTimerConfig.MinSE,
			config.SessionTimerConfig.DefaultSE,
			config.SessionTimerConfig.RequireSupport,
		)
		vm.AddValidator(sessionTimerValidator)
	}
	
	// Authentication validator (priority 20 - after session timer)
	if config.AuthConfig.Enabled {
		authValidator := validation.NewAuthValidator(
			config.AuthConfig.RequireAuth,
			config.AuthConfig.Realm,
		)
		vm.AddValidator(authValidator)
	}
}

// ValidationConfig holds configuration for validators
type ValidationConfig struct {
	SessionTimerConfig SessionTimerConfig
	AuthConfig         AuthConfig
}

// SessionTimerConfig holds Session-Timer validation configuration
type SessionTimerConfig struct {
	Enabled        bool
	MinSE          int
	DefaultSE      int
	RequireSupport bool
}

// AuthConfig holds authentication validation configuration
type AuthConfig struct {
	Enabled     bool
	RequireAuth bool
	Realm       string
}

// DefaultValidationConfig returns a default validation configuration
func DefaultValidationConfig() ValidationConfig {
	return ValidationConfig{
		SessionTimerConfig: SessionTimerConfig{
			Enabled:        true,
			MinSE:          90,
			DefaultSE:      1800,
			RequireSupport: true,
		},
		AuthConfig: AuthConfig{
			Enabled:     true,
			RequireAuth: true,
			Realm:       "sip-server",
		},
	}
}