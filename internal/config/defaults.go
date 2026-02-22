package config

import "time"

// Default returns a Config with sensible defaults.
func Default() *Config {
	return &Config{
		Server: ServerConfig{
			Address: "0.0.0.0:8080",
			TLS:     TLSConfig{Auto: false},
			HTTP3:   false,
		},
		PHP: PHPConfig{
			Binary: "php",
			Worker: "",
			INI: map[string]string{
				"memory_limit":       "256M",
				"max_execution_time": "30",
			},
		},
		Pool: PoolConfig{
			MinWorkers:      4,
			MaxWorkers:      32,
			MaxJobs:         10000,
			MaxMemory:       "128M",
			IdleTimeout:     Duration(60 * time.Second),
			AllocateTimeout: Duration(30 * time.Second),
			RequestTimeout:  Duration(30 * time.Second),
		},
		WebSocket: WebSocketConfig{
			Enabled:        false,
			Path:           "/ws",
			Worker:         "",
			MaxConnections: 10000,
			PingInterval:   Duration(30 * time.Second),
		},
		Static: StaticConfig{
			Root:         "public",
			CacheControl: "public, max-age=3600",
		},
		Logging: LogConfig{
			Level:  "info",
			Format: "json",
			Output: "stdout",
		},
		Metrics: MetricsConfig{
			Enabled: true,
			Path:    "/metrics",
		},
		Watch: WatchConfig{
			Enabled:  false,
			Dirs:     []string{},
			Interval: Duration(2 * time.Second),
		},
	}
}
