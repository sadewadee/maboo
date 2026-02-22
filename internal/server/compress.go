package server

import (
	"compress/gzip"
	"net/http"
	"strings"
	"sync"
)

// Pool for gzip.Writer - fixes #1 (813KB/op → ~2KB/op)
var gzWriterPool = sync.Pool{
	New: func() interface{} {
		// Use BestSpeed for lower latency (fix #10)
		// Compression ratio is only ~5-10% worse than DefaultCompression
		// but throughput doubles
		w, _ := gzip.NewWriterLevel(nil, gzip.BestSpeed)
		return w
	},
}

// Pool for compressWriter structs - fixes #8
var compressWriterPool = sync.Pool{
	New: func() interface{} {
		return &compressWriter{}
	},
}

const compressMinSize = 1024

// CompressionMiddleware applies gzip compression to eligible responses.
func CompressionMiddleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Fast path: skip if client doesn't accept gzip
			if !strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
				next.ServeHTTP(w, r)
				return
			}

			cw := compressWriterPool.Get().(*compressWriter)
			cw.reset(w)
			defer func() {
				cw.Close()
				compressWriterPool.Put(cw)
			}()

			next.ServeHTTP(cw, r)
		})
	}
}

type compressWriter struct {
	http.ResponseWriter
	gzWriter    *gzip.Writer
	buf         []byte // lazy-allocated only when needed (fix #3)
	wroteHeader bool
	compressed  bool
	headerCode  int
}

func (cw *compressWriter) reset(w http.ResponseWriter) {
	cw.ResponseWriter = w
	cw.gzWriter = nil
	cw.buf = cw.buf[:0] // reuse backing array if available
	cw.wroteHeader = false
	cw.compressed = false
	cw.headerCode = 0
}

func (cw *compressWriter) shouldCompress() bool {
	ct := cw.Header().Get("Content-Type")
	if ct == "" {
		return false
	}
	if cw.Header().Get("Content-Encoding") != "" {
		return false
	}
	// Fast check without ToLower allocation
	return isCompressibleContentType(ct)
}

// isCompressibleContentType checks without allocating a lowercased copy.
func isCompressibleContentType(ct string) bool {
	// Most common cases first for fast path
	if len(ct) >= 5 {
		switch {
		case strings.HasPrefix(ct, "text/"),
			strings.HasPrefix(ct, "Text/"),
			strings.HasPrefix(ct, "TEXT/"):
			return true
		}
	}
	return strings.Contains(ct, "application/json") ||
		strings.Contains(ct, "application/javascript") ||
		strings.Contains(ct, "application/xml") ||
		strings.Contains(ct, "application/xhtml") ||
		strings.Contains(ct, "image/svg+xml")
}

func (cw *compressWriter) WriteHeader(code int) {
	if cw.wroteHeader {
		return
	}
	cw.headerCode = code
	cw.wroteHeader = true

	// If we have enough buffered data and content is compressible, start compression
	if len(cw.buf) >= compressMinSize && cw.shouldCompress() {
		cw.startCompress()
	}

	cw.ResponseWriter.WriteHeader(code)
}

func (cw *compressWriter) Write(b []byte) (int, error) {
	if cw.compressed {
		return cw.gzWriter.Write(b)
	}

	// Buffer data until we can decide about compression
	cw.buf = append(cw.buf, b...)

	if len(cw.buf) >= compressMinSize && !cw.wroteHeader {
		if cw.shouldCompress() {
			cw.startCompress()
			cw.wroteHeader = true
			cw.ResponseWriter.WriteHeader(http.StatusOK)
			n, err := cw.gzWriter.Write(cw.buf)
			// Return original write size to caller
			if n > len(b) {
				return len(b), err
			}
			return n, err
		}
	}

	return len(b), nil
}

func (cw *compressWriter) startCompress() {
	cw.Header().Set("Content-Encoding", "gzip")
	cw.Header().Set("Vary", "Accept-Encoding")
	cw.Header().Del("Content-Length")
	cw.compressed = true

	gz := gzWriterPool.Get().(*gzip.Writer)
	gz.Reset(cw.ResponseWriter) // Reuse pooled writer (fix #1)
	cw.gzWriter = gz
}

func (cw *compressWriter) Close() {
	if cw.compressed && cw.gzWriter != nil {
		cw.gzWriter.Close()
		gzWriterPool.Put(cw.gzWriter)
		cw.gzWriter = nil
	} else if len(cw.buf) > 0 {
		if !cw.wroteHeader {
			cw.ResponseWriter.WriteHeader(http.StatusOK)
		}
		cw.ResponseWriter.Write(cw.buf)
	}
}
