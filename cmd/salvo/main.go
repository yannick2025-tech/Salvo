// Package main is the entry point for the Salvo performance testing engine.
//
// Usage:
//
//	salvo -config configs/salvo.yaml
//	salvo -version
package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/yannick2025-tech/Salvo/internal/config"
	"github.com/yannick2025-tech/Salvo/internal/logger"
)

func main() {
	configPath := flag.String("config", "configs/salvo.yaml", "path to configuration file")
	version := flag.Bool("version", false, "print version and exit")
	flag.Parse()

	if *version {
		fmt.Println("Salvo v0.1.0")
		os.Exit(0)
	}

	cfg, err := config.Load(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to load config: %v\n", err)
		os.Exit(1)
	}

	log, err := logger.New(logger.Config{
		Level:      cfg.Log.Level,
		Format:     cfg.Log.Format,
		Output:     cfg.Log.Output,
		TimeFormat: cfg.Log.TimeFormat,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to initialize logger: %v\n", err)
		os.Exit(1)
	}
	defer func() { _ = log.Sync() }()

	log.Info("salvo starting",
		logger.F("addr", cfg.ServerAddr()),
		logger.F("pool_workers", cfg.Pool.WorkerCount),
		logger.F("run_mode", string(cfg.Pool.RunMode)),
	)

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	sig := <-sigCh
	log.Info("received shutdown signal", logger.F("signal", sig.String()))
	log.Info("salvo stopped")
}
