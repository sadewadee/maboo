package worker

import (
	"fmt"
	"sync"
	"sync/atomic"

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
	id      int
	engine  *phpengine.Engine
	state   atomic.Int32
	jobs    atomic.Int64
	maxJobs int

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

	return &Worker{
		id:      id,
		engine:  engine,
		maxJobs: cfg.Pool.MaxJobs,
	}, nil
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
	w.state.Store(int32(StateIdle))
	return w.engine.Startup()
}

// Stop shuts down the worker.
func (w *Worker) Stop() error {
	w.state.Store(int32(StateStopped))
	return w.engine.Shutdown()
}

// Exec executes a PHP request.
func (w *Worker) Exec(ctx *phpengine.Context, script string) (*phpengine.Response, error) {
	w.state.Store(int32(StateBusy))
	defer w.state.Store(int32(StateIdle))

	resp, err := w.engine.Execute(ctx, script)
	if err != nil {
		return nil, err
	}

	w.jobs.Add(1)
	return resp, nil
}

// NeedsRecycle checks if worker should be recycled.
func (w *Worker) NeedsRecycle() bool {
	return w.maxJobs > 0 && w.jobs.Load() >= int64(w.maxJobs)
}
