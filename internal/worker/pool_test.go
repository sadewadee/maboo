package worker_test

import (
	"testing"

	"github.com/sadewadee/maboo/internal/config"
	"github.com/sadewadee/maboo/internal/worker"
)

func TestNewPool(t *testing.T) {
	cfg := config.Default()
	pool := worker.NewPool(cfg)

	if pool == nil {
		t.Error("expected pool to be created")
	}
}

func TestPoolMode(t *testing.T) {
	cfg := config.Default()
	cfg.PHP.Mode = "worker"

	pool := worker.NewPool(cfg)
	if pool.Mode() != "worker" {
		t.Errorf("expected worker mode, got %s", pool.Mode())
	}
}

func TestPoolModeRequest(t *testing.T) {
	cfg := config.Default()
	cfg.PHP.Mode = "request"

	pool := worker.NewPool(cfg)
	if pool.Mode() != "request" {
		t.Errorf("expected request mode, got %s", pool.Mode())
	}
}

func TestPoolStats(t *testing.T) {
	cfg := config.Default()
	pool := worker.NewPool(cfg)

	stats := pool.Stats()
	if stats.TotalWorkers() != 0 {
		t.Errorf("expected 0 total workers before start, got %d", stats.TotalWorkers())
	}
}
