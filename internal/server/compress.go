package server

import (
	"compress/gzip"
	"io"
	"net/http"
	"strings"
	"sync"
)

var gzipPool = sync.Pool{
	New: func() interface{} {
		w, _ := gzip.NewWriterLevel(io.Discard, gzip.DefaultCompression)
		return w
	},
}

// CompressionMiddleware applies gzip compression to eligible responses.
func CompressionMiddleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
				next.ServeHTTP(w, r)
				return
			}

			cw := &compressWriter{
				ResponseWriter: w,
				request:        r,
				minSize:        1024,
			}
			defer cw.Close()

			next.ServeHTTP(cw, r)
		})
	}
}

type compressWriter struct {
	http.ResponseWriter
	request     *http.Request
	gzWriter    *gzip.Writer
	minSize     int
	buf         []byte
	wroteHeader bool
	compressed  bool
}

func (cw *compressWriter) shouldCompress() bool {
	ct := cw.Header().Get("Content-Type")
	if ct == "" {
		return false
	}
	// Already compressed
	if cw.Header().Get("Content-Encoding") != "" {
		return false
	}
	ct = strings.ToLower(ct)
	return strings.HasPrefix(ct, "text/") ||
		strings.Contains(ct, "application/json") ||
		strings.Contains(ct, "application/javascript") ||
		strings.Contains(ct, "application/xml") ||
		strings.Contains(ct, "application/xhtml") ||
		strings.Contains(ct, "image/svg+xml")
}

func (cw *compressWriter) WriteHeader(code int) {
	if cw.wroteHeader {
		return
	}
	cw.wroteHeader = true

	if cw.shouldCompress() && len(cw.buf) >= cw.minSize {
		cw.startCompress()
	}

	cw.ResponseWriter.WriteHeader(code)
}

func (cw *compressWriter) Write(b []byte) (int, error) {
	if cw.compressed {
		return cw.gzWriter.Write(b)
	}

	cw.buf = append(cw.buf, b...)

	if len(cw.buf) >= cw.minSize && !cw.wroteHeader {
		if cw.shouldCompress() {
			cw.startCompress()
			cw.wroteHeader = true
			cw.ResponseWriter.WriteHeader(http.StatusOK)
			return cw.gzWriter.Write(cw.buf)
		}
	}

	return len(b), nil
}

func (cw *compressWriter) startCompress() {
	cw.Header().Set("Content-Encoding", "gzip")
	cw.Header().Set("Vary", "Accept-Encoding")
	cw.Header().Del("Content-Length")
	cw.compressed = true

	gz := gzipPool.Get().(*gzip.Writer)
	gz.Reset(cw.ResponseWriter)
	cw.gzWriter = gz
}

func (cw *compressWriter) Close() {
	if cw.compressed && cw.gzWriter != nil {
		cw.gzWriter.Close()
		gzipPool.Put(cw.gzWriter)
	} else if len(cw.buf) > 0 {
		// Flush buffered data uncompressed
		if !cw.wroteHeader {
			cw.ResponseWriter.WriteHeader(http.StatusOK)
		}
		cw.ResponseWriter.Write(cw.buf)
	}
}
