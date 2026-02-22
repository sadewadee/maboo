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
	WebSocket WebSocketConfig `yaml:"websocket"`
	Static    StaticConfig    `yaml:"static"`
	Logging   LogConfig       `yaml:"logging"`
	Metrics   MetricsConfig   `yaml:"metrics"`
	Watch     WatchConfig     `yaml:"watch"`
	Workers   []WorkerConfig  `yaml:"workers"`
}

type ServerConfig struct {
	Address string    `yaml:"address"`
	TLS     TLSConfig `yaml:"tls"`
	HTTP3   bool      `yaml:"http3"`
}

type TLSConfig struct {
	Auto bool   `yaml:"auto"`
	Cert string `yaml:"cert"`
	Key  string `yaml:"key"`
}

type PHPConfig struct {
	Binary string            `yaml:"binary"`
	Worker string            `yaml:"worker"`
	INI    map[string]string `yaml:"ini"`
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
	if c.PHP.Worker == "" && len(c.Workers) == 0 {
		return fmt.Errorf("php.worker or workers[] is required")
	}
	if c.Server.Address == "" {
		return fmt.Errorf("server.address is required")
	}
	if c.WebSocket.Enabled && c.WebSocket.Worker == "" {
		return fmt.Errorf("websocket.worker is required when websocket is enabled")
	}
	return nil
}
