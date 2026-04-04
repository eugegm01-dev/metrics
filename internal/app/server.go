// Package app contains the main application logic for the agent and server.
package app

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"net/http/pprof"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/avast/retry-go/v4"
	"github.com/go-chi/chi/v5"
	chiMiddleware "github.com/go-chi/chi/v5/middleware"
	_ "github.com/jackc/pgx/v5/stdlib"
	"go.uber.org/zap"

	"github.com/eugegm01-dev/metrics/internal/audit"
	"github.com/eugegm01-dev/metrics/internal/config"
	"github.com/eugegm01-dev/metrics/internal/handler"
	"github.com/eugegm01-dev/metrics/internal/middleware"
	"github.com/eugegm01-dev/metrics/internal/repository"
)

// RunServer запускает HTTP-сервер с конфигурацией из флагов и переменных окружения.
// Инициализирует хранилище, базу данных (если настроена) и регистрирует все маршруты.
func RunServer() error {
	logger, err := zap.NewDevelopment()
	if err != nil {
		return fmt.Errorf("failed to initialize logger: %w", err)
	}
	defer func() { _ = logger.Sync() }()

	cfg, err := config.ParseServerConfig()
	if err != nil {
		return fmt.Errorf("failed to parse config: %w", err)
	}

	logger.Info("Server configuration",
		zap.String("address", cfg.Addr),
		zap.Duration("store_interval", cfg.StoreInterval),
		zap.String("file_storage_path", cfg.FileStoragePath),
		zap.Bool("restore", cfg.Restore),
		zap.String("database_dsn", cfg.DatabaseDSN),
		zap.String("key", cfg.Key),
	)

	// 1️⃣ Инициализация БД
	var db *sql.DB
	if cfg.DatabaseDSN != "" {
		db, err = initDB(cfg, logger)
		if err != nil {
			return fmt.Errorf("failed to initialize database: %w", err)
		}
		defer db.Close()
	}

	// 2️⃣ Инициализация хранилища
	storage, err := initStorage(cfg, db, logger)
	if err != nil {
		return fmt.Errorf("failed to create storage: %w", err)
	}
	defer storage.Close()

	// 3️⃣ Инициализация аудита
	var auditSubject *audit.Subject
	if cfg.AuditFile != "" || cfg.AuditURL != "" {
		auditSubject = audit.NewSubject()
		if cfg.AuditFile != "" {
			fileObs, err := audit.NewFileObserver(cfg.AuditFile)
			if err != nil {
				logger.Error("Failed to create file audit observer", zap.Error(err))
			} else {
				auditSubject.Attach(fileObs)
			}
		}
		if cfg.AuditURL != "" {
			httpObs := audit.NewHTTPObserver(cfg.AuditURL)
			auditSubject.Attach(httpObs)
		}
	}

	// 4️⃣ Роутер
	r := initRouter(storage, logger, cfg.Key, auditSubject)

	// 5️⃣ Сервер
	srv := &http.Server{
		Addr:         cfg.Addr,
		Handler:      r,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  30 * time.Second,
	}

	// 6️⃣ Запуск
	err = runServer(srv, logger)

	// 7️⃣ Закрытие аудита ПОСЛЕ остановки сервера
	if auditSubject != nil {
		_ = auditSubject.Close()
	}
	return err
}

func initDB(cfg *config.ServerConfig, logger *zap.Logger) (*sql.DB, error) {
	dsn := strings.Trim(cfg.DatabaseDSN, "'\"`")
	if dsn == "" {
		return nil, nil
	}
	if !strings.Contains(dsn, "://") {
		dsn = "postgres://postgres@" + dsn
	}

	logger.Info("Connecting to database", zap.String("dsn", dsn))

	var db *sql.DB
	var err error

	// Используем retry-go
	err = retry.Do(
		func() error {
			db, err = sql.Open("pgx", dsn)
			if err != nil {
				return err
			}
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			return db.PingContext(ctx)
		},
		retry.Attempts(3),
		retry.DelayType(retry.BackOffDelay),
		retry.Delay(1*time.Second),
		retry.OnRetry(func(n uint, err error) {
			logger.Warn("Database connection attempt failed",
				zap.Uint("attempt", n+1),
				zap.Error(err))
		}),
	)

	if err != nil {
		if db != nil {
			db.Close()
		}
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	logger.Info("Database connected successfully")
	return db, nil
}

func initStorage(cfg *config.ServerConfig, db *sql.DB, logger *zap.Logger) (repository.Storage, error) {
	logger.Debug("Initializing storage",
		zap.String("file_path", cfg.FileStoragePath),
		zap.Duration("store_interval", cfg.StoreInterval),
		zap.Bool("restore", cfg.Restore),
		zap.Bool("has_db", db != nil))
	return repository.NewStorage(cfg.FileStoragePath, cfg.StoreInterval, cfg.Restore, db)
}

func initRouter(storage repository.Storage, logger *zap.Logger, key string, auditSubject *audit.Subject) *chi.Mux {
	r := chi.NewRouter()
	r.Use(chiMiddleware.Recoverer)
	r.Use(middleware.GzipMiddleware)
	r.Use(middleware.LoggingMiddleware(logger))
	r.Use(middleware.HashMiddleware(key))

	// pprof endpoints
	r.HandleFunc("/debug/pprof/", pprof.Index)
	r.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
	r.HandleFunc("/debug/pprof/profile", pprof.Profile)
	r.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	r.HandleFunc("/debug/pprof/trace", pprof.Trace)
	r.Mount("/debug/pprof", http.HandlerFunc(pprof.Index))

	r.Get("/ping", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("pong"))
	})

	r.Get("/", handler.IndexHTMLHandler(storage, key))
	r.Post("/update/{type}/{name}/{value}", handler.UpdateHandler(storage, key))
	r.Get("/value/{type}/{name}", handler.GetMetricHandler(storage, key))
	r.Post("/update", handler.UpdateJSONHandler(storage, key))
	r.Post("/update/", handler.UpdateJSONHandler(storage, key))
	r.Post("/value", handler.ValueJSONHandler(storage, key))
	r.Post("/value/", handler.ValueJSONHandler(storage, key))

	// ✅ Только ОДНА регистрация /updates с auditSubject
	r.Post("/updates", handler.UpdatesHandler(storage, key, auditSubject))
	r.Post("/updates/", handler.UpdatesHandler(storage, key, auditSubject))

	r.Post("/save", func(w http.ResponseWriter, r *http.Request) {
		if fs, ok := storage.(interface{ SaveToFile() error }); ok {
			if err := fs.SaveToFile(); err != nil {
				logger.Error("Failed to force save metrics to file", zap.Error(err))
				http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
				return
			}
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("Metrics saved successfully\n"))
		} else {
			http.Error(w, "Storage does not support file saving", http.StatusBadRequest)
		}
	})

	return r
}

func runServer(srv *http.Server, logger *zap.Logger) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go func() {
		logger.Info("Starting HTTP server", zap.String("address", srv.Addr))
		if err := chi.Walk(srv.Handler.(*chi.Mux), func(method, route string, handler http.Handler, middlewares ...func(http.Handler) http.Handler) error {
			logger.Debug("Registered route", zap.String("method", method), zap.String("route", route))
			return nil
		}); err != nil {
			logger.Warn("Failed to walk routes", zap.Error(err))
		}
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal("Server failed to start", zap.Error(err))
		}
	}()

	<-ctx.Done()
	logger.Info("Received shutdown signal, gracefully shutting down...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("server shutdown error: %w", err)
	}

	logger.Info("Server stopped")
	return nil
}
