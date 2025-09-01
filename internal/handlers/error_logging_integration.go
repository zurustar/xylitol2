package handlers

import (
	"time"

	"github.com/zurustar/xylitol2/internal/logging"
	"github.com/zurustar/xylitol2/internal/parser"
)

// ErrorLoggingManager integrates enhanced error logging with the SIP server
type ErrorLoggingManager struct {
	errorLogger    ErrorLogger
	logger         logging.Logger
	summaryTicker  *time.Ticker
	stopSummary    chan bool
}

// NewErrorLoggingManager creates a new error logging manager
func NewErrorLoggingManager(logger logging.Logger, enableDebug bool) *ErrorLoggingManager {
	errorLogger := NewDetailedErrorLogger(LogLevelInfo, enableDebug, &LoggerAdapter{logger: logger})
	
	return &ErrorLoggingManager{
		errorLogger: errorLogger,
		logger:      logger,
		stopSummary: make(chan bool),
	}
}

// LoggerAdapter adapts the logging.Logger interface to our Logger interface
type LoggerAdapter struct {
	logger logging.Logger
}

func (la *LoggerAdapter) Debug(msg string, fields ...Field) {
	logFields := make([]logging.Field, len(fields))
	for i, field := range fields {
		logFields[i] = logging.Field{Key: field.Key, Value: field.Value}
	}
	la.logger.Debug(msg, logFields...)
}

func (la *LoggerAdapter) Info(msg string, fields ...Field) {
	logFields := make([]logging.Field, len(fields))
	for i, field := range fields {
		logFields[i] = logging.Field{Key: field.Key, Value: field.Value}
	}
	la.logger.Info(msg, logFields...)
}

func (la *LoggerAdapter) Warn(msg string, fields ...Field) {
	logFields := make([]logging.Field, len(fields))
	for i, field := range fields {
		logFields[i] = logging.Field{Key: field.Key, Value: field.Value}
	}
	la.logger.Warn(msg, logFields...)
}

func (la *LoggerAdapter) Error(msg string, fields ...Field) {
	logFields := make([]logging.Field, len(fields))
	for i, field := range fields {
		logFields[i] = logging.Field{Key: field.Key, Value: field.Value}
	}
	la.logger.Error(msg, logFields...)
}

// Start begins the error logging manager with periodic summaries
func (elm *ErrorLoggingManager) Start() {
	// Start periodic error summary logging (every 5 minutes)
	elm.summaryTicker = time.NewTicker(5 * time.Minute)
	
	go func() {
		for {
			select {
			case <-elm.summaryTicker.C:
				if detailedLogger, ok := elm.errorLogger.(*DetailedErrorLogger); ok {
					detailedLogger.LogErrorSummary()
				}
			case <-elm.stopSummary:
				return
			}
		}
	}()
	
	elm.logger.Info("Error logging manager started with periodic summaries")
}

// Stop stops the error logging manager
func (elm *ErrorLoggingManager) Stop() {
	if elm.summaryTicker != nil {
		elm.summaryTicker.Stop()
	}
	
	close(elm.stopSummary)
	elm.logger.Info("Error logging manager stopped")
}

// LogParseError logs a parse error with context
func (elm *ErrorLoggingManager) LogParseError(err error, rawMessage []byte, sourceAddr, transport string) {
	context := map[string]interface{}{
		"source_addr": sourceAddr,
		"transport":   transport,
		"timestamp":   time.Now().Unix(),
	}
	
	elm.errorLogger.LogParseError(err, rawMessage, context)
}

// LogValidationError logs a validation error with context
func (elm *ErrorLoggingManager) LogValidationError(validationErr *DetailedValidationError, req *parser.SIPMessage, sourceAddr, transport string) {
	context := map[string]interface{}{
		"source_addr": sourceAddr,
		"transport":   transport,
		"timestamp":   time.Now().Unix(),
	}
	
	elm.errorLogger.LogValidationError(validationErr, req, context)
}

// LogProcessingError logs a processing error with context
func (elm *ErrorLoggingManager) LogProcessingError(err error, req *parser.SIPMessage, component, operation string) {
	context := map[string]interface{}{
		"component": component,
		"operation": operation,
		"timestamp": time.Now().Unix(),
	}
	
	elm.errorLogger.LogProcessingError(err, req, context)
}

// LogTransportError logs a transport error with context
func (elm *ErrorLoggingManager) LogTransportError(err error, req *parser.SIPMessage, transport, localAddr, remoteAddr string) {
	context := map[string]interface{}{
		"transport":   transport,
		"local_addr":  localAddr,
		"remote_addr": remoteAddr,
		"timestamp":   time.Now().Unix(),
	}
	
	elm.errorLogger.LogTransportError(err, req, context)
}

// LogAuthenticationError logs an authentication error with context
func (elm *ErrorLoggingManager) LogAuthenticationError(err error, req *parser.SIPMessage, username, realm, sourceAddr string) {
	context := map[string]interface{}{
		"username":    username,
		"realm":       realm,
		"source_addr": sourceAddr,
		"timestamp":   time.Now().Unix(),
	}
	
	elm.errorLogger.LogAuthenticationError(err, req, context)
}

// LogSessionTimerError logs a session timer error with context
func (elm *ErrorLoggingManager) LogSessionTimerError(err error, req *parser.SIPMessage, sessionID string, expectedExpires, actualExpires int) {
	context := map[string]interface{}{
		"session_id":       sessionID,
		"expected_expires": expectedExpires,
		"actual_expires":   actualExpires,
		"timestamp":        time.Now().Unix(),
	}
	
	elm.errorLogger.LogSessionTimerError(err, req, context)
}

// GetErrorStatistics returns current error statistics
func (elm *ErrorLoggingManager) GetErrorStatistics() ErrorStatistics {
	return elm.errorLogger.GetErrorStatistics()
}

// GetDetailedStatistics returns detailed error statistics
func (elm *ErrorLoggingManager) GetDetailedStatistics() DetailedErrorStatistics {
	return elm.errorLogger.GetDetailedStatistics()
}

// GetErrorPatterns returns current error patterns for monitoring
func (elm *ErrorLoggingManager) GetErrorPatterns() map[string]*ErrorPattern {
	if detailedLogger, ok := elm.errorLogger.(*DetailedErrorLogger); ok {
		return detailedLogger.GetErrorPatterns()
	}
	return make(map[string]*ErrorPattern)
}

// ResetStatistics resets all error statistics
func (elm *ErrorLoggingManager) ResetStatistics() {
	elm.errorLogger.ResetStatistics()
	elm.logger.Info("Error statistics reset")
}

// SetLogLevel changes the error logging level
func (elm *ErrorLoggingManager) SetLogLevel(level LogLevel) {
	elm.errorLogger.SetLogLevel(level)
	elm.logger.Info("Error logging level changed", logging.Field{Key: "level", Value: level.String()})
}

// EnableDebugMode enables or disables debug mode
func (elm *ErrorLoggingManager) EnableDebugMode(enable bool) {
	elm.errorLogger.EnableDebugMode(enable)
	elm.logger.Info("Error logging debug mode changed", logging.Field{Key: "enabled", Value: enable})
}

// LogErrorSummary manually triggers an error summary log
func (elm *ErrorLoggingManager) LogErrorSummary() {
	if detailedLogger, ok := elm.errorLogger.(*DetailedErrorLogger); ok {
		detailedLogger.LogErrorSummary()
	}
}

// CheckSystemHealth checks for systematic error patterns and logs warnings
func (elm *ErrorLoggingManager) CheckSystemHealth() {
	stats := elm.GetDetailedStatistics()
	patterns := elm.GetErrorPatterns()
	
	// Check for high error rates
	totalErrors := stats.ParseErrors + stats.ValidationErrors + stats.ProcessingErrors + 
		stats.TransportErrors + stats.AuthErrors + stats.SessionTimerErrors
	
	if totalErrors > 1000 { // Threshold for high error rate
		elm.logger.Warn("High total error rate detected",
			logging.Field{Key: "total_errors", Value: totalErrors},
			logging.Field{Key: "parse_errors", Value: stats.ParseErrors},
			logging.Field{Key: "validation_errors", Value: stats.ValidationErrors},
			logging.Field{Key: "processing_errors", Value: stats.ProcessingErrors},
			logging.Field{Key: "transport_errors", Value: stats.TransportErrors},
			logging.Field{Key: "auth_errors", Value: stats.AuthErrors},
			logging.Field{Key: "session_timer_errors", Value: stats.SessionTimerErrors},
		)
	}
	
	// Check for concerning error patterns
	for pattern, info := range patterns {
		if info.Count > 50 && info.Severity == LogLevelError {
			elm.logger.Warn("High frequency error pattern detected",
				logging.Field{Key: "pattern", Value: pattern},
				logging.Field{Key: "count", Value: info.Count},
				logging.Field{Key: "last_seen", Value: info.LastSeen},
				logging.Field{Key: "severity", Value: info.Severity.String()},
			)
		}
	}
	
	// Check peak hour statistics
	currentHour := time.Now().Hour()
	if stats.ErrorsByHour[currentHour] > 100 {
		elm.logger.Warn("High error rate in current hour",
			logging.Field{Key: "hour", Value: currentHour},
			logging.Field{Key: "error_count", Value: stats.ErrorsByHour[currentHour]},
		)
	}
}