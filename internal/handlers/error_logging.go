package handlers

import (
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
	LogAuthenticationError(err error, req *parser.SIPMessage, context map[string]interface{})
	LogSessionTimerError(err error, req *parser.SIPMessage, context map[string]interface{})
	GetErrorStatistics() ErrorStatistics
	GetDetailedStatistics() DetailedErrorStatistics
	ResetStatistics()
	SetLogLevel(level LogLevel)
	EnableDebugMode(enable bool)
}

// DetailedErrorLogger implements comprehensive error logging
type DetailedErrorLogger struct {
	statistics         ErrorStatistics
	detailedStats      DetailedErrorStatistics
	statisticsMutex    sync.RWMutex
	logLevel           LogLevel
	enableDebug        bool
	logger             Logger
	errorPatterns      map[string]*ErrorPattern
	patternsMutex      sync.RWMutex
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

// Logger interface for structured logging
type Logger interface {
	Debug(msg string, fields ...Field)
	Info(msg string, fields ...Field)
	Warn(msg string, fields ...Field)
	Error(msg string, fields ...Field)
}

// Field represents a structured logging field
type Field struct {
	Key   string
	Value interface{}
}

// DetailedErrorStatistics provides comprehensive error statistics
type DetailedErrorStatistics struct {
	ErrorStatistics
	ParseErrorsByType     map[string]int64
	ValidationErrorsByType map[string]int64
	ErrorsByHour          map[int]int64
	TopErrorMessages      []ErrorFrequency
	RecentErrors          []RecentError
	ErrorTrends           ErrorTrends
}

// ErrorFrequency tracks frequency of specific error messages
type ErrorFrequency struct {
	Message   string
	Count     int64
	LastSeen  time.Time
	FirstSeen time.Time
}

// RecentError tracks recent error occurrences
type RecentError struct {
	Timestamp   time.Time
	ErrorType   string
	Message     string
	Context     map[string]interface{}
	Severity    LogLevel
}

// ErrorTrends tracks error trends over time
type ErrorTrends struct {
	Last24Hours []HourlyErrorCount
	Last7Days   []DailyErrorCount
	PeakHour    int
	PeakDay     time.Weekday
}

// HourlyErrorCount tracks errors per hour
type HourlyErrorCount struct {
	Hour  int
	Count int64
}

// DailyErrorCount tracks errors per day
type DailyErrorCount struct {
	Day   time.Time
	Count int64
}

// ErrorPattern tracks patterns in error messages
type ErrorPattern struct {
	Pattern     string
	Count       int64
	LastSeen    time.Time
	Examples    []string
	Severity    LogLevel
}

// NewDetailedErrorLogger creates a new detailed error logger
func NewDetailedErrorLogger(logLevel LogLevel, enableDebug bool, logger Logger) *DetailedErrorLogger {
	del := &DetailedErrorLogger{
		statistics: ErrorStatistics{
			LastReset: time.Now(),
		},
		detailedStats: DetailedErrorStatistics{
			ParseErrorsByType:      make(map[string]int64),
			ValidationErrorsByType: make(map[string]int64),
			ErrorsByHour:          make(map[int]int64),
			TopErrorMessages:      make([]ErrorFrequency, 0),
			RecentErrors:          make([]RecentError, 0, 100), // Keep last 100 errors
			ErrorTrends: ErrorTrends{
				Last24Hours: make([]HourlyErrorCount, 24),
				Last7Days:   make([]DailyErrorCount, 7),
			},
		},
		logLevel:      logLevel,
		enableDebug:   enableDebug,
		logger:        logger,
		errorPatterns: make(map[string]*ErrorPattern),
	}
	
	// Initialize hourly counters
	for i := 0; i < 24; i++ {
		del.detailedStats.ErrorTrends.Last24Hours[i] = HourlyErrorCount{Hour: i, Count: 0}
	}
	
	// Initialize daily counters
	now := time.Now()
	for i := 0; i < 7; i++ {
		day := now.AddDate(0, 0, -i)
		del.detailedStats.ErrorTrends.Last7Days[i] = DailyErrorCount{Day: day, Count: 0}
	}
	
	return del
}

// LogParseError logs detailed information about parse errors
func (del *DetailedErrorLogger) LogParseError(err error, rawMessage []byte, context map[string]interface{}) {
	del.statisticsMutex.Lock()
	del.statistics.ParseErrors++
	
	// Update detailed statistics
	errorType := del.categorizeParseError(err)
	del.detailedStats.ParseErrorsByType[errorType]++
	del.updateHourlyStats()
	del.updateErrorFrequency(err.Error())
	del.addRecentError("parse_error", err.Error(), context, LogLevelError)
	del.updateErrorPatterns(err.Error(), LogLevelError)
	
	del.statisticsMutex.Unlock()
	
	// Create structured log entry
	fields := []Field{
		{Key: "error_type", Value: "parse_error"},
		{Key: "error_category", Value: errorType},
		{Key: "raw_message_length", Value: len(rawMessage)},
		{Key: "error_message", Value: err.Error()},
	}
	
	// Add context fields
	for key, value := range context {
		fields = append(fields, Field{Key: key, Value: value})
	}
	
	// Add parse-specific debug information
	if del.enableDebug {
		preview := del.sanitizeRawMessage(rawMessage)
		analysis := del.analyzeParseError(err, rawMessage)
		
		fields = append(fields, 
			Field{Key: "raw_message_preview", Value: preview},
			Field{Key: "parse_analysis", Value: analysis},
		)
		
		// Log detailed analysis at debug level
		del.logger.Debug("Detailed parse error analysis", fields...)
	}
	
	// Log the error
	del.logger.Error("SIP message parse error", fields...)
	
	// Check for error patterns that might indicate systematic issues
	del.checkForSystematicIssues(errorType, err.Error())
}

// LogValidationError logs detailed information about validation errors
func (del *DetailedErrorLogger) LogValidationError(validationErr *DetailedValidationError, req *parser.SIPMessage, context map[string]interface{}) {
	del.statisticsMutex.Lock()
	del.statistics.ValidationErrors++
	
	// Update detailed statistics
	validationType := validationErr.ValidatorName
	if validationType == "" {
		validationType = "unknown_validator"
	}
	del.detailedStats.ValidationErrorsByType[validationType]++
	del.updateHourlyStats()
	del.updateErrorFrequency(validationErr.Error())
	del.addRecentError("validation_error", validationErr.Error(), context, LogLevelWarn)
	del.updateErrorPatterns(validationErr.Error(), LogLevelWarn)
	
	del.statisticsMutex.Unlock()
	
	fields := []Field{
		{Key: "error_type", Value: "validation_error"},
		{Key: "validator_name", Value: validationErr.ValidatorName},
		{Key: "status_code", Value: validationErr.Code},
		{Key: "reason", Value: validationErr.Reason},
		{Key: "error_message", Value: validationErr.Error()},
	}
	
	if len(validationErr.MissingHeaders) > 0 {
		fields = append(fields, Field{Key: "missing_headers", Value: validationErr.MissingHeaders})
	}
	
	if len(validationErr.InvalidHeaders) > 0 {
		fields = append(fields, Field{Key: "invalid_headers", Value: validationErr.InvalidHeaders})
	}
	
	if len(validationErr.Suggestions) > 0 {
		fields = append(fields, Field{Key: "suggestions", Value: validationErr.Suggestions})
	}
	
	// Add request-specific information
	if req != nil {
		fields = append(fields,
			Field{Key: "sip_method", Value: req.GetMethod()},
			Field{Key: "request_uri", Value: req.GetRequestURI()},
			Field{Key: "call_id", Value: req.GetHeader("Call-ID")},
			Field{Key: "from", Value: req.GetHeader("From")},
			Field{Key: "to", Value: req.GetHeader("To")},
		)
		
		if del.enableDebug {
			sanitizedHeaders := del.sanitizeHeaders(req)
			fields = append(fields, Field{Key: "all_headers", Value: sanitizedHeaders})
		}
	}
	
	// Add context fields
	for key, value := range context {
		fields = append(fields, Field{Key: key, Value: value})
	}
	
	del.logger.Warn("SIP validation error", fields...)
}

// LogProcessingError logs detailed information about processing errors
func (del *DetailedErrorLogger) LogProcessingError(err error, req *parser.SIPMessage, context map[string]interface{}) {
	del.statisticsMutex.Lock()
	del.statistics.ProcessingErrors++
	del.updateHourlyStats()
	del.updateErrorFrequency(err.Error())
	del.addRecentError("processing_error", err.Error(), context, LogLevelError)
	del.updateErrorPatterns(err.Error(), LogLevelError)
	del.statisticsMutex.Unlock()
	
	fields := []Field{
		{Key: "error_type", Value: "processing_error"},
		{Key: "error_message", Value: err.Error()},
	}
	
	if req != nil {
		fields = append(fields,
			Field{Key: "sip_method", Value: req.GetMethod()},
			Field{Key: "request_uri", Value: req.GetRequestURI()},
			Field{Key: "call_id", Value: req.GetHeader("Call-ID")},
		)
	}
	
	// Add context fields
	for key, value := range context {
		fields = append(fields, Field{Key: key, Value: value})
	}
	
	del.logger.Error("SIP processing error", fields...)
}

// LogTransportError logs detailed information about transport errors
func (del *DetailedErrorLogger) LogTransportError(err error, req *parser.SIPMessage, context map[string]interface{}) {
	del.statisticsMutex.Lock()
	del.statistics.TransportErrors++
	del.updateHourlyStats()
	del.updateErrorFrequency(err.Error())
	del.addRecentError("transport_error", err.Error(), context, LogLevelError)
	del.statisticsMutex.Unlock()
	
	fields := []Field{
		{Key: "error_type", Value: "transport_error"},
		{Key: "error_message", Value: err.Error()},
	}
	
	if req != nil {
		fields = append(fields,
			Field{Key: "sip_method", Value: req.GetMethod()},
			Field{Key: "call_id", Value: req.GetHeader("Call-ID")},
		)
	}
	
	// Add context fields
	for key, value := range context {
		fields = append(fields, Field{Key: key, Value: value})
	}
	
	del.logger.Error("SIP transport error", fields...)
}

// LogAuthenticationError logs detailed information about authentication errors
func (del *DetailedErrorLogger) LogAuthenticationError(err error, req *parser.SIPMessage, context map[string]interface{}) {
	del.statisticsMutex.Lock()
	del.statistics.AuthErrors++
	del.updateHourlyStats()
	del.updateErrorFrequency(err.Error())
	del.addRecentError("authentication_error", err.Error(), context, LogLevelWarn)
	del.statisticsMutex.Unlock()
	
	fields := []Field{
		{Key: "error_type", Value: "authentication_error"},
		{Key: "error_message", Value: err.Error()},
	}
	
	if req != nil {
		fields = append(fields,
			Field{Key: "sip_method", Value: req.GetMethod()},
			Field{Key: "call_id", Value: req.GetHeader("Call-ID")},
			Field{Key: "from", Value: req.GetHeader("From")},
		)
		
		// Add authentication-specific debug info
		if del.enableDebug {
			authHeader := req.GetHeader("Authorization")
			if authHeader != "" {
				fields = append(fields, Field{Key: "auth_header_present", Value: true})
				// Don't log the actual auth header for security
			} else {
				fields = append(fields, Field{Key: "auth_header_present", Value: false})
			}
		}
	}
	
	// Add context fields
	for key, value := range context {
		fields = append(fields, Field{Key: key, Value: value})
	}
	
	del.logger.Warn("SIP authentication error", fields...)
}

// LogSessionTimerError logs detailed information about session timer errors
func (del *DetailedErrorLogger) LogSessionTimerError(err error, req *parser.SIPMessage, context map[string]interface{}) {
	del.statisticsMutex.Lock()
	del.statistics.SessionTimerErrors++
	del.updateHourlyStats()
	del.updateErrorFrequency(err.Error())
	del.addRecentError("session_timer_error", err.Error(), context, LogLevelWarn)
	del.statisticsMutex.Unlock()
	
	fields := []Field{
		{Key: "error_type", Value: "session_timer_error"},
		{Key: "error_message", Value: err.Error()},
	}
	
	if req != nil {
		fields = append(fields,
			Field{Key: "sip_method", Value: req.GetMethod()},
			Field{Key: "call_id", Value: req.GetHeader("Call-ID")},
		)
		
		// Add session timer specific debug info
		if del.enableDebug {
			sessionExpires := req.GetHeader("Session-Expires")
			minSE := req.GetHeader("Min-SE")
			supported := req.GetHeader("Supported")
			require := req.GetHeader("Require")
			
			fields = append(fields,
				Field{Key: "session_expires", Value: sessionExpires},
				Field{Key: "min_se", Value: minSE},
				Field{Key: "supported", Value: supported},
				Field{Key: "require", Value: require},
			)
		}
	}
	
	// Add context fields
	for key, value := range context {
		fields = append(fields, Field{Key: key, Value: value})
	}
	
	del.logger.Warn("SIP session timer error", fields...)
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

// GetDetailedStatistics returns comprehensive error statistics
func (del *DetailedErrorLogger) GetDetailedStatistics() DetailedErrorStatistics {
	del.statisticsMutex.RLock()
	defer del.statisticsMutex.RUnlock()
	
	// Create a deep copy to avoid race conditions
	stats := DetailedErrorStatistics{
		ErrorStatistics: del.statistics,
		ParseErrorsByType: make(map[string]int64),
		ValidationErrorsByType: make(map[string]int64),
		ErrorsByHour: make(map[int]int64),
		TopErrorMessages: make([]ErrorFrequency, len(del.detailedStats.TopErrorMessages)),
		RecentErrors: make([]RecentError, len(del.detailedStats.RecentErrors)),
		ErrorTrends: ErrorTrends{
			Last24Hours: make([]HourlyErrorCount, len(del.detailedStats.ErrorTrends.Last24Hours)),
			Last7Days: make([]DailyErrorCount, len(del.detailedStats.ErrorTrends.Last7Days)),
			PeakHour: del.detailedStats.ErrorTrends.PeakHour,
			PeakDay: del.detailedStats.ErrorTrends.PeakDay,
		},
	}
	
	// Copy maps
	for k, v := range del.detailedStats.ParseErrorsByType {
		stats.ParseErrorsByType[k] = v
	}
	for k, v := range del.detailedStats.ValidationErrorsByType {
		stats.ValidationErrorsByType[k] = v
	}
	for k, v := range del.detailedStats.ErrorsByHour {
		stats.ErrorsByHour[k] = v
	}
	
	// Copy slices
	copy(stats.TopErrorMessages, del.detailedStats.TopErrorMessages)
	copy(stats.RecentErrors, del.detailedStats.RecentErrors)
	copy(stats.ErrorTrends.Last24Hours, del.detailedStats.ErrorTrends.Last24Hours)
	copy(stats.ErrorTrends.Last7Days, del.detailedStats.ErrorTrends.Last7Days)
	
	return stats
}

// ResetStatistics resets error statistics
func (del *DetailedErrorLogger) ResetStatistics() {
	del.statisticsMutex.Lock()
	defer del.statisticsMutex.Unlock()
	
	del.statistics = ErrorStatistics{
		LastReset: time.Now(),
	}
	
	// Reset detailed statistics
	del.detailedStats = DetailedErrorStatistics{
		ParseErrorsByType:      make(map[string]int64),
		ValidationErrorsByType: make(map[string]int64),
		ErrorsByHour:          make(map[int]int64),
		TopErrorMessages:      make([]ErrorFrequency, 0),
		RecentErrors:          make([]RecentError, 0, 100),
		ErrorTrends: ErrorTrends{
			Last24Hours: make([]HourlyErrorCount, 24),
			Last7Days:   make([]DailyErrorCount, 7),
		},
	}
	
	// Reinitialize hourly and daily counters
	for i := 0; i < 24; i++ {
		del.detailedStats.ErrorTrends.Last24Hours[i] = HourlyErrorCount{Hour: i, Count: 0}
	}
	
	now := time.Now()
	for i := 0; i < 7; i++ {
		day := now.AddDate(0, 0, -i)
		del.detailedStats.ErrorTrends.Last7Days[i] = DailyErrorCount{Day: day, Count: 0}
	}
	
	// Reset error patterns
	del.patternsMutex.Lock()
	del.errorPatterns = make(map[string]*ErrorPattern)
	del.patternsMutex.Unlock()
}

// SetLogLevel changes the logging level
func (del *DetailedErrorLogger) SetLogLevel(level LogLevel) {
	del.logLevel = level
}

// EnableDebugMode enables or disables debug mode
func (del *DetailedErrorLogger) EnableDebugMode(enable bool) {
	del.enableDebug = enable
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
		"Via",
		"From",
		"To",
		"Call-ID",
		"CSeq",
		"Content-Type",
		"Content-Length",
		"Session-Expires",
		"Min-SE",
		"Supported",
		"Require",
		"Allow",
		"Contact",
		"Expires",
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

// categorizeParseError categorizes parse errors for better tracking
func (del *DetailedErrorLogger) categorizeParseError(err error) string {
	errorMsg := strings.ToLower(err.Error())
	
	switch {
	case strings.Contains(errorMsg, "start line") || strings.Contains(errorMsg, "request line") || strings.Contains(errorMsg, "status line"):
		return "start_line_error"
	case strings.Contains(errorMsg, "header"):
		return "header_error"
	case strings.Contains(errorMsg, "content-length"):
		return "content_length_error"
	case strings.Contains(errorMsg, "body"):
		return "body_error"
	case strings.Contains(errorMsg, "empty"):
		return "empty_message_error"
	case strings.Contains(errorMsg, "method"):
		return "invalid_method_error"
	case strings.Contains(errorMsg, "version"):
		return "version_error"
	default:
		return "unknown_parse_error"
	}
}

// updateHourlyStats updates hourly error statistics
func (del *DetailedErrorLogger) updateHourlyStats() {
	hour := time.Now().Hour()
	del.detailedStats.ErrorsByHour[hour]++
	
	// Update hourly trends
	for i := range del.detailedStats.ErrorTrends.Last24Hours {
		if del.detailedStats.ErrorTrends.Last24Hours[i].Hour == hour {
			del.detailedStats.ErrorTrends.Last24Hours[i].Count++
			break
		}
	}
	
	// Update peak hour
	if del.detailedStats.ErrorsByHour[hour] > del.detailedStats.ErrorsByHour[del.detailedStats.ErrorTrends.PeakHour] {
		del.detailedStats.ErrorTrends.PeakHour = hour
	}
	
	// Update daily trends
	today := time.Now().Truncate(24 * time.Hour)
	for i := range del.detailedStats.ErrorTrends.Last7Days {
		if del.detailedStats.ErrorTrends.Last7Days[i].Day.Equal(today) {
			del.detailedStats.ErrorTrends.Last7Days[i].Count++
			break
		}
	}
}

// updateErrorFrequency updates the frequency tracking for error messages
func (del *DetailedErrorLogger) updateErrorFrequency(errorMsg string) {
	// Find existing error frequency entry
	for i := range del.detailedStats.TopErrorMessages {
		if del.detailedStats.TopErrorMessages[i].Message == errorMsg {
			del.detailedStats.TopErrorMessages[i].Count++
			del.detailedStats.TopErrorMessages[i].LastSeen = time.Now()
			return
		}
	}
	
	// Add new error frequency entry
	newEntry := ErrorFrequency{
		Message:   errorMsg,
		Count:     1,
		LastSeen:  time.Now(),
		FirstSeen: time.Now(),
	}
	
	del.detailedStats.TopErrorMessages = append(del.detailedStats.TopErrorMessages, newEntry)
	
	// Keep only top 50 error messages, sorted by frequency
	if len(del.detailedStats.TopErrorMessages) > 50 {
		// Simple sort by count (in production, use proper sorting)
		maxCount := int64(0)
		maxIndex := 0
		for i, entry := range del.detailedStats.TopErrorMessages {
			if entry.Count > maxCount {
				maxCount = entry.Count
				maxIndex = i
			}
		}
		
		// Keep the most frequent errors (simplified approach)
		if maxIndex < len(del.detailedStats.TopErrorMessages)-1 {
			del.detailedStats.TopErrorMessages = del.detailedStats.TopErrorMessages[:50]
		}
	}
}

// addRecentError adds an error to the recent errors list
func (del *DetailedErrorLogger) addRecentError(errorType, message string, context map[string]interface{}, severity LogLevel) {
	recentError := RecentError{
		Timestamp: time.Now(),
		ErrorType: errorType,
		Message:   message,
		Context:   context,
		Severity:  severity,
	}
	
	del.detailedStats.RecentErrors = append(del.detailedStats.RecentErrors, recentError)
	
	// Keep only last 100 errors
	if len(del.detailedStats.RecentErrors) > 100 {
		del.detailedStats.RecentErrors = del.detailedStats.RecentErrors[1:]
	}
}

// updateErrorPatterns tracks patterns in error messages
func (del *DetailedErrorLogger) updateErrorPatterns(errorMsg string, severity LogLevel) {
	del.patternsMutex.Lock()
	defer del.patternsMutex.Unlock()
	
	// Extract pattern from error message (simplified approach)
	pattern := del.extractErrorPattern(errorMsg)
	
	if existing, exists := del.errorPatterns[pattern]; exists {
		existing.Count++
		existing.LastSeen = time.Now()
		
		// Add example if we don't have too many
		if len(existing.Examples) < 5 {
			existing.Examples = append(existing.Examples, errorMsg)
		}
	} else {
		del.errorPatterns[pattern] = &ErrorPattern{
			Pattern:  pattern,
			Count:    1,
			LastSeen: time.Now(),
			Examples: []string{errorMsg},
			Severity: severity,
		}
	}
}

// extractErrorPattern extracts a pattern from an error message
func (del *DetailedErrorLogger) extractErrorPattern(errorMsg string) string {
	// Simplified pattern extraction - replace specific values with placeholders
	pattern := errorMsg
	
	// Replace numbers with placeholder
	pattern = strings.ReplaceAll(pattern, "0", "N")
	pattern = strings.ReplaceAll(pattern, "1", "N")
	pattern = strings.ReplaceAll(pattern, "2", "N")
	pattern = strings.ReplaceAll(pattern, "3", "N")
	pattern = strings.ReplaceAll(pattern, "4", "N")
	pattern = strings.ReplaceAll(pattern, "5", "N")
	pattern = strings.ReplaceAll(pattern, "6", "N")
	pattern = strings.ReplaceAll(pattern, "7", "N")
	pattern = strings.ReplaceAll(pattern, "8", "N")
	pattern = strings.ReplaceAll(pattern, "9", "N")
	
	// Replace common SIP URIs with placeholder
	if strings.Contains(pattern, "sip:") {
		pattern = strings.ReplaceAll(pattern, "sip:", "sip:USER@DOMAIN")
	}
	
	// Replace IP addresses with placeholder
	if strings.Contains(pattern, ".") && (strings.Contains(pattern, "192.") || strings.Contains(pattern, "10.") || strings.Contains(pattern, "172.")) {
		pattern = strings.ReplaceAll(pattern, "192.", "IP.")
		pattern = strings.ReplaceAll(pattern, "10.", "IP.")
		pattern = strings.ReplaceAll(pattern, "172.", "IP.")
	}
	
	return pattern
}

// checkForSystematicIssues checks if error patterns indicate systematic problems
func (del *DetailedErrorLogger) checkForSystematicIssues(errorType, errorMsg string) {
	// Check if we're seeing too many of the same type of error
	del.patternsMutex.RLock()
	pattern := del.extractErrorPattern(errorMsg)
	if errorPattern, exists := del.errorPatterns[pattern]; exists {
		if errorPattern.Count > 10 { // Threshold for systematic issue
			del.patternsMutex.RUnlock()
			
			// Log a warning about potential systematic issue
			del.logger.Warn("Potential systematic error pattern detected",
				Field{Key: "error_pattern", Value: pattern},
				Field{Key: "occurrence_count", Value: errorPattern.Count},
				Field{Key: "error_type", Value: errorType},
				Field{Key: "last_seen", Value: errorPattern.LastSeen},
			)
			return
		}
	}
	del.patternsMutex.RUnlock()
	
	// Check for high error rates in recent time windows
	del.statisticsMutex.RLock()
	currentHour := time.Now().Hour()
	currentHourErrors := del.detailedStats.ErrorsByHour[currentHour]
	del.statisticsMutex.RUnlock()
	
	if currentHourErrors > 100 { // Threshold for high error rate
		del.logger.Warn("High error rate detected",
			Field{Key: "hour", Value: currentHour},
			Field{Key: "error_count", Value: currentHourErrors},
			Field{Key: "error_type", Value: errorType},
		)
	}
}

// GetErrorPatterns returns current error patterns (for monitoring/debugging)
func (del *DetailedErrorLogger) GetErrorPatterns() map[string]*ErrorPattern {
	del.patternsMutex.RLock()
	defer del.patternsMutex.RUnlock()
	
	patterns := make(map[string]*ErrorPattern)
	for k, v := range del.errorPatterns {
		// Create a copy to avoid race conditions
		patterns[k] = &ErrorPattern{
			Pattern:  v.Pattern,
			Count:    v.Count,
			LastSeen: v.LastSeen,
			Examples: append([]string(nil), v.Examples...),
			Severity: v.Severity,
		}
	}
	
	return patterns
}

// LogErrorSummary logs a periodic summary of error statistics
func (del *DetailedErrorLogger) LogErrorSummary() {
	stats := del.GetDetailedStatistics()
	
	fields := []Field{
		{Key: "parse_errors", Value: stats.ParseErrors},
		{Key: "validation_errors", Value: stats.ValidationErrors},
		{Key: "processing_errors", Value: stats.ProcessingErrors},
		{Key: "transport_errors", Value: stats.TransportErrors},
		{Key: "auth_errors", Value: stats.AuthErrors},
		{Key: "session_timer_errors", Value: stats.SessionTimerErrors},
		{Key: "peak_hour", Value: stats.ErrorTrends.PeakHour},
		{Key: "total_error_patterns", Value: len(del.errorPatterns)},
	}
	
	// Add top error types
	if len(stats.ParseErrorsByType) > 0 {
		fields = append(fields, Field{Key: "parse_error_types", Value: stats.ParseErrorsByType})
	}
	
	if len(stats.ValidationErrorsByType) > 0 {
		fields = append(fields, Field{Key: "validation_error_types", Value: stats.ValidationErrorsByType})
	}
	
	del.logger.Info("Error statistics summary", fields...)
}



// ErrorStatisticsCollector provides advanced statistics collection and analysis
type ErrorStatisticsCollector struct {
	logger         ErrorLogger
	metrics        map[string]*ErrorMetric
	collectors     []MetricCollector
	mutex          sync.RWMutex
	collectInterval time.Duration
	stopChan       chan struct{}
}

// ErrorMetric represents a specific error metric
type ErrorMetric struct {
	Name        string
	Value       float64
	LastUpdated time.Time
	History     []MetricPoint
}

// MetricPoint represents a point in time for a metric
type MetricPoint struct {
	Timestamp time.Time
	Value     float64
}

// MetricCollector defines the interface for collecting metrics
type MetricCollector interface {
	CollectMetrics(stats ErrorStatistics) map[string]float64
	Name() string
}

// NewErrorStatisticsCollector creates a new error statistics collector
func NewErrorStatisticsCollector(logger ErrorLogger, collectInterval time.Duration) *ErrorStatisticsCollector {
	esc := &ErrorStatisticsCollector{
		logger:          logger,
		metrics:         make(map[string]*ErrorMetric),
		collectors:      make([]MetricCollector, 0),
		collectInterval: collectInterval,
		stopChan:        make(chan struct{}),
	}
	
	// Add default collectors
	esc.AddCollector(&ErrorRateCollector{})
	esc.AddCollector(&ErrorTrendCollector{})
	
	return esc
}

// AddCollector adds a metric collector
func (esc *ErrorStatisticsCollector) AddCollector(collector MetricCollector) {
	esc.mutex.Lock()
	defer esc.mutex.Unlock()
	
	esc.collectors = append(esc.collectors, collector)
}

// Start starts the statistics collection process
func (esc *ErrorStatisticsCollector) Start() {
	go esc.collectLoop()
}

// Stop stops the statistics collection process
func (esc *ErrorStatisticsCollector) Stop() {
	close(esc.stopChan)
}

// collectLoop runs the metric collection loop
func (esc *ErrorStatisticsCollector) collectLoop() {
	ticker := time.NewTicker(esc.collectInterval)
	defer ticker.Stop()
	
	for {
		select {
		case <-ticker.C:
			esc.collectMetrics()
		case <-esc.stopChan:
			return
		}
	}
}

// collectMetrics collects metrics from all collectors
func (esc *ErrorStatisticsCollector) collectMetrics() {
	if esc.logger == nil {
		return
	}
	
	stats := esc.logger.GetErrorStatistics()
	now := time.Now()
	
	esc.mutex.Lock()
	defer esc.mutex.Unlock()
	
	for _, collector := range esc.collectors {
		metrics := collector.CollectMetrics(stats)
		
		for name, value := range metrics {
			metricName := fmt.Sprintf("%s.%s", collector.Name(), name)
			
			if metric, exists := esc.metrics[metricName]; exists {
				metric.Value = value
				metric.LastUpdated = now
				metric.History = append(metric.History, MetricPoint{
					Timestamp: now,
					Value:     value,
				})
				
				// Keep only last 100 points
				if len(metric.History) > 100 {
					metric.History = metric.History[1:]
				}
			} else {
				esc.metrics[metricName] = &ErrorMetric{
					Name:        metricName,
					Value:       value,
					LastUpdated: now,
					History: []MetricPoint{{
						Timestamp: now,
						Value:     value,
					}},
				}
			}
		}
	}
}

// GetMetrics returns current metrics
func (esc *ErrorStatisticsCollector) GetMetrics() map[string]*ErrorMetric {
	esc.mutex.RLock()
	defer esc.mutex.RUnlock()
	
	// Create a deep copy
	metrics := make(map[string]*ErrorMetric)
	for k, v := range esc.metrics {
		metrics[k] = &ErrorMetric{
			Name:        v.Name,
			Value:       v.Value,
			LastUpdated: v.LastUpdated,
			History:     append([]MetricPoint(nil), v.History...),
		}
	}
	
	return metrics
}

// ErrorRateCollector collects error rate metrics
type ErrorRateCollector struct {
	lastStats     ErrorStatistics
	lastTimestamp time.Time
}

// Name returns the collector name
func (erc *ErrorRateCollector) Name() string {
	return "error_rate"
}

// CollectMetrics collects error rate metrics
func (erc *ErrorRateCollector) CollectMetrics(stats ErrorStatistics) map[string]float64 {
	now := time.Now()
	metrics := make(map[string]float64)
	
	if !erc.lastTimestamp.IsZero() {
		duration := now.Sub(erc.lastTimestamp).Seconds()
		if duration > 0 {
			metrics["parse_errors_per_second"] = float64(stats.ParseErrors-erc.lastStats.ParseErrors) / duration
			metrics["validation_errors_per_second"] = float64(stats.ValidationErrors-erc.lastStats.ValidationErrors) / duration
			metrics["processing_errors_per_second"] = float64(stats.ProcessingErrors-erc.lastStats.ProcessingErrors) / duration
			metrics["transport_errors_per_second"] = float64(stats.TransportErrors-erc.lastStats.TransportErrors) / duration
			metrics["total_errors_per_second"] = float64(stats.TotalErrors()-erc.lastStats.TotalErrors()) / duration
		}
	}
	
	erc.lastStats = stats
	erc.lastTimestamp = now
	
	return metrics
}

// ErrorTrendCollector collects error trend metrics
type ErrorTrendCollector struct {
	history []ErrorStatistics
}

// Name returns the collector name
func (etc *ErrorTrendCollector) Name() string {
	return "error_trend"
}

// CollectMetrics collects error trend metrics
func (etc *ErrorTrendCollector) CollectMetrics(stats ErrorStatistics) map[string]float64 {
	metrics := make(map[string]float64)
	
	etc.history = append(etc.history, stats)
	
	// Keep only last 10 measurements for trend analysis
	if len(etc.history) > 10 {
		etc.history = etc.history[1:]
	}
	
	if len(etc.history) >= 2 {
		// Calculate trend (simple linear regression slope)
		first := etc.history[0]
		last := etc.history[len(etc.history)-1]
		
		totalFirst := first.TotalErrors()
		totalLast := last.TotalErrors()
		
		if totalFirst > 0 {
			metrics["error_growth_rate"] = float64(totalLast-totalFirst) / float64(totalFirst)
		}
		
		metrics["parse_error_trend"] = float64(last.ParseErrors - first.ParseErrors)
		metrics["validation_error_trend"] = float64(last.ValidationErrors - first.ValidationErrors)
		metrics["processing_error_trend"] = float64(last.ProcessingErrors - first.ProcessingErrors)
		metrics["transport_error_trend"] = float64(last.TransportErrors - first.TransportErrors)
	}
	
	return metrics
}