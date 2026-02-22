package pool

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Watcher monitors PHP files for changes and triggers pool reload.
type Watcher struct {
	dirs     []string
	exts     []string
	interval time.Duration
	logger   *slog.Logger
	onChange func()
	ctx      context.Context
	cancel   context.CancelFunc
	mtimes   map[string]time.Time
}

// NewWatcher creates a file watcher for the given directories.
func NewWatcher(dirs []string, interval time.Duration, logger *slog.Logger, onChange func()) *Watcher {
	ctx, cancel := context.WithCancel(context.Background())
	return &Watcher{
		dirs:     dirs,
		exts:     []string{".php", ".inc", ".phtml"},
		interval: interval,
		logger:   logger,
		onChange: onChange,
		ctx:      ctx,
		cancel:   cancel,
		mtimes:   make(map[string]time.Time),
	}
}

// Start begins watching for file changes.
func (w *Watcher) Start() {
	w.scan()

	go func() {
		ticker := time.NewTicker(w.interval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				if w.detectChanges() {
					w.logger.Info("file changes detected, reloading workers")
					w.onChange()
				}
			case <-w.ctx.Done():
				return
			}
		}
	}()

	w.logger.Info("file watcher started", "dirs", w.dirs, "interval", w.interval)
}

// Stop stops the file watcher.
func (w *Watcher) Stop() {
	w.cancel()
}

func (w *Watcher) scan() {
	for _, dir := range w.dirs {
		filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return nil
			}
			if info.IsDir() {
				name := info.Name()
				if name == "vendor" || name == "node_modules" || name == ".git" {
					return filepath.SkipDir
				}
				return nil
			}
			if w.isWatchedFile(path) {
				w.mtimes[path] = info.ModTime()
			}
			return nil
		})
	}
}

func (w *Watcher) detectChanges() bool {
	changed := false
	currentFiles := make(map[string]time.Time)

	for _, dir := range w.dirs {
		filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return nil
			}
			if info.IsDir() {
				name := info.Name()
				if name == "vendor" || name == "node_modules" || name == ".git" {
					return filepath.SkipDir
				}
				return nil
			}
			if w.isWatchedFile(path) {
				currentFiles[path] = info.ModTime()
				if oldTime, exists := w.mtimes[path]; exists {
					if info.ModTime().After(oldTime) {
						w.logger.Debug("file changed", "path", path)
						changed = true
					}
				} else {
					w.logger.Debug("new file detected", "path", path)
					changed = true
				}
			}
			return nil
		})
	}

	for path := range w.mtimes {
		if _, exists := currentFiles[path]; !exists {
			w.logger.Debug("file deleted", "path", path)
			changed = true
		}
	}

	w.mtimes = currentFiles
	return changed
}

func (w *Watcher) isWatchedFile(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	for _, e := range w.exts {
		if ext == e {
			return true
		}
	}
	return false
}
