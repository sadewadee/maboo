package server

import (
	"encoding/json"
	"net/http"
	"runtime"
	"time"

	"github.com/sadewadee/maboo/internal/pool"
)

var startTime = time.Now()

// HealthHandler serves health check and readiness endpoints.
type HealthHandler struct {
	pool *pool.Pool
}

// NewHealthHandler creates a new health check handler.
func NewHealthHandler(p *pool.Pool) *HealthHandler {
	return &HealthHandler{pool: p}
}

func (h *HealthHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch r.URL.Path {
	case "/ready", "/readyz":
		h.readiness(w)
	default:
		h.liveness(w)
	}
}

func (h *HealthHandler) liveness(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status": "ok",
		"uptime": time.Since(startTime).String(),
	})
}

func (h *HealthHandler) readiness(w http.ResponseWriter) {
	stats := h.pool.Stats()

	ready := stats.IdleWorkers > 0
	status := http.StatusOK
	statusStr := "ready"
	if !ready {
		status = http.StatusServiceUnavailable
		statusStr = "not_ready"
	}

	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":         statusStr,
		"uptime":         time.Since(startTime).String(),
		"uptime_seconds": time.Since(startTime).Seconds(),
		"workers": map[string]interface{}{
			"total": stats.TotalWorkers,
			"busy":  stats.BusyWorkers,
			"idle":  stats.IdleWorkers,
		},
		"requests_total": stats.TotalRequests,
		"memory": map[string]interface{}{
			"alloc_mb":  mem.Alloc / 1024 / 1024,
			"sys_mb":    mem.Sys / 1024 / 1024,
			"gc_cycles": mem.NumGC,
		},
		"go_version": runtime.Version(),
		"goroutines": runtime.NumGoroutine(),
	})
}
