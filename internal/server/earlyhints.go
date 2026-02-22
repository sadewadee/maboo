package server

import (
	"net/http"
)

// EarlyHintsMiddleware sends HTTP 103 Early Hints headers to the client.
// This allows the browser to start preloading resources before the main response.
func EarlyHintsMiddleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Check if client supports early hints (HTTP/2+)
			if r.ProtoMajor >= 2 {
				// Early hints can be sent here for known resources
				// For now, this is a placeholder for future implementation
				// Example: w.Header().Add("Link", "</style.css>; rel=preload; as=style")
			}
			next.ServeHTTP(w, r)
		})
	}
}
