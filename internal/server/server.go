package server

import (
	"context"
	"crypto/tls"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/sadewadee/maboo/internal/config"
)

// Server is the main maboo HTTP server.
type Server struct {
	cfg         *config.Config
	pool        Pool
	logger      *slog.Logger
	http        *http.Server
	http3       *HTTP3Server
	router      *Router
	metrics     *Metrics
	redirectSrv *http.Server // HTTP redirect server for ACME
}

// New creates a new maboo server.
func New(cfg *config.Config, workerPool Pool, logger *slog.Logger) *Server {
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

	// Enable HTTP/2 if configured
	if cfg.Server.HTTP2 {
		useTLS := cfg.Server.TLS.Auto || (cfg.Server.TLS.Cert != "" && cfg.Server.TLS.Key != "") || cfg.Server.TLS.ACME.Email != ""
		if err := EnableHTTP2(s.http, useTLS); err != nil {
			logger.Warn("failed to enable HTTP/2", "error", err)
		} else {
			logger.Debug("HTTP/2 enabled")
		}
	}

	return s
}

// Start begins listening for HTTP connections.
func (s *Server) Start() error {
	s.logger.Info("maboo server starting",
		"address", s.cfg.Server.Address,
		"http2", s.cfg.Server.HTTP2,
		"http3", s.cfg.Server.HTTP3,
		"tls", s.cfg.Server.TLS.Auto,
	)

	if s.cfg.Server.TLS.Auto || (s.cfg.Server.TLS.Cert != "" && s.cfg.Server.TLS.Key != "") || s.cfg.Server.TLS.ACME.Email != "" {
		return s.startTLS()
	}
	return s.http.ListenAndServe()
}

// Stop gracefully shuts down the server.
func (s *Server) Stop(ctx context.Context) error {
	s.logger.Info("maboo server shutting down")

	// Stop HTTP/3 server if running
	if s.http3 != nil {
		if err := s.http3.Stop(ctx); err != nil {
			s.logger.Warn("error shutting down HTTP/3 server", "error", err)
		}
	}

	// Stop HTTP redirect server if running
	if s.redirectSrv != nil {
		if err := s.redirectSrv.Shutdown(ctx); err != nil {
			s.logger.Warn("error shutting down HTTP redirect server", "error", err)
		}
	}

	return s.http.Shutdown(ctx)
}

func (s *Server) startTLS() error {
	var tlsConfig *tls.Config

	// Check for ACME config first (Let's Encrypt)
	if s.cfg.Server.TLS.ACME.Email != "" {
		var err error
		tlsConfig, s.redirectSrv, err = SetupACME(s.cfg, s.logger)
		if err != nil {
			return fmt.Errorf("setting up ACME: %w", err)
		}
	} else if s.cfg.Server.TLS.Cert != "" && s.cfg.Server.TLS.Key != "" {
		// Use custom cert/key if provided
		return s.http.ListenAndServeTLS(s.cfg.Server.TLS.Cert, s.cfg.Server.TLS.Key)
	} else if s.cfg.Server.TLS.Auto {
		// Self-signed cert for development
		s.logger.Warn("auto-TLS: using self-signed certificate for development")

		cert, key, err := generateSelfSignedCert()
		if err != nil {
			return fmt.Errorf("generating self-signed cert: %w", err)
		}

		tlsCert, err := tls.X509KeyPair(cert, key)
		if err != nil {
			return fmt.Errorf("parsing self-signed cert: %w", err)
		}

		tlsConfig = &tls.Config{
			Certificates: []tls.Certificate{tlsCert},
			MinVersion:   tls.VersionTLS12,
		}
	} else {
		return fmt.Errorf("TLS enabled but no cert/key provided and auto-TLS is disabled")
	}

	s.http.TLSConfig = tlsConfig

	// Start HTTP/3 server if enabled
	if s.cfg.Server.HTTP3 {
		s.http3 = NewHTTP3Server(s.cfg, s.buildMiddleware(s.router), tlsConfig, s.logger)
		go func() {
			if err := s.http3.Start(); err != nil {
				s.logger.Error("HTTP/3 server error", "error", err)
			}
		}()
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

	// Add Alt-Svc header for HTTP/3 advertisement
	if s.cfg.Server.HTTP3 {
		handler = AltSvcMiddleware(443)(handler)
	}

	return handler
}
