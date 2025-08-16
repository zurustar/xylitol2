package transaction

import (
	"testing"

	"github.com/zurustar/xylitol2/internal/parser"
)

func TestManagerCreateTransaction(t *testing.T) {
	sentMessages := []*parser.SIPMessage{}
	sendFunc := func(msg *parser.SIPMessage) error {
		sentMessages = append(sentMessages, msg)
		return nil
	}

	manager := NewManager(sendFunc)
	defer manager.Stop()

	// Create INVITE request
	invite := createTestMessage(parser.MethodINVITE, map[string]string{
		parser.HeaderVia:    "SIP/2.0/UDP 192.168.1.1:5060;branch=z9hG4bKtest123",
		parser.HeaderCallID: "test-call-id",
	})

	// Create server transaction
	transaction := manager.CreateTransaction(invite)
	if transaction == nil {
		t.Fatal("CreateTransaction returned nil")
	}

	if !transaction.IsClient() {
		// Should be server transaction
		if transaction.GetState() != StateProceeding {
			t.Errorf("Expected state Proceeding, got %v", transaction.GetState())
		}
	}

	// Should be able to find the transaction
	found := manager.FindTransaction(invite)
	if found == nil {
		t.Error("FindTransaction returned nil")
	}

	if found.GetID() != transaction.GetID() {
		t.Error("Found transaction ID doesn't match created transaction ID")
	}
}

func TestManagerCreateClientTransaction(t *testing.T) {
	sentMessages := []*parser.SIPMessage{}
	sendFunc := func(msg *parser.SIPMessage) error {
		sentMessages = append(sentMessages, msg)
		return nil
	}

	manager := NewManager(sendFunc)
	defer manager.Stop()

	// Create INVITE request
	invite := createTestMessage(parser.MethodINVITE, map[string]string{
		parser.HeaderVia:    "SIP/2.0/UDP 192.168.1.1:5060;branch=z9hG4bKtest123",
		parser.HeaderCallID: "test-call-id",
	})

	// Create client transaction
	transaction := manager.CreateClientTransaction(invite)
	if transaction == nil {
		t.Fatal("CreateClientTransaction returned nil")
	}

	if !transaction.IsClient() {
		t.Error("Expected client transaction")
	}

	if transaction.GetState() != StateCalling {
		t.Errorf("Expected state Calling, got %v", transaction.GetState())
	}

	// Should be able to find the transaction
	found := manager.FindTransaction(invite)
	if found == nil {
		t.Error("FindTransaction returned nil")
	}

	if found.GetID() != transaction.GetID() {
		t.Error("Found transaction ID doesn't match created transaction ID")
	}
}

func TestManagerFindTransaction(t *testing.T) {
	sendFunc := func(msg *parser.SIPMessage) error {
		return nil
	}

	manager := NewManager(sendFunc)
	defer manager.Stop()

	// Create INVITE request
	invite := createTestMessage(parser.MethodINVITE, map[string]string{
		parser.HeaderVia:    "SIP/2.0/UDP 192.168.1.1:5060;branch=z9hG4bKtest123",
		parser.HeaderCallID: "test-call-id",
	})

	// Should not find non-existent transaction
	found := manager.FindTransaction(invite)
	if found != nil {
		t.Error("FindTransaction should return nil for non-existent transaction")
	}

	// Create transaction
	transaction := manager.CreateTransaction(invite)
	if transaction == nil {
		t.Fatal("CreateTransaction returned nil")
	}

	// Should find the transaction
	found = manager.FindTransaction(invite)
	if found == nil {
		t.Error("FindTransaction returned nil for existing transaction")
	}

	// Test FindTransactionByID
	foundByID := manager.FindTransactionByID(transaction.GetID())
	if foundByID == nil {
		t.Error("FindTransactionByID returned nil")
	}

	if foundByID.GetID() != transaction.GetID() {
		t.Error("Found transaction ID doesn't match")
	}
}

func TestManagerTransactionMatching(t *testing.T) {
	sendFunc := func(msg *parser.SIPMessage) error {
		return nil
	}

	manager := NewManager(sendFunc)
	defer manager.Stop()

	// Create INVITE request
	invite := createTestMessage(parser.MethodINVITE, map[string]string{
		parser.HeaderVia:    "SIP/2.0/UDP 192.168.1.1:5060;branch=z9hG4bKtest123",
		parser.HeaderCallID: "test-call-id",
	})

	// Create transaction
	transaction := manager.CreateTransaction(invite)
	if transaction == nil {
		t.Fatal("CreateTransaction returned nil")
	}

	// Create ACK with same branch - should match INVITE transaction
	ack := createTestMessage(parser.MethodACK, map[string]string{
		parser.HeaderVia:    "SIP/2.0/UDP 192.168.1.1:5060;branch=z9hG4bKtest123",
		parser.HeaderCallID: "test-call-id",
	})

	foundACK := manager.FindTransaction(ack)
	if foundACK == nil {
		t.Error("ACK should match INVITE transaction")
	}

	if foundACK.GetID() != transaction.GetID() {
		t.Error("ACK transaction ID should match INVITE transaction ID")
	}

	// Create CANCEL with same branch - should match INVITE transaction
	cancel := createTestMessage(parser.MethodCANCEL, map[string]string{
		parser.HeaderVia:    "SIP/2.0/UDP 192.168.1.1:5060;branch=z9hG4bKtest123",
		parser.HeaderCallID: "test-call-id",
	})

	foundCancel := manager.FindTransaction(cancel)
	if foundCancel == nil {
		t.Error("CANCEL should match INVITE transaction")
	}

	if foundCancel.GetID() != transaction.GetID() {
		t.Error("CANCEL transaction ID should match INVITE transaction ID")
	}

	// Create different method with same branch - should NOT match
	register := createTestMessage(parser.MethodREGISTER, map[string]string{
		parser.HeaderVia:    "SIP/2.0/UDP 192.168.1.1:5060;branch=z9hG4bKtest123",
		parser.HeaderCallID: "test-call-id",
	})

	foundRegister := manager.FindTransaction(register)
	if foundRegister != nil && foundRegister.GetID() == transaction.GetID() {
		t.Error("REGISTER should not match INVITE transaction")
	}
}

func TestManagerProcessMessage(t *testing.T) {
	sentMessages := []*parser.SIPMessage{}
	sendFunc := func(msg *parser.SIPMessage) error {
		sentMessages = append(sentMessages, msg)
		return nil
	}

	manager := NewManager(sendFunc)
	defer manager.Stop()

	// Process incoming INVITE request
	invite := createTestMessage(parser.MethodINVITE, map[string]string{
		parser.HeaderVia:    "SIP/2.0/UDP 192.168.1.1:5060;branch=z9hG4bKtest123",
		parser.HeaderCallID: "test-call-id",
	})

	err := manager.ProcessMessage(invite)
	if err != nil {
		t.Errorf("ProcessMessage failed: %v", err)
	}

	// Should have created a server transaction
	transaction := manager.FindTransaction(invite)
	if transaction == nil {
		t.Error("ProcessMessage should have created a server transaction")
	}

	if transaction.IsClient() {
		t.Error("Should have created server transaction")
	}

	// Process response to non-existent client transaction
	response := parser.NewResponseMessage(200, "OK")
	response.SetHeader(parser.HeaderVia, invite.GetHeader(parser.HeaderVia))
	response.SetHeader(parser.HeaderFrom, invite.GetHeader(parser.HeaderFrom))
	response.SetHeader(parser.HeaderTo, invite.GetHeader(parser.HeaderTo))
	response.SetHeader(parser.HeaderCallID, invite.GetHeader(parser.HeaderCallID))
	response.SetHeader(parser.HeaderCSeq, invite.GetHeader(parser.HeaderCSeq))

	err = manager.ProcessMessage(response)
	if err != nil {
		t.Errorf("ProcessMessage failed: %v", err)
	}
}

func TestManagerSendRequest(t *testing.T) {
	sentMessages := []*parser.SIPMessage{}
	sendFunc := func(msg *parser.SIPMessage) error {
		sentMessages = append(sentMessages, msg)
		return nil
	}

	manager := NewManager(sendFunc)
	defer manager.Stop()

	// Send INVITE request
	invite := createTestMessage(parser.MethodINVITE, map[string]string{
		parser.HeaderVia:    "SIP/2.0/UDP 192.168.1.1:5060;branch=z9hG4bKtest123",
		parser.HeaderCallID: "test-call-id",
	})

	transaction, err := manager.SendRequest(invite)
	if err != nil {
		t.Errorf("SendRequest failed: %v", err)
	}

	if transaction == nil {
		t.Fatal("SendRequest returned nil transaction")
	}

	if !transaction.IsClient() {
		t.Error("SendRequest should create client transaction")
	}

	// Should have sent the message
	if len(sentMessages) == 0 {
		t.Error("SendRequest should have sent the message")
	}

	// Should be able to find the transaction
	found := manager.FindTransaction(invite)
	if found == nil {
		t.Error("Should be able to find sent transaction")
	}

	if found.GetID() != transaction.GetID() {
		t.Error("Found transaction ID doesn't match sent transaction ID")
	}
}

func TestManagerSendResponse(t *testing.T) {
	sentMessages := []*parser.SIPMessage{}
	sendFunc := func(msg *parser.SIPMessage) error {
		sentMessages = append(sentMessages, msg)
		return nil
	}

	manager := NewManager(sendFunc)
	defer manager.Stop()

	// Create server transaction
	invite := createTestMessage(parser.MethodINVITE, map[string]string{
		parser.HeaderVia:    "SIP/2.0/UDP 192.168.1.1:5060;branch=z9hG4bKtest123",
		parser.HeaderCallID: "test-call-id",
	})

	transaction := manager.CreateTransaction(invite)
	if transaction == nil {
		t.Fatal("CreateTransaction returned nil")
	}

	// Send response
	response := parser.NewResponseMessage(200, "OK")
	err := manager.SendResponse(response, transaction.GetID())
	if err != nil {
		t.Errorf("SendResponse failed: %v", err)
	}

	// Should have sent the response
	if len(sentMessages) == 0 {
		t.Error("SendResponse should have sent the response")
	}
}

func TestManagerRemoveTransaction(t *testing.T) {
	sendFunc := func(msg *parser.SIPMessage) error {
		return nil
	}

	manager := NewManager(sendFunc)
	defer manager.Stop()

	// Create transaction
	invite := createTestMessage(parser.MethodINVITE, map[string]string{
		parser.HeaderVia:    "SIP/2.0/UDP 192.168.1.1:5060;branch=z9hG4bKtest123",
		parser.HeaderCallID: "test-call-id",
	})

	transaction := manager.CreateTransaction(invite)
	if transaction == nil {
		t.Fatal("CreateTransaction returned nil")
	}

	// Should be able to find the transaction
	found := manager.FindTransaction(invite)
	if found == nil {
		t.Error("Should be able to find transaction")
	}

	// Remove transaction
	manager.RemoveTransaction(transaction.GetID())

	// Should not be able to find the transaction
	found = manager.FindTransaction(invite)
	if found != nil {
		t.Error("Should not be able to find removed transaction")
	}
}

func TestManagerCleanupExpired(t *testing.T) {
	sendFunc := func(msg *parser.SIPMessage) error {
		return nil
	}

	manager := NewManager(sendFunc)
	defer manager.Stop()

	// Create transaction
	invite := createTestMessage(parser.MethodINVITE, map[string]string{
		parser.HeaderVia:    "SIP/2.0/UDP 192.168.1.1:5060;branch=z9hG4bKtest123",
		parser.HeaderCallID: "test-call-id",
	})

	transaction := manager.CreateTransaction(invite)
	if transaction == nil {
		t.Fatal("CreateTransaction returned nil")
	}

	initialCount := manager.GetTransactionCount()
	if initialCount == 0 {
		t.Error("Should have at least one transaction")
	}

	// Manually set transaction to terminated state
	if st, ok := transaction.(*ServerTransaction); ok {
		st.setState(StateTerminated)
	}

	// Run cleanup
	manager.CleanupExpired()

	// Should have removed terminated transaction
	finalCount := manager.GetTransactionCount()
	if finalCount >= initialCount {
		t.Error("Cleanup should have removed terminated transaction")
	}
}

func TestManagerGetTransactionCount(t *testing.T) {
	sendFunc := func(msg *parser.SIPMessage) error {
		return nil
	}

	manager := NewManager(sendFunc)
	defer manager.Stop()

	// Should start with 0 transactions
	if manager.GetTransactionCount() != 0 {
		t.Error("Should start with 0 transactions")
	}

	// Create transaction
	invite := createTestMessage(parser.MethodINVITE, map[string]string{
		parser.HeaderVia:    "SIP/2.0/UDP 192.168.1.1:5060;branch=z9hG4bKtest123",
		parser.HeaderCallID: "test-call-id",
	})

	manager.CreateTransaction(invite)

	// Should have 1 transaction
	if manager.GetTransactionCount() != 1 {
		t.Error("Should have 1 transaction")
	}

	// Create another transaction
	invite2 := createTestMessage(parser.MethodINVITE, map[string]string{
		parser.HeaderVia:    "SIP/2.0/UDP 192.168.1.1:5060;branch=z9hG4bKtest456",
		parser.HeaderCallID: "test-call-id-2",
	})

	manager.CreateTransaction(invite2)

	// Should have 2 transactions
	if manager.GetTransactionCount() != 2 {
		t.Error("Should have 2 transactions")
	}
}

func TestManagerGetTransactions(t *testing.T) {
	sendFunc := func(msg *parser.SIPMessage) error {
		return nil
	}

	manager := NewManager(sendFunc)
	defer manager.Stop()

	// Create transactions
	invite1 := createTestMessage(parser.MethodINVITE, map[string]string{
		parser.HeaderVia:    "SIP/2.0/UDP 192.168.1.1:5060;branch=z9hG4bKtest123",
		parser.HeaderCallID: "test-call-id-1",
	})

	invite2 := createTestMessage(parser.MethodINVITE, map[string]string{
		parser.HeaderVia:    "SIP/2.0/UDP 192.168.1.1:5060;branch=z9hG4bKtest456",
		parser.HeaderCallID: "test-call-id-2",
	})

	t1 := manager.CreateTransaction(invite1)
	t2 := manager.CreateTransaction(invite2)

	transactions := manager.GetTransactions()
	if len(transactions) != 2 {
		t.Errorf("Expected 2 transactions, got %d", len(transactions))
	}

	// Check that both transactions are present
	found1 := false
	found2 := false
	for _, transaction := range transactions {
		if transaction.GetID() == t1.GetID() {
			found1 = true
		}
		if transaction.GetID() == t2.GetID() {
			found2 = true
		}
	}

	if !found1 || !found2 {
		t.Error("Not all transactions found in GetTransactions result")
	}
}

func TestManagerStop(t *testing.T) {
	sendFunc := func(msg *parser.SIPMessage) error {
		return nil
	}

	manager := NewManager(sendFunc)

	// Create some transactions
	invite := createTestMessage(parser.MethodINVITE, map[string]string{
		parser.HeaderVia:    "SIP/2.0/UDP 192.168.1.1:5060;branch=z9hG4bKtest123",
		parser.HeaderCallID: "test-call-id",
	})

	manager.CreateTransaction(invite)

	if manager.GetTransactionCount() == 0 {
		t.Error("Should have transactions before stop")
	}

	// Stop manager
	manager.Stop()

	// Should have cleaned up all transactions
	if manager.GetTransactionCount() != 0 {
		t.Error("Should have cleaned up all transactions after stop")
	}
}

func TestManagerCleanupRoutine(t *testing.T) {
	sendFunc := func(msg *parser.SIPMessage) error {
		return nil
	}

	manager := NewManager(sendFunc)
	defer manager.Stop()

	// Create transaction and set it to terminated
	invite := createTestMessage(parser.MethodINVITE, map[string]string{
		parser.HeaderVia:    "SIP/2.0/UDP 192.168.1.1:5060;branch=z9hG4bKtest123",
		parser.HeaderCallID: "test-call-id",
	})

	transaction := manager.CreateTransaction(invite)
	if transaction == nil {
		t.Fatal("CreateTransaction returned nil")
	}

	// Set transaction to terminated
	if st, ok := transaction.(*ServerTransaction); ok {
		st.setState(StateTerminated)
	}

	initialCount := manager.GetTransactionCount()

	// Wait a bit for cleanup routine to run (it runs every 30 seconds, but we can trigger it manually)
	manager.CleanupExpired()

	finalCount := manager.GetTransactionCount()
	if finalCount >= initialCount {
		t.Error("Cleanup routine should have removed terminated transaction")
	}
}