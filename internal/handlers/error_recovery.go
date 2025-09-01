package handlers

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/zurustar/xylitol2/internal/parser"
)

// ErrorRecoveryManager manages error recovery strategies and mechanisms
type ErrorRecoveryManager struct {
	strategies      map[ErrorType]RecoveryStrategy
	circuitBreakers map[string]*CircuitBreaker
	retryPolicies   map[ErrorType]*RetryPolicy
	fallbackHandlers map[ErrorType]FallbackHandler
	mutex           sync.RWMutex
	logger          *DetailedErrorLogger
	monitor         *ErrorMonitor
}

// RecoveryStrategy defines how to recover from specific error types
type RecoveryStrategy interface {
	CanRecover(err error, context map[string]interface{}) bool
	Recover(err error, context map[string]interface{}) RecoveryResult
	GetRecoveryType() RecoveryType
}

// RecoveryType represents different types of recovery mechanisms
type RecoveryType int

const (
	RecoveryTypeRetry RecoveryType = iota
	RecoveryTypeFallback
	RecoveryTypeCircuitBreaker
	RecoveryTypeGracefulDegradation
	RecoveryTypeAutoCorrection
)

// String returns the string representation of RecoveryType
func (rt RecoveryType) String() string {
	switch rt {
	case RecoveryTypeRetry:
		return "Retry"
	case RecoveryTypeFallback:
		return "Fallback"
	case RecoveryTypeCircuitBreaker:
		return "CircuitBreaker"
	case RecoveryTypeGracefulDegradation:
		return "GracefulDegradation"
	case RecoveryTypeAutoCorrection:
		return "AutoCorrection"
	default:
		return "Unknown"
	}
}

// RecoveryResult contains the result of a recovery attempt
type RecoveryResult struct {
	Success         bool
	RecoveredData   interface{}
	RecoveryType    RecoveryType
	RecoveryMessage string
	ShouldRetry     bool
	RetryAfter      time.Duration
	Context         map[string]interface{}
}

// RetryPolicy defines retry behavior for different error types
type RetryPolicy struct {
	MaxAttempts     int
	InitialDelay    time.Duration
	MaxDelay        time.Duration
	BackoffFactor   float64
	RetryableErrors []string
}

// FallbackHandler provides fallback behavior when recovery fails
type FallbackHandler interface {
	HandleFallback(err error, context map[string]interface{}) *parser.SIPMessage
	CanHandle(errorType ErrorType) bool
}

// CircuitBreaker implements circuit breaker pattern for error recovery
type CircuitBreaker struct {
	name            string
	maxFailures     int
	timeout         time.Duration
	state           CircuitBreakerState
	failures        int
	lastFailureTime time.Time
	mutex           sync.RWMutex
}

// CircuitBreakerState represents the state of a circuit breaker
type CircuitBreakerState int

const (
	CircuitBreakerClosed CircuitBreakerState = iota
	CircuitBreakerOpen
	CircuitBreakerHalfOpen
)

// String returns the string representation of CircuitBreakerState
func (cbs CircuitBreakerState) String() string {
	switch cbs {
	case CircuitBreakerClosed:
		return "Closed"
	case CircuitBreakerOpen:
		return "Open"
	case CircuitBreakerHalfOpen:
		return "HalfOpen"
	default:
		return "Unknown"
	}
}

// NewErrorRecoveryManager creates a new error recovery manager
func NewErrorRecoveryManager(logger *DetailedErrorLogger, monitor *ErrorMonitor) *ErrorRecoveryManager {
	erm := &ErrorRecoveryManager{
		strategies:       make(map[ErrorType]RecoveryStrategy),
		circuitBreakers:  make(map[string]*CircuitBreaker),
		retryPolicies:    make(map[ErrorType]*RetryPolicy),
		fallbackHandlers: make(map[ErrorType]FallbackHandler),
		logger:           logger,
		monitor:          monitor,
	}
	
	// Initialize default recovery strategies
	erm.initializeDefaultStrategies()
	
	return erm
}

// initializeDefaultStrategies sets up default recovery strategies for different error types
func (erm *ErrorRecoveryManager) initializeDefaultStrategies() {
	// Parse error recovery
	erm.strategies[ErrorTypeParseError] = &ParseErrorRecoveryStrategy{}
	erm.retryPolicies[ErrorTypeParseError] = &RetryPolicy{
		MaxAttempts:   1, // Don't retry parse errors
		InitialDelay:  0,
		MaxDelay:      0,
		BackoffFactor: 1.0,
	}
	
	// Validation error recovery
	erm.strategies[ErrorTypeValidationError] = &ValidationErrorRecoveryStrategy{}
	erm.retryPolicies[ErrorTypeValidationError] = &RetryPolicy{
		MaxAttempts:   1, // Don't retry validation errors
		InitialDelay:  0,
		MaxDelay:      0,
		BackoffFactor: 1.0,
	}
	
	// Processing error recovery
	erm.strategies[ErrorTypeProcessingError] = &ProcessingErrorRecoveryStrategy{}
	erm.retryPolicies[ErrorTypeProcessingError] = &RetryPolicy{
		MaxAttempts:   3,
		InitialDelay:  100 * time.Millisecond,
		MaxDelay:      5 * time.Second,
		BackoffFactor: 2.0,
		RetryableErrors: []string{"database", "connection", "timeout"},
	}
	
	// Transport error recovery
	erm.strategies[ErrorTypeTransportError] = &TransportErrorRecoveryStrategy{}
	erm.retryPolicies[ErrorTypeTransportError] = &RetryPolicy{
		MaxAttempts:   5,
		InitialDelay:  50 * time.Millisecond,
		MaxDelay:      2 * time.Second,
		BackoffFactor: 1.5,
		RetryableErrors: []string{"connection", "network", "timeout"},
	}
	
	// Authentication error recovery
	erm.strategies[ErrorTypeAuthenticationError] = &AuthenticationErrorRecoveryStrategy{}
	erm.retryPolicies[ErrorTypeAuthenticationError] = &RetryPolicy{
		MaxAttempts:   1, // Don't retry auth errors
		InitialDelay:  0,
		MaxDelay:      0,
		BackoffFactor: 1.0,
	}
	
	// Session timer error recovery
	erm.strategies[ErrorTypeSessionTimerError] = &SessionTimerErrorRecoveryStrategy{}
	erm.retryPolicies[ErrorTypeSessionTimerError] = &RetryPolicy{
		MaxAttempts:   2,
		InitialDelay:  100 * time.Millisecond,
		MaxDelay:      1 * time.Second,
		BackoffFactor: 2.0,
	}
	
	// Initialize circuit breakers
	erm.circuitBreakers["database"] = NewCircuitBreaker("database", 5, 30*time.Second)
	erm.circuitBreakers["transport"] = NewCircuitBreaker("transport", 10, 10*time.Second)
	erm.circuitBreakers["authentication"] = NewCircuitBreaker("authentication", 20, 60*time.Second)
	
	// Initialize fallback handlers
	erm.fallbackHandlers[ErrorTypeParseError] = &ParseErrorFallbackHandler{}
	erm.fallbackHandlers[ErrorTypeValidationError] = &ValidationErrorFallbackHandler{}
	erm.fallbackHandlers[ErrorTypeProcessingError] = &ProcessingErrorFallbackHandler{}
	erm.fallbackHandlers[ErrorTypeTransportError] = &TransportErrorFallbackHandler{}
}

// AttemptRecovery attempts to recover from an error using appropriate strategies
func (erm *ErrorRecoveryManager) AttemptRecovery(errorType ErrorType, err error, context map[string]interface{}) RecoveryResult {
	erm.mutex.RLock()
	strategy, exists := erm.strategies[errorType]
	erm.mutex.RUnlock()
	
	if !exists {
		return RecoveryResult{
			Success:         false,
			RecoveryMessage: "No recovery strategy available for error type",
			Context:         context,
		}
	}
	
	// Check if recovery is possible
	if !strategy.CanRecover(err, context) {
		return RecoveryResult{
			Success:         false,
			RecoveryMessage: "Error is not recoverable",
			RecoveryType:    strategy.GetRecoveryType(),
			Context:         context,
		}
	}
	
	// Check circuit breaker if applicable
	if circuitBreakerName, exists := context["circuit_breaker"]; exists {
		if cb, found := erm.circuitBreakers[circuitBreakerName.(string)]; found {
			if !cb.CanExecute() {
				return RecoveryResult{
					Success:         false,
					RecoveryMessage: fmt.Sprintf("Circuit breaker %s is open", circuitBreakerName),
					RecoveryType:    RecoveryTypeCircuitBreaker,
					ShouldRetry:     true,
					RetryAfter:      cb.timeout,
					Context:         context,
				}
			}
		}
	}
	
	// Attempt recovery
	result := strategy.Recover(err, context)
	
	// Update circuit breaker based on result
	if circuitBreakerName, exists := context["circuit_breaker"]; exists {
		if cb, found := erm.circuitBreakers[circuitBreakerName.(string)]; found {
			if result.Success {
				cb.RecordSuccess()
			} else {
				cb.RecordFailure()
			}
		}
	}
	
	// Log recovery attempt
	erm.logger.logger.Info("Error recovery attempted",
		Field{Key: "error_type", Value: errorType.String()},
		Field{Key: "recovery_type", Value: result.RecoveryType.String()},
		Field{Key: "success", Value: result.Success},
		Field{Key: "recovery_message", Value: result.RecoveryMessage},
		Field{Key: "should_retry", Value: result.ShouldRetry},
	)
	
	return result
}

// GetFallbackResponse gets a fallback response when recovery fails
func (erm *ErrorRecoveryManager) GetFallbackResponse(errorType ErrorType, err error, context map[string]interface{}) *parser.SIPMessage {
	erm.mutex.RLock()
	handler, exists := erm.fallbackHandlers[errorType]
	erm.mutex.RUnlock()
	
	if !exists || !handler.CanHandle(errorType) {
		return nil
	}
	
	response := handler.HandleFallback(err, context)
	
	// Log fallback usage
	erm.logger.logger.Warn("Using fallback response",
		Field{Key: "error_type", Value: errorType.String()},
		Field{Key: "error_message", Value: err.Error()},
		Field{Key: "fallback_used", Value: response != nil},
	)
	
	return response
}

// NewCircuitBreaker creates a new circuit breaker
func NewCircuitBreaker(name string, maxFailures int, timeout time.Duration) *CircuitBreaker {
	return &CircuitBreaker{
		name:        name,
		maxFailures: maxFailures,
		timeout:     timeout,
		state:       CircuitBreakerClosed,
	}
}

// CanExecute checks if the circuit breaker allows execution
func (cb *CircuitBreaker) CanExecute() bool {
	cb.mutex.RLock()
	defer cb.mutex.RUnlock()
	
	switch cb.state {
	case CircuitBreakerClosed:
		return true
	case CircuitBreakerOpen:
		if time.Since(cb.lastFailureTime) > cb.timeout {
			cb.state = CircuitBreakerHalfOpen
			return true
		}
		return false
	case CircuitBreakerHalfOpen:
		return true
	default:
		return false
	}
}

// RecordSuccess records a successful execution
func (cb *CircuitBreaker) RecordSuccess() {
	cb.mutex.Lock()
	defer cb.mutex.Unlock()
	
	cb.failures = 0
	cb.state = CircuitBreakerClosed
}

// RecordFailure records a failed execution
func (cb *CircuitBreaker) RecordFailure() {
	cb.mutex.Lock()
	defer cb.mutex.Unlock()
	
	cb.failures++
	cb.lastFailureTime = time.Now()
	
	if cb.failures >= cb.maxFailures {
		cb.state = CircuitBreakerOpen
	}
}

// GetState returns the current state of the circuit breaker
func (cb *CircuitBreaker) GetState() CircuitBreakerState {
	cb.mutex.RLock()
	defer cb.mutex.RUnlock()
	return cb.state
}

// ParseErrorRecoveryStrategy implements recovery for parse errors
type ParseErrorRecoveryStrategy struct{}

func (pers *ParseErrorRecoveryStrategy) CanRecover(err error, context map[string]interface{}) bool {
	// Parse errors are generally not recoverable, but we can try auto-correction
	errorMsg := err.Error()
	return contains(errorMsg, []string{"line ending", "header format", "content-length"})
}

func (pers *ParseErrorRecoveryStrategy) Recover(err error, context map[string]interface{}) RecoveryResult {
	errorMsg := err.Error()
	
	// Attempt auto-correction for common parse errors
	if contains(errorMsg, []string{"line ending"}) {
		return RecoveryResult{
			Success:         false, // Can't auto-correct in real-time
			RecoveryType:    RecoveryTypeAutoCorrection,
			RecoveryMessage: "Line ending issue detected - suggest client fix CRLF usage",
			ShouldRetry:     false,
		}
	}
	
	if contains(errorMsg, []string{"header format"}) {
		return RecoveryResult{
			Success:         false,
			RecoveryType:    RecoveryTypeAutoCorrection,
			RecoveryMessage: "Header format issue detected - suggest client check header syntax",
			ShouldRetry:     false,
		}
	}
	
	return RecoveryResult{
		Success:         false,
		RecoveryType:    RecoveryTypeAutoCorrection,
		RecoveryMessage: "Parse error cannot be auto-corrected",
		ShouldRetry:     false,
	}
}

func (pers *ParseErrorRecoveryStrategy) GetRecoveryType() RecoveryType {
	return RecoveryTypeAutoCorrection
}

// ValidationErrorRecoveryStrategy implements recovery for validation errors
type ValidationErrorRecoveryStrategy struct{}

func (vers *ValidationErrorRecoveryStrategy) CanRecover(err error, context map[string]interface{}) bool {
	// Some validation errors can be recovered by providing helpful responses
	return true
}

func (vers *ValidationErrorRecoveryStrategy) Recover(err error, context map[string]interface{}) RecoveryResult {
	return RecoveryResult{
		Success:         true,
		RecoveryType:    RecoveryTypeGracefulDegradation,
		RecoveryMessage: "Providing detailed error response to help client fix validation issues",
		ShouldRetry:     false,
	}
}

func (vers *ValidationErrorRecoveryStrategy) GetRecoveryType() RecoveryType {
	return RecoveryTypeGracefulDegradation
}

// ProcessingErrorRecoveryStrategy implements recovery for processing errors
type ProcessingErrorRecoveryStrategy struct{}

func (pers *ProcessingErrorRecoveryStrategy) CanRecover(err error, context map[string]interface{}) bool {
	errorMsg := err.Error()
	// Processing errors related to temporary issues can be retried
	return contains(errorMsg, []string{"database", "connection", "timeout", "temporary"})
}

func (pers *ProcessingErrorRecoveryStrategy) Recover(err error, context map[string]interface{}) RecoveryResult {
	errorMsg := err.Error()
	
	if contains(errorMsg, []string{"database"}) {
		return RecoveryResult{
			Success:         false,
			RecoveryType:    RecoveryTypeRetry,
			RecoveryMessage: "Database error detected - retry recommended",
			ShouldRetry:     true,
			RetryAfter:      500 * time.Millisecond,
		}
	}
	
	if contains(errorMsg, []string{"connection", "timeout"}) {
		return RecoveryResult{
			Success:         false,
			RecoveryType:    RecoveryTypeRetry,
			RecoveryMessage: "Connection/timeout error - retry with backoff",
			ShouldRetry:     true,
			RetryAfter:      1 * time.Second,
		}
	}
	
	return RecoveryResult{
		Success:         false,
		RecoveryType:    RecoveryTypeFallback,
		RecoveryMessage: "Processing error - using fallback response",
		ShouldRetry:     false,
	}
}

func (pers *ProcessingErrorRecoveryStrategy) GetRecoveryType() RecoveryType {
	return RecoveryTypeRetry
}

// TransportErrorRecoveryStrategy implements recovery for transport errors
type TransportErrorRecoveryStrategy struct{}

func (ters *TransportErrorRecoveryStrategy) CanRecover(err error, context map[string]interface{}) bool {
	errorMsg := err.Error()
	// Transport errors are often temporary and can be retried
	return contains(errorMsg, []string{"connection", "network", "timeout", "refused", "reset"})
}

func (ters *TransportErrorRecoveryStrategy) Recover(err error, context map[string]interface{}) RecoveryResult {
	errorMsg := err.Error()
	
	if contains(errorMsg, []string{"connection refused", "connection reset"}) {
		return RecoveryResult{
			Success:         false,
			RecoveryType:    RecoveryTypeRetry,
			RecoveryMessage: "Connection issue - retry with exponential backoff",
			ShouldRetry:     true,
			RetryAfter:      2 * time.Second,
		}
	}
	
	if contains(errorMsg, []string{"timeout"}) {
		return RecoveryResult{
			Success:         false,
			RecoveryType:    RecoveryTypeRetry,
			RecoveryMessage: "Timeout error - retry with increased timeout",
			ShouldRetry:     true,
			RetryAfter:      1 * time.Second,
		}
	}
	
	return RecoveryResult{
		Success:         false,
		RecoveryType:    RecoveryTypeCircuitBreaker,
		RecoveryMessage: "Transport error - circuit breaker activated",
		ShouldRetry:     true,
		RetryAfter:      5 * time.Second,
	}
}

func (ters *TransportErrorRecoveryStrategy) GetRecoveryType() RecoveryType {
	return RecoveryTypeRetry
}

// AuthenticationErrorRecoveryStrategy implements recovery for authentication errors
type AuthenticationErrorRecoveryStrategy struct{}

func (aers *AuthenticationErrorRecoveryStrategy) CanRecover(err error, context map[string]interface{}) bool {
	// Authentication errors are generally not recoverable, but we can provide helpful responses
	return true
}

func (aers *AuthenticationErrorRecoveryStrategy) Recover(err error, context map[string]interface{}) RecoveryResult {
	return RecoveryResult{
		Success:         true,
		RecoveryType:    RecoveryTypeGracefulDegradation,
		RecoveryMessage: "Providing authentication challenge to help client authenticate",
		ShouldRetry:     false,
	}
}

func (aers *AuthenticationErrorRecoveryStrategy) GetRecoveryType() RecoveryType {
	return RecoveryTypeGracefulDegradation
}

// SessionTimerErrorRecoveryStrategy implements recovery for session timer errors
type SessionTimerErrorRecoveryStrategy struct{}

func (sters *SessionTimerErrorRecoveryStrategy) CanRecover(err error, context map[string]interface{}) bool {
	// Session timer errors can often be recovered by providing helpful responses
	return true
}

func (sters *SessionTimerErrorRecoveryStrategy) Recover(err error, context map[string]interface{}) RecoveryResult {
	errorMsg := err.Error()
	
	if contains(errorMsg, []string{"too small", "brief"}) {
		return RecoveryResult{
			Success:         true,
			RecoveryType:    RecoveryTypeGracefulDegradation,
			RecoveryMessage: "Providing Min-SE header to help client adjust session timer",
			ShouldRetry:     false,
		}
	}
	
	return RecoveryResult{
		Success:         true,
		RecoveryType:    RecoveryTypeGracefulDegradation,
		RecoveryMessage: "Providing session timer guidance to client",
		ShouldRetry:     false,
	}
}

func (sters *SessionTimerErrorRecoveryStrategy) GetRecoveryType() RecoveryType {
	return RecoveryTypeGracefulDegradation
}

// Fallback handlers

// ParseErrorFallbackHandler provides fallback responses for parse errors
type ParseErrorFallbackHandler struct{}

func (pefh *ParseErrorFallbackHandler) CanHandle(errorType ErrorType) bool {
	return errorType == ErrorTypeParseError
}

func (pefh *ParseErrorFallbackHandler) HandleFallback(err error, context map[string]interface{}) *parser.SIPMessage {
	// Create a generic 400 Bad Request response
	response := parser.NewResponseMessage(parser.StatusBadRequest, "Bad Request")
	response.SetHeader(parser.HeaderContentType, "text/plain")
	response.SetHeader(parser.HeaderContentLength, "0")
	return response
}

// ValidationErrorFallbackHandler provides fallback responses for validation errors
type ValidationErrorFallbackHandler struct{}

func (vefh *ValidationErrorFallbackHandler) CanHandle(errorType ErrorType) bool {
	return errorType == ErrorTypeValidationError
}

func (vefh *ValidationErrorFallbackHandler) HandleFallback(err error, context map[string]interface{}) *parser.SIPMessage {
	// Create a generic 400 Bad Request response
	response := parser.NewResponseMessage(parser.StatusBadRequest, "Bad Request")
	response.SetHeader(parser.HeaderContentType, "text/plain")
	response.SetHeader(parser.HeaderContentLength, "0")
	return response
}

// ProcessingErrorFallbackHandler provides fallback responses for processing errors
type ProcessingErrorFallbackHandler struct{}

func (pefh *ProcessingErrorFallbackHandler) CanHandle(errorType ErrorType) bool {
	return errorType == ErrorTypeProcessingError
}

func (pefh *ProcessingErrorFallbackHandler) HandleFallback(err error, context map[string]interface{}) *parser.SIPMessage {
	// Create a 500 Internal Server Error response
	response := parser.NewResponseMessage(parser.StatusServerInternalError, "Internal Server Error")
	response.SetHeader(parser.HeaderContentType, "text/plain")
	response.SetHeader(parser.HeaderContentLength, "0")
	return response
}

// TransportErrorFallbackHandler provides fallback responses for transport errors
type TransportErrorFallbackHandler struct{}

func (tefh *TransportErrorFallbackHandler) CanHandle(errorType ErrorType) bool {
	return errorType == ErrorTypeTransportError
}

func (tefh *TransportErrorFallbackHandler) HandleFallback(err error, context map[string]interface{}) *parser.SIPMessage {
	// Create a 503 Service Unavailable response
	response := parser.NewResponseMessage(parser.StatusServiceUnavailable, "Service Unavailable")
	response.SetHeader(parser.HeaderContentType, "text/plain")
	response.SetHeader(parser.HeaderContentLength, "0")
	return response
}

// Helper functions

// contains checks if any of the substrings are contained in the main string
func contains(str string, substrings []string) bool {
	for _, substring := range substrings {
		if strings.Contains(str, substring) {
			return true
		}
	}
	return false
}

// GetRecoveryStatistics returns statistics about recovery attempts
func (erm *ErrorRecoveryManager) GetRecoveryStatistics() map[string]interface{} {
	erm.mutex.RLock()
	defer erm.mutex.RUnlock()
	
	stats := make(map[string]interface{})
	
	// Circuit breaker states
	cbStates := make(map[string]string)
	for name, cb := range erm.circuitBreakers {
		cbStates[name] = cb.GetState().String()
	}
	stats["circuit_breaker_states"] = cbStates
	
	// Recovery strategies count
	stats["recovery_strategies_count"] = len(erm.strategies)
	stats["fallback_handlers_count"] = len(erm.fallbackHandlers)
	
	return stats
}