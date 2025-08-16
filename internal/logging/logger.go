package logging

import (
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"time"
)

// LogLevel represents the logging level
type LogLevel int

const (
	DebugLevel LogLevel = iota
	InfoLevel
	WarnLevel
	ErrorLevel
)

// String returns the string representation of the log level
func (l LogLevel) String() string {
	switch l {
	case DebugLevel:
		return "DEBUG"
	case InfoLevel:
		return "INFO"
	case WarnLevel:
		return "WARN"
	case ErrorLevel:
		return "ERROR"
	default:
		return "UNKNOWN"
	}
}

// ParseLogLevel parses a string into a LogLevel
func ParseLogLevel(level string) (LogLevel, error) {
	switch strings.ToLower(level) {
	case "debug":
		return DebugLevel, nil
	case "info":
		return InfoLevel, nil
	case "warn":
		return WarnLevel, nil
	case "error":
		return ErrorLevel, nil
	default:
		return InfoLevel, fmt.Errorf("invalid log level: %s", level)
	}
}

// StructuredLogger implements the Logger interface with structured logging
type StructuredLogger struct {
	level  LogLevel
	logger *log.Logger
	writer io.Writer
}

// NewStructuredLogger creates a new structured logger
func NewStructuredLogger(level LogLevel, writer io.Writer) *StructuredLogger {
	return &StructuredLogger{
		level:  level,
		logger: log.New(writer, "", 0), // We'll format timestamps ourselves
		writer: writer,
	}
}

// NewFileLogger creates a logger that writes to a file
func NewFileLogger(level LogLevel, filename string) (*StructuredLogger, error) {
	file, err := os.OpenFile(filename, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		return nil, fmt.Errorf("failed to open log file %s: %w", filename, err)
	}
	
	return NewStructuredLogger(level, file), nil
}

// NewConsoleLogger creates a logger that writes to stdout
func NewConsoleLogger(level LogLevel) *StructuredLogger {
	return NewStructuredLogger(level, os.Stdout)
}

// NewMultiLogger creates a logger that writes to multiple outputs
func NewMultiLogger(level LogLevel, writers ...io.Writer) *StructuredLogger {
	multiWriter := io.MultiWriter(writers...)
	return NewStructuredLogger(level, multiWriter)
}

// Debug logs a debug message with optional fields
func (l *StructuredLogger) Debug(msg string, fields ...Field) {
	if l.level <= DebugLevel {
		l.log(DebugLevel, msg, fields...)
	}
}

// Info logs an info message with optional fields
func (l *StructuredLogger) Info(msg string, fields ...Field) {
	if l.level <= InfoLevel {
		l.log(InfoLevel, msg, fields...)
	}
}

// Warn logs a warning message with optional fields
func (l *StructuredLogger) Warn(msg string, fields ...Field) {
	if l.level <= WarnLevel {
		l.log(WarnLevel, msg, fields...)
	}
}

// Error logs an error message with optional fields
func (l *StructuredLogger) Error(msg string, fields ...Field) {
	if l.level <= ErrorLevel {
		l.log(ErrorLevel, msg, fields...)
	}
}

// log formats and writes the log message
func (l *StructuredLogger) log(level LogLevel, msg string, fields ...Field) {
	timestamp := time.Now().Format("2006-01-02 15:04:05.000")
	
	// Build the log message
	logMsg := fmt.Sprintf("[%s] %s: %s", timestamp, level.String(), msg)
	
	// Add structured fields if any
	if len(fields) > 0 {
		fieldStrs := make([]string, len(fields))
		for i, field := range fields {
			fieldStrs[i] = fmt.Sprintf("%s=%v", field.Key, field.Value)
		}
		logMsg += " | " + strings.Join(fieldStrs, " ")
	}
	
	l.logger.Println(logMsg)
}

// SetLevel changes the logging level
func (l *StructuredLogger) SetLevel(level LogLevel) {
	l.level = level
}

// GetLevel returns the current logging level
func (l *StructuredLogger) GetLevel() LogLevel {
	return l.level
}

// Close closes the logger if it's writing to a file
func (l *StructuredLogger) Close() error {
	if closer, ok := l.writer.(io.Closer); ok {
		return closer.Close()
	}
	return nil
}

// Helper functions for creating common fields

// StringField creates a string field
func StringField(key, value string) Field {
	return Field{Key: key, Value: value}
}

// IntField creates an integer field
func IntField(key string, value int) Field {
	return Field{Key: key, Value: value}
}

// ErrorField creates an error field
func ErrorField(err error) Field {
	return Field{Key: "error", Value: err.Error()}
}

// TransactionField creates a transaction ID field
func TransactionField(txnID string) Field {
	return Field{Key: "transaction_id", Value: txnID}
}

// SessionField creates a session ID field
func SessionField(sessionID string) Field {
	return Field{Key: "session_id", Value: sessionID}
}

// MethodField creates a SIP method field
func MethodField(method string) Field {
	return Field{Key: "sip_method", Value: method}
}

// AddressField creates an address field
func AddressField(key, address string) Field {
	return Field{Key: key, Value: address}
}

// CallIDField creates a Call-ID field
func CallIDField(callID string) Field {
	return Field{Key: "call_id", Value: callID}
}

// UserField creates a user field
func UserField(user string) Field {
	return Field{Key: "user", Value: user}
}

// LoggerConfig represents logger configuration
type LoggerConfig struct {
	Level string
	File  string
}

// NewLoggerFromConfig creates a logger based on configuration
func NewLoggerFromConfig(config LoggerConfig) (Logger, error) {
	level, err := ParseLogLevel(config.Level)
	if err != nil {
		return nil, err
	}

	if config.File == "" || config.File == "stdout" {
		// Log to console only
		return NewConsoleLogger(level), nil
	}

	// Create file logger
	fileLogger, err := NewFileLogger(level, config.File)
	if err != nil {
		return nil, err
	}

	// Also log to console for important messages (warn and error)
	if level <= WarnLevel {
		consoleWriter := os.Stdout
		return NewMultiLogger(level, fileLogger.writer, consoleWriter), nil
	}

	return fileLogger, nil
}