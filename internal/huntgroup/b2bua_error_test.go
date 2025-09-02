package huntgroup

import (
	"fmt"
	"testing"

	"github.com/zurustar/xylitol2/internal/parser"
)

func TestHuntGroupErrorAggregator(t *testing.T) {
	aggregator := NewHuntGroupErrorAggregator("test-session", 3)

	// Test initial state
	if aggregator.IsComplete() {
		t.Error("Aggregator should not be complete initially")
	}

	statusCode, reasonPhrase := aggregator.GetBestErrorResponse()
	if statusCode != 0 || reasonPhrase != "" {
		t.Error("Should not have error response when not complete")
	}

	// Add responses
	aggregator.AddResponse("sip:member1@example.com", 486) // Busy
	aggregator.AddResponse("sip:member2@example.com", 480) // Unavailable
	aggregator.AddResponse("sip:member3@example.com", 404) // Not Found

	// Test completion
	if !aggregator.IsComplete() {
		t.Error("Aggregator should be complete after all responses")
	}

	// Test best error response (should prioritize busy)
	statusCode, reasonPhrase = aggregator.GetBestErrorResponse()
	if statusCode != 486 || reasonPhrase != "Busy Here" {
		t.Errorf("Expected 486 Busy Here, got %d %s", statusCode, reasonPhrase)
	}

	// Verify member categorization
	if len(aggregator.BusyMembers) != 1 {
		t.Errorf("Expected 1 busy member, got %d", len(aggregator.BusyMembers))
	}

	if len(aggregator.UnavailableMembers) != 1 {
		t.Errorf("Expected 1 unavailable member, got %d", len(aggregator.UnavailableMembers))
	}

	if len(aggregator.FailedMembers) != 1 {
		t.Errorf("Expected 1 failed member, got %d", len(aggregator.FailedMembers))
	}
}

func TestHuntGroupErrorAggregatorPriority(t *testing.T) {
	tests := []struct {
		name           string
		responses      []int
		expectedStatus int
		expectedReason string
	}{
		{
			name:           "All busy",
			responses:      []int{486, 486, 486},
			expectedStatus: 486,
			expectedReason: "Busy Here",
		},
		{
			name:           "All unavailable",
			responses:      []int{480, 480, 503},
			expectedStatus: 480,
			expectedReason: "Temporarily Unavailable",
		},
		{
			name:           "Mixed errors",
			responses:      []int{404, 500, 403},
			expectedStatus: 500,
			expectedReason: "Internal Server Error",
		},
		{
			name:           "Busy takes priority over unavailable",
			responses:      []int{480, 486, 503},
			expectedStatus: 486,
			expectedReason: "Busy Here",
		},
		{
			name:           "Unavailable takes priority over other errors",
			responses:      []int{404, 480, 500},
			expectedStatus: 480,
			expectedReason: "Temporarily Unavailable",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			aggregator := NewHuntGroupErrorAggregator("test-session", len(test.responses))

			for i, statusCode := range test.responses {
				memberURI := fmt.Sprintf("sip:member%d@example.com", i+1)
				aggregator.AddResponse(memberURI, statusCode)
			}

			statusCode, reasonPhrase := aggregator.GetBestErrorResponse()
			if statusCode != test.expectedStatus || reasonPhrase != test.expectedReason {
				t.Errorf("Expected %d %s, got %d %s", 
					test.expectedStatus, test.expectedReason, statusCode, reasonPhrase)
			}
		})
	}
}

func TestHuntGroupErrorHandling(t *testing.T) {
	b2bua := createTestB2BUA()
	defer b2bua.Stop()

	invite := createTestInvite()

	// Test different error types
	errorTypes := []struct {
		errorType      HuntGroupErrorType
		expectedStatus int
	}{
		{HuntGroupErrorAllBusy, 486},
		{HuntGroupErrorAllUnavailable, 480},
		{HuntGroupErrorNoMembers, 404},
		{HuntGroupErrorTimeout, 408},
		{HuntGroupErrorInternalError, 500},
	}

	for _, test := range errorTypes {
		// Create a new session for each test
		testSession, err := b2bua.CreateSession(invite, "sip:callee@example.com")
		if err != nil {
			t.Fatalf("Failed to create test session: %v", err)
		}

		// Store session ID before handling error
		sessionID := testSession.SessionID
		
		err = b2bua.HandleHuntGroupError(sessionID, test.errorType, "test error")
		if err != nil {
			t.Errorf("Failed to handle error type %s: %v", test.errorType, err)
		}

		// Verify session status was set to failed before it was ended
		// Since EndSession removes the session from the map, we check the original session object
		// The HandleHuntGroupError should have updated the session status before calling EndSession
		if testSession.GetStatus() != B2BUAStatusFailed {
			t.Errorf("Expected session status to be failed for error type %s, got %s", test.errorType, testSession.GetStatus())
		}
	}
}

func TestCancelPendingLegs(t *testing.T) {
	b2bua := createTestB2BUA()
	defer b2bua.Stop()

	invite := createTestInvite()
	huntGroup := &HuntGroup{
		ID:        1,
		Extension: "100",
		Strategy:  StrategySimultaneous,
		Enabled:   true,
	}

	session, err := b2bua.CreateHuntGroupSession(invite, huntGroup)
	if err != nil {
		t.Fatalf("Failed to create hunt group session: %v", err)
	}

	// Add multiple pending legs
	leg1, err := b2bua.AddPendingLeg(session.SessionID, "sip:member1@example.com")
	if err != nil {
		t.Fatalf("Failed to add pending leg 1: %v", err)
	}

	leg2, err := b2bua.AddPendingLeg(session.SessionID, "sip:member2@example.com")
	if err != nil {
		t.Fatalf("Failed to add pending leg 2: %v", err)
	}

	leg3, err := b2bua.AddPendingLeg(session.SessionID, "sip:member3@example.com")
	if err != nil {
		t.Fatalf("Failed to add pending leg 3: %v", err)
	}

	// Verify all legs are pending
	if len(session.PendingLegs) != 3 {
		t.Errorf("Expected 3 pending legs, got %d", len(session.PendingLegs))
	}

	// Cancel all except leg2
	err = b2bua.CancelPendingLegs(session.SessionID, leg2.LegID)
	if err != nil {
		t.Fatalf("Failed to cancel pending legs: %v", err)
	}

	// Verify only leg2 remains
	if len(session.PendingLegs) != 1 {
		t.Errorf("Expected 1 pending leg after cancellation, got %d", len(session.PendingLegs))
	}

	remainingLeg := session.GetPendingLeg(leg2.LegID)
	if remainingLeg == nil {
		t.Error("Leg2 should remain after cancellation")
	}

	// Verify other legs are cancelled
	if leg1.GetStatus() != CallLegStatusCancelled {
		t.Error("Leg1 should be cancelled")
	}

	if leg3.GetStatus() != CallLegStatusCancelled {
		t.Error("Leg3 should be cancelled")
	}

	// Verify leg2 is not cancelled
	if leg2.GetStatus() == CallLegStatusCancelled {
		t.Error("Leg2 should not be cancelled")
	}
}

func TestAddPendingLeg(t *testing.T) {
	b2bua := createTestB2BUA()
	defer b2bua.Stop()

	invite := createTestInvite()
	huntGroup := &HuntGroup{
		ID:        1,
		Extension: "100",
		Strategy:  StrategySimultaneous,
		Enabled:   true,
	}

	session, err := b2bua.CreateHuntGroupSession(invite, huntGroup)
	if err != nil {
		t.Fatalf("Failed to create hunt group session: %v", err)
	}

	memberURI := "sip:member@example.com"
	leg, err := b2bua.AddPendingLeg(session.SessionID, memberURI)
	if err != nil {
		t.Fatalf("Failed to add pending leg: %v", err)
	}

	// Verify leg properties
	if leg.LegID == "" {
		t.Error("Leg ID should not be empty")
	}

	if leg.CallID == "" {
		t.Error("Call ID should not be empty")
	}

	if leg.FromTag == "" {
		t.Error("From tag should not be empty")
	}

	if leg.ToURI != "<sip:member@example.com>" {
		t.Errorf("Expected To URI '<sip:member@example.com>', got '%s'", leg.ToURI)
	}

	if leg.GetStatus() != CallLegStatusInitial {
		t.Errorf("Expected initial status, got %s", leg.GetStatus())
	}

	// Verify leg is added to session
	if len(session.PendingLegs) != 1 {
		t.Errorf("Expected 1 pending leg, got %d", len(session.PendingLegs))
	}

	foundLeg := session.GetPendingLeg(leg.LegID)
	if foundLeg == nil {
		t.Error("Should find pending leg in session")
	}

	// Verify session indices are updated
	foundSession, err := b2bua.GetSessionByLegID(leg.LegID)
	if err != nil {
		t.Errorf("Should find session by leg ID: %v", err)
	}

	if foundSession.SessionID != session.SessionID {
		t.Error("Found session should match original session")
	}
}

func TestHandleMemberResponse(t *testing.T) {
	b2bua := createTestB2BUA()
	defer b2bua.Stop()

	invite := createTestInvite()
	huntGroup := &HuntGroup{
		ID:        1,
		Extension: "100",
		Strategy:  StrategySimultaneous,
		Enabled:   true,
	}

	session, err := b2bua.CreateHuntGroupSession(invite, huntGroup)
	if err != nil {
		t.Fatalf("Failed to create hunt group session: %v", err)
	}

	// Add pending legs
	leg1, _ := b2bua.AddPendingLeg(session.SessionID, "sip:member1@example.com")
	leg2, _ := b2bua.AddPendingLeg(session.SessionID, "sip:member2@example.com")
	leg3, _ := b2bua.AddPendingLeg(session.SessionID, "sip:member3@example.com")

	aggregator := NewHuntGroupErrorAggregator(session.SessionID, 3)

	// Test successful response (200 OK)
	successResponse := parser.NewResponseMessage(200, "OK")
	successResponse.SetHeader(parser.HeaderCallID, leg1.CallID)
	successResponse.SetHeader(parser.HeaderFrom, leg1.FromURI)
	successResponse.SetHeader(parser.HeaderTo, leg1.ToURI)

	err = b2bua.HandleMemberResponse(session.SessionID, leg1.LegID, successResponse, aggregator)
	if err != nil {
		t.Errorf("Failed to handle successful response: %v", err)
	}

	// Verify leg1 becomes the answered leg
	if session.AnsweredLegID != leg1.LegID {
		t.Error("Leg1 should be the answered leg")
	}

	if session.CalleeLeg == nil || session.CalleeLeg.LegID != leg1.LegID {
		t.Error("Callee leg should be set to leg1")
	}

	// Verify other legs are cancelled
	if leg2.GetStatus() != CallLegStatusCancelled {
		t.Error("Leg2 should be cancelled")
	}

	if leg3.GetStatus() != CallLegStatusCancelled {
		t.Error("Leg3 should be cancelled")
	}

	// Verify pending legs are cleared (except answered leg)
	if len(session.PendingLegs) != 0 {
		t.Errorf("Expected 0 pending legs after answer, got %d", len(session.PendingLegs))
	}
}

func TestErrorResponseMapping(t *testing.T) {
	b2bua := createTestB2BUA()
	defer b2bua.Stop()

	tests := []struct {
		statusCode    int
		expectedError HuntGroupErrorType
	}{
		{486, HuntGroupErrorAllBusy},
		{600, HuntGroupErrorAllBusy},
		{480, HuntGroupErrorAllUnavailable},
		{503, HuntGroupErrorAllUnavailable},
		{408, HuntGroupErrorTimeout},
		{404, HuntGroupErrorNoMembers},
		{500, HuntGroupErrorInternalError},
		{403, HuntGroupErrorInternalError}, // Default case
	}

	for _, test := range tests {
		errorType := b2bua.mapStatusToErrorType(test.statusCode)
		if errorType != test.expectedError {
			t.Errorf("Status code %d should map to %s, got %s", 
				test.statusCode, test.expectedError, errorType)
		}
	}
}