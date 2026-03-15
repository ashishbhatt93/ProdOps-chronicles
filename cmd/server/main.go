package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"

	"github.com/prodops-chronicles/prodops/internal/api"
	"github.com/prodops-chronicles/prodops/internal/api/handler"
	appconfig "github.com/prodops-chronicles/prodops/internal/config"
	"github.com/prodops-chronicles/prodops/internal/content"
	"github.com/prodops-chronicles/prodops/internal/repository/postgres"
	"github.com/prodops-chronicles/prodops/internal/runtime/compose"
	"github.com/prodops-chronicles/prodops/internal/runtime/k3s"
	"github.com/prodops-chronicles/prodops/internal/service"
	"github.com/prodops-chronicles/prodops/internal/uow"
)

func main() {
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})))
	if err := run(); err != nil {
		slog.Error("fatal", "err", err)
		os.Exit(1)
	}
}

func run() error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// CONFIG_PATH env var sets the path; fallback to production default.
	// All paths (pgdata, modules, sync, compose file) derive from this file.
	// No path is hardcoded anywhere else in the codebase.
	configPath := envOr("CONFIG_PATH", "/opt/prodops/base_configs.yaml")

	cfg, err := appconfig.Load(configPath)
	if err != nil {
		return fmt.Errorf("load config from %s: %w", configPath, err)
	}

	slog.Info("config loaded",
		"base_path", cfg.Storage.BasePath,
		"runtime", cfg.Runtime,
		"difficulty", cfg.DifficultyLabel(),
		"enabled_modules", cfg.EnabledModuleIDs(),
	)

	// Database
	dbURL := cfg.DatabaseURL()
	pool, err := postgres.NewPool(ctx, dbURL)
	if err != nil {
		return fmt.Errorf("database connect: %w", err)
	}
	defer pool.Close()

	// Migrations
	m, err := migrate.New("file://migrations", dbURL)
	if err != nil {
		return fmt.Errorf("migrate init: %w", err)
	}
	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("migrate up: %w", err)
	}
	slog.Info("migrations applied")

	// Repositories
	playerRepo   := postgres.NewPlayerRepo(pool)
	runRepo      := postgres.NewRunRepo(pool)
	moduleRepo   := postgres.NewModuleRepo(pool)
	progressRepo := postgres.NewProgressRepo(pool)
	decisionRepo := postgres.NewDecisionRepo(pool)
	verifyRepo   := postgres.NewVerificationRepo(pool)
	scoreRepo    := postgres.NewScoreRepo(pool)

	// Unit of Work
	uowFactory := uow.NewFactory(pool)

	// Runtime adapter — selected from base_configs.yaml runtime: compose|k3s
	var runtimeAdapter interface {
		StartModule(context.Context, string) error
		StopModule(context.Context, string) error
		WriteModuleDefinition(context.Context, string) error
		RemoveModuleDefinition(context.Context, string) error
	}
	switch cfg.Runtime {
	case "k3s":
		runtimeAdapter = k3s.New()
		slog.Info("runtime: k3s")
	default:
		runtimeAdapter = compose.New(
			cfg.ComposeFile(),
			cfg.Storage.BasePath,
			envOr("IMAGE_PREFIX", "prodops"),
			envOr("IMAGE_TAG", cfg.Versions.Backend),
		)
		slog.Info("runtime: docker compose", "compose_file", cfg.ComposeFile())
	}

	// Services — cfg passed to services that need difficulty or module gating
	playerSvc   := service.NewPlayerService(playerRepo, progressRepo, runRepo)
	runSvc      := service.NewRunService(runRepo, playerRepo, scoreRepo, uowFactory)
	moduleSvc   := service.NewModuleService(moduleRepo, scoreRepo, runtimeAdapter, cfg)
	gameSvc     := service.NewGameService(progressRepo, decisionRepo, moduleRepo, scoreRepo, moduleSvc, uowFactory, cfg)
	verifySvc   := service.NewVerificationService(progressRepo, verifyRepo, scoreRepo, envOr("LEARNER_HOME", "/home/learner"), cfg)
	progressSvc := service.NewProgressService(progressRepo, decisionRepo, scoreRepo, moduleRepo)
	syncSvc     := service.NewSyncService(playerRepo, progressRepo, decisionRepo, scoreRepo, moduleRepo)
	yrSvc       := service.NewYearlyReviewService(scoreRepo, progressRepo, moduleRepo)

	contentCache := make(map[string]*content.ModuleContent)

	shutdownCtx, shutdownCancel := context.WithCancel(ctx)
	handlers := &api.Handlers{
		System:   handler.NewSystemHandler(shutdownCancel),
		Player:   handler.NewPlayerHandler(playerSvc),
		Runs:     handler.NewRunHandler(runSvc),
		Modules:  handler.NewModuleHandler(moduleSvc, runSvc),
		Gameplay: handler.NewGameplayHandler(gameSvc, verifySvc, runSvc, contentCache),
		Progress: handler.NewProgressHandler(progressSvc, runSvc, yrSvc),
		Config:   handler.NewConfigHandler(configPath),
		Sync:     handler.NewSyncHandler(syncSvc, runSvc, cfg.Storage.SyncPath),
		Internal: handler.NewInternalHandler(gameSvc, moduleRepo, contentCache),
	}

	port := envOr("PORT", "7741")
	srv := &http.Server{
		Addr:         ":" + port,
		Handler:      api.NewRouter(handlers),
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		slog.Info("backend listening", "port", port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	select {
	case sig := <-sigCh:
		slog.Info("signal received", "signal", sig)
	case <-shutdownCtx.Done():
		slog.Info("shutdown requested via API")
	case err := <-errCh:
		return err
	}

	stopCtx, stopCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer stopCancel()
	return srv.Shutdown(stopCtx)
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
