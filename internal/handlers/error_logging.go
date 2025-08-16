package handlers

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/zurustar/xylitol2/internal/parser"
)

// ErrorLogger provides detailed logging for different types of errors
type ErrorLogger interface {
	LogParseError(err error, rawMessage []byte, context map[string]interface{})
	LogValidationError(validationErr *DetailedValidationError, req *parser.SIPMessage, context map[string]interface{})
	LogProcessingError(err error, req *parser.SIPMessage, context map[string]interface{})
	LogTransportError(err error, req *parser.SIPMessage, context map[string]interface{})
	GetErrorStatistics() ErrorStatistics
	ResetStatistics()
}

// DetailedErrorLogger implements comprehensive error logging
type DetailedErrorLogger struct {
	statistics      ErrorStatistics
	statisticsMutex sync.RWMutex
	logLevel        LogLevel
	enableDebug     bool
}

// LogLevel represents different logging levels
type LogLevel int

const (
	LogLevelError LogLevel = iota
	LogLevelWarn
	LogLevelInfo
	LogLevelDebug
)

// String returns the string representation of LogLevel
func (ll LogLevel) String() string {
	switch ll {
	case LogLevelError:
		return "ERROR"
	case LogLevelWarn:
		return "WARN"
	case LogLevelInfo:
		return "INFO"
	case LogLevelDebug:
		return "DEBUG"
	default:
		return "UNKNOWN"
	}
}

// NewDetailedErrorLogger creates a new detailed error logger
func NewDetailedErrorLogger(logLevel LogLevel, enableDebug bool) *DetailedErrorLogger {
	return &DetailedErrorLogger{
		statistics: ErrorStatistics{
			LastReset: time.Now(),
		},
		logLevel:    logLevel,
		enableDebug: enableDebug,
	}
}

// LogParseError logs detailed information about parse errors
func (del *DetailedErrorLogger) LogParseError(err error, rawMessage []byte, context map[string]interface{}) {
	del.statisticsMutex.Lock()
	del.statistics.ParseErrors++
	del.statisticsMutex.Unlock()
	
	logEntry := del.createLogEntry(LogLevelError, "Parse Error", err.Error(), context)
	logEntry["error_type"] = "parse_error"
	logEntry["raw_message_length"] = len(rawMessage)
	
	// Add parse-specific debug information
	if del.enableDebug {
		logEntry["raw_message_preview"] = del.sanitizeRawMessage(rawMessage)
		logEntry["parse_analysis"] = del.analyzeParseError(err, rawMessage)
	}
	
	del.outputLog(logEntry)
}

// LogValidationError logs detailed information about validation errors
func (del *DetailedErrorLogger) LogValidationError(validationErr *DetailedValidationError, req *parser.SIPMessage, context map[string]interface{}) {
	del.statisticsMutex.Lock()
	del.statistics.ValidationErrors++
	del.statisticsMutex.Unlock()
	
	logEntry := del.createLogEntry(LogLevelWarn, "Validation Error", validationErr.Error(), context)
	logEntry["error_type"] = "validation_error"
	logEntry["validator_name"] = validationErr.ValidatorName
	logEntry["status_code"] = validationErr.Code
	logEntry["reason"] = validationErr.Reason
	
	if len(validationErr.MissingHeaders) > 0 {
		logEntry["missing_headers"] = validationErr.MissingHeaders
	}
	
	if len(validationErr.InvalidHeaders) > 0 {
		logEntry["invalid_headers"] = validationErr.InvalidHeaders
	}
	
	if len(validationErr.Suggestions) > 0 {
		logEntry["suggestions"] = validationErr.Suggestions
	}
	
	// Add request-specific information
	if req != nil {
		logEntry["sip_method"] = req.GetMethod()
		logEntry["request_uri"] = req.GetRequestURI()
		logEntry["call_id"] = req.GetHeader(parser.HeaderCallID)
		logEntry["from"] = req.GetHeader(parser.HeaderFrom)
		logEntry["to"] = req.GetHeader(parser.HeaderTo)
		
		if del.enableDebug {
			logEntry["all_headers"] = del.sanitizeHeaders(req)
		}
	}
	
	del.outputLog(logEntry)
}

// LogProcessingError logs detailed information about processing errors
func (del *DetailedErrorLogger) LogProcessingError(err error, req *parser.SIPMessage, context map[string]interface{}) {
	del.statisticsMutex.Lock()
	del.statistics.ProcessingErrors++
	del.statisticsMutex.Unlock()
	
	logEntry := del.createLogEntry(LogLevelError, "Processing Error", err.Error(), context)
	logEntry["error_type"] = "processing_error"
	
	if req != nil {
		logEntry["sip_method"] = req.GetMethod()
		logEntry["request_uri"] = req.GetRequestURI()
		logEntry["call_id"] = req.GetHeader(parser.HeaderCallID)
	}
	
	del.outputLog(logEntry)
}

// LogTransportError logs detailed information about transport errors
func (del *DetailedErrorLogger) LogTransportError(err error, req *parser.SIPMessage, context map[string]interface{}) {
	del.statisticsMutex.Lock()
	del.statistics.TransportErrors++
	del.statisticsMutex.Unlock()
	
	logEntry := del.createLogEntry(LogLevelError, "Transport Error", err.Error(), context)
	logEntry["error_type"] = "transport_error"
	
	if req != nil {
		logEntry["sip_method"] = req.GetMethod()
		logEntry["call_id"] = req.GetHeader(parser.HeaderCallID)
	}
	
	del.outputLog(logEntry)
}

// GetErrorStatistics returns current error statistics
func (del *DetailedErrorLogger) GetErrorStatistics() ErrorStatistics {
	del.statisticsMutex.RLock()
	defer del.statisticsMutex.RUnlock()
	
	return ErrorStatistics{
		ParseErrors:        del.statistics.ParseErrors,
		ValidationErrors:   del.statistics.ValidationErrors,
		ProcessingErrors:   del.statistics.ProcessingErrors,
		TransportErrors:    del.statistics.TransportErrors,
		AuthErrors:         del.statistics.AuthErrors,
		SessionTimerErrors: del.statistics.SessionTimerErrors,
		LastReset:          del.statistics.LastReset,
	}
}

// ResetStatistics resets error statistics
func (del *DetailedErrorLogger) ResetStatistics() {
	del.statisticsMutex.Lock()
	defer del.statisticsMutex.Unlock()
	
	del.statistics = ErrorStatistics{
		LastReset: time.Now(),
	}
}

// createLogEntry creates a base log entry with common fields
func (del *DetailedErrorLogger) createLogEntry(level LogLevel, category, message string, context map[string]interface{}) map[string]interface{} {
	entry := map[string]interface{}{
		"timestamp": time.Now().UTC().Format(time.RFC3339),
		"level":     level.String(),
		"category":  category,
		"message":   message,
	}
	
	// Add context information
	if context != nil {
		for key, value := range context {
			entry[key] = value
		}
	}
	
	return entry
}

// outputLog outputs the log entry (in production, this would use a proper logger)
func (del *DetailedErrorLogger) outputLog(entry map[string]interface{}) {
	// Convert to JSON for structured logging
	jsonBytes, err := json.Marshal(entry)
	if err != nil {
		fmt.Printf("ERROR: Failed to marshal log entry: %v\n", err)
		return
	}
	
	// In production, this would use a proper logging framework
	fmt.Printf("%s\n", string(jsonBytes))
}

// sanitizeRawMessage creates a safe preview of raw message for logging
func (del *DetailedErrorLogger) sanitizeRawMessage(rawMessage []byte) string {
	const maxLength = 200
	message := string(rawMessage)
	
	// Replace sensitive information
	message = del.sanitizeSensitiveInfo(message)
	
	// Truncate if too long
	if len(message) > maxLength {
		message = message[:maxLength] + "..."
	}
	
	// Replace control characters for safe logging
	message = strings.ReplaceAll(message, "\r", "\\r")
	message = strings.ReplaceAll(message, "\n", "\\n")
	
	return message
}

// sanitizeHeaders creates a safe representation of headers for logging
func (del *DetailedErrorLogger) sanitizeHeaders(req *parser.SIPMessage) map[string]string {
	headers := make(map[string]string)
	
	// List of headers to include in debug logs
	debugHeaders := []string{
		parser.HeaderVia,
		parser.HeaderFrom,
		parser.HeaderTo,
		parser.HeaderCallID,
		parser.HeaderCSeq,
		parser.HeaderContentType,
		parser.HeaderContentLength,
		parser.HeaderSessionExpires,
		parser.HeaderMinSE,
		parser.HeaderSupported,
		parser.HeaderRequire,
		parser.HeaderAllow,
		parser.HeaderContact,
		parser.HeaderExpires,
	}
	
	for _, header := range debugHeaders {
		if value := req.GetHeader(header); value != "" {
			headers[header] = del.sanitizeSensitiveInfo(value)
		}
	}
	
	return headers
}

// sanitizeSensitiveInfo removes or masks sensitive information
func (del *DetailedErrorLogger) sanitizeSensitiveInfo(text string) string {
	// Remove or mask authentication information
	text = strings.ReplaceAll(text, "password=", "password=***")
	text = strings.ReplaceAll(text, "response=", "response=***")
	
	// Mask IP addresses in private ranges (for privacy)
	// This is a simple implementation - in production, use proper regex
	if strings.Contains(text, "192.168.") || strings.Contains(text, "10.") || strings.Contains(text, "172.") {
		// Keep the format but mask some digits
		// This is a simplified approach
	}
	
	return text
}

// analyzeParseError provides detailed analysis of parse errors
func (del *DetailedErrorLogger) analyzeParseError(err error, rawMessage []byte) map[string]interface{} {
	analysis := make(map[string]interface{})
	
	errorMsg := strings.ToLower(err.Error())
	messageStr := string(rawMessage)
	
	// Analyze common parse error patterns
	if strings.Contains(errorMsg, "request line") {
		analysis["issue"] = "invalid_request_line"
		analysis["expected_format"] = "METHOD sip:user@domain SIP/2.0"
		
		lines := strings.Split(messageStr, "\r\n")
		if len(lines) > 0 {
			analysis["actual_first_line"] = del.sanitizeSensitiveInfo(lines[0])
		}
	}
	
	if strings.Contains(errorMsg, "status line") {
		analysis["issue"] = "invalid_status_line"
		analysis["expected_format"] = "SIP/2.0 CODE REASON"
		
		lines := strings.Split(messageStr, "\r\n")
		if len(lines) > 0 {
			analysis["actual_first_line"] = del.sanitizeSensitiveInfo(lines[0])
		}
	}
	
	if strings.Contains(errorMsg, "header") {
		analysis["issue"] = "invalid_header_format"
		analysis["expected_format"] = "Header-Name: value"
		
		// Count lines to help identify problematic header
		lines := strings.Split(messageStr, "\r\n")
		analysis["total_lines"] = len(lines)
		
		// Look for lines without colons (invalid headers)
		invalidLines := 0
		for i, line := range lines[1:] { // Skip first line (request/status line)
			if line == "" {
				break // End of headers
			}
			if !strings.Contains(line, ":") {
				invalidLines++
				if invalidLines == 1 {
					analysis["first_invalid_line"] = i + 2 // +2 because we skip first line and arrays are 0-based
					analysis["first_invalid_content"] = del.sanitizeSensitiveInfo(line)
				}
			}
		}
		analysis["invalid_header_lines"] = invalidLines
	}
	
	if strings.Contains(errorMsg, "content-length") {
		analysis["issue"] = "content_length_mismatch"
		analysis["suggestion"] = "Verify Content-Length header matches actual body size"
		
		// Try to extract Content-Length header
		if strings.Contains(messageStr, "Content-Length:") {
			// Simple extraction - in production, use proper parsing
			parts := strings.Split(messageStr, "Content-Length:")
			if len(parts) > 1 {
				headerLine := strings.Split(parts[1], "\r\n")[0]
				analysis["content_length_header"] = strings.TrimSpace(headerLine)
			}
		}
		
		// Calculate actual body size
		if bodyStart := strings.Index(messageStr, "\r\n\r\n"); bodyStart != -1 {
			bodySize := len(messageStr) - bodyStart - 4 // -4 for \r\n\r\n
			analysis["actual_body_size"] = bodySize
		}
	}
	
	// General message statistics
	analysis["message_size"] = len(rawMessage)
	analysis["line_count"] = len(strings.Split(messageStr, "\r\n"))
	analysis["has_body"] = strings.Contains(messageStr, "\r\n\r\n")
	
	return analysis
}

// ErrorMonitor provides monitoring and alerting for error patterns
type ErrorMonitor struct {
	logger     ErrorLogger
	thresholds map[ErrorType]int64
	windows    map[ErrorType]time.Duration
	counters   map[ErrorType]*ErrorCounter
	mutex      sync.RWMutex
}

// ErrorCounter tracks errors within a time window
type ErrorCounter struct {
	count      int64
	windowStart time.Time
	window     time.Duration
}

// NewErrorMonitor creates a new error monitor
func NewErrorMonitor(logger ErrorLogger) *ErrorMonitor {
	monitor := &ErrorMonitor{
		logger:     logger,
		thresholds: make(map[ErrorType]int64),
		windows:    make(map[ErrorType]time.Duration),
		counters:   make(map[ErrorType]*ErrorCounter),
	}
	
	// Set default thresholds and windows
	monitor.SetThreshold(ErrorTypeParseError, 10, 5*time.Minute)
	monitor.SetThreshold(ErrorTypeValidationError, 50, 5*time.Minute)
	monitor.SetThreshold(ErrorTypeProcessingError, 5, 5*time.Minute)
	monitor.SetThreshold(ErrorTypeTransportError, 20, 5*time.Minute)
	
	return monitor
}

// SetThreshold sets the error threshold for a specific error type
func (em *ErrorMonitor) SetThreshold(errorType ErrorType, threshold int64, window time.Duration) {
	em.mutex.Lock()
	defer em.mutex.Unlock()
	
	em.thresholds[errorType] = threshold
	em.windows[errorType] = window
	em.counters[errorType] = &ErrorCounter{
		windowStart: time.Now(),
		window:     window,
	}
}

// RecordError records an error and checks if thresholds are exceeded
func (em *ErrorMonitor) RecordError(errorType ErrorType) bool {
	em.mutex.Lock()
	defer em.mutex.Unlock()
	
	counter, exists := em.counters[errorType]
	if !exists {
		return false
	}
	
	now := time.Now()
	
	// Reset counter if window has expired
	if now.Sub(counter.windowStart) > counter.window {
		counter.count = 0
		counter.windowStart = now
	}
	
	counter.count++
	
	// Check if threshold is exceeded
	threshold, exists := em.thresholds[errorType]
	if exists && counter.count >= threshold {
		return true // Threshold exceeded
	}
	
	return false
}

// GetErrorCounts returns current error counts for all types
func (em *ErrorMonitor) GetErrorCounts() map[ErrorType]int64 {
	em.mutex.RLock()
	defer em.mutex.RUnlock()
	
	counts := make(map[ErrorType]int64)
	for errorType, counter := range em.counters {
		counts[errorType] = counter.count
	}
	
	return counts
}

// ResetCounters resets all error counters
func (em *ErrorMonitor) ResetCounters() {
	em.mutex.Lock()
	defer em.mutex.Unlock()
	
	now := time.Now()
	for _, counter := range em.counters {
		counter.count = 0
		counter.windowStart = now
	}
}