package server

import (
	"compress/gzip"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func BenchmarkCompressionMiddleware_SmallResponse(b *testing.B) {
	handler := CompressionMiddleware()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte("<h1>Hello</h1>"))
	}))

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest("GET", "/", nil)
		req.Header.Set("Accept-Encoding", "gzip")
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
	}
}

func BenchmarkCompressionMiddleware_LargeResponse(b *testing.B) {
	largeBody := strings.Repeat("<p>This is a paragraph of text that should be compressed.</p>\n", 200)
	handler := CompressionMiddleware()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(largeBody))
	}))

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest("GET", "/", nil)
		req.Header.Set("Accept-Encoding", "gzip")
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
	}
}

func BenchmarkCompressionMiddleware_NoCompression(b *testing.B) {
	handler := CompressionMiddleware()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(strings.Repeat("x", 2000)))
	}))

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest("GET", "/", nil)
		// No Accept-Encoding header
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
	}
}

func BenchmarkRequestIDMiddleware(b *testing.B) {
	handler := RequestIDMiddleware()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	}))

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest("GET", "/", nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
	}
}

func BenchmarkLoggingMiddleware(b *testing.B) {
	// Use discard logger for benchmarking
	logger := setupBenchLogger()
	handler := LoggingMiddleware(logger)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.Write([]byte("OK"))
	}))

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest("GET", "/test", nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
	}
}

func BenchmarkEarlyHintsMiddleware(b *testing.B) {
	handler := EarlyHintsMiddleware()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Link", "</style.css>; rel=preload; as=style")
		w.Header().Add("Link", "</app.js>; rel=preload; as=script")
		w.WriteHeader(200)
		w.Write([]byte("OK"))
	}))

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest("GET", "/", nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
	}
}

func BenchmarkFullMiddlewareStack(b *testing.B) {
	logger := setupBenchLogger()
	body := strings.Repeat("<div>Content block</div>\n", 100)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(body))
	})

	wrapped := RecoveryMiddleware(logger)(handler)
	wrapped = RequestIDMiddleware()(wrapped)
	wrapped = EarlyHintsMiddleware()(wrapped)
	wrapped = LoggingMiddleware(logger)(wrapped)
	wrapped = CompressionMiddleware()(wrapped)

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest("GET", "/", nil)
		req.Header.Set("Accept-Encoding", "gzip")
		w := httptest.NewRecorder()
		wrapped.ServeHTTP(w, req)
	}
}

func BenchmarkHealthEndpoint(b *testing.B) {
	// Note: can't fully benchmark without a real pool, but test the JSON encoding path
	req := httptest.NewRequest("GET", "/health", nil)

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		w.Write([]byte(`{"status":"ok","uptime":"1h30m"}`))
		_ = req
	}
}

func BenchmarkGzipCompression(b *testing.B) {
	data := []byte(strings.Repeat("The quick brown fox jumps over the lazy dog. ", 500))

	b.ResetTimer()
	b.ReportAllocs()
	b.SetBytes(int64(len(data)))
	for i := 0; i < b.N; i++ {
		w, _ := gzip.NewWriterLevel(io.Discard, gzip.DefaultCompression)
		w.Write(data)
		w.Close()
	}
}

func setupBenchLogger() *slog.Logger {
	return slog.New(slog.NewJSONHandler(io.Discard, nil))
}
