package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"

	"go.uber.org/zap"

	"github.com/eugegm01-dev/metrics/pkg/app"
	"github.com/eugegm01-dev/metrics/pkg/config"
	"github.com/eugegm01-dev/metrics/pkg/repository"
	"github.com/eugegm01-dev/metrics/pkg/server"
)

func main() {
	logger, err := zap.NewDevelopment()
	if err != nil {
		log.Fatalf("Failed to init logger: %v", err)
	}
	zap.ReplaceGlobals(logger)
	defer func() { _ = logger.Sync() }()

	cfg, err := config.ParseServerConfig()
	if err != nil {
		logger.Fatal("Failed to parse config", zap.Error(err))
	}

	db, err := app.InitDB(cfg, logger)
	if err != nil {
		logger.Fatal("DB init failed", zap.Error(err))
	}
	if db != nil && cfg.MigrationsDir != "" {
		if err := repository.RunMigrations(db, cfg.MigrationsDir); err != nil {
			logger.Fatal("Migrations failed", zap.Error(err))
		}
	}

	if db != nil {
		defer func() { _ = db.Close() }()
	}

	storage, err := app.InitStorage(cfg, db)
	if err != nil {
		logger.Fatal("Failed to create storage", zap.Error(err))
	}
	defer storage.Close()

	r := server.NewRouter(storage, logger)

	srv := &http.Server{
		Addr:         cfg.Addr,
		Handler:      r,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  30 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go func() {
		logger.Info("Starting HTTP server", zap.String("address", cfg.Addr))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal("Server failed to start", zap.Error(err))
		}
	}()

	<-ctx.Done()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error("Server shutdown error", zap.Error(err))
	}
	logger.Info("Server stopped")
}

