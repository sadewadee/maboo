package server

import (
	"log/slog"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/sadewadee/maboo/internal/config"
	"github.com/sadewadee/maboo/internal/phpengine"
)

// Router dispatches incoming HTTP requests to the appropriate handler.
type Router struct {
	cfg           *config.Config
	pool          Pool
	logger        *slog.Logger
	static        http.Handler
	phpHandler    http.Handler
	healthHandler *HealthHandler
}

// NewRouter creates a new request router.
func NewRouter(cfg *config.Config, workerPool Pool, logger *slog.Logger) *Router {
	r := &Router{
		cfg:    cfg,
		pool:   workerPool,
		logger: logger,
	}

	// Static file handler
	if cfg.Static.Root != "" {
		r.static = http.FileServer(http.Dir(cfg.Static.Root))
	}

	// PHP handler
	r.phpHandler = r.newPHPHandler()

	// Health check handler
	r.healthHandler = NewHealthHandler(workerPool)

	return r
}

func (r *Router) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	// Health check endpoints
	switch req.URL.Path {
	case "/health", "/healthz", "/ready", "/readyz":
		r.healthHandler.ServeHTTP(w, req)
		return
	}

	// Check if it's a static file first
	if r.static != nil && r.isStaticFile(req.URL.Path) {
		if r.cfg.Static.CacheControl != "" {
			w.Header().Set("Cache-Control", r.cfg.Static.CacheControl)
		}
		r.static.ServeHTTP(w, req)
		return
	}

	// Forward everything else to PHP
	r.phpHandler.ServeHTTP(w, req)
}

func (r *Router) isStaticFile(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".css", ".js", ".png", ".jpg", ".jpeg", ".gif", ".svg", ".ico",
		".woff", ".woff2", ".ttf", ".eot", ".map", ".webp", ".avif",
		".mp4", ".webm", ".pdf", ".txt", ".xml", ".json":
		return true
	}
	return false
}

func (r *Router) newPHPHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		// Determine document root and entry point
		docRoot := r.cfg.App.Root
		if docRoot == "" {
			docRoot = "."
		}

		entryPoint := phpengine.DetectEntryPoint(docRoot, r.cfg.App.Entry)
		script := filepath.Join(docRoot, entryPoint)

		// Create PHP context from HTTP request
		ctx := phpengine.NewContext(req, docRoot, entryPoint)

		// Dispatch to worker pool
		resp, err := r.pool.Exec(ctx, script)
		if err != nil {
			r.logger.Error("worker exec", "error", err)
			http.Error(w, "Internal Server Error: "+err.Error(), http.StatusBadGateway)
			return
		}

		// Write response headers
		for k, v := range resp.Headers {
			w.Header().Set(k, v)
		}
		w.WriteHeader(resp.Status)
		w.Write(resp.Body)
	})
}
