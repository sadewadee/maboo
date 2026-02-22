package pool

import (
	"testing"
	"time"

	"github.com/maboo-dev/maboo/internal/config"
)

func TestPoolStatsDefaults(t *testing.T) {
	stats := PoolStats{}
	if stats.TotalWorkers != 0 {
		t.Errorf("expected 0 total workers, got %d", stats.TotalWorkers)
	}
	if stats.BusyWorkers != 0 {
		t.Errorf("expected 0 busy workers, got %d", stats.BusyWorkers)
	}
}

func TestPoolNeedsRecycle(t *testing.T) {
	cfg := config.PoolConfig{
		MinWorkers:      1,
		MaxWorkers:      4,
		MaxJobs:         100,
		MaxMemory:       "128M",
		IdleTimeout:     config.Duration(60 * time.Second),
		AllocateTimeout: config.Duration(30 * time.Second),
	}

	phpCfg := config.PHPConfig{
		Binary: "php",
		Worker: "test.php",
	}

	p := New(cfg, phpCfg, nil)

	// Worker with 99 jobs should not need recycling
	w := &Worker{}
	w.jobs.Store(99)
	if p.needsRecycle(w) {
		t.Error("worker with 99 jobs should not need recycling (max=100)")
	}

	// Worker with 100 jobs should need recycling
	w.jobs.Store(100)
	if !p.needsRecycle(w) {
		t.Error("worker with 100 jobs should need recycling (max=100)")
	}
}

func TestPoolBuildEnv(t *testing.T) {
	cfg := config.PoolConfig{
		MaxJobs: 5000,
	}
	phpCfg := config.PHPConfig{
		INI: map[string]string{
			"memory_limit": "256M",
		},
	}

	p := New(cfg, phpCfg, nil)
	env := p.buildEnv()

	found := false
	for _, e := range env {
		if e == "MAX_REQUESTS=5000" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected MAX_REQUESTS=5000 in env, got %v", env)
	}
}
