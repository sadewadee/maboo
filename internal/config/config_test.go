package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestDefault(t *testing.T) {
	cfg := Default()

	if cfg.Server.Address != "0.0.0.0:8080" {
		t.Errorf("expected default address 0.0.0.0:8080, got %s", cfg.Server.Address)
	}
	if cfg.Pool.MinWorkers != 4 {
		t.Errorf("expected min_workers 4, got %d", cfg.Pool.MinWorkers)
	}
	if cfg.Pool.MaxWorkers != 32 {
		t.Errorf("expected max_workers 32, got %d", cfg.Pool.MaxWorkers)
	}
	if cfg.Pool.MaxJobs != 10000 {
		t.Errorf("expected max_jobs 10000, got %d", cfg.Pool.MaxJobs)
	}
	if cfg.Pool.IdleTimeout.Duration() != 60*time.Second {
		t.Errorf("expected idle_timeout 60s, got %s", cfg.Pool.IdleTimeout.Duration())
	}
	if cfg.Logging.Level != "info" {
		t.Errorf("expected log level info, got %s", cfg.Logging.Level)
	}
}

func TestLoadValidConfig(t *testing.T) {
	yaml := `
server:
  address: "0.0.0.0:9090"
php:
  binary: "/usr/bin/php"
  worker: "worker.php"
pool:
  min_workers: 2
  max_workers: 16
  max_jobs: 5000
  max_memory: "256M"
  idle_timeout: "120s"
  allocate_timeout: "15s"
logging:
  level: "debug"
`
	dir := t.TempDir()
	path := filepath.Join(dir, "maboo.yaml")
	if err := os.WriteFile(path, []byte(yaml), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}

	if cfg.Server.Address != "0.0.0.0:9090" {
		t.Errorf("expected address 0.0.0.0:9090, got %s", cfg.Server.Address)
	}
	if cfg.PHP.Binary != "/usr/bin/php" {
		t.Errorf("expected php binary /usr/bin/php, got %s", cfg.PHP.Binary)
	}
	if cfg.PHP.Worker != "worker.php" {
		t.Errorf("expected worker worker.php, got %s", cfg.PHP.Worker)
	}
	if cfg.Pool.MinWorkers != 2 {
		t.Errorf("expected min_workers 2, got %d", cfg.Pool.MinWorkers)
	}
	if cfg.Pool.MaxWorkers != 16 {
		t.Errorf("expected max_workers 16, got %d", cfg.Pool.MaxWorkers)
	}
	if cfg.Pool.IdleTimeout.Duration() != 120*time.Second {
		t.Errorf("expected idle_timeout 120s, got %s", cfg.Pool.IdleTimeout.Duration())
	}
	if cfg.Logging.Level != "debug" {
		t.Errorf("expected log level debug, got %s", cfg.Logging.Level)
	}
}

func TestLoadMissingFile(t *testing.T) {
	_, err := Load("/nonexistent/maboo.yaml")
	if err == nil {
		t.Error("expected error for missing file, got nil")
	}
}

func TestValidateMinWorkersZero(t *testing.T) {
	cfg := Default()
	cfg.PHP.Worker = "worker.php"
	cfg.Pool.MinWorkers = 0
	if err := cfg.Validate(); err == nil {
		t.Error("expected validation error for min_workers=0")
	}
}

func TestValidateMaxLessThanMin(t *testing.T) {
	cfg := Default()
	cfg.PHP.Worker = "worker.php"
	cfg.Pool.MinWorkers = 8
	cfg.Pool.MaxWorkers = 4
	if err := cfg.Validate(); err == nil {
		t.Error("expected validation error for max_workers < min_workers")
	}
}

func TestValidateMissingWorker(t *testing.T) {
	cfg := Default()
	cfg.PHP.Worker = ""
	if err := cfg.Validate(); err == nil {
		t.Error("expected validation error for missing worker")
	}
}

func TestValidateWebSocketWorkerRequired(t *testing.T) {
	cfg := Default()
	cfg.PHP.Worker = "worker.php"
	cfg.WebSocket.Enabled = true
	cfg.WebSocket.Worker = ""
	if err := cfg.Validate(); err == nil {
		t.Error("expected validation error for enabled websocket without worker")
	}
}
