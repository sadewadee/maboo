package phpengine

import (
	"fmt"
	"sync"
)

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
	// For now, return placeholder response showing the request info
	body := `<!DOCTYPE html>
<html>
<head>
    <title>Maboo - PHP Placeholder</title>
    <style>
        body { font-family: system-ui, sans-serif; max-width: 800px; margin: 50px auto; padding: 20px; }
        h1 { color: #6366f1; }
        .info { background: #f3f4f6; padding: 15px; border-radius: 8px; margin: 10px 0; }
        .warning { background: #fef3c7; padding: 10px; border-radius: 8px; margin: 10px 0; }
    </style>
</head>
<body>
    <h1>👻 Maboo - PHP Placeholder Response</h1>
    <div class="warning">
        <strong>Note:</strong> This is a placeholder response. Actual PHP execution requires CGO bindings to libphp.
    </div>
    <div class="info">
        <h3>Request Info</h3>
        <ul>
            <li><strong>Script:</strong> ` + script + `</li>
            <li><strong>PHP Version:</strong> ` + e.version + `</li>
            <li><strong>Method:</strong> ` + ctx.Server["REQUEST_METHOD"] + `</li>
            <li><strong>URI:</strong> ` + ctx.Server["REQUEST_URI"] + `</li>
            <li><strong>Document Root:</strong> ` + ctx.DocumentRoot + `</li>
        </ul>
    </div>
    <div class="info">
        <h3>$_SERVER</h3>
        <pre>` + formatMap(ctx.Server) + `</pre>
    </div>
</body>
</html>`

	return &Response{
		Status: 200,
		Headers: map[string]string{
			"Content-Type": "text/html; charset=utf-8",
		},
		Body: []byte(body),
	}, nil
}

func formatMap(m map[string]string) string {
	result := ""
	for k, v := range m {
		result += k + ": " + v + "\n"
	}
	return result
}

// Response represents the result of PHP execution.
type Response struct {
	Status  int
	Headers map[string]string
	Body    []byte
}
