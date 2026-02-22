package phpengine

import (
	_ "embed"
	"fmt"
	"strings"
	"sync"
)

//go:embed placeholder.html
var placeholderHTML string

// Engine represents an embedded PHP interpreter instance.
type Engine struct {
	version string
	mu      sync.RWMutex
	started bool
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
		version: version,
		started: false,
	}, nil
}

// Version returns the PHP version this engine uses.
func (e *Engine) Version() string {
	return e.version
}

// Startup initializes the PHP interpreter.
// This is called once per worker in worker mode.
func (e *Engine) Startup() error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.started {
		return nil
	}

	// TODO: Call CGO php_startup()
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

	// TODO: Call CGO php_shutdown()
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

	// TODO: Call CGO php_execute()
	// For now, return placeholder response
	body := strings.ReplaceAll(placeholderHTML, "{{PHP_VERSION}}", e.version)

	return &Response{
		Status: 200,
		Headers: map[string]string{
			"Content-Type": "text/html; charset=utf-8",
		},
		Body: []byte(body),
	}, nil
}

// Response represents the result of PHP execution.
type Response struct {
	Status  int
	Headers map[string]string
	Body    []byte
}
