package server

import (
	"net/http"
	"os"
	"path/filepath"
)

// StaticHandler wraps http.FileServer with additional features.
type StaticHandler struct {
	root         string
	cacheControl string
	fileServer   http.Handler
}

// NewStaticHandler creates a new static file handler.
func NewStaticHandler(root, cacheControl string) *StaticHandler {
	return &StaticHandler{
		root:         root,
		cacheControl: cacheControl,
		fileServer:   http.FileServer(http.Dir(root)),
	}
}

func (h *StaticHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Check if file exists
	path := filepath.Join(h.root, filepath.Clean(r.URL.Path))
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		http.NotFound(w, r)
		return
	}

	if h.cacheControl != "" {
		w.Header().Set("Cache-Control", h.cacheControl)
	}

	h.fileServer.ServeHTTP(w, r)
}
