package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	_ "github.com/lib/pq"

	"github.com/go-chi/chi/v5"
	chiMiddleware "github.com/go-chi/chi/v5/middleware"
	"go.uber.org/zap"

	"github.com/eugegm01-dev/metrics/internal/config"
	"github.com/eugegm01-dev/metrics/internal/handler"
	"github.com/eugegm01-dev/metrics/internal/middleware"
	"github.com/eugegm01-dev/metrics/internal/repository"
)

func main() {
	// Инициализация логгера ДО всего остального
	logger, err := zap.NewDevelopment()
	if err != nil {
		log.Fatalf("Failed to initialize zap logger: %v", err)
	}
	defer logger.Sync()

	cfg, err := config.ParseServerConfig()
	if err != nil {
		logger.Fatal("Failed to parse config", zap.Error(err))
	}

	// Логируем конфигурацию сервера
	logger.Info("Server configuration",
		zap.String("address", cfg.Addr),
		zap.Duration("store_interval", cfg.StoreInterval),
		zap.String("file_storage_path", cfg.FileStoragePath),
		zap.Bool("restore", cfg.Restore),
		zap.String("database_dsn", cfg.DatabaseDSN),
	)

	var db *sql.DB
	var storage repository.Storage

	// Приоритет 1: PostgreSQL (если указан DSN)
	if cfg.DatabaseDSN != "" {
		var dbErr error
		db, dbErr = sql.Open("postgres", cfg.DatabaseDSN)
		if dbErr != nil {
			logger.Error("Failed to open database connection, falling back to file/memory storage",
				zap.Error(dbErr))
			db = nil
		} else {
			// Проверяем подключение
			if pingErr := db.Ping(); pingErr != nil {
				logger.Error("Database ping failed, falling back to file/memory storage",
					zap.Error(pingErr))
				db.Close()
				db = nil
			} else {
				logger.Info("Connected to PostgreSQL successfully")
			}
		}
	}

	// Создаём хранилище с учётом приоритетов из ТЗ:
	// 1. PostgreSQL (если db != nil)
	// 2. Файловое хранилище (если указан filePath)
	// 3. In-memory хранилище (во всех остальных случаях)
	storage, err = repository.NewStorage(cfg.FileStoragePath, cfg.StoreInterval, cfg.Restore, db)
	if err != nil {
		logger.Fatal("Failed to create storage", zap.Error(err))
	}
	defer storage.Close()

	// Настраиваем роутер
	r := chi.NewRouter()

	// Middleware
	r.Use(chiMiddleware.Recoverer)
	r.Use(middleware.GzipMiddleware)
	r.Use(middleware.LoggingMiddleware(logger))

	// Роуты
	r.Get("/ping", func(w http.ResponseWriter, r *http.Request) {
		if db != nil {
			if err := db.Ping(); err != nil {
				logger.Error("Database ping failed", zap.Error(err))
				http.Error(w, "database unavailable", http.StatusInternalServerError)
				return
			}
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("pong"))
	})

	r.Get("/", handler.IndexHTMLHandler(storage))
	r.Post("/update/{type}/{name}/{value}", handler.UpdateHandler(storage))
	r.Get("/value/{type}/{name}", handler.GetMetricHandler(storage))

	r.Post("/update", handler.UpdateJSONHandler(storage))
	r.Post("/update/", handler.UpdateJSONHandler(storage))

	r.Post("/value", handler.ValueJSONHandler(storage))
	r.Post("/value/", handler.ValueJSONHandler(storage))

	// Принудительное сохранение метрик в файл (удобно для тестов и отладки)
	r.Post("/save", func(w http.ResponseWriter, r *http.Request) {
		if err := storage.SaveToFile(); err != nil {
			logger.Error("Failed to force save metrics to file",
				zap.Error(err),
				zap.String("file_path", cfg.FileStoragePath),
			)
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("Metrics saved successfully\n"))
	})

	// Настраиваем HTTP-сервер
	srv := &http.Server{
		Addr:         cfg.Addr,
		Handler:      r,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  30 * time.Second,
	}

	// Graceful shutdown
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go func() {
		logger.Info("Starting HTTP server", zap.String("address", cfg.Addr))

		// Выводим все маршруты (только для отладки/разработки)
		walkFunc := func(method string, route string, handler http.Handler, middlewares ...func(http.Handler) http.Handler) error {
			fmt.Printf("%-8s %s\n", method, route)
			return nil
		}
		if err := chi.Walk(r, walkFunc); err != nil {
			logger.Warn("Failed to walk routes", zap.Error(err))
		}

		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal("Server failed to start", zap.Error(err))
		}
	}()

	// Ожидание сигнала завершения
	<-ctx.Done()
	logger.Info("Received shutdown signal, gracefully shutting down...")

	// Даём время на завершение текущих запросов
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error("Server shutdown error", zap.Error(err))
	}

	logger.Info("Server stopped")
}
