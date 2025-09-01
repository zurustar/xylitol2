package main

import (
	"errors"
	"fmt"
	"time"

	"github.com/zurustar/xylitol2/internal/handlers"
	"github.com/zurustar/xylitol2/internal/logging"
	"github.com/zurustar/xylitol2/internal/parser"
)

func main() {
	// Create a logger
	logger := logging.NewConsoleLogger(logging.InfoLevel)
	
	// Create error logging manager
	errorManager := handlers.NewErrorLoggingManager(logger, true)
	
	// Start the error logging manager
	errorManager.Start()
	defer errorManager.Stop()
	
	// Simulate various error scenarios
	simulateParseErrors(errorManager)
	simulateValidationErrors(errorManager)
	simulateProcessingErrors(errorManager)
	simulateTransportErrors(errorManager)
	simulateAuthenticationErrors(errorManager)
	simulateSessionTimerErrors(errorManager)
	
	// Wait a bit for logging to complete
	time.Sleep(1 * time.Second)
	
	// Get and display statistics
	displayStatistics(errorManager)
	
	// Check system health
	errorManager.CheckSystemHealth()
	
	// Log error summary
	errorManager.LogErrorSummary()
}

func simulateParseErrors(errorManager *handlers.ErrorLoggingManager) {
	fmt.Println("Simulating parse errors...")
	
	// Simulate various parse errors
	parseErrors := []struct {
		err        error
		rawMessage []byte
	}{
		{
			err:        errors.New("failed to parse start line: invalid format"),
			rawMessage: []byte("INVALID SIP MESSAGE\r\n"),
		},
		{
			err:        errors.New("failed to parse headers: missing colon"),
			rawMessage: []byte("INVITE sip:user@example.com SIP/2.0\r\nInvalid Header Line\r\n"),
		},
		{
			err:        errors.New("invalid Content-Length: abc"),
			rawMessage: []byte("INVITE sip:user@example.com SIP/2.0\r\nContent-Length: abc\r\n"),
		},
		{
			err:        errors.New("empty message data"),
			rawMessage: []byte(""),
		},
	}
	
	for _, pe := range parseErrors {
		errorManager.LogParseError(pe.err, pe.rawMessage, "192.168.1.100:5060", "UDP")
	}
}

func simulateValidationErrors(errorManager *handlers.ErrorLoggingManager) {
	fmt.Println("Simulating validation errors...")
	
	// Create a test SIP message
	req := parser.NewRequestMessage("INVITE", "sip:user@example.com")
	req.SetHeader("Call-ID", "test-call-id-123")
	req.SetHeader("From", "sip:caller@example.com")
	req.SetHeader("To", "sip:callee@example.com")
	req.SetHeader("CSeq", "1 INVITE")
	req.SetHeader("Via", "SIP/2.0/UDP 192.168.1.100:5060")
	
	// Create validation errors
	validationErrors := []*handlers.DetailedValidationError{
		{
			ValidationError: &handlers.ValidationError{
				ValidatorName: "SessionTimerValidator",
				Code:          421,
				Reason:        "Extension Required",
				Details:       "Session-Timer extension is required",
			},
			ErrorType:      handlers.ErrorTypeValidationError,
			MissingHeaders: []string{"Session-Expires"},
			InvalidHeaders: make(map[string]string),
			Suggestions:    []string{"Add Session-Expires header with appropriate value"},
			Context:        make(map[string]interface{}),
		},
		{
			ValidationError: &handlers.ValidationError{
				ValidatorName: "AuthenticationValidator",
				Code:          401,
				Reason:        "Unauthorized",
				Details:       "Authentication required",
			},
			ErrorType:      handlers.ErrorTypeValidationError,
			MissingHeaders: []string{"Authorization"},
			InvalidHeaders: make(map[string]string),
			Suggestions:    []string{"Include proper Authorization header"},
			Context:        make(map[string]interface{}),
		},
	}
	
	for _, ve := range validationErrors {
		errorManager.LogValidationError(ve, req, "192.168.1.100:5060", "UDP")
	}
}

func simulateProcessingErrors(errorManager *handlers.ErrorLoggingManager) {
	fmt.Println("Simulating processing errors...")
	
	req := parser.NewRequestMessage("INVITE", "sip:user@example.com")
	req.SetHeader("Call-ID", "test-call-id-456")
	
	processingErrors := []error{
		errors.New("failed to lookup user in database"),
		errors.New("transaction creation failed"),
		errors.New("proxy forwarding failed: no route found"),
	}
	
	components := []string{"database", "transaction_manager", "proxy_engine"}
	operations := []string{"user_lookup", "create_transaction", "forward_request"}
	
	for i, err := range processingErrors {
		errorManager.LogProcessingError(err, req, components[i], operations[i])
	}
}

func simulateTransportErrors(errorManager *handlers.ErrorLoggingManager) {
	fmt.Println("Simulating transport errors...")
	
	req := parser.NewRequestMessage("INVITE", "sip:user@example.com")
	req.SetHeader("Call-ID", "test-call-id-789")
	
	transportErrors := []error{
		errors.New("connection refused"),
		errors.New("network timeout"),
		errors.New("TCP connection reset by peer"),
	}
	
	for _, err := range transportErrors {
		errorManager.LogTransportError(err, req, "TCP", "192.168.1.1:5060", "192.168.1.100:5060")
	}
}

func simulateAuthenticationErrors(errorManager *handlers.ErrorLoggingManager) {
	fmt.Println("Simulating authentication errors...")
	
	req := parser.NewRequestMessage("REGISTER", "sip:example.com")
	req.SetHeader("Call-ID", "test-call-id-auth")
	req.SetHeader("From", "sip:testuser@example.com")
	
	authErrors := []error{
		errors.New("invalid credentials"),
		errors.New("user not found"),
		errors.New("password hash mismatch"),
	}
	
	for _, err := range authErrors {
		errorManager.LogAuthenticationError(err, req, "testuser", "example.com", "192.168.1.100:5060")
	}
}

func simulateSessionTimerErrors(errorManager *handlers.ErrorLoggingManager) {
	fmt.Println("Simulating session timer errors...")
	
	req := parser.NewRequestMessage("INVITE", "sip:user@example.com")
	req.SetHeader("Call-ID", "test-call-id-session")
	req.SetHeader("Session-Expires", "90")
	
	sessionErrors := []error{
		errors.New("session interval too small"),
		errors.New("session timer not supported"),
		errors.New("session refresh timeout"),
	}
	
	for _, err := range sessionErrors {
		errorManager.LogSessionTimerError(err, req, "session-123", 1800, 90)
	}
}

func displayStatistics(errorManager *handlers.ErrorLoggingManager) {
	fmt.Println("\n=== Error Statistics ===")
	
	stats := errorManager.GetErrorStatistics()
	fmt.Printf("Parse Errors: %d\n", stats.ParseErrors)
	fmt.Printf("Validation Errors: %d\n", stats.ValidationErrors)
	fmt.Printf("Processing Errors: %d\n", stats.ProcessingErrors)
	fmt.Printf("Transport Errors: %d\n", stats.TransportErrors)
	fmt.Printf("Authentication Errors: %d\n", stats.AuthErrors)
	fmt.Printf("Session Timer Errors: %d\n", stats.SessionTimerErrors)
	
	detailedStats := errorManager.GetDetailedStatistics()
	fmt.Printf("\nParse Errors by Type:\n")
	for errorType, count := range detailedStats.ParseErrorsByType {
		fmt.Printf("  %s: %d\n", errorType, count)
	}
	
	fmt.Printf("\nValidation Errors by Type:\n")
	for errorType, count := range detailedStats.ValidationErrorsByType {
		fmt.Printf("  %s: %d\n", errorType, count)
	}
	
	fmt.Printf("\nRecent Errors: %d\n", len(detailedStats.RecentErrors))
	
	patterns := errorManager.GetErrorPatterns()
	fmt.Printf("\nError Patterns: %d\n", len(patterns))
	for pattern, info := range patterns {
		if info.Count > 1 {
			fmt.Printf("  Pattern: %s (Count: %d, Last Seen: %s)\n", 
				pattern, info.Count, info.LastSeen.Format("15:04:05"))
		}
	}
}