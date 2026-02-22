package server

import (
	"context"
	"crypto/tls"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/sadewadee/maboo/internal/config"
	"github.com/quic-go/quic-go/http3"
)

// HTTP3Server wraps the HTTP/3 (QUIC) server.
type HTTP3Server struct {
	server *http3.Server
	logger *slog.Logger
}

// NewHTTP3Server creates an HTTP/3 server.
func NewHTTP3Server(cfg *config.Config, handler http.Handler, tlsConfig *tls.Config, logger *slog.Logger) *HTTP3Server {
	if !cfg.Server.HTTP3 {
		return nil
	}

	if tlsConfig == nil {
		logger.Warn("HTTP/3 requires TLS, but no TLS config provided")
		return nil
	}

	server := &http3.Server{
		Addr:      cfg.Server.Address,
		Handler:   handler,
		TLSConfig: tlsConfig,
	}

	return &HTTP3Server{server: server, logger: logger}
}

// Start begins listening for HTTP/3 connections.
func (s *HTTP3Server) Start() error {
	if s == nil {
		return nil
	}
	s.logger.Info("starting HTTP/3 server", "address", s.server.Addr)
	return s.server.ListenAndServe()
}

// Stop gracefully shuts down the HTTP/3 server.
func (s *HTTP3Server) Stop(ctx context.Context) error {
	if s == nil {
		return nil
	}
	return s.server.Close()
}

// AltSvcHeader returns the Alt-Svc header value for HTTP/3 advertisement.
func AltSvcHeader(port int) string {
	return fmt.Sprintf(`h3=":%d"; ma=86400`, port)
}

// AltSvcMiddleware adds Alt-Svc header to advertise HTTP/3 support.
func AltSvcMiddleware(port int) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Alt-Svc", AltSvcHeader(port))
			next.ServeHTTP(w, r)
		})
	}
}
