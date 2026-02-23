package worker

import (
	"fmt"
	"runtime"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/sadewadee/maboo/internal/config"
	"github.com/sadewadee/maboo/internal/phpengine"
)

// WorkerState represents the current state of a worker.
type WorkerState int

const (
	StateIdle WorkerState = iota
	StateBusy
	StateStopped
)

// Worker represents an embedded PHP worker.
type Worker struct {
	id         int
	engine     *phpengine.Engine
	extensions *phpengine.ExtensionManager
	state      atomic.Int32
	jobs       atomic.Int64
	maxJobs    int
	maxMemory  int64

	// Memory tracking
	memStart  int64
	memLimit  int64

	// Lifecycle
	startedAt time.Time
	lastJobAt time.Time

	mu sync.RWMutex
}

// NewWorker creates a new embedded PHP worker.
func NewWorker(id int, cfg *config.Config) (*Worker, error) {
	// Determine PHP version
	version := phpengine.SelectVersion(cfg.App.Root, cfg.PHP.Version)

	engine, err := phpengine.NewEngine(version)
	if err != nil {
		return nil, fmt.Errorf("creating PHP engine: %w", err)
	}

	// Create extension manager if extensions are configured
	var extManager *phpengine.ExtensionManager
	if len(cfg.PHP.Extensions.Required) > 0 || len(cfg.PHP.Extensions.Optional) > 0 {
		extManager = phpengine.NewExtensionManager(version, &phpengine.ExtensionConfig{
			Required: cfg.PHP.Extensions.Required,
			Optional: cfg.PHP.Extensions.Optional,
		})
		engine.SetExtensions(extManager)
	}

	// Parse max_memory config
	var maxMemory int64
	if cfg.Pool.MaxMemory != "" {
		maxMemory = parseMemoryString(cfg.Pool.MaxMemory)
	}

	return &Worker{
		id:         id,
		engine:     engine,
		extensions: extManager,
		maxJobs:    cfg.Pool.MaxJobs,
		maxMemory:  maxMemory,
	}, nil
}

// parseMemoryString parses memory strings like "128M", "1G", etc.
func parseMemoryString(s string) int64 {
	if s == "" {
		return 0
	}

	multiplier := int64(1)
	switch s[len(s)-1] {
	case 'K', 'k':
		multiplier = 1024
		s = s[:len(s)-1]
	case 'M', 'm':
		multiplier = 1024 * 1024
		s = s[:len(s)-1]
	case 'G', 'g':
		multiplier = 1024 * 1024 * 1024
		s = s[:len(s)-1]
	}

	val, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0
	}
	return val * multiplier
}

// ID returns the worker ID.
func (w *Worker) ID() int {
	return w.id
}

// State returns the current worker state.
func (w *Worker) State() WorkerState {
	return WorkerState(w.state.Load())
}

// Jobs returns the number of requests handled.
func (w *Worker) Jobs() int64 {
	return w.jobs.Load()
}

// Start initializes the worker (worker mode only).
func (w *Worker) Start() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	w.state.Store(int32(StateIdle))
	w.startedAt = time.Now()

	// Record baseline memory
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	w.memStart = int64(m.Alloc)

	return w.engine.Startup()
}

// Stop shuts down the worker.
func (w *Worker) Stop() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	w.state.Store(int32(StateStopped))
	return w.engine.Shutdown()
}

// Exec executes a PHP request with memory protection.
func (w *Worker) Exec(ctx *phpengine.Context, script string) (*phpengine.Response, error) {
	w.state.Store(int32(StateBusy))
	defer w.state.Store(int32(StateIdle))

	// Check memory before execution
	if w.checkMemoryLimit() {
		return nil, fmt.Errorf("worker memory limit exceeded")
	}

	resp, err := w.engine.Execute(ctx, script)
	if err != nil {
		return nil, err
	}

	// Update stats
	w.jobs.Add(1)
	w.lastJobAt = time.Now()

	// Check if needs recycling
	if w.NeedsRecycle() {
		go w.recycle()
	}

	return resp, nil
}

// NeedsRecycle checks if worker should be recycled.
func (w *Worker) NeedsRecycle() bool {
	// Max jobs reached
	if w.maxJobs > 0 && w.jobs.Load() >= int64(w.maxJobs) {
		return true
	}

	// Memory limit reached
	if w.checkMemoryLimit() {
		return true
	}

	return false
}

// checkMemoryLimit checks if memory usage exceeds limit
func (w *Worker) checkMemoryLimit() bool {
	if w.maxMemory <= 0 {
		return false
	}

	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	current := int64(m.Alloc)

	return (current - w.memStart) > w.maxMemory
}

// getMemoryUsage returns current memory allocation
func (w *Worker) getMemoryUsage() int64 {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	return int64(m.Alloc)
}

// recycle gracefully restarts the worker
func (w *Worker) recycle() {
	w.Stop()
	w.Start()
	w.jobs.Store(0)
}

// Stats returns worker statistics
func (w *Worker) Stats() WorkerStats {
	w.mu.RLock()
	defer w.mu.RUnlock()

	return WorkerStats{
		ID:         w.id,
		State:      w.State(),
		Jobs:       w.jobs.Load(),
		MaxJobs:    w.maxJobs,
		Memory:     w.getMemoryUsage(),
		MaxMemory:  w.maxMemory,
		StartedAt:  w.startedAt,
		LastJobAt:  w.lastJobAt,
		Uptime:     time.Since(w.startedAt),
		NeedsRecyc: w.NeedsRecycle(),
	}
}

// WorkerStats contains worker statistics
type WorkerStats struct {
	ID         int
	State      WorkerState
	Jobs       int64
	MaxJobs    int
	Memory     int64
	MaxMemory  int64
	StartedAt  time.Time
	LastJobAt  time.Time
	Uptime     time.Duration
	NeedsRecyc bool
}
