package config

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

// Config holds the complete maboo server configuration.
type Config struct {
	Server    ServerConfig    `yaml:"server"`
	PHP       PHPConfig       `yaml:"php"`
	Pool      PoolConfig      `yaml:"pool"`
	App       AppConfig       `yaml:"app"`
	WebSocket WebSocketConfig `yaml:"websocket"`
	Static    StaticConfig    `yaml:"static"`
	Logging   LogConfig       `yaml:"logging"`
	Metrics   MetricsConfig   `yaml:"metrics"`
	Watch     WatchConfig     `yaml:"watch"`
	Workers   []WorkerConfig  `yaml:"workers"`
}

// ServerMode defines the server operation mode
type ServerMode string

const (
	ModeNative ServerMode = "native"
	ModeCaddy  ServerMode = "caddy"
)

type ServerConfig struct {
	Address      string      `yaml:"address"`
	Mode         ServerMode  `yaml:"mode"`
	HTTP2        bool        `yaml:"http2"`
	HTTP3        bool        `yaml:"http3"`
	TLS          TLSConfig   `yaml:"tls"`
	HTTPRedirect bool        `yaml:"http_redirect"`
}

type TLSConfig struct {
	Auto  bool       `yaml:"auto"`
	Cert  string     `yaml:"cert"`
	Key   string     `yaml:"key"`
	ACME  ACMEConfig `yaml:"acme"`
}

type ACMEConfig struct {
	Email    string   `yaml:"email"`
	Domains  []string `yaml:"domains"`
	CacheDir string   `yaml:"cache_dir"`
	Staging  bool     `yaml:"staging"`
}

type PHPConfig struct {
	Version    string            `yaml:"version"`    // auto, 7.4, 8.0, 8.1, 8.2, 8.3, 8.4
	Mode       string            `yaml:"mode"`       // worker, request
	Binary     string            `yaml:"binary"`     // Optional: use system PHP instead of bundled
	Worker     string            `yaml:"worker"`     // Legacy: path to worker script
	INI        map[string]string `yaml:"ini"`        // PHP ini settings
	Extensions ExtensionsConfig  `yaml:"extensions"` // Extension configuration
}

// ExtensionsConfig defines required and optional extensions
type ExtensionsConfig struct {
	Required []string `yaml:"required"` // Extensions that must load (fail if missing)
	Optional []string `yaml:"optional"` // Extensions that should load (skip if missing)
}

type AppConfig struct {
	Root  string            `yaml:"root"`  // Document root
	Entry string            `yaml:"entry"` // auto, or explicit path like "public/index.php"
	Env   map[string]string `yaml:"env"`   // Environment variables
}

type PoolConfig struct {
	MinWorkers      int      `yaml:"min_workers"`
	MaxWorkers      int      `yaml:"max_workers"`
	MaxJobs         int      `yaml:"max_jobs"`
	MaxMemory       string   `yaml:"max_memory"`
	IdleTimeout     Duration `yaml:"idle_timeout"`
	AllocateTimeout Duration `yaml:"allocate_timeout"`
	RequestTimeout  Duration `yaml:"request_timeout"`
}

type WebSocketConfig struct {
	Enabled        bool     `yaml:"enabled"`
	Path           string   `yaml:"path"`
	Worker         string   `yaml:"worker"`
	MaxConnections int      `yaml:"max_connections"`
	PingInterval   Duration `yaml:"ping_interval"`
}

type StaticConfig struct {
	Root         string `yaml:"root"`
	CacheControl string `yaml:"cache_control"`
}

type LogConfig struct {
	Level  string `yaml:"level"`
	Format string `yaml:"format"`
	Output string `yaml:"output"`
}

type MetricsConfig struct {
	Enabled bool   `yaml:"enabled"`
	Path    string `yaml:"path"`
}

type WatchConfig struct {
	Enabled  bool     `yaml:"enabled"`
	Dirs     []string `yaml:"dirs"`
	Interval Duration `yaml:"interval"`
}

type WorkerConfig struct {
	Script  string   `yaml:"script"`
	Pattern string   `yaml:"pattern"`
	Count   int      `yaml:"count"`
	Watch   []string `yaml:"watch"`
}

// Duration is a time.Duration that supports YAML string unmarshaling.
type Duration time.Duration

func (d *Duration) UnmarshalYAML(value *yaml.Node) error {
	var s string
	if err := value.Decode(&s); err != nil {
		return err
	}
	parsed, err := time.ParseDuration(s)
	if err != nil {
		return fmt.Errorf("invalid duration %q: %w", s, err)
	}
	*d = Duration(parsed)
	return nil
}

func (d Duration) MarshalYAML() (interface{}, error) {
	return time.Duration(d).String(), nil
}

func (d Duration) Duration() time.Duration {
	return time.Duration(d)
}

// Load reads config from a YAML file, applying defaults for missing values.
func Load(path string) (*Config, error) {
	cfg := Default()

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading config file: %w", err)
	}

	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parsing config file: %w", err)
	}

	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	return cfg, nil
}

// Validate checks the config for invalid values.
func (c *Config) Validate() error {
	if c.Pool.MinWorkers < 1 {
		return fmt.Errorf("pool.min_workers must be >= 1, got %d", c.Pool.MinWorkers)
	}
	if c.Pool.MaxWorkers < c.Pool.MinWorkers {
		return fmt.Errorf("pool.max_workers (%d) must be >= pool.min_workers (%d)", c.Pool.MaxWorkers, c.Pool.MinWorkers)
	}
	if c.Pool.MaxJobs < 0 {
		return fmt.Errorf("pool.max_jobs must be >= 0, got %d", c.Pool.MaxJobs)
	}

	// Validate PHP mode
	validModes := map[string]bool{"worker": true, "request": true}
	if !validModes[c.PHP.Mode] {
		return fmt.Errorf("php.mode must be 'worker' or 'request', got %q", c.PHP.Mode)
	}

	// Validate PHP version
	validVersions := map[string]bool{
		"auto": true, "7.4": true, "8.0": true,
		"8.1": true, "8.2": true, "8.3": true, "8.4": true,
	}
	if !validVersions[c.PHP.Version] {
		return fmt.Errorf("php.version must be auto or specific version (7.4-8.4), got %q", c.PHP.Version)
	}

	// Legacy: php.worker is only required for external PHP worker mode
	// Embedded PHP mode (default) doesn't need worker script
	if c.PHP.Binary != "" && c.PHP.Worker == "" && len(c.Workers) == 0 {
		return fmt.Errorf("php.worker or workers[] is required when using external PHP binary")
	}

	if c.Server.Address == "" {
		return fmt.Errorf("server.address is required")
	}
	if c.WebSocket.Enabled && c.WebSocket.Worker == "" {
		return fmt.Errorf("websocket.worker is required when websocket is enabled")
	}
	return nil
}
