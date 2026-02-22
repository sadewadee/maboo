package server

import (
	"net/http"

	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
)

// EnableHTTP2 configures HTTP/2 for the server.
// If TLS is enabled, HTTP/2 is automatic.
// If no TLS, enables h2c (HTTP/2 cleartext).
func EnableHTTP2(srv *http.Server, useTLS bool) error {
	if useTLS {
		// HTTP/2 is automatically enabled for TLS servers
		return nil
	}
	// Enable h2c for non-TLS
	srv.Handler = h2c.NewHandler(srv.Handler, &http2.Server{})
	return nil
}
