package server

import (
	"fmt"
	"net/http"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/sadewadee/maboo/internal/pool"
)

// Metrics collects Prometheus-compatible metrics.
type Metrics struct {
	totalRequests  sync.Map // "method:status" -> *atomic.Int64
	activeRequests atomic.Int32
	totalBytes     atomic.Int64

	durationBuckets []float64
	durationCounts  sync.Map // bucket key -> *atomic.Int64
	durationSum     atomic.Int64
	durationCount   atomic.Int64

	pool *pool.Pool
}

// NewMetrics creates a new metrics collector.
func NewMetrics(p *pool.Pool) *Metrics {
	return &Metrics{
		pool:            p,
		durationBuckets: []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1.0, 2.5, 5.0, 10.0},
	}
}

// Middleware returns a middleware that collects metrics and serves the metrics endpoint.
func (m *Metrics) Middleware(metricsPath string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == metricsPath {
				m.serveMetrics(w)
				return
			}

			start := time.Now()
			m.activeRequests.Add(1)
			defer m.activeRequests.Add(-1)

			rw := &metricsResponseWriter{ResponseWriter: w, statusCode: 200}
			next.ServeHTTP(rw, r)

			duration := time.Since(start)

			key := fmt.Sprintf("%s:%d", r.Method, rw.statusCode)
			counter, _ := m.totalRequests.LoadOrStore(key, &atomic.Int64{})
			counter.(*atomic.Int64).Add(1)

			m.totalBytes.Add(int64(rw.bytesWritten))

			m.durationSum.Add(int64(duration))
			m.durationCount.Add(1)
			durationSec := duration.Seconds()
			for _, bucket := range m.durationBuckets {
				if durationSec <= bucket {
					bkey := fmt.Sprintf("%.3f", bucket)
					bc, _ := m.durationCounts.LoadOrStore(bkey, &atomic.Int64{})
					bc.(*atomic.Int64).Add(1)
				}
			}
		})
	}
}

func (m *Metrics) serveMetrics(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")

	var b strings.Builder

	b.WriteString("# HELP maboo_http_requests_total Total number of HTTP requests.\n")
	b.WriteString("# TYPE maboo_http_requests_total counter\n")
	m.totalRequests.Range(func(key, value interface{}) bool {
		parts := strings.SplitN(key.(string), ":", 2)
		method, status := parts[0], parts[1]
		count := value.(*atomic.Int64).Load()
		fmt.Fprintf(&b, "maboo_http_requests_total{method=\"%s\",status=\"%s\"} %d\n", method, status, count)
		return true
	})

	b.WriteString("# HELP maboo_http_requests_active Current number of active HTTP requests.\n")
	b.WriteString("# TYPE maboo_http_requests_active gauge\n")
	fmt.Fprintf(&b, "maboo_http_requests_active %d\n", m.activeRequests.Load())

	b.WriteString("# HELP maboo_http_response_bytes_total Total bytes sent in HTTP responses.\n")
	b.WriteString("# TYPE maboo_http_response_bytes_total counter\n")
	fmt.Fprintf(&b, "maboo_http_response_bytes_total %d\n", m.totalBytes.Load())

	b.WriteString("# HELP maboo_http_request_duration_seconds HTTP request duration in seconds.\n")
	b.WriteString("# TYPE maboo_http_request_duration_seconds histogram\n")
	cumulative := int64(0)
	totalCount := m.durationCount.Load()
	for _, bucket := range m.durationBuckets {
		bkey := fmt.Sprintf("%.3f", bucket)
		if bc, ok := m.durationCounts.Load(bkey); ok {
			cumulative += bc.(*atomic.Int64).Load()
		}
		fmt.Fprintf(&b, "maboo_http_request_duration_seconds_bucket{le=\"%.3f\"} %d\n", bucket, cumulative)
	}
	fmt.Fprintf(&b, "maboo_http_request_duration_seconds_bucket{le=\"+Inf\"} %d\n", totalCount)
	fmt.Fprintf(&b, "maboo_http_request_duration_seconds_sum %.6f\n", float64(m.durationSum.Load())/float64(time.Second))
	fmt.Fprintf(&b, "maboo_http_request_duration_seconds_count %d\n", totalCount)

	if m.pool != nil {
		stats := m.pool.Stats()
		b.WriteString("# HELP maboo_workers_total Total number of PHP workers.\n")
		b.WriteString("# TYPE maboo_workers_total gauge\n")
		fmt.Fprintf(&b, "maboo_workers_total %d\n", stats.TotalWorkers)

		b.WriteString("# HELP maboo_workers_busy Number of busy PHP workers.\n")
		b.WriteString("# TYPE maboo_workers_busy gauge\n")
		fmt.Fprintf(&b, "maboo_workers_busy %d\n", stats.BusyWorkers)

		b.WriteString("# HELP maboo_workers_idle Number of idle PHP workers.\n")
		b.WriteString("# TYPE maboo_workers_idle gauge\n")
		fmt.Fprintf(&b, "maboo_workers_idle %d\n", stats.IdleWorkers)

		b.WriteString("# HELP maboo_pool_requests_total Total requests processed by worker pool.\n")
		b.WriteString("# TYPE maboo_pool_requests_total counter\n")
		fmt.Fprintf(&b, "maboo_pool_requests_total %d\n", stats.TotalRequests)
	}

	b.WriteString("# HELP maboo_go_goroutines Number of goroutines.\n")
	b.WriteString("# TYPE maboo_go_goroutines gauge\n")
	fmt.Fprintf(&b, "maboo_go_goroutines %d\n", runtime.NumGoroutine())

	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)
	b.WriteString("# HELP maboo_go_memstats_alloc_bytes Number of bytes allocated.\n")
	b.WriteString("# TYPE maboo_go_memstats_alloc_bytes gauge\n")
	fmt.Fprintf(&b, "maboo_go_memstats_alloc_bytes %d\n", mem.Alloc)

	w.Write([]byte(b.String()))
}

type metricsResponseWriter struct {
	http.ResponseWriter
	statusCode   int
	bytesWritten int
}

func (rw *metricsResponseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

func (rw *metricsResponseWriter) Write(b []byte) (int, error) {
	n, err := rw.ResponseWriter.Write(b)
	rw.bytesWritten += n
	return n, err
}
