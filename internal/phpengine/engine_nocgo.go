//go:build !php_embed

package phpengine

import (
	_ "embed"
	"fmt"
	"runtime"
	"strings"
	"sync"
)

//go:embed placeholder.html
var placeholderHTML string

// Engine represents an embedded PHP interpreter instance.
type Engine struct {
	version   string
	mu        sync.RWMutex
	started   bool
	threadID  int32
	extensions *ExtensionManager
}

// NewEngine creates a new embedded PHP engine for the specified version.
// Valid versions: 7.4, 8.0, 8.1, 8.2, 8.3, 8.4
func NewEngine(version string) (*Engine, error) {
	validVersions := map[string]bool{
		"7.4": true, "8.0": true, "8.1": true,
		"8.2": true, "8.3": true, "8.4": true,
	}

	if !validVersions[version] {
		return nil, fmt.Errorf("unsupported PHP version: %s", version)
	}

	return &Engine{
		version:  version,
		started:  false,
		threadID: getThreadID(),
	}, nil
}

// Version returns the PHP version this engine uses.
func (e *Engine) Version() string {
	return e.version
}

// SetExtensions sets the extension manager for this engine
func (e *Engine) SetExtensions(em *ExtensionManager) {
	e.extensions = em
}

// Startup initializes the PHP interpreter.
// This is called once per worker in worker mode.
func (e *Engine) Startup() error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.started {
		return nil
	}

	// Placeholder - actual PHP startup requires libphp
	e.started = true
	return nil
}

// Shutdown cleans up the PHP interpreter.
func (e *Engine) Shutdown() error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if !e.started {
		return nil
	}

	e.started = false
	return nil
}

// Execute runs a PHP script with the given context.
func (e *Engine) Execute(ctx *Context, script string) (*Response, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	if !e.started {
		return nil, fmt.Errorf("engine not started")
	}

	// Placeholder response - actual execution requires libphp
	body := strings.ReplaceAll(placeholderHTML, "{{PHP_VERSION}}", e.version)

	return &Response{
		Status: 200,
		Headers: map[string]string{
			"Content-Type": "text/html; charset=utf-8",
		},
		Body: []byte(body),
	}, nil
}

// MemoryStats returns current memory usage
func (e *Engine) MemoryStats() (alloc, total uint64) {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	return m.Alloc, m.Sys
}

// Response represents the result of PHP execution.
type Response struct {
	Status  int
	Headers map[string]string
	Body    []byte
}
