package validation

import (
	"testing"

	"github.com/zurustar/xylitol2/internal/parser"
)

// TestValidationIntegrationWithMessageParsing tests that validation works correctly
// with parsed SIP messages from the transport layer
func TestValidationIntegrationWithMessageParsing(t *testing.T) {
	tests := []struct {
		name         string
		rawMessage   string
		expectedCode int
		description  string
	}{
		{
			name: "Complete valid INVITE with all validations passing",
			rawMessage: "INVITE sip:test@example.com SIP/2.0\r\n" +
				"Via: SIP/2.0/UDP 192.168.1.1:5060;branch=z9hG4bK123\r\n" +
				"From: sip:alice@example.com;tag=abc123\r\n" +
				"To: sip:bob@example.com\r\n" +
				"Call-ID: call123@example.com\r\n" +
				"CSeq: 1 INVITE\r\n" +
				"Supported: timer\r\n" +
				"Authorization: Digest username=\"alice\", realm=\"example.com\", nonce=\"abc123\", uri=\"sip:test@example.com\", response=\"def456\"\r\n" +
				"Content-Length: 0\r\n\r\n",
			expectedCode: 0, // No error response expected
			description:  "Valid request should pass all validations",
		},
		{
			name: "INVITE with syntax error",
			rawMessage: "INVITE sip:test@example.com SIP/2.0\r\n" +
				"Via: SIP/2.0/UDP 192.168.1.1:5060;branch=z9hG4bK123\r\n" +
				"From: sip:alice@example.com;tag=abc123\r\n" +
				// Missing To header - syntax error
				"Call-ID: call123@example.com\r\n" +
				"CSeq: 1 INVITE\r\n" +
				"Content-Length: 0\r\n\r\n",
			expectedCode: 400,
			description:  "Should fail syntax validation first",
		},
		{
			name: "INVITE without Session-Timer support",
			rawMessage: "INVITE sip:test@example.com SIP/2.0\r\n" +
				"Via: SIP/2.0/UDP 192.168.1.1:5060;branch=z9hG4bK123\r\n" +
				"From: sip:alice@example.com;tag=abc123\r\n" +
				"To: sip:bob@example.com\r\n" +
				"Call-ID: call123@example.com\r\n" +
				"CSeq: 1 INVITE\r\n" +
				"Content-Length: 0\r\n\r\n",
			expectedCode: 421,
			description:  "Should fail Session-Timer validation after syntax passes",
		},
		{
			name: "INVITE with Session-Timer but no auth",
			rawMessage: "INVITE sip:test@example.com SIP/2.0\r\n" +
				"Via: SIP/2.0/UDP 192.168.1.1:5060;branch=z9hG4bK123\r\n" +
				"From: sip:alice@example.com;tag=abc123\r\n" +
				"To: sip:bob@example.com\r\n" +
				"Call-ID: call123@example.com\r\n" +
				"CSeq: 1 INVITE\r\n" +
				"Supported: timer\r\n" +
				"Content-Length: 0\r\n\r\n",
			expectedCode: 401,
			description:  "Should pass Session-Timer but fail auth validation",
		},
		{
			name: "REGISTER without auth",
			rawMessage: "REGISTER sip:example.com SIP/2.0\r\n" +
				"Via: SIP/2.0/UDP 192.168.1.1:5060;branch=z9hG4bK123\r\n" +
				"From: sip:alice@example.com;tag=abc123\r\n" +
				"To: sip:alice@example.com\r\n" +
				"Call-ID: call123@example.com\r\n" +
				"CSeq: 1 REGISTER\r\n" +
				"Content-Length: 0\r\n\r\n",
			expectedCode: 401,
			description:  "REGISTER should skip Session-Timer but fail auth",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create processor with all validators (simulating server setup)
			processor := NewMessageProcessor()
			processor.AddValidator(NewSyntaxValidator())
			processor.AddValidator(NewSessionTimerValidator(90, 1800, true))
			processor.AddValidator(NewAuthValidator(true, "example.com"))

			// Parse the raw message (simulating transport layer)
			messageParser := parser.NewParser()
			req, err := messageParser.Parse([]byte(tt.rawMessage))
			if err != nil {
				t.Fatalf("Failed to parse message: %v", err)
			}

			// Process through validation chain
			resp, err := processor.ProcessRequest(req)
			if err != nil {
				t.Fatalf("Expected no error, got %v", err)
			}

			if tt.expectedCode == 0 {
				// No error response expected
				if resp != nil {
					t.Errorf("Expected no error response, got status %d", resp.GetStatusCode())
				}
			} else {
				// Error response expected
				if resp == nil {
					t.Fatal("Expected error response, got nil")
				}
				if resp.GetStatusCode() != tt.expectedCode {
					t.Errorf("Expected status code %d, got %d", tt.expectedCode, resp.GetStatusCode())
				}
			}
		})
	}
}

// TestValidationChainWithTransactionManagement tests that validation works correctly
// in the context of transaction management
func TestValidationChainWithTransactionManagement(t *testing.T) {
	// Create processor with all validators
	processor := NewMessageProcessor()
	processor.AddValidator(NewSyntaxValidator())
	processor.AddValidator(NewSessionTimerValidator(90, 1800, true))
	processor.AddValidator(NewAuthValidator(true, "sip-server"))

	// Test different transaction scenarios
	tests := []struct {
		name        string
		method      string
		callID      string
		cseq        string
		expectedErr bool
		description string
	}{
		{
			name:        "INVITE transaction",
			method:      "INVITE",
			callID:      "call-123@example.com",
			cseq:        "1 INVITE",
			expectedErr: true, // Will fail Session-Timer validation
			description: "INVITE should be validated for Session-Timer",
		},
		{
			name:        "REGISTER transaction",
			method:      "REGISTER",
			callID:      "reg-456@example.com",
			cseq:        "1 REGISTER",
			expectedErr: true, // Will fail auth validation
			description: "REGISTER should skip Session-Timer but check auth",
		},
		{
			name:        "OPTIONS transaction",
			method:      "OPTIONS",
			callID:      "opt-789@example.com",
			cseq:        "1 OPTIONS",
			expectedErr: false, // OPTIONS doesn't require Session-Timer or auth in this test
			description: "OPTIONS should pass validation",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create request message
			req := parser.NewRequestMessage(tt.method, "sip:test@example.com")
			req.SetHeader("Via", "SIP/2.0/UDP 192.168.1.1:5060;branch=z9hG4bK123")
			req.SetHeader("From", "sip:alice@example.com;tag=abc123")
			req.SetHeader("To", "sip:bob@example.com")
			req.SetHeader("Call-ID", tt.callID)
			req.SetHeader("CSeq", tt.cseq)

			// Process through validation chain
			resp, err := processor.ProcessRequest(req)
			if err != nil {
				t.Fatalf("Expected no error, got %v", err)
			}

			if tt.expectedErr {
				if resp == nil {
					t.Error("Expected error response for validation failure")
				}
			} else {
				if resp != nil {
					t.Errorf("Expected no error response, got status %d", resp.GetStatusCode())
				}
			}
		})
	}
}

// TestValidationChainErrorRecovery tests that validation chain handles errors gracefully
// and provides appropriate error responses
func TestValidationChainErrorRecovery(t *testing.T) {
	processor := NewMessageProcessor()
	processor.AddValidator(NewSyntaxValidator())
	processor.AddValidator(NewSessionTimerValidator(90, 1800, true))
	processor.AddValidator(NewAuthValidator(true, "example.com"))

	// Test error recovery scenarios
	tests := []struct {
		name         string
		setupRequest func() *parser.SIPMessage
		expectedCode int
		description  string
	}{
		{
			name: "Request with missing required headers",
			setupRequest: func() *parser.SIPMessage {
				// Create request with minimal headers but missing some required ones
				req := parser.NewRequestMessage("INVITE", "sip:test@example.com")
				req.SetHeader("Via", "SIP/2.0/UDP 192.168.1.1:5060;branch=z9hG4bK123")
				req.SetHeader("From", "sip:alice@example.com;tag=abc123")
				req.SetHeader("Call-ID", "call123@example.com")
				req.SetHeader("CSeq", "1 INVITE")
				// Missing To header
				return req
			},
			expectedCode: 400,
			description:  "Should handle requests with missing required headers gracefully",
		},
		{
			name: "Request with invalid CSeq",
			setupRequest: func() *parser.SIPMessage {
				req := parser.NewRequestMessage("INVITE", "sip:test@example.com")
				req.SetHeader("Via", "SIP/2.0/UDP 192.168.1.1:5060;branch=z9hG4bK123")
				req.SetHeader("From", "sip:alice@example.com;tag=abc123")
				req.SetHeader("To", "sip:bob@example.com")
				req.SetHeader("Call-ID", "call123@example.com")
				req.SetHeader("CSeq", "invalid-cseq") // Invalid CSeq format
				return req
			},
			expectedCode: 400,
			description:  "Should handle invalid CSeq format gracefully",
		},
		{
			name: "Request with invalid Session-Expires",
			setupRequest: func() *parser.SIPMessage {
				req := parser.NewRequestMessage("INVITE", "sip:test@example.com")
				req.SetHeader("Via", "SIP/2.0/UDP 192.168.1.1:5060;branch=z9hG4bK123")
				req.SetHeader("From", "sip:alice@example.com;tag=abc123")
				req.SetHeader("To", "sip:bob@example.com")
				req.SetHeader("Call-ID", "call123@example.com")
				req.SetHeader("CSeq", "1 INVITE")
				req.SetHeader("Session-Expires", "invalid") // Invalid Session-Expires value
				return req
			},
			expectedCode: 400,
			description:  "Should handle invalid Session-Expires gracefully",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := tt.setupRequest()

			resp, err := processor.ProcessRequest(req)
			if err != nil {
				t.Fatalf("Expected no error, got %v", err)
			}

			if resp == nil {
				t.Fatal("Expected error response, got nil")
			}

			if resp.GetStatusCode() != tt.expectedCode {
				t.Errorf("Expected status code %d, got %d", tt.expectedCode, resp.GetStatusCode())
			}

			// Verify response has headers that were present in the request
			if req.GetHeader("Via") != "" && resp.GetHeader("Via") == "" {
				t.Error("Error response should have Via header when present in request")
			}
			if req.GetHeader("From") != "" && resp.GetHeader("From") == "" {
				t.Error("Error response should have From header when present in request")
			}
			if req.GetHeader("Call-ID") != "" && resp.GetHeader("Call-ID") == "" {
				t.Error("Error response should have Call-ID header when present in request")
			}
			if req.GetHeader("CSeq") != "" && resp.GetHeader("CSeq") == "" {
				t.Error("Error response should have CSeq header when present in request")
			}
		})
	}
}