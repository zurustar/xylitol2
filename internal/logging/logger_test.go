package logging

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseLogLevel(t *testing.T) {
	tests := []struct {
		input       string
		expected    LogLevel
		expectError bool
	}{
		{"debug", DebugLevel, false},
		{"info", InfoLevel, false},
		{"warn", WarnLevel, false},
		{"error", ErrorLevel, false},
		{"DEBUG", DebugLevel, false},
		{"INFO", InfoLevel, false},
		{"WARN", WarnLevel, false},
		{"ERROR", ErrorLevel, false},
		{"invalid", InfoLevel, true},
		{"", InfoLevel, true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			level, err := ParseLogLevel(tt.input)
			
			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error for input '%s'", tt.input)
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error for input '%s': %v", tt.input, err)
				}
				if level != tt.expected {
					t.Errorf("Expected level %v, got %v", tt.expected, level)
				}
			}
		})
	}
}

func TestLogLevel_String(t *testing.T) {
	tests := []struct {
		level    LogLevel
		expected string
	}{
		{DebugLevel, "DEBUG"},
		{InfoLevel, "INFO"},
		{WarnLevel, "WARN"},
		{ErrorLevel, "ERROR"},
		{LogLevel(999), "UNKNOWN"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := tt.level.String()
			if result != tt.expected {
				t.Errorf("Expected %s, got %s", tt.expected, result)
			}
		})
	}
}

func TestStructuredLogger_LogLevels(t *testing.T) {
	var buf bytes.Buffer
	logger := NewStructuredLogger(InfoLevel, &buf)

	// Test that debug messages are filtered out
	logger.Debug("debug message")
	if buf.Len() > 0 {
		t.Errorf("Debug message should be filtered out at Info level")
	}

	// Test that info messages are logged
	logger.Info("info message")
	output := buf.String()
	if !strings.Contains(output, "INFO: info message") {
		t.Errorf("Info message not found in output: %s", output)
	}

	// Reset buffer
	buf.Reset()

	// Test that warn messages are logged
	logger.Warn("warn message")
	output = buf.String()
	if !strings.Contains(output, "WARN: warn message") {
		t.Errorf("Warn message not found in output: %s", output)
	}

	// Reset buffer
	buf.Reset()

	// Test that error messages are logged
	logger.Error("error message")
	output = buf.String()
	if !strings.Contains(output, "ERROR: error message") {
		t.Errorf("Error message not found in output: %s", output)
	}
}

func TestStructuredLogger_WithFields(t *testing.T) {
	var buf bytes.Buffer
	logger := NewStructuredLogger(InfoLevel, &buf)

	logger.Info("test message", 
		StringField("key1", "value1"),
		IntField("key2", 42),
		ErrorField(errors.New("test error")))

	output := buf.String()
	
	expectedParts := []string{
		"INFO: test message",
		"key1=value1",
		"key2=42",
		"error=test error",
	}

	for _, part := range expectedParts {
		if !strings.Contains(output, part) {
			t.Errorf("Expected output to contain '%s', got: %s", part, output)
		}
	}
}

func TestStructuredLogger_SetLevel(t *testing.T) {
	var buf bytes.Buffer
	logger := NewStructuredLogger(InfoLevel, &buf)

	// Initially at Info level, debug should be filtered
	logger.Debug("debug message")
	if buf.Len() > 0 {
		t.Errorf("Debug message should be filtered out")
	}

	// Change to Debug level
	logger.SetLevel(DebugLevel)
	logger.Debug("debug message")
	
	output := buf.String()
	if !strings.Contains(output, "DEBUG: debug message") {
		t.Errorf("Debug message not found after level change: %s", output)
	}

	// Verify GetLevel
	if logger.GetLevel() != DebugLevel {
		t.Errorf("Expected level %v, got %v", DebugLevel, logger.GetLevel())
	}
}

func TestNewFileLogger(t *testing.T) {
	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "test.log")

	logger, err := NewFileLogger(InfoLevel, logFile)
	if err != nil {
		t.Fatalf("Failed to create file logger: %v", err)
	}
	defer logger.Close()

	logger.Info("test message")

	// Read the file content
	content, err := os.ReadFile(logFile)
	if err != nil {
		t.Fatalf("Failed to read log file: %v", err)
	}

	output := string(content)
	if !strings.Contains(output, "INFO: test message") {
		t.Errorf("Expected message not found in log file: %s", output)
	}
}

func TestNewFileLogger_InvalidPath(t *testing.T) {
	// Try to create logger with invalid path
	_, err := NewFileLogger(InfoLevel, "/invalid/path/test.log")
	if err == nil {
		t.Errorf("Expected error for invalid file path")
	}
}

func TestNewConsoleLogger(t *testing.T) {
	logger := NewConsoleLogger(InfoLevel)
	if logger == nil {
		t.Errorf("Console logger should not be nil")
	}
	
	if logger.GetLevel() != InfoLevel {
		t.Errorf("Expected level %v, got %v", InfoLevel, logger.GetLevel())
	}
}

func TestNewMultiLogger(t *testing.T) {
	var buf1, buf2 bytes.Buffer
	logger := NewMultiLogger(InfoLevel, &buf1, &buf2)

	logger.Info("test message")

	// Check that message was written to both buffers
	output1 := buf1.String()
	output2 := buf2.String()

	if !strings.Contains(output1, "INFO: test message") {
		t.Errorf("Message not found in first buffer: %s", output1)
	}
	if !strings.Contains(output2, "INFO: test message") {
		t.Errorf("Message not found in second buffer: %s", output2)
	}
}

func TestHelperFields(t *testing.T) {
	tests := []struct {
		name     string
		field    Field
		expected string
	}{
		{
			name:     "StringField",
			field:    StringField("key", "value"),
			expected: "key=value",
		},
		{
			name:     "IntField",
			field:    IntField("count", 42),
			expected: "count=42",
		},
		{
			name:     "ErrorField",
			field:    ErrorField(errors.New("test error")),
			expected: "error=test error",
		},
		{
			name:     "TransactionField",
			field:    TransactionField("txn123"),
			expected: "transaction_id=txn123",
		},
		{
			name:     "SessionField",
			field:    SessionField("sess456"),
			expected: "session_id=sess456",
		},
		{
			name:     "MethodField",
			field:    MethodField("INVITE"),
			expected: "sip_method=INVITE",
		},
		{
			name:     "AddressField",
			field:    AddressField("remote_addr", "192.168.1.1:5060"),
			expected: "remote_addr=192.168.1.1:5060",
		},
		{
			name:     "CallIDField",
			field:    CallIDField("call123@example.com"),
			expected: "call_id=call123@example.com",
		},
		{
			name:     "UserField",
			field:    UserField("alice@example.com"),
			expected: "user=alice@example.com",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			logger := NewStructuredLogger(InfoLevel, &buf)

			logger.Info("test", tt.field)

			output := buf.String()
			if !strings.Contains(output, tt.expected) {
				t.Errorf("Expected output to contain '%s', got: %s", tt.expected, output)
			}
		})
	}
}

func TestNewLoggerFromConfig(t *testing.T) {
	tests := []struct {
		name        string
		config      LoggerConfig
		expectError bool
	}{
		{
			name: "console logger",
			config: LoggerConfig{
				Level: "info",
				File:  "",
			},
			expectError: false,
		},
		{
			name: "stdout logger",
			config: LoggerConfig{
				Level: "debug",
				File:  "stdout",
			},
			expectError: false,
		},
		{
			name: "file logger",
			config: LoggerConfig{
				Level: "warn",
				File:  filepath.Join(t.TempDir(), "test.log"),
			},
			expectError: false,
		},
		{
			name: "invalid level",
			config: LoggerConfig{
				Level: "invalid",
				File:  "",
			},
			expectError: true,
		},
		{
			name: "invalid file path",
			config: LoggerConfig{
				Level: "info",
				File:  "/invalid/path/test.log",
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger, err := NewLoggerFromConfig(tt.config)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				if logger == nil {
					t.Errorf("Logger should not be nil")
				}

				// Test that logger works
				logger.Info("test message")

				// Close if it's a file logger
				if closer, ok := logger.(*StructuredLogger); ok {
					closer.Close()
				}
			}
		})
	}
}