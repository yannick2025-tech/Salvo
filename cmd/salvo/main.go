package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/yannick2025-tech/Salvo/internal/api"
	"github.com/yannick2025-tech/Salvo/internal/auth"
	"github.com/yannick2025-tech/Salvo/internal/config"
	"github.com/yannick2025-tech/Salvo/internal/logger"
	"github.com/yannick2025-tech/Salvo/internal/mock"
	"github.com/yannick2025-tech/Salvo/internal/store/migration"
	"github.com/yannick2025-tech/Salvo/internal/store/sqlite"
)

func main() {
	configPath := flag.String("config", "configs/salvo.yaml", "path to configuration file")
	showVersion := flag.Bool("version", false, "print version and exit")
	flag.Parse()

	if *showVersion {
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
		MaxSize:    cfg.Log.MaxSize,
		MaxBackups: cfg.Log.MaxBackups,
		MaxAge:     cfg.Log.MaxAge,
		TimeFormat: cfg.Log.TimeFormat,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to initialize logger: %v\n", err)
		os.Exit(1)
	}
	defer func() { _ = log.Sync() }()

	db, err := sqlite.Open(cfg.Database.DSN, 1)
	if err != nil {
		log.Fatal("failed to open database", logger.F("dsn", cfg.Database.DSN), logger.F("error", err))
	}
	defer func() { _ = db.Close() }()

	if err := migration.Migrate(db.DB); err != nil {
		log.Fatal("failed to run migrations", logger.F("error", err))
	}

	jwtManager := auth.NewJWTManager(cfg.Auth.JWTSecret, 24*time.Hour)

	users := sqlite.NewUserRepo(db)
	roles := sqlite.NewRoleRepo(db)
	perms := sqlite.NewPermissionRepo(db)
	rp := sqlite.NewRolePermissionRepo(db)

	rbacChecker := auth.NewRBACChecker(perms, rp)

	seedCfg := auth.DefaultSeedConfig()
	seed := auth.NewSeeders(users, roles, perms, rp, seedCfg)
	if err := seed.Seed(context.Background()); err != nil {
		log.Fatal("failed to seed data", logger.F("error", err))
	}

	srv := api.New(api.Config{
		Addr:      cfg.ServerAddr(),
		DB:        db,
		Logger:    log,
		JWT:       jwtManager,
		RBAC:      rbacChecker,
		WebDir:    cfg.Server.WebDir,
		Variables: cfg.Variables,
	})

	go func() {
		if err := srv.Start(); err != nil {
			log.Fatal("api server error", logger.F("error", err))
		}
	}()

	var mockSrv *mock.MockServer
	if cfg.Mock.Enabled {
		mockSrv = mock.NewMockServer(cfg.Mock.Port)
		if err := mockSrv.Start(); err != nil {
			log.Error("mock server start error", logger.F("error", err))
		} else {
			log.Info("mock HTTP server started", logger.F("port", cfg.Mock.Port))
		}
	}

	log.Info("salvo started",
		logger.F("addr", cfg.ServerAddr()),
		logger.F("pool_workers", cfg.Pool.WorkerCount),
		logger.F("run_mode", string(cfg.Pool.RunMode)),
	)

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	sig := <-sigCh
	log.Info("received shutdown signal", logger.F("signal", sig.String()))

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Error("server shutdown error", logger.F("error", err))
	}

	if mockSrv != nil {
		_ = mockSrv.Stop()
		log.Info("mock server stopped")
	}

	log.Info("salvo stopped")
}
