package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/maboo-dev/maboo/internal/config"
	"github.com/maboo-dev/maboo/internal/pool"
	"github.com/maboo-dev/maboo/internal/server"
)

var version = "0.1.0-dev"

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	switch os.Args[1] {
	case "serve", "start":
		serve()
	case "version":
		fmt.Printf("maboo v%s\n", version)
	case "help":
		printUsage()
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", os.Args[1])
		printUsage()
		os.Exit(1)
	}
}

func serve() {
	cfgPath := "maboo.yaml"
	if len(os.Args) > 2 {
		cfgPath = os.Args[2]
	}

	logger := setupLogger("info", "json")
	logger.Info("maboo starting", "version", version)

	cfg, err := config.Load(cfgPath)
	if err != nil {
		logger.Error("failed to load config", "path", cfgPath, "error", err)
		os.Exit(1)
	}

	logger = setupLogger(cfg.Logging.Level, cfg.Logging.Format)

	// Create worker pool
	workerPool := pool.New(cfg.Pool, cfg.PHP, logger)
	if err := workerPool.Start(); err != nil {
		logger.Error("failed to start worker pool", "error", err)
		os.Exit(1)
	}

	// Set up file watcher for development
	if cfg.Watch.Enabled && len(cfg.Watch.Dirs) > 0 {
		watcher := pool.NewWatcher(
			cfg.Watch.Dirs,
			cfg.Watch.Interval.Duration(),
			logger,
			func() { workerPool.Reload() },
		)
		watcher.Start()
		defer watcher.Stop()
	}

	// Create HTTP server
	srv := server.New(cfg, workerPool, logger)

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	// Handle SIGUSR1 for graceful reload
	reload := make(chan os.Signal, 1)
	signal.Notify(reload, syscall.SIGUSR1)
	go func() {
		for range reload {
			logger.Info("SIGUSR1 received, reloading workers")
			if err := workerPool.Reload(); err != nil {
				logger.Error("reload failed", "error", err)
			}
		}
	}()

	// Start server
	go func() {
		if err := srv.Start(); err != nil {
			logger.Error("server error", "error", err)
			quit <- syscall.SIGTERM
		}
	}()

	logger.Info("maboo ready", "address", cfg.Server.Address)

	<-quit
	logger.Info("shutdown signal received")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := srv.Stop(ctx); err != nil {
		logger.Error("server shutdown error", "error", err)
	}

	if err := workerPool.Stop(); err != nil {
		logger.Error("pool shutdown error", "error", err)
	}

	logger.Info("maboo stopped")
}

func setupLogger(level, format string) *slog.Logger {
	var lvl slog.Level
	switch level {
	case "debug":
		lvl = slog.LevelDebug
	case "warn":
		lvl = slog.LevelWarn
	case "error":
		lvl = slog.LevelError
	default:
		lvl = slog.LevelInfo
	}

	opts := &slog.HandlerOptions{Level: lvl}

	var handler slog.Handler
	if format == "text" {
		handler = slog.NewTextHandler(os.Stdout, opts)
	} else {
		handler = slog.NewJSONHandler(os.Stdout, opts)
	}

	return slog.New(handler)
}

func printUsage() {
	fmt.Println(`maboo - PHP Application Server

Usage:
  maboo <command> [options]

Commands:
  serve [config]   Start the server (default config: maboo.yaml)
  start [config]   Alias for serve
  version          Show version
  help             Show this help

Signals:
  SIGUSR1          Graceful worker reload (zero-downtime)
  SIGINT/SIGTERM   Graceful shutdown

Examples:
  maboo serve
  maboo serve /etc/maboo/maboo.yaml
  maboo version
  kill -USR1 $(pidof maboo)   # Reload workers`)
}
