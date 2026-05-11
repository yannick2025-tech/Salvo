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
	configPath := flag.String("config", "configs/salvo.yaml", "path to config file")
	flag.Parse()

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
		Compress:   cfg.Log.Compress,
		TimeFormat: cfg.Log.TimeFormat,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to create logger: %v\n", err)
		os.Exit(1)
	}

	db, err := sqlite.Open(cfg.Database.DSN, 1)
	if err != nil {
		log.Error("failed to open database", logger.F("error", err))
		os.Exit(1)
	}
	defer db.Close()

	if err := migration.Migrate(db.DB); err != nil {
		log.Error("failed to run migrations", logger.F("error", err))
		os.Exit(1)
	}

	jwtMgr := auth.NewJWTManager(cfg.Auth.JWTSecret, 24*time.Hour)
	rbacChecker := auth.NewRBACChecker(
		sqlite.NewPermissionRepo(db),
		sqlite.NewRolePermissionRepo(db),
	)

	seeders := auth.NewSeeders(
		sqlite.NewUserRepo(db),
		sqlite.NewRoleRepo(db),
		sqlite.NewPermissionRepo(db),
		sqlite.NewRolePermissionRepo(db),
		auth.DefaultSeedConfig(),
	)
	if err := seeders.Seed(context.Background()); err != nil {
		log.Error("failed to seed initial data", logger.F("error", err))
		os.Exit(1)
	}

	srv := api.New(api.Config{
		Addr:      cfg.ServerAddr(),
		DB:        db,
		Logger:    log,
		JWT:       jwtMgr,
		RBAC:      rbacChecker,
		WebDir:    cfg.Server.WebDir,
		Variables: cfg.Variables,
	})

	var mockSrv *mock.MockServer
	if cfg.Mock.Enabled {
		mockSrv = mock.NewMockServer(cfg.Mock.Port)
		go func() {
			if err := mockSrv.Start(); err != nil {
				log.Error("mock server stopped", logger.F("error", err))
			}
		}()
		log.Info("mock server started", logger.F("port", cfg.Mock.Port))
	}

	go func() {
		if err := srv.Start(); err != nil {
			log.Error("api server stopped", logger.F("error", err))
		}
	}()

	log.Info("salvo started",
		logger.F("addr", cfg.ServerAddr()),
		logger.F("config", *configPath),
	)

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Info("shutting down...")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	srv.Shutdown(ctx)

	if mockSrv != nil {
		mockSrv.Stop()
	}

	log.Info("salvo stopped")
}
