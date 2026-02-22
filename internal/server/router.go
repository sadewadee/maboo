package server

import (
	"io"
	"log/slog"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/sadewadee/maboo/internal/config"
	"github.com/sadewadee/maboo/internal/pool"
	"github.com/sadewadee/maboo/internal/protocol"
)

// Router dispatches incoming HTTP requests to the appropriate handler.
type Router struct {
	cfg           *config.Config
	pool          *pool.Pool
	logger        *slog.Logger
	static        http.Handler
	phpHandler    http.Handler
	healthHandler *HealthHandler
}

// NewRouter creates a new request router.
func NewRouter(cfg *config.Config, workerPool *pool.Pool, logger *slog.Logger) *Router {
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
		// Read request body
		var body []byte
		if req.Body != nil {
			var err error
			body, err = io.ReadAll(req.Body)
			if err != nil {
				r.logger.Error("reading request body", "error", err)
				http.Error(w, "Failed to read request body", http.StatusBadRequest)
				return
			}
			defer req.Body.Close()
		}

		// Build protocol headers
		headers := make(map[string]string)
		for k, v := range req.Header {
			headers[k] = strings.Join(v, ", ")
		}

		reqHeader := &protocol.RequestHeader{
			Method:      req.Method,
			URI:         req.URL.Path,
			QueryString: req.URL.RawQuery,
			Headers:     headers,
			RemoteAddr:  req.RemoteAddr,
			ServerName:  req.Host,
			ServerPort:  r.extractPort(req),
			Protocol:    req.Proto,
		}

		// Encode request as protocol frame
		frame, err := protocol.EncodeRequest(reqHeader, body)
		if err != nil {
			r.logger.Error("encoding request", "error", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}

		// Dispatch to worker pool
		respFrame, err := r.pool.Exec(frame)
		if err != nil {
			r.logger.Error("worker exec", "error", err)
			http.Error(w, "Internal Server Error: "+err.Error(), http.StatusBadGateway)
			return
		}

		// Decode response
		resp, respBody, err := protocol.DecodeResponse(respFrame)
		if err != nil {
			r.logger.Error("decoding response", "error", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}

		// Write response headers
		for k, v := range resp.Headers {
			w.Header().Set(k, v)
		}
		w.WriteHeader(resp.Status)
		w.Write(respBody)
	})
}

func (r *Router) extractPort(req *http.Request) string {
	if i := strings.LastIndex(req.Host, ":"); i != -1 {
		return req.Host[i+1:]
	}
	if req.TLS != nil {
		return "443"
	}
	return "80"
}
