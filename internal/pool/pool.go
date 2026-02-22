package pool

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/sadewadee/maboo/internal/config"
	"github.com/sadewadee/maboo/internal/protocol"
)

// Pool manages a pool of PHP worker processes.
type Pool struct {
	cfg    config.PoolConfig
	php    config.PHPConfig
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

// New creates a new worker pool with the given configuration.
func New(poolCfg config.PoolConfig, phpCfg config.PHPConfig, logger *slog.Logger) *Pool {
	ctx, cancel := context.WithCancel(context.Background())

	p := &Pool{
		cfg:       poolCfg,
		php:       phpCfg,
		logger:    logger,
		available: make(chan *Worker, poolCfg.MaxWorkers),
		ctx:       ctx,
		cancel:    cancel,
	}

	return p
}

// Start initializes the pool by spawning the minimum number of workers.
func (p *Pool) Start() error {
	p.logger.Info("starting worker pool",
		"min_workers", p.cfg.MinWorkers,
		"max_workers", p.cfg.MaxWorkers,
		"max_jobs", p.cfg.MaxJobs,
		"max_memory", p.cfg.MaxMemory,
	)

	for i := 0; i < p.cfg.MinWorkers; i++ {
		w, err := p.spawnWorker()
		if err != nil {
			return fmt.Errorf("spawning initial worker %d: %w", i, err)
		}
		p.available <- w
	}

	// Start watchdog goroutine
	go p.watchdog()

	return nil
}

// Exec dispatches a request to an available worker and returns the response.
func (p *Pool) Exec(req *protocol.Frame) (*protocol.Frame, error) {
	p.totalRequests.Add(1)

	// Get an available worker with timeout
	var w *Worker
	select {
	case w = <-p.available:
	case <-time.After(p.cfg.AllocateTimeout.Duration()):
		return nil, fmt.Errorf("no available worker within %s (pool exhausted)", p.cfg.AllocateTimeout.Duration())
	case <-p.ctx.Done():
		return nil, fmt.Errorf("pool shutting down")
	}

	p.busyWorkers.Add(1)
	defer p.busyWorkers.Add(-1)

	// Execute request with timeout
	type execResult struct {
		frame *protocol.Frame
		err   error
	}
	done := make(chan execResult, 1)
	go func() {
		f, e := w.Exec(req)
		done <- execResult{f, e}
	}()

	var resp *protocol.Frame
	var err error
	if p.cfg.RequestTimeout.Duration() > 0 {
		select {
		case result := <-done:
			resp, err = result.frame, result.err
		case <-time.After(p.cfg.RequestTimeout.Duration()):
			p.logger.Error("worker request timeout", "worker_id", w.ID(), "timeout", p.cfg.RequestTimeout.Duration())
			go p.replaceWorker(w)
			return nil, fmt.Errorf("request timeout after %s", p.cfg.RequestTimeout.Duration())
		case <-p.ctx.Done():
			return nil, fmt.Errorf("pool shutting down")
		}
	} else {
		result := <-done
		resp, err = result.frame, result.err
	}

	if err != nil {
		p.logger.Error("worker exec failed", "worker_id", w.ID(), "error", err)
		go p.replaceWorker(w)
		return nil, fmt.Errorf("worker %d exec failed: %w", w.ID(), err)
	}

	// Check if worker needs recycling
	if p.needsRecycle(w) {
		go p.replaceWorker(w)
	} else {
		// Wait for WORKER_READY before returning to pool
		ready, err := w.ReadFrame()
		if err != nil || ready.Type != protocol.TypeWorkerReady {
			go p.replaceWorker(w)
		} else {
			p.available <- w
		}
	}

	return resp, nil
}

// Stop gracefully shuts down all workers in the pool.
func (p *Pool) Stop() error {
	p.logger.Info("stopping worker pool")
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
			if err := w.Stop(); err != nil {
				p.logger.Warn("error stopping worker", "worker_id", w.ID(), "error", err)
			}
		}(w)
	}
	wg.Wait()

	close(p.available)
	p.logger.Info("worker pool stopped")
	return nil
}

// Stats returns current pool statistics.
func (p *Pool) Stats() PoolStats {
	p.mu.RLock()
	total := len(p.workers)
	p.mu.RUnlock()

	return PoolStats{
		TotalWorkers:  total,
		ActiveWorkers: int(p.activeWorkers.Load()),
		BusyWorkers:   int(p.busyWorkers.Load()),
		IdleWorkers:   total - int(p.busyWorkers.Load()),
		TotalRequests: p.totalRequests.Load(),
		QueueDepth:    len(p.available),
	}
}

// PoolStats holds pool metrics.
type PoolStats struct {
	TotalWorkers  int   `json:"total_workers"`
	ActiveWorkers int   `json:"active_workers"`
	BusyWorkers   int   `json:"busy_workers"`
	IdleWorkers   int   `json:"idle_workers"`
	TotalRequests int64 `json:"total_requests"`
	QueueDepth    int   `json:"queue_depth"`
}

func (p *Pool) spawnWorker() (*Worker, error) {
	id := int(p.nextID.Add(1))

	env := p.buildEnv()
	w, err := NewWorker(id, p.php.Binary, p.php.Worker, env)
	if err != nil {
		return nil, err
	}

	p.mu.Lock()
	p.workers = append(p.workers, w)
	p.activeWorkers.Add(1)
	p.mu.Unlock()

	p.logger.Debug("worker spawned", "worker_id", id)
	return w, nil
}

func (p *Pool) replaceWorker(old *Worker) {
	p.logger.Debug("replacing worker", "worker_id", old.ID(), "jobs", old.Jobs())

	if err := old.Stop(); err != nil {
		p.logger.Warn("error stopping old worker", "worker_id", old.ID(), "error", err)
	}

	p.removeWorker(old)

	// Only spawn replacement if pool is still running
	if p.ctx.Err() != nil {
		return
	}

	w, err := p.spawnWorker()
	if err != nil {
		p.logger.Error("failed to spawn replacement worker", "error", err)
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

func (p *Pool) needsRecycle(w *Worker) bool {
	if p.cfg.MaxJobs > 0 && w.Jobs() >= int64(p.cfg.MaxJobs) {
		return true
	}
	// Memory check is done on the PHP side - worker exits on its own
	return false
}

func (p *Pool) buildEnv() []string {
	env := []string{}
	if p.cfg.MaxJobs > 0 {
		env = append(env, fmt.Sprintf("MAX_REQUESTS=%d", p.cfg.MaxJobs))
	}

	// Add PHP INI settings as env vars
	for k, v := range p.php.INI {
		env = append(env, fmt.Sprintf("PHP_INI_%s=%s", k, v))
	}

	return env
}

// watchdog monitors worker health and pool scaling.
func (p *Pool) watchdog() {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			p.checkHealth()
			p.autoScale()
		case <-p.ctx.Done():
			return
		}
	}
}

func (p *Pool) checkHealth() {
	p.mu.RLock()
	workers := make([]*Worker, len(p.workers))
	copy(workers, p.workers)
	p.mu.RUnlock()

	for _, w := range workers {
		if w.State() == StateBusy {
			continue
		}
		if !w.IsAlive() {
			p.logger.Warn("dead worker detected", "worker_id", w.ID())
			go p.replaceWorker(w)
		}
	}
}

func (p *Pool) autoScale() {
	stats := p.Stats()

	// Scale up if busy percentage exceeds threshold (80%)
	if stats.TotalWorkers > 0 {
		busyPct := float64(stats.BusyWorkers) / float64(stats.TotalWorkers) * 100
		if busyPct >= 80 && stats.TotalWorkers < p.cfg.MaxWorkers {
			p.logger.Info("scaling up workers", "busy_pct", busyPct, "current", stats.TotalWorkers)
			w, err := p.spawnWorker()
			if err != nil {
				p.logger.Error("scale-up failed", "error", err)
				return
			}
			p.available <- w
		}

		// Scale down if idle workers exceed threshold and above minimum
		if busyPct <= 20 && stats.TotalWorkers > p.cfg.MinWorkers {
			// Find and stop an idle worker
			select {
			case w := <-p.available:
				p.logger.Info("scaling down workers", "busy_pct", busyPct, "current", stats.TotalWorkers)
				go func() {
					w.Stop()
					p.removeWorker(w)
				}()
			default:
				// No idle workers available to remove
			}
		}
	}
}

// Reload gracefully replaces all workers (zero-downtime restart).
func (p *Pool) Reload() error {
	p.logger.Info("graceful reload starting")

	p.mu.RLock()
	oldWorkers := make([]*Worker, len(p.workers))
	copy(oldWorkers, p.workers)
	p.mu.RUnlock()

	// Spawn new workers first (ensures zero-downtime)
	newWorkers := make([]*Worker, 0, p.cfg.MinWorkers)
	for i := 0; i < p.cfg.MinWorkers; i++ {
		w, err := p.spawnWorker()
		if err != nil {
			p.logger.Error("reload: failed to spawn new worker", "error", err)
			for _, nw := range newWorkers {
				nw.Stop()
			}
			return fmt.Errorf("reload failed: %w", err)
		}
		newWorkers = append(newWorkers, w)
		p.available <- w
	}

	p.logger.Info("reload: new workers spawned", "count", len(newWorkers))

	// Drain and stop old workers in background
	go func() {
		for _, w := range oldWorkers {
			for w.State() == StateBusy {
				time.Sleep(100 * time.Millisecond)
			}
			if err := w.Stop(); err != nil {
				p.logger.Warn("reload: error stopping old worker", "worker_id", w.ID(), "error", err)
			}
			p.removeWorker(w)
		}
		p.logger.Info("graceful reload complete", "old_stopped", len(oldWorkers), "new_active", len(newWorkers))
	}()

	return nil
}
