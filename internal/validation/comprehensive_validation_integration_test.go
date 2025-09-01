package validation

import (
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/zurustar/xylitol2/internal/parser"
)

// TestValidationPriorityOrder tests that validators are executed in the correct priority order:
// syntax → session-timer → authentication
func TestValidationPriorityOrder(t *testing.T) {
	tests := []struct {
		name                string
		setupValidators     func() *MessageProcessor
		expectedOrder       []string
		description         string
	}{
		{
			name: "All validators in correct order",
			setupValidators: func() *MessageProcessor {
				processor := NewMessageProcessor()
				// Add in reverse order to test sorting
				processor.AddValidator(NewAuthValidator(true, "example.com"))      // Priority 20
				processor.AddValidator(NewSessionTimerValidator(90, 1800, true))   // Priority 10
				processor.AddValidator(NewSyntaxValidator())                       // Priority 1
				return processor
			},
			expectedOrder: []string{"SyntaxValidator", "SessionTimerValidator", "AuthValidator"},
			description:   "Validators should be ordered by priority: syntax (1), session-timer (10), auth (20)",
		},
		{
			name: "Validators added in random order",
			setupValidators: func() *MessageProcessor {
				processor := NewMessageProcessor()
				processor.AddValidator(NewSessionTimerValidator(90, 1800, true))   // Priority 10
				processor.AddValidator(NewSyntaxValidator())                       // Priority 1
				processor.AddValidator(NewAuthValidator(true, "example.com"))      // Priority 20
				return processor
			},
			expectedOrder: []string{"SyntaxValidator", "SessionTimerValidator", "AuthValidator"},
			description:   "Priority order should be maintained regardless of addition order",
		},
		{
			name: "Only syntax and auth validators",
			setupValidators: func() *MessageProcessor {
				processor := NewMessageProcessor()
				processor.AddValidator(NewAuthValidator(true, "example.com"))      // Priority 20
				processor.AddValidator(NewSyntaxValidator())                       // Priority 1
				return processor
			},
			expectedOrder: []string{"SyntaxValidator", "AuthValidator"},
			description:   "Should work with subset of validators",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			processor := tt.setupValidators()
			validators := processor.GetValidators()

			if len(validators) != len(tt.expectedOrder) {
				t.Errorf("Expected %d validators, got %d", len(tt.expectedOrder), len(validators))
			}

			for i, expectedName := range tt.expectedOrder {
				if i >= len(validators) {
					t.Errorf("Missing validator at position %d, expected %s", i, expectedName)
					continue
				}
				if validators[i].Name() != expectedName {
					t.Errorf("Position %d: expected %s, got %s", i, expectedName, validators[i].Name())
				}
			}
		})
	}
}

// TestValidationErrorResponseGeneration tests that proper error responses are generated
// for each validation type
func TestValidationErrorResponseGeneration(t *testing.T) {
	tests := []struct {
		name           string
		setupRequest   func() *parser.SIPMessage
		setupProcessor func() *MessageProcessor
		expectedCode   int
		expectedReason string
		expectedHeaders map[string]string
		description    string
	}{
		{
			name: "Syntax validation failure - missing method",
			setupRequest: func() *parser.SIPMessage {
				// Create malformed request with empty method
				req := &parser.SIPMessage{}
				req.StartLine = &parser.RequestLine{Method: "", RequestURI: "sip:test@example.com", Version: "SIP/2.0"}
				req.Headers = make(map[string][]string)
				return req
			},
			setupProcessor: func() *MessageProcessor {
				processor := NewMessageProcessor()
				processor.AddValidator(NewSyntaxValidator())
				return processor
			},
			expectedCode:   400,
			expectedReason: "Bad Request",
			expectedHeaders: map[string]string{
				// Content-Length will be set based on body content
			},
			description: "Should return 400 Bad Request for syntax errors",
		},
		{
			name: "Session-Timer validation failure - extension required",
			setupRequest: func() *parser.SIPMessage {
				req := parser.NewRequestMessage("INVITE", "sip:test@example.com")
				req.SetHeader("Via", "SIP/2.0/UDP 192.168.1.1:5060;branch=z9hG4bK123")
				req.SetHeader("From", "sip:alice@example.com;tag=abc123")
				req.SetHeader("To", "sip:bob@example.com")
				req.SetHeader("Call-ID", "call123@example.com")
				req.SetHeader("CSeq", "1 INVITE")
				// No Session-Timer support
				return req
			},
			setupProcessor: func() *MessageProcessor {
				processor := NewMessageProcessor()
				processor.AddValidator(NewSyntaxValidator())
				processor.AddValidator(NewSessionTimerValidator(90, 1800, true))
				return processor
			},
			expectedCode:   421,
			expectedReason: "Extension Required",
			expectedHeaders: map[string]string{
				"Require": "timer",
			},
			description: "Should return 421 Extension Required for missing Session-Timer support",
		},
		{
			name: "Session-Timer validation failure - interval too small",
			setupRequest: func() *parser.SIPMessage {
				req := parser.NewRequestMessage("INVITE", "sip:test@example.com")
				req.SetHeader("Via", "SIP/2.0/UDP 192.168.1.1:5060;branch=z9hG4bK123")
				req.SetHeader("From", "sip:alice@example.com;tag=abc123")
				req.SetHeader("To", "sip:bob@example.com")
				req.SetHeader("Call-ID", "call123@example.com")
				req.SetHeader("CSeq", "1 INVITE")
				req.SetHeader("Session-Expires", "60") // Less than minimum 90
				return req
			},
			setupProcessor: func() *MessageProcessor {
				processor := NewMessageProcessor()
				processor.AddValidator(NewSyntaxValidator())
				processor.AddValidator(NewSessionTimerValidator(90, 1800, false))
				return processor
			},
			expectedCode:   422,
			expectedReason: "Session Interval Too Small",
			expectedHeaders: map[string]string{
				"Min-SE": "90",
			},
			description: "Should return 422 Session Interval Too Small for small Session-Expires",
		},
		{
			name: "Authentication validation failure - no authorization",
			setupRequest: func() *parser.SIPMessage {
				req := parser.NewRequestMessage("REGISTER", "sip:example.com")
				req.SetHeader("Via", "SIP/2.0/UDP 192.168.1.1:5060;branch=z9hG4bK123")
				req.SetHeader("From", "sip:alice@example.com;tag=abc123")
				req.SetHeader("To", "sip:alice@example.com")
				req.SetHeader("Call-ID", "call123@example.com")
				req.SetHeader("CSeq", "1 REGISTER")
				// No Authorization header
				return req
			},
			setupProcessor: func() *MessageProcessor {
				processor := NewMessageProcessor()
				processor.AddValidator(NewSyntaxValidator())
				processor.AddValidator(NewAuthValidator(true, "example.com"))
				return processor
			},
			expectedCode:   401,
			expectedReason: "Unauthorized",
			expectedHeaders: map[string]string{
				"WWW-Authenticate": "Digest realm=\"example.com\"", // Partial match
			},
			description: "Should return 401 Unauthorized for missing authentication",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			processor := tt.setupProcessor()
			req := tt.setupRequest()

			resp, err := processor.ProcessRequest(req)
			if err != nil {
				t.Fatalf("Expected no error, got %v", err)
			}

			if resp == nil {
				t.Fatal("Expected error response, got nil")
			}

			// Check status code
			if resp.GetStatusCode() != tt.expectedCode {
				t.Errorf("Expected status code %d, got %d", tt.expectedCode, resp.GetStatusCode())
			}

			// Check reason phrase
			if resp.GetReasonPhrase() != tt.expectedReason {
				t.Errorf("Expected reason '%s', got '%s'", tt.expectedReason, resp.GetReasonPhrase())
			}

			// Check expected headers
			for headerName, expectedValue := range tt.expectedHeaders {
				actualValue := resp.GetHeader(headerName)
				if headerName == "WWW-Authenticate" {
					// For WWW-Authenticate, just check it contains the realm
					if !strings.Contains(actualValue, expectedValue) {
						t.Errorf("Expected header %s to contain '%s', got '%s'", headerName, expectedValue, actualValue)
					}
				} else {
					if actualValue != expectedValue {
						t.Errorf("Expected header %s='%s', got '%s'", headerName, expectedValue, actualValue)
					}
				}
			}
		})
	}
}

// TestValidationChainPerformance tests performance of validation chain processing
func TestValidationChainPerformance(t *testing.T) {
	// Create processor with all validators
	processor := NewMessageProcessor()
	processor.AddValidator(NewSyntaxValidator())
	processor.AddValidator(NewSessionTimerValidator(90, 1800, true))
	processor.AddValidator(NewAuthValidator(true, "example.com"))

	// Create test request
	req := parser.NewRequestMessage("INVITE", "sip:test@example.com")
	req.SetHeader("Via", "SIP/2.0/UDP 192.168.1.1:5060;branch=z9hG4bK123")
	req.SetHeader("From", "sip:alice@example.com;tag=abc123")
	req.SetHeader("To", "sip:bob@example.com")
	req.SetHeader("Call-ID", "call123@example.com")
	req.SetHeader("CSeq", "1 INVITE")
	req.SetHeader("Supported", "timer")
	req.SetHeader("Authorization", `Digest username="alice", realm="example.com", nonce="abc123", uri="sip:test@example.com", response="def456"`)

	// Performance test parameters
	numIterations := 10000
	maxDurationPerRequest := 1 * time.Millisecond

	start := time.Now()

	for i := 0; i < numIterations; i++ {
		_, err := processor.ProcessRequest(req)
		if err != nil {
			t.Fatalf("Iteration %d failed: %v", i, err)
		}
	}

	duration := time.Since(start)
	avgDuration := duration / time.Duration(numIterations)

	t.Logf("Processed %d requests in %v (avg: %v per request)", numIterations, duration, avgDuration)

	if avgDuration > maxDurationPerRequest {
		t.Errorf("Average processing time %v exceeds maximum %v", avgDuration, maxDurationPerRequest)
	}

	// Test concurrent processing
	t.Run("Concurrent processing", func(t *testing.T) {
		numGoroutines := 10
		requestsPerGoroutine := 1000

		var wg sync.WaitGroup
		start := time.Now()

		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func(goroutineID int) {
				defer wg.Done()
				for j := 0; j < requestsPerGoroutine; j++ {
					// Create unique request for each goroutine/iteration
					testReq := parser.NewRequestMessage("INVITE", fmt.Sprintf("sip:test%d-%d@example.com", goroutineID, j))
					testReq.SetHeader("Via", fmt.Sprintf("SIP/2.0/UDP 192.168.1.1:5060;branch=z9hG4bK%d-%d", goroutineID, j))
					testReq.SetHeader("From", fmt.Sprintf("sip:alice%d@example.com;tag=abc%d", goroutineID, j))
					testReq.SetHeader("To", fmt.Sprintf("sip:bob%d@example.com", goroutineID))
					testReq.SetHeader("Call-ID", fmt.Sprintf("call%d-%d@example.com", goroutineID, j))
					testReq.SetHeader("CSeq", fmt.Sprintf("%d INVITE", j+1))
					testReq.SetHeader("Supported", "timer")
					testReq.SetHeader("Authorization", `Digest username="alice", realm="example.com", nonce="abc123", uri="sip:test@example.com", response="def456"`)

					_, err := processor.ProcessRequest(testReq)
					if err != nil {
						t.Errorf("Goroutine %d, iteration %d failed: %v", goroutineID, j, err)
					}
				}
			}(i)
		}

		wg.Wait()
		duration := time.Since(start)
		totalRequests := numGoroutines * requestsPerGoroutine
		avgDuration := duration / time.Duration(totalRequests)

		t.Logf("Concurrent test: processed %d requests in %v (avg: %v per request)", totalRequests, duration, avgDuration)

		if avgDuration > maxDurationPerRequest {
			t.Errorf("Concurrent average processing time %v exceeds maximum %v", avgDuration, maxDurationPerRequest)
		}
	})
}

// TestValidationChainStopsOnFirstFailureComprehensive tests that validation chain stops
// processing on the first validation failure (comprehensive version)
func TestValidationChainStopsOnFirstFailureComprehensive(t *testing.T) {
	// Create a custom validator that tracks if it was called
	calledValidators := make(map[string]bool)
	var mu sync.Mutex

	trackingValidator := &trackingValidator{
		name:     "TrackingValidator",
		priority: 25, // After auth validator
		applies:  true,
		valid:    true,
		onValidate: func() {
			mu.Lock()
			calledValidators["TrackingValidator"] = true
			mu.Unlock()
		},
	}

	processor := NewMessageProcessor()
	processor.AddValidator(NewSyntaxValidator())
	processor.AddValidator(NewSessionTimerValidator(90, 1800, true))
	processor.AddValidator(NewAuthValidator(true, "example.com"))
	processor.AddValidator(trackingValidator)

	// Create request that will fail Session-Timer validation
	req := parser.NewRequestMessage("INVITE", "sip:test@example.com")
	req.SetHeader("Via", "SIP/2.0/UDP 192.168.1.1:5060;branch=z9hG4bK123")
	req.SetHeader("From", "sip:alice@example.com;tag=abc123")
	req.SetHeader("To", "sip:bob@example.com")
	req.SetHeader("Call-ID", "call123@example.com")
	req.SetHeader("CSeq", "1 INVITE")
	// No Session-Timer support - should fail at SessionTimerValidator

	resp, err := processor.ProcessRequest(req)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if resp == nil {
		t.Fatal("Expected error response")
	}

	// Should fail at Session-Timer validation (421)
	if resp.GetStatusCode() != 421 {
		t.Errorf("Expected status code 421, got %d", resp.GetStatusCode())
	}

	// TrackingValidator should NOT have been called
	mu.Lock()
	wasCalled := calledValidators["TrackingValidator"]
	mu.Unlock()

	if wasCalled {
		t.Error("TrackingValidator should not have been called after Session-Timer validation failure")
	}
}

// TestValidationPriorityWithRealScenarios tests validation priority with realistic SIP scenarios
func TestValidationPriorityWithRealScenarios(t *testing.T) {
	tests := []struct {
		name            string
		message         string
		expectedCode    int
		expectedHeaders map[string]string
		description     string
	}{
		{
			name: "Malformed INVITE - syntax error first",
			message: "INVITE sip:test@example.com SIP/2.0\r\n" +
				"Via: SIP/2.0/UDP 192.168.1.1:5060;branch=z9hG4bK123\r\n" +
				"From: sip:alice@example.com;tag=abc123\r\n" +
				// Missing To header - syntax error
				"Call-ID: call123@example.com\r\n" +
				"CSeq: 1 INVITE\r\n" +
				"Content-Length: 0\r\n\r\n",
			expectedCode: 400,
			expectedHeaders: map[string]string{
				// Content-Length will be set based on body content
			},
			description: "Syntax validation should catch missing headers before other validations",
		},
		{
			name: "Valid syntax, missing Session-Timer",
			message: "INVITE sip:test@example.com SIP/2.0\r\n" +
				"Via: SIP/2.0/UDP 192.168.1.1:5060;branch=z9hG4bK123\r\n" +
				"From: sip:alice@example.com;tag=abc123\r\n" +
				"To: sip:bob@example.com\r\n" +
				"Call-ID: call123@example.com\r\n" +
				"CSeq: 1 INVITE\r\n" +
				"Content-Length: 0\r\n\r\n",
			expectedCode: 421,
			expectedHeaders: map[string]string{
				"Require": "timer",
			},
			description: "Session-Timer validation should run after syntax validation passes",
		},
		{
			name: "Valid syntax and Session-Timer, missing auth",
			message: "INVITE sip:test@example.com SIP/2.0\r\n" +
				"Via: SIP/2.0/UDP 192.168.1.1:5060;branch=z9hG4bK123\r\n" +
				"From: sip:alice@example.com;tag=abc123\r\n" +
				"To: sip:bob@example.com\r\n" +
				"Call-ID: call123@example.com\r\n" +
				"CSeq: 1 INVITE\r\n" +
				"Supported: timer\r\n" +
				"Content-Length: 0\r\n\r\n",
			expectedCode: 401,
			expectedHeaders: map[string]string{
				"WWW-Authenticate": "Digest realm=", // Partial match
			},
			description: "Auth validation should run after syntax and Session-Timer validations pass",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create processor with all validators
			processor := NewMessageProcessor()
			processor.AddValidator(NewSyntaxValidator())
			processor.AddValidator(NewSessionTimerValidator(90, 1800, true))
			processor.AddValidator(NewAuthValidator(true, "sip-server"))

			// Parse the message
			messageParser := parser.NewParser()
			req, err := messageParser.Parse([]byte(tt.message))
			if err != nil {
				t.Fatalf("Failed to parse message: %v", err)
			}

			// Process the request
			resp, err := processor.ProcessRequest(req)
			if err != nil {
				t.Fatalf("Expected no error, got %v", err)
			}

			if resp == nil {
				t.Fatal("Expected error response, got nil")
			}

			// Check status code
			if resp.GetStatusCode() != tt.expectedCode {
				t.Errorf("Expected status code %d, got %d", tt.expectedCode, resp.GetStatusCode())
			}

			// Check expected headers
			for headerName, expectedValue := range tt.expectedHeaders {
				actualValue := resp.GetHeader(headerName)
				if !strings.Contains(actualValue, expectedValue) {
					t.Errorf("Expected header %s to contain '%s', got '%s'", headerName, expectedValue, actualValue)
				}
			}
		})
	}
}

// trackingValidator is a validator that tracks when it's called
type trackingValidator struct {
	name       string
	priority   int
	applies    bool
	valid      bool
	onValidate func()
}

func (tv *trackingValidator) Validate(req *parser.SIPMessage) ValidationResult {
	if tv.onValidate != nil {
		tv.onValidate()
	}
	return ValidationResult{
		Valid: tv.valid,
		Context: map[string]interface{}{
			"validator": tv.name,
		},
	}
}

func (tv *trackingValidator) Priority() int {
	return tv.priority
}

func (tv *trackingValidator) Name() string {
	return tv.name
}

func (tv *trackingValidator) AppliesTo(req *parser.SIPMessage) bool {
	return tv.applies
}