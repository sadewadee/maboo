package pool

import (
	"fmt"
	"io"
	"os/exec"
	"sync"
	"sync/atomic"
	"time"

	"github.com/maboo-dev/maboo/internal/protocol"
)

// WorkerState represents the current state of a worker.
type WorkerState int

const (
	StateIdle    WorkerState = iota // Worker is ready for a request
	StateBusy                       // Worker is processing a request
	StateStopped                    // Worker has been stopped
)

// Worker represents a single PHP worker process.
type Worker struct {
	id       int
	cmd      *exec.Cmd
	stdin    io.WriteCloser
	stdout   io.ReadCloser
	state    atomic.Int32
	jobs     atomic.Int64
	lastUsed atomic.Int64 // unix timestamp
	mu       sync.Mutex
}

// NewWorker creates and starts a new PHP worker process.
func NewWorker(id int, phpBinary string, workerScript string, env []string) (*Worker, error) {
	cmd := exec.Command(phpBinary, workerScript)
	cmd.Env = env

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("creating stdin pipe: %w", err)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("creating stdout pipe: %w", err)
	}

	// Capture stderr for logging
	cmd.Stderr = nil // TODO: connect to logger

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("starting PHP worker: %w", err)
	}

	w := &Worker{
		id:     id,
		cmd:    cmd,
		stdin:  stdin,
		stdout: stdout,
	}
	w.state.Store(int32(StateIdle))
	w.lastUsed.Store(time.Now().Unix())

	// Wait for WORKER_READY signal from PHP
	frame, err := protocol.ReadFrame(stdout)
	if err != nil {
		cmd.Process.Kill()
		return nil, fmt.Errorf("waiting for worker ready: %w", err)
	}
	if frame.Type != protocol.TypeWorkerReady {
		cmd.Process.Kill()
		return nil, fmt.Errorf("expected WORKER_READY, got type 0x%02x", frame.Type)
	}

	return w, nil
}

// ID returns the worker's unique identifier.
func (w *Worker) ID() int {
	return w.id
}

// State returns the current worker state.
func (w *Worker) State() WorkerState {
	return WorkerState(w.state.Load())
}

// Jobs returns the number of requests this worker has handled.
func (w *Worker) Jobs() int64 {
	return w.jobs.Load()
}

// Exec sends a request frame to the worker and reads the response.
func (w *Worker) Exec(req *protocol.Frame) (*protocol.Frame, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	w.state.Store(int32(StateBusy))
	defer func() {
		w.state.Store(int32(StateIdle))
		w.lastUsed.Store(time.Now().Unix())
		w.jobs.Add(1)
	}()

	// Send request to PHP worker
	if err := protocol.WriteFrame(w.stdin, req); err != nil {
		return nil, fmt.Errorf("sending request to worker %d: %w", w.id, err)
	}

	// Read response from PHP worker
	resp, err := protocol.ReadFrame(w.stdout)
	if err != nil {
		return nil, fmt.Errorf("reading response from worker %d: %w", w.id, err)
	}

	return resp, nil
}

// ExecStream sends a stream frame to the worker (non-blocking response).
func (w *Worker) ExecStream(frame *protocol.Frame) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	return protocol.WriteFrame(w.stdin, frame)
}

// ReadFrame reads a single frame from the worker's stdout.
func (w *Worker) ReadFrame() (*protocol.Frame, error) {
	return protocol.ReadFrame(w.stdout)
}

// Ping sends a health check to the worker and waits for a pong.
func (w *Worker) Ping(timeout time.Duration) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if err := protocol.WriteFrame(w.stdin, protocol.NewPingFrame()); err != nil {
		return fmt.Errorf("sending ping to worker %d: %w", w.id, err)
	}

	// TODO: implement timeout using goroutine + channel
	frame, err := protocol.ReadFrame(w.stdout)
	if err != nil {
		return fmt.Errorf("reading pong from worker %d: %w", w.id, err)
	}
	if frame.Type != protocol.TypePing {
		return fmt.Errorf("expected PONG from worker %d, got type 0x%02x", w.id, frame.Type)
	}
	return nil
}

// Stop gracefully stops the worker process.
func (w *Worker) Stop() error {
	w.state.Store(int32(StateStopped))

	// Try graceful shutdown first
	_ = protocol.WriteFrame(w.stdin, protocol.NewWorkerStopFrame())
	w.stdin.Close()

	// Wait for process to exit (with timeout)
	done := make(chan error, 1)
	go func() {
		done <- w.cmd.Wait()
	}()

	select {
	case err := <-done:
		return err
	case <-time.After(5 * time.Second):
		// Force kill if graceful shutdown fails
		return w.cmd.Process.Kill()
	}
}

// IsAlive checks if the worker process is still running.
func (w *Worker) IsAlive() bool {
	if w.cmd.Process == nil {
		return false
	}
	return w.cmd.ProcessState == nil || !w.cmd.ProcessState.Exited()
}
