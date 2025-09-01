package config

// Config represents the server configuration
type Config struct {
	Server struct {
		UDPPort int `yaml:"udp_port"`
		TCPPort int `yaml:"tcp_port"`
	} `yaml:"server"`
	
	Database struct {
		Path string `yaml:"path"`
	} `yaml:"database"`
	
	Authentication struct {
		Enabled     bool   `yaml:"enabled"`
		RequireAuth bool   `yaml:"require_auth"`
		Realm       string `yaml:"realm"`
		NonceExpiry int    `yaml:"nonce_expiry"`
	} `yaml:"authentication"`
	
	SessionTimer struct {
		Enabled        bool `yaml:"enabled"`
		RequireSupport bool `yaml:"require_support"`
		DefaultExpires int  `yaml:"default_expires"`
		MinSE          int  `yaml:"min_se"`
		MaxSE          int  `yaml:"max_se"`
	} `yaml:"session_timer"`
	
	HuntGroups struct {
		Enabled         bool `yaml:"enabled"`
		RingTimeout     int  `yaml:"ring_timeout"`     // Timeout in seconds for each member
		MaxConcurrent   int  `yaml:"max_concurrent"`   // Maximum concurrent calls per group
		CallWaitingTime int  `yaml:"call_waiting_time"` // Time to wait before trying next strategy
	} `yaml:"hunt_groups"`
	
	WebAdmin struct {
		Port    int  `yaml:"port"`
		Enabled bool `yaml:"enabled"`
	} `yaml:"web_admin"`
	
	Logging struct {
		Level string `yaml:"level"`
		File  string `yaml:"file"`
	} `yaml:"logging"`
}

// ConfigManager defines the interface for configuration management
type ConfigManager interface {
	Load(filename string) (*Config, error)
	Validate(config *Config) error
}