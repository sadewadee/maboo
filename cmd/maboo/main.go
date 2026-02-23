package main

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/sadewadee/maboo/internal/config"
	"github.com/sadewadee/maboo/internal/phpengine"
	"github.com/sadewadee/maboo/internal/server"
	"github.com/sadewadee/maboo/internal/worker"
)

var version = "0.2.0-dev"

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

	logger, startupCloser := setupLogger("info", "json", "stdout")
	if startupCloser != nil {
		defer startupCloser.Close()
	}
	logger.Info("maboo starting", "version", version)

	cfg, err := config.Load(cfgPath)
	if err != nil {
		logger.Error("failed to load config", "path", cfgPath, "error", err)
		os.Exit(1)
	}

	if startupCloser != nil {
		_ = startupCloser.Close()
		startupCloser = nil
	}
	logger, logCloser := setupLogger(cfg.Logging.Level, cfg.Logging.Format, cfg.Logging.Output)
	if logCloser != nil {
		defer logCloser.Close()
	}

	// Create embedded worker pool
	phpengine.SetLogger(logger)

	workerPool := worker.NewPool(cfg)
	workerPool.SetLogger(logger)

	if err := workerPool.Start(); err != nil {
		logger.Error("failed to start worker pool", "error", err)
		os.Exit(1)
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

func setupLogger(level, format, output string) (*slog.Logger, io.Closer) {
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

	writer, closer := resolveLogOutput(output)
	opts := &slog.HandlerOptions{Level: lvl}

	var handler slog.Handler
	if format == "text" {
		handler = slog.NewTextHandler(writer, opts)
	} else {
		handler = slog.NewJSONHandler(writer, opts)
	}

	return slog.New(handler), closer
}

func resolveLogOutput(output string) (io.Writer, io.Closer) {
	switch output {
	case "", "stdout":
		return os.Stdout, nil
	case "stderr":
		return os.Stderr, nil
	default:
		f, err := os.OpenFile(output, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
		if err != nil {
			return os.Stdout, nil
		}
		return f, f
	}
}

func printUsage() {
	fmt.Println(`maboo - Embedded PHP Application Server

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
  kill -USR1 $(pidof maboo)   # Reload workers

Embedded PHP Version: 7.4, 8.0, 8.1, 8.2, 8.3, 8.4`)
}
