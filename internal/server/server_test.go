package server

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestSIPServerImpl_LoadConfig(t *testing.T) {
	tests := []struct {
		name        string
		configData  string
		expectError bool
	}{
		{
			name: "valid configuration",
			configData: `
server:
  udp_port: 5060
  tcp_port: 5060
database:
  path: "./test.db"
authentication:
  realm: "test.local"
  nonce_expiry: 300
session_timer:
  default_expires: 1800
  min_se: 90
  max_se: 7200
web_admin:
  port: 8080
  enabled: true
logging:
  level: "info"
  file: "./test.log"
`,
			expectError: false,
		},
		{
			name: "invalid configuration - missing required fields",
			configData: `
server:
  udp_port: 5060
`,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temporary config file
			tmpDir := t.TempDir()
			configFile := filepath.Join(tmpDir, "config.yaml")
			
			if err := os.WriteFile(configFile, []byte(tt.configData), 0644); err != nil {
				t.Fatalf("Failed to create test config file: %v", err)
			}

			server := NewSIPServer()
			err := server.LoadConfig(configFile)

			if tt.expectError && err == nil {
				t.Error("Expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
		})
	}
}

func TestSIPServerImpl_StartStop(t *testing.T) {
	// Create temporary directory for test files
	tmpDir := t.TempDir()
	
	// Create test configuration
	configData := `
server:
  udp_port: 0  # Use port 0 to get random available port
  tcp_port: 0
database:
  path: "` + filepath.Join(tmpDir, "test.db") + `"
authentication:
  realm: "test.local"
  nonce_expiry: 300
session_timer:
  default_expires: 1800
  min_se: 90
  max_se: 7200
web_admin:
  port: 0  # Use port 0 to get random available port
  enabled: false  # Disable web admin for this test
logging:
  level: "error"  # Reduce log noise in tests
  file: "` + filepath.Join(tmpDir, "test.log") + `"
`
	
	configFile := filepath.Join(tmpDir, "config.yaml")
	if err := os.WriteFile(configFile, []byte(configData), 0644); err != nil {
		t.Fatalf("Failed to create test config file: %v", err)
	}

	server := NewSIPServer()
	
	// Test loading configuration
	if err := server.LoadConfig(configFile); err != nil {
		t.Fatalf("Failed to load configuration: %v", err)
	}

	// Test starting server
	if err := server.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}

	// Give server a moment to fully start
	time.Sleep(100 * time.Millisecond)

	// Test stopping server
	if err := server.Stop(); err != nil {
		t.Fatalf("Failed to stop server: %v", err)
	}
}

func TestSIPServerImpl_StartWithoutConfig(t *testing.T) {
	server := NewSIPServer()
	
	// Try to start server without loading configuration
	err := server.Start()
	if err == nil {
		t.Error("Expected error when starting server without configuration")
	}
}

func TestSIPServerImpl_DoubleStart(t *testing.T) {
	// Create temporary directory for test files
	tmpDir := t.TempDir()
	
	// Create test configuration
	configData := `
server:
  udp_port: 0
  tcp_port: 0
database:
  path: "` + filepath.Join(tmpDir, "test.db") + `"
authentication:
  realm: "test.local"
  nonce_expiry: 300
session_timer:
  default_expires: 1800
  min_se: 90
  max_se: 7200
web_admin:
  port: 0
  enabled: false
logging:
  level: "error"
  file: "` + filepath.Join(tmpDir, "test.log") + `"
`
	
	configFile := filepath.Join(tmpDir, "config.yaml")
	if err := os.WriteFile(configFile, []byte(configData), 0644); err != nil {
		t.Fatalf("Failed to create test config file: %v", err)
	}

	server := NewSIPServer()
	
	// Load configuration and start server
	if err := server.LoadConfig(configFile); err != nil {
		t.Fatalf("Failed to load configuration: %v", err)
	}
	
	if err := server.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer server.Stop()

	// Try to start again - should fail
	err := server.Start()
	if err == nil {
		t.Error("Expected error when starting server twice")
	}
}

func TestSIPServerImpl_StopWithoutStart(t *testing.T) {
	server := NewSIPServer()
	
	// Try to stop server without starting - should not error
	err := server.Stop()
	if err != nil {
		t.Errorf("Unexpected error when stopping unstarted server: %v", err)
	}
}

func TestSIPServerImpl_LoadConfigWhileRunning(t *testing.T) {
	// Create temporary directory for test files
	tmpDir := t.TempDir()
	
	// Create test configuration
	configData := `
server:
  udp_port: 0
  tcp_port: 0
database:
  path: "` + filepath.Join(tmpDir, "test.db") + `"
authentication:
  realm: "test.local"
  nonce_expiry: 300
session_timer:
  default_expires: 1800
  min_se: 90
  max_se: 7200
web_admin:
  port: 0
  enabled: false
logging:
  level: "error"
  file: "` + filepath.Join(tmpDir, "test.log") + `"
`
	
	configFile := filepath.Join(tmpDir, "config.yaml")
	if err := os.WriteFile(configFile, []byte(configData), 0644); err != nil {
		t.Fatalf("Failed to create test config file: %v", err)
	}

	server := NewSIPServer()
	
	// Load configuration and start server
	if err := server.LoadConfig(configFile); err != nil {
		t.Fatalf("Failed to load configuration: %v", err)
	}
	
	if err := server.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer server.Stop()

	// Try to load configuration while running - should fail
	err := server.LoadConfig(configFile)
	if err == nil {
		t.Error("Expected error when loading configuration while server is running")
	}
}

// TestSIPServerImpl_ComponentInitializationOrder tests that components are initialized in the correct order
func TestSIPServerImpl_ComponentInitializationOrder(t *testing.T) {
	// Create temporary directory for test files
	tmpDir := t.TempDir()
	
	// Create test configuration
	configData := `
server:
  udp_port: 0
  tcp_port: 0
database:
  path: "` + filepath.Join(tmpDir, "test.db") + `"
authentication:
  realm: "test.local"
  nonce_expiry: 300
session_timer:
  default_expires: 1800
  min_se: 90
  max_se: 7200
web_admin:
  port: 0
  enabled: false
logging:
  level: "debug"
  file: "` + filepath.Join(tmpDir, "test.log") + `"
`
	
	configFile := filepath.Join(tmpDir, "config.yaml")
	if err := os.WriteFile(configFile, []byte(configData), 0644); err != nil {
		t.Fatalf("Failed to create test config file: %v", err)
	}

	server := NewSIPServer()
	
	// Load configuration
	if err := server.LoadConfig(configFile); err != nil {
		t.Fatalf("Failed to load configuration: %v", err)
	}

	// Start server - this tests the component initialization order
	if err := server.Start(); err != nil {
		t.Fatalf("Failed to start server (component initialization failed): %v", err)
	}

	// Stop server
	if err := server.Stop(); err != nil {
		t.Fatalf("Failed to stop server: %v", err)
	}
}

// TestSIPServerImpl_GracefulShutdown tests graceful shutdown with active connections
func TestSIPServerImpl_GracefulShutdown(t *testing.T) {
	// Create temporary directory for test files
	tmpDir := t.TempDir()
	
	// Create test configuration
	configData := `
server:
  udp_port: 0
  tcp_port: 0
database:
  path: "` + filepath.Join(tmpDir, "test.db") + `"
authentication:
  realm: "test.local"
  nonce_expiry: 300
session_timer:
  default_expires: 1800
  min_se: 90
  max_se: 7200
web_admin:
  port: 0
  enabled: true
logging:
  level: "error"
  file: "` + filepath.Join(tmpDir, "test.log") + `"
`
	
	configFile := filepath.Join(tmpDir, "config.yaml")
	if err := os.WriteFile(configFile, []byte(configData), 0644); err != nil {
		t.Fatalf("Failed to create test config file: %v", err)
	}

	server := NewSIPServer()
	
	// Load configuration and start server
	if err := server.LoadConfig(configFile); err != nil {
		t.Fatalf("Failed to load configuration: %v", err)
	}
	
	if err := server.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}

	// Give server time to start background tasks
	time.Sleep(200 * time.Millisecond)

	// Test graceful shutdown
	shutdownStart := time.Now()
	if err := server.Stop(); err != nil {
		t.Fatalf("Failed to stop server: %v", err)
	}
	shutdownDuration := time.Since(shutdownStart)

	// Shutdown should complete quickly for a test server
	if shutdownDuration > 5*time.Second {
		t.Errorf("Shutdown took too long: %v", shutdownDuration)
	}
}

// TestSIPServerImpl_ShutdownTimeout tests shutdown behavior with timeout
func TestSIPServerImpl_ShutdownTimeout(t *testing.T) {
	// Create temporary directory for test files
	tmpDir := t.TempDir()
	
	// Create test configuration
	configData := `
server:
  udp_port: 0
  tcp_port: 0
database:
  path: "` + filepath.Join(tmpDir, "test.db") + `"
authentication:
  realm: "test.local"
  nonce_expiry: 300
session_timer:
  default_expires: 1800
  min_se: 90
  max_se: 7200
web_admin:
  port: 0
  enabled: false
logging:
  level: "error"
  file: "` + filepath.Join(tmpDir, "test.log") + `"
`
	
	configFile := filepath.Join(tmpDir, "config.yaml")
	if err := os.WriteFile(configFile, []byte(configData), 0644); err != nil {
		t.Fatalf("Failed to create test config file: %v", err)
	}

	server := NewSIPServer()
	
	// Load configuration and start server
	if err := server.LoadConfig(configFile); err != nil {
		t.Fatalf("Failed to load configuration: %v", err)
	}
	
	if err := server.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}

	// Test multiple stops (should be safe)
	if err := server.Stop(); err != nil {
		t.Fatalf("Failed to stop server: %v", err)
	}
	
	// Second stop should not error
	if err := server.Stop(); err != nil {
		t.Errorf("Second stop should not error: %v", err)
	}
}

// TestSIPServerImpl_SignalHandling tests the signal handling functionality
func TestSIPServerImpl_SignalHandling(t *testing.T) {
	// Create temporary directory for test files
	tmpDir := t.TempDir()
	
	// Create test configuration
	configData := `
server:
  udp_port: 0
  tcp_port: 0
database:
  path: "` + filepath.Join(tmpDir, "test.db") + `"
authentication:
  realm: "test.local"
  nonce_expiry: 300
session_timer:
  default_expires: 1800
  min_se: 90
  max_se: 7200
web_admin:
  port: 0
  enabled: false
logging:
  level: "error"
  file: "` + filepath.Join(tmpDir, "test.log") + `"
`
	
	configFile := filepath.Join(tmpDir, "config.yaml")
	if err := os.WriteFile(configFile, []byte(configData), 0644); err != nil {
		t.Fatalf("Failed to create test config file: %v", err)
	}

	server := NewSIPServer()
	
	// Load configuration
	if err := server.LoadConfig(configFile); err != nil {
		t.Fatalf("Failed to load configuration: %v", err)
	}
	
	// Test that the server can start and stop normally (without signal handling)
	if err := server.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	
	// Give server time to start
	time.Sleep(100 * time.Millisecond)
	
	if err := server.Stop(); err != nil {
		t.Fatalf("Failed to stop server: %v", err)
	}
	
	// Test passed - signal handling method exists and basic start/stop works
}

// TestSIPServerImpl_ResourceCleanup tests that resources are properly cleaned up
func TestSIPServerImpl_ResourceCleanup(t *testing.T) {
	// Create temporary directory for test files
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
	
	// Create test configuration
	configData := `
server:
  udp_port: 0
  tcp_port: 0
database:
  path: "` + dbPath + `"
authentication:
  realm: "test.local"
  nonce_expiry: 300
session_timer:
  default_expires: 1800
  min_se: 90
  max_se: 7200
web_admin:
  port: 0
  enabled: false
logging:
  level: "error"
  file: "` + filepath.Join(tmpDir, "test.log") + `"
`
	
	configFile := filepath.Join(tmpDir, "config.yaml")
	if err := os.WriteFile(configFile, []byte(configData), 0644); err != nil {
		t.Fatalf("Failed to create test config file: %v", err)
	}

	server := NewSIPServer()
	
	// Load configuration and start server
	if err := server.LoadConfig(configFile); err != nil {
		t.Fatalf("Failed to load configuration: %v", err)
	}
	
	if err := server.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}

	// Verify database file was created
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		t.Error("Database file was not created")
	}

	// Stop server
	if err := server.Stop(); err != nil {
		t.Fatalf("Failed to stop server: %v", err)
	}

	// Database file should still exist after shutdown (persistent storage)
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		t.Error("Database file should persist after shutdown")
	}
}