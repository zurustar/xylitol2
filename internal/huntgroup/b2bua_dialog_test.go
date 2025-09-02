package huntgroup

import (
	"testing"

	"github.com/zurustar/xylitol2/internal/parser"
)

func TestDialogCreation(t *testing.T) {
	logger := &mockLogger{}
	dm := NewDialogManager(logger)

	callID := "test-call-id@example.com"
	localURI := "sip:local@example.com"
	remoteURI := "sip:remote@example.com"
	localTag := "local-tag-123"
	remoteTag := "remote-tag-456"

	dialog := dm.CreateDialog(callID, localURI, remoteURI, localTag, remoteTag)

	if dialog == nil {
		t.Fatal("Dialog should not be nil")
	}

	if dialog.CallID != callID {
		t.Errorf("Expected Call-ID %s, got %s", callID, dialog.CallID)
	}

	if dialog.LocalURI != localURI {
		t.Errorf("Expected local URI %s, got %s", localURI, dialog.LocalURI)
	}

	if dialog.RemoteURI != remoteURI {
		t.Errorf("Expected remote URI %s, got %s", remoteURI, dialog.RemoteURI)
	}

	if dialog.LocalTag != localTag {
		t.Errorf("Expected local tag %s, got %s", localTag, dialog.LocalTag)
	}

	if dialog.RemoteTag != remoteTag {
		t.Errorf("Expected remote tag %s, got %s", remoteTag, dialog.RemoteTag)
	}

	if dialog.State != DialogStateEarly {
		t.Errorf("Expected state %s, got %s", DialogStateEarly, dialog.State)
	}

	if dialog.LocalCSeq != 1 {
		t.Errorf("Expected local CSeq 1, got %d", dialog.LocalCSeq)
	}
}

func TestDialogLookup(t *testing.T) {
	logger := &mockLogger{}
	dm := NewDialogManager(logger)

	callID := "test-call-id@example.com"
	localTag := "local-tag-123"
	remoteTag := "remote-tag-456"

	// Create dialog
	dialog := dm.CreateDialog(callID, "sip:local@example.com", "sip:remote@example.com", localTag, remoteTag)

	// Test lookup by dialog ID
	foundDialog := dm.GetDialog(dialog.DialogID)
	if foundDialog == nil {
		t.Error("Should find dialog by ID")
	}
	if foundDialog.DialogID != dialog.DialogID {
		t.Error("Found dialog ID mismatch")
	}

	// Test lookup by Call-ID and tags
	foundDialog = dm.FindDialog(callID, localTag, remoteTag)
	if foundDialog == nil {
		t.Error("Should find dialog by Call-ID and tags")
	}
	if foundDialog.DialogID != dialog.DialogID {
		t.Error("Found dialog ID mismatch")
	}

	// Test lookup with reversed tags (should still match)
	foundDialog = dm.FindDialog(callID, remoteTag, localTag)
	if foundDialog == nil {
		t.Error("Should find dialog with reversed tags")
	}
	if foundDialog.DialogID != dialog.DialogID {
		t.Error("Found dialog ID mismatch with reversed tags")
	}

	// Test lookup with non-existent dialog
	foundDialog = dm.FindDialog("non-existent", "tag1", "tag2")
	if foundDialog != nil {
		t.Error("Should not find non-existent dialog")
	}
}

func TestDialogStateManagement(t *testing.T) {
	logger := &mockLogger{}
	dm := NewDialogManager(logger)

	dialog := dm.CreateDialog("call-id", "local", "remote", "ltag", "rtag")

	// Test initial state
	if dialog.State != DialogStateEarly {
		t.Error("Initial state should be early")
	}

	if dialog.IsConfirmed() {
		t.Error("Dialog should not be confirmed initially")
	}

	if dialog.IsTerminated() {
		t.Error("Dialog should not be terminated initially")
	}

	// Test confirmation
	dialog.ConfirmDialog()
	if dialog.State != DialogStateConfirmed {
		t.Error("State should be confirmed")
	}

	if !dialog.IsConfirmed() {
		t.Error("Dialog should be confirmed")
	}

	// Test termination
	dm.TerminateDialog(dialog.DialogID)
	if dialog.State != DialogStateTerminated {
		t.Error("State should be terminated")
	}

	if !dialog.IsTerminated() {
		t.Error("Dialog should be terminated")
	}
}

func TestDialogCSeqManagement(t *testing.T) {
	logger := &mockLogger{}
	dm := NewDialogManager(logger)

	dialog := dm.CreateDialog("call-id", "local", "remote", "ltag", "rtag")

	// Test initial CSeq
	if dialog.LocalCSeq != 1 {
		t.Errorf("Initial local CSeq should be 1, got %d", dialog.LocalCSeq)
	}

	// Test getting next CSeq
	nextCSeq := dialog.GetNextLocalCSeq()
	if nextCSeq != 2 {
		t.Errorf("Next CSeq should be 2, got %d", nextCSeq)
	}

	if dialog.LocalCSeq != 2 {
		t.Errorf("Local CSeq should be updated to 2, got %d", dialog.LocalCSeq)
	}

	// Test updating remote CSeq
	dialog.UpdateRemoteCSeq(5)
	if dialog.RemoteCSeq != 5 {
		t.Errorf("Remote CSeq should be 5, got %d", dialog.RemoteCSeq)
	}

	// Test updating with lower CSeq (should not update)
	dialog.UpdateRemoteCSeq(3)
	if dialog.RemoteCSeq != 5 {
		t.Errorf("Remote CSeq should remain 5, got %d", dialog.RemoteCSeq)
	}
}

func TestTransactionCorrelation(t *testing.T) {
	logger := &mockLogger{}
	dm := NewDialogManager(logger)

	alegTxnID := "aleg-txn-123"
	blegTxnID := "bleg-txn-456"
	method := parser.MethodINVITE

	// Create correlation
	correlation := dm.CreateCorrelation(alegTxnID, blegTxnID, method)

	if correlation == nil {
		t.Fatal("Correlation should not be nil")
	}

	if correlation.AlegTransactionID != alegTxnID {
		t.Errorf("Expected A-leg transaction ID %s, got %s", alegTxnID, correlation.AlegTransactionID)
	}

	if correlation.BlegTransactionID != blegTxnID {
		t.Errorf("Expected B-leg transaction ID %s, got %s", blegTxnID, correlation.BlegTransactionID)
	}

	if correlation.Method != method {
		t.Errorf("Expected method %s, got %s", method, correlation.Method)
	}

	if correlation.State != CorrelationStateActive {
		t.Errorf("Expected state %s, got %s", CorrelationStateActive, correlation.State)
	}

	// Test lookup by A-leg
	foundCorrelation := dm.FindCorrelationByAleg(alegTxnID)
	if foundCorrelation == nil {
		t.Error("Should find correlation by A-leg transaction ID")
	}
	if foundCorrelation.AlegTransactionID != alegTxnID {
		t.Error("Found correlation A-leg transaction ID mismatch")
	}

	// Test lookup by B-leg
	foundCorrelation = dm.FindCorrelationByBleg(blegTxnID)
	if foundCorrelation == nil {
		t.Error("Should find correlation by B-leg transaction ID")
	}
	if foundCorrelation.BlegTransactionID != blegTxnID {
		t.Error("Found correlation B-leg transaction ID mismatch")
	}

	// Test state transitions
	if !correlation.IsActive() {
		t.Error("Correlation should be active initially")
	}

	correlation.Complete()
	if correlation.State != CorrelationStateCompleted {
		t.Error("Correlation state should be completed")
	}

	correlation.Terminate()
	if correlation.State != CorrelationStateTerminated {
		t.Error("Correlation state should be terminated")
	}

	if correlation.IsActive() {
		t.Error("Terminated correlation should not be active")
	}
}

func TestHeaderUtilities(t *testing.T) {
	tests := []struct {
		name     string
		header   string
		expected string
	}{
		{
			name:     "Simple tag extraction",
			header:   "<sip:user@example.com>;tag=abc123",
			expected: "abc123",
		},
		{
			name:     "Tag with display name",
			header:   "\"Display Name\" <sip:user@example.com>;tag=xyz789;other=param",
			expected: "xyz789",
		},
		{
			name:     "No tag",
			header:   "<sip:user@example.com>",
			expected: "",
		},
		{
			name:     "Tag at end",
			header:   "sip:user@example.com;tag=simple",
			expected: "simple",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := ExtractTagFromHeader(test.header)
			if result != test.expected {
				t.Errorf("ExtractTagFromHeader(%s) = %s, expected %s", test.header, result, test.expected)
			}
		})
	}
}

func TestURIExtraction(t *testing.T) {
	tests := []struct {
		name     string
		header   string
		expected string
	}{
		{
			name:     "URI with angle brackets",
			header:   "\"Display Name\" <sip:user@example.com>;tag=abc123",
			expected: "sip:user@example.com",
		},
		{
			name:     "Simple URI with brackets",
			header:   "<sip:user@example.com>",
			expected: "sip:user@example.com",
		},
		{
			name:     "Bare URI with parameters",
			header:   "sip:user@example.com;tag=abc123",
			expected: "sip:user@example.com",
		},
		{
			name:     "Bare URI",
			header:   "sip:user@example.com",
			expected: "sip:user@example.com",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := ExtractURIFromHeader(test.header)
			if result != test.expected {
				t.Errorf("ExtractURIFromHeader(%s) = %s, expected %s", test.header, result, test.expected)
			}
		})
	}
}

func TestHeaderBuilding(t *testing.T) {
	tests := []struct {
		name        string
		uri         string
		displayName string
		tag         string
		expected    string
	}{
		{
			name:        "URI with display name and tag",
			uri:         "sip:user@example.com",
			displayName: "John Doe",
			tag:         "abc123",
			expected:    "\"John Doe\" <sip:user@example.com>;tag=abc123",
		},
		{
			name:        "URI with tag only",
			uri:         "sip:user@example.com",
			displayName: "",
			tag:         "abc123",
			expected:    "<sip:user@example.com>;tag=abc123",
		},
		{
			name:        "URI without tag",
			uri:         "sip:user@example.com",
			displayName: "",
			tag:         "",
			expected:    "<sip:user@example.com>",
		},
		{
			name:        "URI with display name only",
			uri:         "sip:user@example.com",
			displayName: "Jane Doe",
			tag:         "",
			expected:    "\"Jane Doe\" <sip:user@example.com>",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := BuildHeaderWithTag(test.uri, test.displayName, test.tag)
			if result != test.expected {
				t.Errorf("BuildHeaderWithTag(%s, %s, %s) = %s, expected %s", 
					test.uri, test.displayName, test.tag, result, test.expected)
			}
		})
	}
}

func TestCSeqExtraction(t *testing.T) {
	tests := []struct {
		name           string
		cseqHeader     string
		expectedNumber uint32
		expectedMethod string
	}{
		{
			name:           "INVITE CSeq",
			cseqHeader:     "123 INVITE",
			expectedNumber: 123,
			expectedMethod: "INVITE",
		},
		{
			name:           "ACK CSeq",
			cseqHeader:     "1 ACK",
			expectedNumber: 1,
			expectedMethod: "ACK",
		},
		{
			name:           "BYE CSeq",
			cseqHeader:     "999 BYE",
			expectedNumber: 999,
			expectedMethod: "BYE",
		},
		{
			name:           "Invalid CSeq",
			cseqHeader:     "invalid",
			expectedNumber: 0,
			expectedMethod: "",
		},
		{
			name:           "Empty CSeq",
			cseqHeader:     "",
			expectedNumber: 0,
			expectedMethod: "",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			number := ExtractCSeqNumber(test.cseqHeader)
			if number != test.expectedNumber {
				t.Errorf("ExtractCSeqNumber(%s) = %d, expected %d", test.cseqHeader, number, test.expectedNumber)
			}

			method := ExtractCSeqMethod(test.cseqHeader)
			if method != test.expectedMethod {
				t.Errorf("ExtractCSeqMethod(%s) = %s, expected %s", test.cseqHeader, method, test.expectedMethod)
			}
		})
	}
}

func TestB2BUADialogIntegration(t *testing.T) {
	b2bua := createTestB2BUA()
	defer b2bua.Stop()

	invite := createTestInvite()
	calleeURI := "sip:callee@example.com"

	// Create session
	session, err := b2bua.CreateSession(invite, calleeURI)
	if err != nil {
		t.Fatalf("Failed to create session: %v", err)
	}

	// Verify dialogs are created
	if session.CallerLeg.DialogID == "" {
		t.Error("Caller leg should have dialog ID")
	}

	if session.CalleeLeg.DialogID == "" {
		t.Error("Callee leg should have dialog ID")
	}

	// Verify dialogs exist in dialog manager
	callerDialog := b2bua.dialogManager.GetDialog(session.CallerLeg.DialogID)
	if callerDialog == nil {
		t.Error("Caller dialog should exist in dialog manager")
	}

	calleeDialog := b2bua.dialogManager.GetDialog(session.CalleeLeg.DialogID)
	if calleeDialog == nil {
		t.Error("Callee dialog should exist in dialog manager")
	}

	// Verify dialog properties
	if callerDialog.CallID != session.CallerLeg.CallID {
		t.Error("Caller dialog Call-ID mismatch")
	}

	if calleeDialog.CallID != session.CalleeLeg.CallID {
		t.Error("Callee dialog Call-ID mismatch")
	}

	// Verify different Call-IDs for different legs
	if callerDialog.CallID == calleeDialog.CallID {
		t.Error("Caller and callee dialogs should have different Call-IDs")
	}
}