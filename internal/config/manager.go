package config

import (
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// Manager implements the ConfigManager interface
type Manager struct{}

// NewManager creates a new configuration manager
func NewManager() *Manager {
	return &Manager{}
}

// Load reads and parses the configuration file
func (m *Manager) Load(filename string) (*Config, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file %s: %w", filename, err)
	}

	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config file %s: %w", filename, err)
	}

	if err := m.Validate(&config); err != nil {
		return nil, fmt.Errorf("config validation failed: %w", err)
	}

	return &config, nil
}

// Validate checks if the configuration values are valid
func (m *Manager) Validate(config *Config) error {
	// Validate server ports (0 is allowed for testing - means "use any available port")
	if config.Server.UDPPort < 0 || config.Server.UDPPort > 65535 {
		return fmt.Errorf("invalid UDP port: %d (must be 0-65535)", config.Server.UDPPort)
	}
	if config.Server.TCPPort < 0 || config.Server.TCPPort > 65535 {
		return fmt.Errorf("invalid TCP port: %d (must be 0-65535)", config.Server.TCPPort)
	}

	// Validate database path
	if strings.TrimSpace(config.Database.Path) == "" {
		return fmt.Errorf("database path cannot be empty")
	}

	// Validate authentication settings
	if strings.TrimSpace(config.Authentication.Realm) == "" {
		return fmt.Errorf("authentication realm cannot be empty")
	}
	if config.Authentication.NonceExpiry < 60 {
		return fmt.Errorf("nonce expiry too short: %d seconds (minimum 60)", config.Authentication.NonceExpiry)
	}

	// Validate session timer settings
	if config.SessionTimer.DefaultExpires < config.SessionTimer.MinSE {
		return fmt.Errorf("default session expires (%d) cannot be less than min SE (%d)", 
			config.SessionTimer.DefaultExpires, config.SessionTimer.MinSE)
	}
	if config.SessionTimer.MaxSE < config.SessionTimer.DefaultExpires {
		return fmt.Errorf("max SE (%d) cannot be less than default expires (%d)", 
			config.SessionTimer.MaxSE, config.SessionTimer.DefaultExpires)
	}
	if config.SessionTimer.MinSE < 90 {
		return fmt.Errorf("min SE too short: %d seconds (RFC4028 minimum is 90)", config.SessionTimer.MinSE)
	}

	// Validate web admin port (0 is allowed for testing)
	if config.WebAdmin.Enabled {
		if config.WebAdmin.Port < 0 || config.WebAdmin.Port > 65535 {
			return fmt.Errorf("invalid web admin port: %d (must be 0-65535)", config.WebAdmin.Port)
		}
		// Check for port conflicts (skip if any port is 0 - testing mode)
		if config.WebAdmin.Port > 0 && config.Server.UDPPort > 0 && config.Server.TCPPort > 0 {
			if config.WebAdmin.Port == config.Server.UDPPort || config.WebAdmin.Port == config.Server.TCPPort {
				return fmt.Errorf("web admin port %d conflicts with SIP server ports", config.WebAdmin.Port)
			}
		}
	}

	// Validate logging settings
	validLogLevels := map[string]bool{
		"debug": true,
		"info":  true,
		"warn":  true,
		"error": true,
	}
	logLevel := strings.ToLower(config.Logging.Level)
	if !validLogLevels[logLevel] {
		return fmt.Errorf("invalid log level: %s (must be debug, info, warn, or error)", config.Logging.Level)
	}

	return nil
}

// GetDefaultConfig returns a configuration with default values
func GetDefaultConfig() *Config {
	return &Config{
		Server: struct {
			UDPPort int `yaml:"udp_port"`
			TCPPort int `yaml:"tcp_port"`
		}{
			UDPPort: 5060,
			TCPPort: 5060,
		},
		Database: struct {
			Path string `yaml:"path"`
		}{
			Path: "./sipserver.db",
		},
		Authentication: struct {
			Realm      string `yaml:"realm"`
			NonceExpiry int   `yaml:"nonce_expiry"`
		}{
			Realm:      "sipserver.local",
			NonceExpiry: 300,
		},
		SessionTimer: struct {
			DefaultExpires int `yaml:"default_expires"`
			MinSE         int `yaml:"min_se"`
			MaxSE         int `yaml:"max_se"`
		}{
			DefaultExpires: 1800,
			MinSE:         90,
			MaxSE:         7200,
		},
		WebAdmin: struct {
			Port    int  `yaml:"port"`
			Enabled bool `yaml:"enabled"`
		}{
			Port:    8080,
			Enabled: true,
		},
		Logging: struct {
			Level string `yaml:"level"`
			File  string `yaml:"file"`
		}{
			Level: "info",
			File:  "./sipserver.log",
		},
	}
}