//go:build php_embed

package phpengine

/*
#cgo CFLAGS: -I${SRCDIR}/sapi
#cgo LDFLAGS: -L${SRCDIR}/lib -lphp -lm -ldl

#include "sapi/maboo_sapi.h"
#include <stdlib.h>
*/
import "C"
import (
	"fmt"
	"runtime"
	"sync"
	"unsafe"
)

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

	// Initialize PHP engine via CGO
	cversion := C.CString(e.version)
	defer C.free(unsafe.Pointer(cversion))

	ret := C.php_engine_startup(cversion)
	if ret != 0 {
		return fmt.Errorf("PHP engine startup failed with code %d", ret)
	}

	// Load extensions if configured
	if e.extensions != nil {
		if err := e.extensions.LoadExtensions(); err != nil {
			C.php_engine_shutdown()
			return fmt.Errorf("loading extensions: %w", err)
		}
	}

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

	C.php_engine_shutdown()
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

	// Create C context
	cctx := C.php_context_new()
	if cctx == nil {
		return nil, fmt.Errorf("failed to create PHP context")
	}
	defer C.php_context_free(cctx)

	// Set thread index for callback routing
	C.php_context_set_thread_index(cctx, C.int(e.threadID))

	// Set up request context for callbacks
	reqCtx := &requestContext{
		server:   make(map[string]string),
		headers:  make(map[string]string),
		postData: ctx.Body,
		cookies:  ctx.Cookies,
		output:   make([]byte, 0, 8192),
	}
	setRequestContext(e.threadID, reqCtx)
	defer clearRequestContext(e.threadID)

	// Set superglobals
	for k, v := range ctx.Server {
		ck := C.CString(k)
		cv := C.CString(v)
		C.php_context_set_server(cctx, ck, cv)
		C.free(unsafe.Pointer(ck))
		C.free(unsafe.Pointer(cv))
	}

	// Set document root and script
	if ctx.DocumentRoot != "" {
		croot := C.CString(ctx.DocumentRoot)
		C.php_context_set_document_root(cctx, croot)
		C.free(unsafe.Pointer(croot))
	}

	cscript := C.CString(script)
	defer C.free(unsafe.Pointer(cscript))

	C.php_context_set_script_filename(cctx, cscript)

	// Set POST data if present
	if len(ctx.Body) > 0 {
		C.php_context_set_post_data(cctx, (*C.char)(unsafe.Pointer(&ctx.Body[0])), C.size_t(len(ctx.Body)))
	}

	// Execute
	resp := C.php_execute(cctx, cscript)
	if resp == nil {
		return nil, fmt.Errorf("PHP execution failed")
	}
	defer C.php_response_free(resp)

	// Convert C response to Go
	result := &Response{
		Status:  int(resp.status),
		Headers: make(map[string]string),
	}

	// Copy headers
	if resp.headers != nil && resp.headers_len > 0 {
		headersStr := C.GoStringN(resp.headers, C.int(resp.headers_len))
		result.Headers = parseHeaders(headersStr, result.Status)
	}

	// Copy body
	if resp.body != nil && resp.body_len > 0 {
		result.Body = C.GoBytes(unsafe.Pointer(resp.body), C.int(resp.body_len))
	} else {
		result.Body = reqCtx.output
	}

	return result, nil
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
