package worker

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/sadewadee/maboo/internal/config"
	"github.com/sadewadee/maboo/internal/phpengine"
)

// StatsGetter is the interface for pool statistics.
type StatsGetter interface {
	TotalWorkers() int
	BusyWorkers() int
	IdleWorkers() int
	TotalRequests() int64
}

// Pool manages embedded PHP workers.
type Pool struct {
	cfg    *config.Config
	logger *slog.Logger

	workers   []*Worker
	mu        sync.RWMutex
	available chan *Worker
	nextID    atomic.Int32

	ctx    context.Context
	cancel context.CancelFunc

	// Metrics
	totalRequests atomic.Int64
	activeWorkers atomic.Int32
	busyWorkers   atomic.Int32
}

// NewPool creates a new embedded worker pool.
func NewPool(cfg *config.Config) *Pool {
	ctx, cancel := context.WithCancel(context.Background())

	return &Pool{
		cfg:       cfg,
		available: make(chan *Worker, cfg.Pool.MaxWorkers),
		ctx:       ctx,
		cancel:    cancel,
	}
}

// SetLogger sets the pool logger.
func (p *Pool) SetLogger(logger *slog.Logger) {
	p.logger = logger
}

// Mode returns the execution mode (worker/request).
func (p *Pool) Mode() string {
	return p.cfg.PHP.Mode
}

// Start initializes the pool.
func (p *Pool) Start() error {
	if p.logger != nil {
		p.logger.Info("starting embedded worker pool",
			"mode", p.cfg.PHP.Mode,
			"min_workers", p.cfg.Pool.MinWorkers,
			"max_workers", p.cfg.Pool.MaxWorkers,
		)
	}

	for i := 0; i < p.cfg.Pool.MinWorkers; i++ {
		w, err := p.spawnWorker()
		if err != nil {
			return fmt.Errorf("spawning initial worker %d: %w", i, err)
		}
		p.available <- w
	}

	go p.watchdog()
	return nil
}

// Exec executes a request using an available worker.
func (p *Pool) Exec(reqCtx *phpengine.Context, script string) (*phpengine.Response, error) {
	p.totalRequests.Add(1)

	var w *Worker
	select {
	case w = <-p.available:
	case <-time.After(p.cfg.Pool.AllocateTimeout.Duration()):
		return nil, fmt.Errorf("no available worker within %s", p.cfg.Pool.AllocateTimeout.Duration())
	case <-p.ctx.Done():
		return nil, fmt.Errorf("pool shutting down")
	}

	p.busyWorkers.Add(1)
	defer p.busyWorkers.Add(-1)

	resp, err := w.Exec(reqCtx, script)

	if w.NeedsRecycle() {
		go p.replaceWorker(w)
	} else {
		p.available <- w
	}

	return resp, err
}

// Stop gracefully shuts down the pool.
func (p *Pool) Stop() error {
	if p.logger != nil {
		p.logger.Info("stopping embedded worker pool")
	}
	p.cancel()

	p.mu.RLock()
	workers := make([]*Worker, len(p.workers))
	copy(workers, p.workers)
	p.mu.RUnlock()

	var wg sync.WaitGroup
	for _, w := range workers {
		wg.Add(1)
		go func(w *Worker) {
			defer wg.Done()
			w.Stop()
		}(w)
	}
	wg.Wait()

	close(p.available)
	return nil
}

// Stats returns pool statistics.
func (p *Pool) Stats() StatsGetter {
	p.mu.RLock()
	total := len(p.workers)
	p.mu.RUnlock()

	return PoolStats{
		totalWorkers:  total,
		activeWorkers: int(p.activeWorkers.Load()),
		busyWorkers:   int(p.busyWorkers.Load()),
		idleWorkers:   total - int(p.busyWorkers.Load()),
		totalRequests: p.totalRequests.Load(),
	}
}

// PoolStats holds pool metrics.
type PoolStats struct {
	totalWorkers  int   `json:"total_workers"`
	activeWorkers int   `json:"active_workers"`
	busyWorkers   int   `json:"busy_workers"`
	idleWorkers   int   `json:"idle_workers"`
	totalRequests int64 `json:"total_requests"`
}

// TotalWorkers returns the total number of workers.
func (s PoolStats) TotalWorkers() int {
	return s.totalWorkers
}

// BusyWorkers returns the number of busy workers.
func (s PoolStats) BusyWorkers() int {
	return s.busyWorkers
}

// IdleWorkers returns the number of idle workers.
func (s PoolStats) IdleWorkers() int {
	return s.idleWorkers
}

// TotalRequests returns the total number of requests.
func (s PoolStats) TotalRequests() int64 {
	return s.totalRequests
}

func (p *Pool) spawnWorker() (*Worker, error) {
	id := int(p.nextID.Add(1))

	w, err := NewWorker(id, p.cfg)
	if err != nil {
		return nil, err
	}

	// In worker mode, start the PHP engine once
	if p.cfg.PHP.Mode == "worker" {
		if err := w.Start(); err != nil {
			return nil, fmt.Errorf("starting worker %d: %w", id, err)
		}
	}

	p.mu.Lock()
	p.workers = append(p.workers, w)
	p.activeWorkers.Add(1)
	p.mu.Unlock()

	return w, nil
}

func (p *Pool) replaceWorker(old *Worker) {
	old.Stop()
	p.removeWorker(old)

	if p.ctx.Err() != nil {
		return
	}

	w, err := p.spawnWorker()
	if err != nil {
		if p.logger != nil {
			p.logger.Error("failed to spawn replacement worker", "error", err)
		}
		return
	}
	p.available <- w
}

func (p *Pool) removeWorker(w *Worker) {
	p.mu.Lock()
	defer p.mu.Unlock()

	for i, worker := range p.workers {
		if worker.ID() == w.ID() {
			p.workers = append(p.workers[:i], p.workers[i+1:]...)
			p.activeWorkers.Add(-1)
			break
		}
	}
}

func (p *Pool) watchdog() {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			p.autoScale()
		case <-p.ctx.Done():
			return
		}
	}
}

func (p *Pool) autoScale() {
	stats := p.Stats()

	if stats.TotalWorkers() > 0 {
		busyPct := float64(stats.BusyWorkers()) / float64(stats.TotalWorkers()) * 100
		if busyPct >= 80 && stats.TotalWorkers() < p.cfg.Pool.MaxWorkers {
			w, err := p.spawnWorker()
			if err == nil {
				p.available <- w
			}
		}

		if busyPct <= 20 && stats.TotalWorkers() > p.cfg.Pool.MinWorkers {
			select {
			case w := <-p.available:
				go func() {
					w.Stop()
					p.removeWorker(w)
				}()
			default:
			}
		}
	}
}

// Reload gracefully replaces all workers.
func (p *Pool) Reload() error {
	if p.logger != nil {
		p.logger.Info("graceful reload starting")
	}

	p.mu.RLock()
	oldWorkers := make([]*Worker, len(p.workers))
	copy(oldWorkers, p.workers)
	p.mu.RUnlock()

	for i := 0; i < p.cfg.Pool.MinWorkers; i++ {
		w, err := p.spawnWorker()
		if err != nil {
			return fmt.Errorf("reload failed: %w", err)
		}
		p.available <- w
	}

	go func() {
		for _, w := range oldWorkers {
			for w.State() == StateBusy {
				time.Sleep(100 * time.Millisecond)
			}
			w.Stop()
			p.removeWorker(w)
		}
		if p.logger != nil {
			p.logger.Info("graceful reload complete")
		}
	}()

	return nil
}
