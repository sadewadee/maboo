package server

import (
	"net/http"
	"strings"
)

// EarlyHintsMiddleware sends HTTP 103 Early Hints for Link headers.
// When the response includes Link headers with rel=preload or rel=preconnect,
// they are sent as 103 Early Hints before the actual response.
func EarlyHintsMiddleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ew := &earlyHintsWriter{
				ResponseWriter: w,
				hintsSent:      false,
			}
			next.ServeHTTP(ew, r)
		})
	}
}

type earlyHintsWriter struct {
	http.ResponseWriter
	hintsSent bool
}

func (w *earlyHintsWriter) WriteHeader(code int) {
	if !w.hintsSent {
		w.hintsSent = true
		links := w.Header().Values("Link")
		for _, link := range links {
			if strings.Contains(link, "rel=preload") || strings.Contains(link, "rel=preconnect") {
				w.ResponseWriter.WriteHeader(http.StatusEarlyHints)
				break
			}
		}
	}
	w.ResponseWriter.WriteHeader(code)
}
