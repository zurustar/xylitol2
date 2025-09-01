package handlers

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/zurustar/xylitol2/internal/parser"
)

// MockLogger for testing
type MockLoggerRecovery struct {
	messages []string
}

func NewMockLoggerRecovery() *MockLoggerRecovery {
	return &MockLoggerRecovery{
		messages: make([]string, 0),
	}
}

func (m *MockLoggerRecovery) Debug(message string, fields ...Field) {
	m.messages = append(m.messages, "DEBUG: "+message)
}

func (m *MockLoggerRecovery) Info(message string, fields ...Field) {
	m.messages = append(m.messages, "INFO: "+message)
}

func (m *MockLoggerRecovery) Warn(message string, fields ...Field) {
	m.messages = append(m.messages, "WARN: "+message)
}

func (m *MockLoggerRecovery) Error(message string, fields ...Field) {
	m.messages = append(m.messages, "ERROR: "+message)
}

func TestNewErrorRecoveryManager(t *testing.T) {
	mockLogger := NewMockLoggerRecovery()
	logger := NewDetailedErrorLogger(LogLevelDebug, true, mockLogger)
	monitor := NewErrorMonitor(logger)
	
	erm := NewErrorRecoveryManager(logger, monitor)
	
	if erm == nil {
		t.Fatal("Expected non-nil error recovery manager")
	}
	
	if len(erm.strategies) == 0 {
		t.Error("Expected recovery strategies to be initialized")
	}
	
	if len(erm.circuitBreakers) == 0 {
		t.Error("Expected circuit breakers to be initialized")
	}
	
	if len(erm.retryPolicies) == 0 {
		t.Error("Expected retry policies to be initialized")
	}
	
	if len(erm.fallbackHandlers) == 0 {
		t.Error("Expected fallback handlers to be initialized")
	}
}

func TestRecoveryType_String(t *testing.T) {
	tests := []struct {
		recoveryType RecoveryType
		expected     string
	}{
		{RecoveryTypeRetry, "Retry"},
		{RecoveryTypeFallback, "Fallback"},
		{RecoveryTypeCircuitBreaker, "CircuitBreaker"},
		{RecoveryTypeGracefulDegradation, "GracefulDegradation"},
		{RecoveryTypeAutoCorrection, "AutoCorrection"},
		{RecoveryType(999), "Unknown"},
	}
	
	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := tt.recoveryType.String()
			if result != tt.expected {
				t.Errorf("Expected %s, got %s", tt.expected, result)
			}
		})
	}
}

func TestCircuitBreakerState_String(t *testing.T) {
	tests := []struct {
		state    CircuitBreakerState
		expected string
	}{
		{CircuitBreakerClosed, "Closed"},
		{CircuitBreakerOpen, "Open"},
		{CircuitBreakerHalfOpen, "HalfOpen"},
		{CircuitBreakerState(999), "Unknown"},
	}
	
	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := tt.state.String()
			if result != tt.expected {
				t.Errorf("Expected %s, got %s", tt.expected, result)
			}
		})
	}
}

func TestCircuitBreaker(t *testing.T) {
	cb := NewCircuitBreaker("test", 3, 1*time.Second)
	
	// Initially closed
	if cb.GetState() != CircuitBreakerClosed {
		t.Error("Circuit breaker should start in closed state")
	}
	
	if !cb.CanExecute() {
		t.Error("Closed circuit breaker should allow execution")
	}
	
	// Record failures
	cb.RecordFailure()
	cb.RecordFailure()
	if cb.GetState() != CircuitBreakerClosed {
		t.Error("Circuit breaker should remain closed until max failures")
	}
	
	cb.RecordFailure() // This should open the circuit
	if cb.GetState() != CircuitBreakerOpen {
		t.Error("Circuit breaker should be open after max failures")
	}
	
	if cb.CanExecute() {
		t.Error("Open circuit breaker should not allow execution")
	}
	
	// Test success resets failures
	cb.RecordSuccess()
	if cb.GetState() != CircuitBreakerClosed {
		t.Error("Circuit breaker should be closed after success")
	}
}

func TestCircuitBreaker_Timeout(t *testing.T) {
	cb := NewCircuitBreaker("test", 1, 100*time.Millisecond)
	
	// Open the circuit
	cb.RecordFailure()
	if cb.GetState() != CircuitBreakerOpen {
		t.Error("Circuit breaker should be open")
	}
	
	// Should not allow execution immediately
	if cb.CanExecute() {
		t.Error("Open circuit breaker should not allow execution")
	}
	
	// Wait for timeout
	time.Sleep(150 * time.Millisecond)
	
	// Should transition to half-open and allow execution
	if !cb.CanExecute() {
		t.Error("Circuit breaker should allow execution after timeout")
	}
	
	if cb.GetState() != CircuitBreakerHalfOpen {
		t.Error("Circuit breaker should be half-open after timeout")
	}
}

func TestParseErrorRecoveryStrategy(t *testing.T) {
	strategy := &ParseErrorRecoveryStrategy{}
	
	tests := []struct {
		name        string
		err         error
		canRecover  bool
		recoveryMsg string
	}{
		{
			name:        "Line ending error",
			err:         errors.New("invalid line ending format"),
			canRecover:  true,
			recoveryMsg: "Line ending issue detected",
		},
		{
			name:        "Header format error",
			err:         errors.New("invalid header format"),
			canRecover:  true,
			recoveryMsg: "Header format issue detected",
		},
		{
			name:        "Content-length error",
			err:         errors.New("content-length mismatch"),
			canRecover:  true,
			recoveryMsg: "Parse error cannot be auto-corrected",
		},
		{
			name:       "Other parse error",
			err:        errors.New("unknown parse error"),
			canRecover: false,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			context := make(map[string]interface{})
			
			canRecover := strategy.CanRecover(tt.err, context)
			if canRecover != tt.canRecover {
				t.Errorf("Expected CanRecover to return %v, got %v", tt.canRecover, canRecover)
			}
			
			if tt.canRecover {
				result := strategy.Recover(tt.err, context)
				if result.RecoveryType != RecoveryTypeAutoCorrection {
					t.Errorf("Expected recovery type %v, got %v", RecoveryTypeAutoCorrection, result.RecoveryType)
				}
				
				if !strings.Contains(result.RecoveryMessage, tt.recoveryMsg) {
					t.Errorf("Expected recovery message to contain '%s', got: %s", tt.recoveryMsg, result.RecoveryMessage)
				}
			}
		})
	}
}

func TestValidationErrorRecoveryStrategy(t *testing.T) {
	strategy := &ValidationErrorRecoveryStrategy{}
	
	err := errors.New("validation failed")
	context := make(map[string]interface{})
	
	if !strategy.CanRecover(err, context) {
		t.Error("Validation error recovery strategy should always be able to recover")
	}
	
	result := strategy.Recover(err, context)
	if !result.Success {
		t.Error("Validation error recovery should succeed")
	}
	
	if result.RecoveryType != RecoveryTypeGracefulDegradation {
		t.Errorf("Expected recovery type %v, got %v", RecoveryTypeGracefulDegradation, result.RecoveryType)
	}
	
	if result.ShouldRetry {
		t.Error("Validation errors should not be retried")
	}
}

func TestProcessingErrorRecoveryStrategy(t *testing.T) {
	strategy := &ProcessingErrorRecoveryStrategy{}
	
	tests := []struct {
		name         string
		err          error
		canRecover   bool
		shouldRetry  bool
		recoveryType RecoveryType
	}{
		{
			name:         "Database error",
			err:          errors.New("database connection failed"),
			canRecover:   true,
			shouldRetry:  true,
			recoveryType: RecoveryTypeRetry,
		},
		{
			name:         "Connection timeout",
			err:          errors.New("connection timeout occurred"),
			canRecover:   true,
			shouldRetry:  true,
			recoveryType: RecoveryTypeRetry,
		},
		{
			name:         "Other processing error",
			err:          errors.New("unknown processing error"),
			canRecover:   false,
			shouldRetry:  false,
			recoveryType: RecoveryTypeFallback,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			context := make(map[string]interface{})
			
			canRecover := strategy.CanRecover(tt.err, context)
			if canRecover != tt.canRecover {
				t.Errorf("Expected CanRecover to return %v, got %v", tt.canRecover, canRecover)
			}
			
			if tt.canRecover {
				result := strategy.Recover(tt.err, context)
				if result.ShouldRetry != tt.shouldRetry {
					t.Errorf("Expected ShouldRetry to be %v, got %v", tt.shouldRetry, result.ShouldRetry)
				}
				
				if result.RecoveryType != tt.recoveryType {
					t.Errorf("Expected recovery type %v, got %v", tt.recoveryType, result.RecoveryType)
				}
			}
		})
	}
}

func TestTransportErrorRecoveryStrategy(t *testing.T) {
	strategy := &TransportErrorRecoveryStrategy{}
	
	tests := []struct {
		name         string
		err          error
		canRecover   bool
		shouldRetry  bool
		recoveryType RecoveryType
	}{
		{
			name:         "Connection refused",
			err:          errors.New("connection refused"),
			canRecover:   true,
			shouldRetry:  true,
			recoveryType: RecoveryTypeRetry,
		},
		{
			name:         "Network timeout",
			err:          errors.New("network timeout"),
			canRecover:   true,
			shouldRetry:  true,
			recoveryType: RecoveryTypeRetry,
		},
		{
			name:         "Connection reset",
			err:          errors.New("connection reset by peer"),
			canRecover:   true,
			shouldRetry:  true,
			recoveryType: RecoveryTypeRetry,
		},
		{
			name:         "Other transport error",
			err:          errors.New("unknown transport error"),
			canRecover:   false,
			shouldRetry:  true,
			recoveryType: RecoveryTypeCircuitBreaker,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			context := make(map[string]interface{})
			
			canRecover := strategy.CanRecover(tt.err, context)
			if canRecover != tt.canRecover {
				t.Errorf("Expected CanRecover to return %v, got %v", tt.canRecover, canRecover)
			}
			
			result := strategy.Recover(tt.err, context)
			if result.ShouldRetry != tt.shouldRetry {
				t.Errorf("Expected ShouldRetry to be %v, got %v", tt.shouldRetry, result.ShouldRetry)
			}
			
			if result.RecoveryType != tt.recoveryType {
				t.Errorf("Expected recovery type %v, got %v", tt.recoveryType, result.RecoveryType)
			}
		})
	}
}

func TestSessionTimerErrorRecoveryStrategy(t *testing.T) {
	strategy := &SessionTimerErrorRecoveryStrategy{}
	
	tests := []struct {
		name         string
		err          error
		recoveryMsg  string
	}{
		{
			name:        "Interval too small",
			err:         errors.New("session interval too small"),
			recoveryMsg: "Min-SE header",
		},
		{
			name:        "Interval too brief",
			err:         errors.New("interval too brief"),
			recoveryMsg: "Min-SE header",
		},
		{
			name:        "Other session timer error",
			err:         errors.New("session timer not supported"),
			recoveryMsg: "session timer guidance",
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			context := make(map[string]interface{})
			
			if !strategy.CanRecover(tt.err, context) {
				t.Error("Session timer error recovery strategy should always be able to recover")
			}
			
			result := strategy.Recover(tt.err, context)
			if !result.Success {
				t.Error("Session timer error recovery should succeed")
			}
			
			if result.RecoveryType != RecoveryTypeGracefulDegradation {
				t.Errorf("Expected recovery type %v, got %v", RecoveryTypeGracefulDegradation, result.RecoveryType)
			}
			
			if !strings.Contains(result.RecoveryMessage, tt.recoveryMsg) {
				t.Errorf("Expected recovery message to contain '%s', got: %s", tt.recoveryMsg, result.RecoveryMessage)
			}
		})
	}
}

func TestErrorRecoveryManager_AttemptRecovery(t *testing.T) {
	mockLogger := NewMockLoggerRecovery()
	logger := NewDetailedErrorLogger(LogLevelDebug, true, mockLogger)
	monitor := NewErrorMonitor(logger)
	erm := NewErrorRecoveryManager(logger, monitor)
	
	// Test recovery for validation error
	err := errors.New("validation failed")
	context := make(map[string]interface{})
	
	result := erm.AttemptRecovery(ErrorTypeValidationError, err, context)
	
	if !result.Success {
		t.Error("Expected successful recovery for validation error")
	}
	
	if result.RecoveryType != RecoveryTypeGracefulDegradation {
		t.Errorf("Expected recovery type %v, got %v", RecoveryTypeGracefulDegradation, result.RecoveryType)
	}
	
	// Test recovery for unknown error type
	result = erm.AttemptRecovery(ErrorType(999), err, context)
	
	if result.Success {
		t.Error("Expected failed recovery for unknown error type")
	}
}

func TestErrorRecoveryManager_CircuitBreaker(t *testing.T) {
	mockLogger := NewMockLoggerRecovery()
	logger := NewDetailedErrorLogger(LogLevelDebug, true, mockLogger)
	monitor := NewErrorMonitor(logger)
	erm := NewErrorRecoveryManager(logger, monitor)
	
	// Test with circuit breaker context
	err := errors.New("database connection failed")
	context := map[string]interface{}{
		"circuit_breaker": "database",
	}
	
	// First attempt should work
	result := erm.AttemptRecovery(ErrorTypeProcessingError, err, context)
	if result.Success {
		t.Error("Expected failed recovery for database error")
	}
	
	// Open the circuit breaker by recording failures
	cb := erm.circuitBreakers["database"]
	for i := 0; i < 5; i++ {
		cb.RecordFailure()
	}
	
	// Next attempt should be blocked by circuit breaker
	result = erm.AttemptRecovery(ErrorTypeProcessingError, err, context)
	if result.Success {
		t.Error("Expected failed recovery due to open circuit breaker")
	}
	
	if result.RecoveryType != RecoveryTypeCircuitBreaker {
		t.Errorf("Expected recovery type %v, got %v", RecoveryTypeCircuitBreaker, result.RecoveryType)
	}
}

func TestFallbackHandlers(t *testing.T) {
	mockLogger := NewMockLoggerRecovery()
	logger := NewDetailedErrorLogger(LogLevelDebug, true, mockLogger)
	monitor := NewErrorMonitor(logger)
	erm := NewErrorRecoveryManager(logger, monitor)
	
	tests := []struct {
		name         string
		errorType    ErrorType
		expectedCode int
	}{
		{
			name:         "Parse error fallback",
			errorType:    ErrorTypeParseError,
			expectedCode: parser.StatusBadRequest,
		},
		{
			name:         "Validation error fallback",
			errorType:    ErrorTypeValidationError,
			expectedCode: parser.StatusBadRequest,
		},
		{
			name:         "Processing error fallback",
			errorType:    ErrorTypeProcessingError,
			expectedCode: parser.StatusServerInternalError,
		},
		{
			name:         "Transport error fallback",
			errorType:    ErrorTypeTransportError,
			expectedCode: parser.StatusServiceUnavailable,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := errors.New("test error")
			context := make(map[string]interface{})
			
			response := erm.GetFallbackResponse(tt.errorType, err, context)
			
			if response == nil {
				t.Fatal("Expected non-nil fallback response")
			}
			
			if response.GetStatusCode() != tt.expectedCode {
				t.Errorf("Expected status code %d, got %d", tt.expectedCode, response.GetStatusCode())
			}
		})
	}
	
	// Test unknown error type
	err := errors.New("test error")
	context := make(map[string]interface{})
	response := erm.GetFallbackResponse(ErrorType(999), err, context)
	
	if response != nil {
		t.Error("Expected nil response for unknown error type")
	}
}

func TestGetRecoveryStatistics(t *testing.T) {
	mockLogger := NewMockLoggerRecovery()
	logger := NewDetailedErrorLogger(LogLevelDebug, true, mockLogger)
	monitor := NewErrorMonitor(logger)
	erm := NewErrorRecoveryManager(logger, monitor)
	
	stats := erm.GetRecoveryStatistics()
	
	if stats == nil {
		t.Fatal("Expected non-nil recovery statistics")
	}
	
	cbStates, exists := stats["circuit_breaker_states"]
	if !exists {
		t.Error("Expected circuit_breaker_states in statistics")
	}
	
	if cbStatesMap, ok := cbStates.(map[string]string); ok {
		if len(cbStatesMap) == 0 {
			t.Error("Expected circuit breaker states to be populated")
		}
	} else {
		t.Error("Expected circuit_breaker_states to be a map[string]string")
	}
	
	strategiesCount, exists := stats["recovery_strategies_count"]
	if !exists {
		t.Error("Expected recovery_strategies_count in statistics")
	}
	
	if count, ok := strategiesCount.(int); ok {
		if count == 0 {
			t.Error("Expected recovery strategies count to be greater than 0")
		}
	} else {
		t.Error("Expected recovery_strategies_count to be an int")
	}
}

func TestContainsHelper(t *testing.T) {
	tests := []struct {
		name       string
		str        string
		substrings []string
		expected   bool
	}{
		{
			name:       "Contains one substring",
			str:        "database connection failed",
			substrings: []string{"database", "network"},
			expected:   true,
		},
		{
			name:       "Contains multiple substrings",
			str:        "network timeout error",
			substrings: []string{"network", "timeout"},
			expected:   true,
		},
		{
			name:       "Contains no substrings",
			str:        "unknown error",
			substrings: []string{"database", "network"},
			expected:   false,
		},
		{
			name:       "Empty substrings",
			str:        "any error",
			substrings: []string{},
			expected:   false,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := contains(tt.str, tt.substrings)
			if result != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, result)
			}
		})
	}
}