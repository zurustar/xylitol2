package handlers

import (
	"fmt"

	"github.com/zurustar/xylitol2/internal/database"
	"github.com/zurustar/xylitol2/internal/parser"
	"github.com/zurustar/xylitol2/internal/sessiontimer"
	"github.com/zurustar/xylitol2/internal/transaction"
)

// TestSessionTimerManager is a mock implementation for testing
type TestSessionTimerManager struct {
	requiresTimer bool
}

func (m *TestSessionTimerManager) CreateSession(callID string, sessionExpires int) *sessiontimer.Session {
	return &sessiontimer.Session{
		CallID: callID,
	}
}

func (m *TestSessionTimerManager) RefreshSession(callID string) error {
	return nil
}

func (m *TestSessionTimerManager) CleanupExpiredSessions() {}

func (m *TestSessionTimerManager) IsSessionTimerRequired(msg *parser.SIPMessage) bool {
	return m.requiresTimer
}

func (m *TestSessionTimerManager) StartCleanupTimer() {}

func (m *TestSessionTimerManager) StopCleanupTimer() {}

func (m *TestSessionTimerManager) SetSessionTerminationCallback(callback func(callID string)) {}

func (m *TestSessionTimerManager) RemoveSession(callID string) {}

// MockAuthProcessor is a mock implementation for testing
type MockAuthProcessor struct {
	shouldFail bool
	user       *database.User
}

func (m *MockAuthProcessor) ProcessIncomingRequest(req *parser.SIPMessage, txn transaction.Transaction) (*parser.SIPMessage, *database.User, error) {
	if m.shouldFail {
		// Return auth challenge response
		response := parser.NewResponseMessage(parser.StatusUnauthorized, "Unauthorized")
		return response, nil, nil
	}
	return nil, m.user, nil
}

func (m *MockAuthProcessor) ProcessREGISTERRequest(request *parser.SIPMessage, txn transaction.Transaction) (*parser.SIPMessage, *database.User, error) {
	return m.ProcessIncomingRequest(request, txn)
}

func (m *MockAuthProcessor) ProcessINVITERequest(request *parser.SIPMessage, txn transaction.Transaction) (*parser.SIPMessage, *database.User, error) {
	return m.ProcessIncomingRequest(request, txn)
}

func (m *MockAuthProcessor) GetAuthenticatedUser(request *parser.SIPMessage) (*database.User, error) {
	if m.shouldFail {
		return nil, fmt.Errorf("authentication failed")
	}
	return m.user, nil
}

func (m *MockAuthProcessor) SetRealm(realm string) {}

func (m *MockAuthProcessor) GetRealm() string {
	return "test.local"
}

// MockUserManager is a mock implementation for testing
type MockUserManager struct {
	user *database.User
}

func (m *MockUserManager) CreateUser(username, realm, password string) error {
	return nil
}

func (m *MockUserManager) AuthenticateUser(username, realm, password string) bool {
	return m.user != nil
}

func (m *MockUserManager) UpdatePassword(username, realm, newPassword string) error {
	return nil
}

func (m *MockUserManager) GetUser(username, realm string) (*database.User, error) {
	if m.user != nil {
		return m.user, nil
	}
	return nil, fmt.Errorf("user not found")
}

func (m *MockUserManager) UpdateUser(user *database.User) error {
	return nil
}

func (m *MockUserManager) DeleteUser(username, realm string) error {
	return nil
}

func (m *MockUserManager) ListUsers() ([]*database.User, error) {
	if m.user != nil {
		return []*database.User{m.user}, nil
	}
	return []*database.User{}, nil
}

func (m *MockUserManager) GeneratePasswordHash(username, realm, password string) string {
	return "mock-hash"
}