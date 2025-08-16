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
		Realm      string `yaml:"realm"`
		NonceExpiry int   `yaml:"nonce_expiry"`
	} `yaml:"authentication"`
	
	SessionTimer struct {
		DefaultExpires int `yaml:"default_expires"`
		MinSE         int `yaml:"min_se"`
		MaxSE         int `yaml:"max_se"`
	} `yaml:"session_timer"`
	
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