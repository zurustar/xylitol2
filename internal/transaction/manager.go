package transaction

import (
	"sync"
	"time"

	"github.com/zurustar/xylitol2/internal/parser"
)

// Manager implements the TransactionManager interface
type Manager struct {
	transactions map[string]Transaction
	mutex        sync.RWMutex
	sendMessage  func(*parser.SIPMessage) error
	cleanupTicker *time.Ticker
	stopCleanup   chan bool
}

// NewManager creates a new transaction manager
func NewManager(sendFunc func(*parser.SIPMessage) error) *Manager {
	m := &Manager{
		transactions: make(map[string]Transaction),
		sendMessage:  sendFunc,
		stopCleanup:  make(chan bool),
	}

	// Start cleanup goroutine
	m.startCleanupRoutine()

	return m
}

// CreateTransaction creates a new transaction based on the message
func (m *Manager) CreateTransaction(msg *parser.SIPMessage) Transaction {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	var transaction Transaction

	if msg.IsRequest() {
		// Create server transaction
		transaction = NewServerTransaction(msg, m.sendMessage)
	} else {
		// This shouldn't happen in normal operation
		// Client transactions are created when sending requests
		return nil
	}

	// Store the transaction
	id := transaction.GetID()
	m.transactions[id] = transaction

	return transaction
}

// CreateClientTransaction creates a new client transaction for outgoing requests
func (m *Manager) CreateClientTransaction(msg *parser.SIPMessage) Transaction {
	if !msg.IsRequest() {
		return nil
	}

	m.mutex.Lock()
	defer m.mutex.Unlock()

	transaction := NewClientTransaction(msg, m.sendMessage)
	id := transaction.GetID()
	m.transactions[id] = transaction

	return transaction
}

// FindTransaction finds an existing transaction based on the message
func (m *Manager) FindTransaction(msg *parser.SIPMessage) Transaction {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	id := generateTransactionID(msg)
	return m.transactions[id]
}

// FindTransactionByID finds a transaction by its ID
func (m *Manager) FindTransactionByID(id string) Transaction {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	return m.transactions[id]
}

// RemoveTransaction removes a transaction from the manager
func (m *Manager) RemoveTransaction(id string) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if transaction, exists := m.transactions[id]; exists {
		// Cancel all timers before removing
		if bt, ok := transaction.(*ClientTransaction); ok {
			bt.CancelAllTimers()
		} else if bt, ok := transaction.(*ServerTransaction); ok {
			bt.CancelAllTimers()
		}
		delete(m.transactions, id)
	}
}

// CleanupExpired removes expired transactions
func (m *Manager) CleanupExpired() {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	expiredIDs := make([]string, 0)

	for id, transaction := range m.transactions {
		// Check if transaction is terminated or expired
		if transaction.GetState() == StateTerminated {
			expiredIDs = append(expiredIDs, id)
		} else {
			// Check if base transaction is expired
			if bt, ok := transaction.(*ClientTransaction); ok && bt.IsExpired() {
				expiredIDs = append(expiredIDs, id)
			} else if bt, ok := transaction.(*ServerTransaction); ok && bt.IsExpired() {
				expiredIDs = append(expiredIDs, id)
			}
		}
	}

	// Remove expired transactions
	for _, id := range expiredIDs {
		if transaction, exists := m.transactions[id]; exists {
			// Cancel all timers before removing
			if bt, ok := transaction.(*ClientTransaction); ok {
				bt.CancelAllTimers()
			} else if bt, ok := transaction.(*ServerTransaction); ok {
				bt.CancelAllTimers()
			}
			delete(m.transactions, id)
		}
	}
}

// GetTransactionCount returns the number of active transactions
func (m *Manager) GetTransactionCount() int {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	return len(m.transactions)
}

// GetTransactions returns a copy of all active transactions
func (m *Manager) GetTransactions() map[string]Transaction {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	transactions := make(map[string]Transaction)
	for id, transaction := range m.transactions {
		transactions[id] = transaction
	}

	return transactions
}

// ProcessMessage processes an incoming message and routes it to the appropriate transaction
func (m *Manager) ProcessMessage(msg *parser.SIPMessage) error {
	// Find existing transaction
	transaction := m.FindTransaction(msg)

	if transaction != nil {
		// Route to existing transaction
		if msg.IsRequest() {
			// Handle special case for ACK to INVITE server transactions
			if msg.GetMethod() == parser.MethodACK {
				if st, ok := transaction.(*ServerTransaction); ok {
					return st.HandleACK(msg)
				}
			}
			return transaction.ProcessMessage(msg)
		} else {
			// Response message
			return transaction.ProcessMessage(msg)
		}
	} else if msg.IsRequest() {
		// Create new server transaction for incoming request
		transaction = m.CreateTransaction(msg)
		if transaction != nil {
			return transaction.ProcessMessage(msg)
		}
	}

	// No transaction found and couldn't create one
	return nil
}

// SendRequest sends a request and creates a client transaction
func (m *Manager) SendRequest(msg *parser.SIPMessage) (Transaction, error) {
	if !msg.IsRequest() {
		return nil, nil
	}

	// Create client transaction
	transaction := m.CreateClientTransaction(msg)
	if transaction == nil {
		return nil, nil
	}

	// Send the message
	if m.sendMessage != nil {
		err := m.sendMessage(msg)
		if err != nil {
			// Remove the transaction if sending failed
			m.RemoveTransaction(transaction.GetID())
			return nil, err
		}
	}

	return transaction, nil
}

// SendResponse sends a response through an existing server transaction
func (m *Manager) SendResponse(msg *parser.SIPMessage, transactionID string) error {
	if !msg.IsResponse() {
		return nil
	}

	transaction := m.FindTransactionByID(transactionID)
	if transaction == nil {
		return nil
	}

	if st, ok := transaction.(*ServerTransaction); ok {
		return st.SendResponse(msg)
	}

	return nil
}

// startCleanupRoutine starts a background goroutine to clean up expired transactions
func (m *Manager) startCleanupRoutine() {
	m.cleanupTicker = time.NewTicker(30 * time.Second) // Cleanup every 30 seconds

	go func() {
		for {
			select {
			case <-m.cleanupTicker.C:
				m.CleanupExpired()
			case <-m.stopCleanup:
				m.cleanupTicker.Stop()
				return
			}
		}
	}()
}

// Stop stops the transaction manager and cleans up resources
func (m *Manager) Stop() {
	// Stop cleanup routine
	if m.stopCleanup != nil {
		close(m.stopCleanup)
	}

	// Clean up all transactions
	m.mutex.Lock()
	defer m.mutex.Unlock()

	for id, transaction := range m.transactions {
		// Cancel all timers
		if bt, ok := transaction.(*ClientTransaction); ok {
			bt.CancelAllTimers()
		} else if bt, ok := transaction.(*ServerTransaction); ok {
			bt.CancelAllTimers()
		}
		delete(m.transactions, id)
	}
}