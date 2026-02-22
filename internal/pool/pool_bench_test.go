package pool

import (
	"testing"
	"time"

	"github.com/maboo-dev/maboo/internal/config"
)

func BenchmarkPoolStats(b *testing.B) {
	cfg := config.PoolConfig{
		MinWorkers:      4,
		MaxWorkers:      32,
		MaxJobs:         10000,
		IdleTimeout:     config.Duration(60 * time.Second),
		AllocateTimeout: config.Duration(30 * time.Second),
		RequestTimeout:  config.Duration(30 * time.Second),
	}
	phpCfg := config.PHPConfig{
		Binary: "php",
		Worker: "worker.php",
	}

	p := New(cfg, phpCfg, nil)
	p.totalRequests.Store(1000000)
	p.busyWorkers.Store(10)
	p.activeWorkers.Store(20)

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		p.Stats()
	}
}

func BenchmarkBuildEnv(b *testing.B) {
	cfg := config.PoolConfig{
		MinWorkers: 4,
		MaxWorkers: 32,
		MaxJobs:    10000,
	}
	phpCfg := config.PHPConfig{
		Binary: "php",
		Worker: "worker.php",
		INI: map[string]string{
			"memory_limit":       "256M",
			"max_execution_time": "30",
			"display_errors":     "Off",
			"opcache.enable":     "1",
		},
	}

	p := New(cfg, phpCfg, nil)

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		p.buildEnv()
	}
}

func BenchmarkNeedsRecycle(b *testing.B) {
	cfg := config.PoolConfig{
		MaxJobs: 10000,
	}
	p := New(cfg, config.PHPConfig{}, nil)

	w := &Worker{}
	w.jobs.Store(5000)

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		p.needsRecycle(w)
	}
}
