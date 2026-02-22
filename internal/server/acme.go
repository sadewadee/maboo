package server

import (
	"crypto/tls"
	"fmt"
	"log/slog"
	"net/http"
	"os"

	"github.com/sadewadee/maboo/internal/config"
	"golang.org/x/crypto/acme"
	"golang.org/x/crypto/acme/autocert"
)

// NewACMEManager creates an autocert manager for Let's Encrypt.
func NewACMEManager(cfg *config.ACMEConfig, logger *slog.Logger) (*autocert.Manager, error) {
	if cfg.Email == "" {
		return nil, fmt.Errorf("ACME email is required")
	}
	if len(cfg.Domains) == 0 {
		return nil, fmt.Errorf("ACME domains are required")
	}

	cacheDir := cfg.CacheDir
	if cacheDir == "" {
		cacheDir = "/var/lib/maboo/certs"
	}

	// Ensure cache directory exists
	if err := os.MkdirAll(cacheDir, 0700); err != nil {
		return nil, fmt.Errorf("creating cert cache dir: %w", err)
	}

	manager := &autocert.Manager{
		Prompt:     autocert.AcceptTOS,
		Email:      cfg.Email,
		HostPolicy: autocert.HostWhitelist(cfg.Domains...),
		Cache:      autocert.DirCache(cacheDir),
	}

	if cfg.Staging {
		// Use Let's Encrypt staging server
		manager.Client = &acme.Client{DirectoryURL: "https://acme-staging-v02.api.letsencrypt.org/directory"}
		logger.Info("using Let's Encrypt staging server")
	}

	return manager, nil
}

// HTTPRedirectServer starts an HTTP server on the given address that redirects to HTTPS.
// It also handles ACME HTTP-01 challenges for Let's Encrypt certificate issuance.
func HTTPRedirectServer(addr string, manager *autocert.Manager, logger *slog.Logger) *http.Server {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		httpsURL := "https://" + r.Host + r.URL.Path
		if r.URL.RawQuery != "" {
			httpsURL += "?" + r.URL.RawQuery
		}
		http.Redirect(w, r, httpsURL, http.StatusMovedPermanently)
	})

	// Handle ACME HTTP-01 challenge
	handler := manager.HTTPHandler(mux)

	srv := &http.Server{Addr: addr, Handler: handler}
	go func() {
		logger.Info("starting HTTP redirect server for ACME challenges", "address", addr)
		if err := srv.ListenAndServe(); err != http.ErrServerClosed {
			logger.Error("HTTP redirect server error", "error", err)
		}
	}()
	return srv
}

// SetupACME configures TLS with ACME (Let's Encrypt) certificate management.
// Returns the TLS config and optionally starts an HTTP redirect server.
func SetupACME(cfg *config.Config, logger *slog.Logger) (*tls.Config, *http.Server, error) {
	if cfg.Server.TLS.ACME.Email == "" {
		return nil, nil, fmt.Errorf("ACME email is required")
	}

	manager, err := NewACMEManager(&cfg.Server.TLS.ACME, logger)
	if err != nil {
		return nil, nil, fmt.Errorf("creating ACME manager: %w", err)
	}

	tlsConfig := &tls.Config{
		GetCertificate: manager.GetCertificate,
		MinVersion:     tls.VersionTLS12,
	}

	var redirectSrv *http.Server
	if cfg.Server.HTTPRedirect {
		redirectSrv = HTTPRedirectServer(":80", manager, logger)
	}

	return tlsConfig, redirectSrv, nil
}
