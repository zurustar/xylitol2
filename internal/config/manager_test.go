package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestManager_Load(t *testing.T) {
	manager := NewManager()

	tests := []struct {
		name        string
		configYAML  string
		expectError bool
		errorMsg    string
	}{
		{
			name: "valid configuration",
			configYAML: `
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
			name: "invalid UDP port",
			configYAML: `
server:
  udp_port: 70000
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
			expectError: true,
			errorMsg:    "invalid UDP port",
		},
		{
			name: "empty realm",
			configYAML: `
server:
  udp_port: 5060
  tcp_port: 5060
database:
  path: "./test.db"
authentication:
  realm: ""
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
			expectError: true,
			errorMsg:    "authentication realm cannot be empty",
		},
		{
			name: "invalid session timer configuration",
			configYAML: `
server:
  udp_port: 5060
  tcp_port: 5060
database:
  path: "./test.db"
authentication:
  realm: "test.local"
  nonce_expiry: 300
session_timer:
  default_expires: 60
  min_se: 90
  max_se: 7200
web_admin:
  port: 8080
  enabled: true
logging:
  level: "info"
  file: "./test.log"
`,
			expectError: true,
			errorMsg:    "default session expires",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temporary config file
			tmpDir := t.TempDir()
			configFile := filepath.Join(tmpDir, "config.yaml")
			
			err := os.WriteFile(configFile, []byte(tt.configYAML), 0644)
			if err != nil {
				t.Fatalf("Failed to create test config file: %v", err)
			}

			config, err := manager.Load(configFile)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error but got none")
				} else if tt.errorMsg != "" && !containsString(err.Error(), tt.errorMsg) {
					t.Errorf("Expected error to contain '%s', got: %v", tt.errorMsg, err)
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				if config == nil {
					t.Errorf("Expected config but got nil")
				}
			}
		})
	}
}

func TestManager_LoadNonExistentFile(t *testing.T) {
	manager := NewManager()
	
	_, err := manager.Load("nonexistent.yaml")
	if err == nil {
		t.Errorf("Expected error for non-existent file")
	}
}

func TestManager_LoadInvalidYAML(t *testing.T) {
	manager := NewManager()
	
	// Create temporary file with invalid YAML
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "invalid.yaml")
	
	invalidYAML := `
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
  invalid_yaml: [unclosed
`
	
	err := os.WriteFile(configFile, []byte(invalidYAML), 0644)
	if err != nil {
		t.Fatalf("Failed to create test config file: %v", err)
	}

	_, err = manager.Load(configFile)
	if err == nil {
		t.Errorf("Expected error for invalid YAML")
	}
}

func TestManager_Validate(t *testing.T) {
	manager := NewManager()

	tests := []struct {
		name        string
		config      *Config
		expectError bool
		errorMsg    string
	}{
		{
			name:        "valid config",
			config:      GetDefaultConfig(),
			expectError: false,
		},
		{
			name: "invalid UDP port - too low",
			config: func() *Config {
				c := GetDefaultConfig()
				c.Server.UDPPort = 0
				return c
			}(),
			expectError: true,
			errorMsg:    "invalid UDP port",
		},
		{
			name: "invalid TCP port - too high",
			config: func() *Config {
				c := GetDefaultConfig()
				c.Server.TCPPort = 70000
				return c
			}(),
			expectError: true,
			errorMsg:    "invalid TCP port",
		},
		{
			name: "empty database path",
			config: func() *Config {
				c := GetDefaultConfig()
				c.Database.Path = ""
				return c
			}(),
			expectError: true,
			errorMsg:    "database path cannot be empty",
		},
		{
			name: "short nonce expiry",
			config: func() *Config {
				c := GetDefaultConfig()
				c.Authentication.NonceExpiry = 30
				return c
			}(),
			expectError: true,
			errorMsg:    "nonce expiry too short",
		},
		{
			name: "min SE too short",
			config: func() *Config {
				c := GetDefaultConfig()
				c.SessionTimer.MinSE = 60
				return c
			}(),
			expectError: true,
			errorMsg:    "min SE too short",
		},
		{
			name: "max SE less than default expires",
			config: func() *Config {
				c := GetDefaultConfig()
				c.SessionTimer.MaxSE = 1000
				c.SessionTimer.DefaultExpires = 1800
				return c
			}(),
			expectError: true,
			errorMsg:    "max SE",
		},
		{
			name: "web admin port conflict with UDP",
			config: func() *Config {
				c := GetDefaultConfig()
				c.WebAdmin.Port = c.Server.UDPPort
				return c
			}(),
			expectError: true,
			errorMsg:    "web admin port",
		},
		{
			name: "invalid log level",
			config: func() *Config {
				c := GetDefaultConfig()
				c.Logging.Level = "invalid"
				return c
			}(),
			expectError: true,
			errorMsg:    "invalid log level",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := manager.Validate(tt.config)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error but got none")
				} else if tt.errorMsg != "" && !containsString(err.Error(), tt.errorMsg) {
					t.Errorf("Expected error to contain '%s', got: %v", tt.errorMsg, err)
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
			}
		})
	}
}

func TestGetDefaultConfig(t *testing.T) {
	config := GetDefaultConfig()
	
	if config == nil {
		t.Fatal("GetDefaultConfig returned nil")
	}

	// Validate that default config is valid
	manager := NewManager()
	err := manager.Validate(config)
	if err != nil {
		t.Errorf("Default config is invalid: %v", err)
	}

	// Check some key default values
	if config.Server.UDPPort != 5060 {
		t.Errorf("Expected UDP port 5060, got %d", config.Server.UDPPort)
	}
	if config.Authentication.Realm != "sipserver.local" {
		t.Errorf("Expected realm 'sipserver.local', got %s", config.Authentication.Realm)
	}
	if config.SessionTimer.MinSE != 90 {
		t.Errorf("Expected MinSE 90, got %d", config.SessionTimer.MinSE)
	}
}

// Helper function to check if a string contains a substring
func containsString(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || 
		(len(s) > len(substr) && 
			(s[:len(substr)] == substr || 
			 s[len(s)-len(substr):] == substr || 
			 containsSubstring(s, substr))))
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}