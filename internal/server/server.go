package server

import (
	"context"
	"crypto/tls"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/sadewadee/maboo/internal/config"
	"github.com/sadewadee/maboo/internal/pool"
)

// Server is the main maboo HTTP server.
type Server struct {
	cfg     *config.Config
	pool    *pool.Pool
	logger  *slog.Logger
	http    *http.Server
	router  *Router
	metrics *Metrics
}

// New creates a new maboo server.
func New(cfg *config.Config, workerPool *pool.Pool, logger *slog.Logger) *Server {
	s := &Server{
		cfg:    cfg,
		pool:   workerPool,
		logger: logger,
	}

	s.metrics = NewMetrics(workerPool)
	s.router = NewRouter(cfg, workerPool, logger)

	s.http = &http.Server{
		Addr:         cfg.Server.Address,
		Handler:      s.buildMiddleware(s.router),
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 60 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	return s
}

// Start begins listening for HTTP connections.
func (s *Server) Start() error {
	s.logger.Info("maboo server starting",
		"address", s.cfg.Server.Address,
		"tls", s.cfg.Server.TLS.Auto,
		"http3", s.cfg.Server.HTTP3,
	)

	if s.cfg.Server.TLS.Auto || (s.cfg.Server.TLS.Cert != "" && s.cfg.Server.TLS.Key != "") {
		return s.startTLS()
	}
	return s.http.ListenAndServe()
}

// Stop gracefully shuts down the server.
func (s *Server) Stop(ctx context.Context) error {
	s.logger.Info("maboo server shutting down")
	return s.http.Shutdown(ctx)
}

func (s *Server) startTLS() error {
	if s.cfg.Server.TLS.Cert != "" && s.cfg.Server.TLS.Key != "" {
		return s.http.ListenAndServeTLS(s.cfg.Server.TLS.Cert, s.cfg.Server.TLS.Key)
	}

	if !s.cfg.Server.TLS.Auto {
		return fmt.Errorf("TLS enabled but no cert/key provided and auto-TLS is disabled")
	}

	s.logger.Warn("auto-TLS: using self-signed certificate for development")

	cert, key, err := generateSelfSignedCert()
	if err != nil {
		return fmt.Errorf("generating self-signed cert: %w", err)
	}

	tlsCert, err := tls.X509KeyPair(cert, key)
	if err != nil {
		return fmt.Errorf("parsing self-signed cert: %w", err)
	}

	s.http.TLSConfig = &tls.Config{
		Certificates: []tls.Certificate{tlsCert},
		MinVersion:   tls.VersionTLS12,
	}

	return s.http.ListenAndServeTLS("", "")
}

func (s *Server) buildMiddleware(handler http.Handler) http.Handler {
	// CoreMiddleware collapses Recovery + RequestID + EarlyHints + Logging
	// into a single handler with one pooled response writer and one context value.
	handler = CoreMiddleware(s.logger)(handler)

	if s.cfg.Metrics.Enabled {
		handler = s.metrics.Middleware(s.cfg.Metrics.Path)(handler)
	}

	// Compression is outermost (wraps everything including metrics)
	handler = CompressionMiddleware()(handler)

	return handler
}
