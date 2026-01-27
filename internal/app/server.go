package app

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/go-chi/chi/v5"
	chiMiddleware "github.com/go-chi/chi/v5/middleware"
	"go.uber.org/zap"

	"github.com/eugegm01-dev/metrics/internal/config"
	"github.com/eugegm01-dev/metrics/internal/handler"
	"github.com/eugegm01-dev/metrics/internal/middleware"
	"github.com/eugegm01-dev/metrics/internal/repository"
)

func RunServer() error {
	logger, err := zap.NewDevelopment()
	if err != nil {
		return fmt.Errorf("failed to initialize logger: %w", err)
	}
	defer logger.Sync()

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
	)

	// Инициализация БД
	var db *sql.DB
	if cfg.DatabaseDSN != "" {
		db, err = initDB(cfg, logger)
		if err != nil {
			return err // fail early
		}
		defer db.Close()
	}

	// Инициализация хранилища
	storage, err := initStorage(cfg, db, logger)
	if err != nil {
		return fmt.Errorf("failed to create storage: %w", err)
	}
	defer storage.Close()

	// Инициализация роутера
	r := initRouter(storage, logger)

	// Настройка сервера
	srv := &http.Server{
		Addr:         cfg.Addr,
		Handler:      r,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  30 * time.Second,
	}

	return runServer(srv, logger)
}

func initDB(cfg *config.ServerConfig, logger *zap.Logger) (*sql.DB, error) {
	dsn := cfg.DatabaseDSN
	if dsn == "" {
		dsn = "postgres://postgres:password@localhost:5432/metrics?sslmode=disable"
	}

	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Проверяем подключение с retry
	const maxAttempts = 3
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		err = db.PingContext(ctx)
		cancel()

		if err == nil {
			return db, nil
		}

		logger.Warn("Database ping attempt failed",
			zap.Int("attempt", attempt),
			zap.Error(err),
		)

		if attempt < maxAttempts {
			time.Sleep(time.Duration(attempt) * time.Second)
		}
	}

	db.Close()
	return nil, fmt.Errorf("failed to connect to database after %d attempts: %w", maxAttempts, err)
}

func initStorage(cfg *config.ServerConfig, db *sql.DB, logger *zap.Logger) (repository.Storage, error) {
	logger.Debug("Initializing storage",
		zap.String("file_path", cfg.FileStoragePath),
		zap.Duration("store_interval", cfg.StoreInterval),
		zap.Bool("restore", cfg.Restore),
		zap.Bool("has_db", db != nil))

	return repository.NewStorage(cfg.FileStoragePath, cfg.StoreInterval, cfg.Restore, db)
}

func initRouter(storage repository.Storage, logger *zap.Logger) *chi.Mux {
	r := chi.NewRouter()

	r.Use(chiMiddleware.Recoverer)
	r.Use(middleware.GzipMiddleware)
	r.Use(middleware.LoggingMiddleware(logger))

	r.Get("/ping", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("pong"))
	})

	r.Get("/", handler.IndexHTMLHandler(storage))
	r.Post("/update/{type}/{name}/{value}", handler.UpdateHandler(storage))
	r.Get("/value/{type}/{name}", handler.GetMetricHandler(storage))
	r.Post("/update", handler.UpdateJSONHandler(storage))
	r.Post("/update/", handler.UpdateJSONHandler(storage))
	r.Post("/value", handler.ValueJSONHandler(storage))
	r.Post("/value/", handler.ValueJSONHandler(storage))
	r.Post("/updates", handler.UpdatesHandler(storage))
	r.Post("/updates/", handler.UpdatesHandler(storage))

	// Endpoint для сохранения в файл (только для FileStorage)
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

		// Логируем маршруты через логгер
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
