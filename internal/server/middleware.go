package server

import (
	"context"
	"encoding/hex"
	"log/slog"
	"net/http"
	"runtime/debug"
	"strings"
	"sync"
	"time"

	"crypto/rand"
)

// --- Single context key for all middleware data (fix #4) ---

type mabooCtxKey struct{}

// MabooRequestCtx carries request metadata through the middleware stack
// using a single context.WithValue call instead of per-middleware chaining.
type MabooRequestCtx struct {
	RequestID string
	StartTime time.Time
}

// GetRequestCtx retrieves the request context from the context.
func GetRequestCtx(ctx context.Context) *MabooRequestCtx {
	if v := ctx.Value(mabooCtxKey{}); v != nil {
		return v.(*MabooRequestCtx)
	}
	return nil
}

// --- Pooled unified response writer (fix #2, #8) ---

var rwPool = sync.Pool{
	New: func() interface{} {
		return &mabooResponseWriter{}
	},
}

type mabooResponseWriter struct {
	http.ResponseWriter
	statusCode   int
	bytesWritten int
	wroteHeader  bool
	hintsSent    bool // early hints tracking baked in (no separate wrapper)
}

func (rw *mabooResponseWriter) reset(w http.ResponseWriter) {
	rw.ResponseWriter = w
	rw.statusCode = 200
	rw.bytesWritten = 0
	rw.wroteHeader = false
	rw.hintsSent = false
}

func (rw *mabooResponseWriter) WriteHeader(code int) {
	// Baked-in early hints check (eliminates earlyHintsWriter allocation)
	if !rw.hintsSent {
		rw.hintsSent = true
		links := rw.Header().Values("Link")
		for _, link := range links {
			if strings.Contains(link, "rel=preload") || strings.Contains(link, "rel=preconnect") {
				rw.ResponseWriter.WriteHeader(http.StatusEarlyHints)
				break
			}
		}
	}

	if rw.wroteHeader {
		return
	}
	rw.wroteHeader = true
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

func (rw *mabooResponseWriter) Write(b []byte) (int, error) {
	if !rw.wroteHeader {
		rw.wroteHeader = true
		rw.statusCode = 200
	}
	n, err := rw.ResponseWriter.Write(b)
	rw.bytesWritten += n
	return n, err
}

func (rw *mabooResponseWriter) Unwrap() http.ResponseWriter {
	return rw.ResponseWriter
}

// --- Request ID generation (fix #7) ---

var ridBufPool = sync.Pool{
	New: func() interface{} {
		b := make([]byte, 8)
		return &b
	},
}

func fastRequestID() string {
	bp := ridBufPool.Get().(*[]byte)
	b := *bp
	rand.Read(b)
	var dst [16]byte
	hex.Encode(dst[:], b)
	ridBufPool.Put(bp)
	return string(dst[:])
}

// --- Collapsed middleware (fix #2, #4) ---
// Recovery + RequestID + EarlyHints + Logging in ONE handler.
// This eliminates 3 closure allocations, 3 function call layers,
// and the separate earlyHintsWriter allocation per request.

// CoreMiddleware combines recovery, request ID, early hints, and logging
// into a single middleware to minimize allocation and call overhead.
func CoreMiddleware(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// 1. Recovery (defer at top)
			defer func() {
				if err := recover(); err != nil {
					logger.Error("panic recovered",
						"error", err,
						"stack", string(debug.Stack()),
						"path", r.URL.Path,
					)
					http.Error(w, "Internal Server Error", http.StatusInternalServerError)
				}
			}()

			// 2. Request ID
			id := r.Header.Get("X-Request-ID")
			if id == "" {
				id = fastRequestID()
				r.Header.Set("X-Request-ID", id)
			}
			w.Header().Set("X-Request-ID", id)

			// 3. Pooled response writer with baked-in early hints
			start := time.Now()
			rw := rwPool.Get().(*mabooResponseWriter)
			rw.reset(w)

			next.ServeHTTP(rw, r)

			// 4. Logging (after response, guarded by level check)
			// Stack-allocated attrs array avoids slice header + grow alloc
			if logger.Enabled(r.Context(), slog.LevelInfo) {
				attrs := [7]slog.Attr{
					slog.String("method", r.Method),
					slog.String("path", r.URL.Path),
					slog.Int("status", rw.statusCode),
					slog.Duration("duration", time.Since(start)),
					slog.Int("bytes", rw.bytesWritten),
					slog.String("remote_addr", r.RemoteAddr),
					slog.String("request_id", id),
				}
				logger.LogAttrs(r.Context(), slog.LevelInfo, "request", attrs[:]...)
			}

			rwPool.Put(rw)
		})
	}
}

// --- Individual middleware kept for backwards compatibility / standalone use ---

func RecoveryMiddleware(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if err := recover(); err != nil {
					logger.Error("panic recovered",
						"error", err,
						"stack", string(debug.Stack()),
						"path", r.URL.Path,
					)
					http.Error(w, "Internal Server Error", http.StatusInternalServerError)
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}

func RequestIDMiddleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			id := r.Header.Get("X-Request-ID")
			if id == "" {
				id = fastRequestID()
				r.Header.Set("X-Request-ID", id)
			}
			w.Header().Set("X-Request-ID", id)
			next.ServeHTTP(w, r)
		})
	}
}

func LoggingMiddleware(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			rw := rwPool.Get().(*mabooResponseWriter)
			rw.reset(w)
			start := time.Now()
			next.ServeHTTP(rw, r)
			if logger.Enabled(r.Context(), slog.LevelInfo) {
				attrs := [7]slog.Attr{
					slog.String("method", r.Method),
					slog.String("path", r.URL.Path),
					slog.Int("status", rw.statusCode),
					slog.Duration("duration", time.Since(start)),
					slog.Int("bytes", rw.bytesWritten),
					slog.String("remote_addr", r.RemoteAddr),
					slog.String("request_id", r.Header.Get("X-Request-ID")),
				}
				logger.LogAttrs(r.Context(), slog.LevelInfo, "request", attrs[:]...)
			}
			rwPool.Put(rw)
		})
	}
}
