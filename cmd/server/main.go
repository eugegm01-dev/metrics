package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/eugegm01-dev/metrics/internal/config"
	"github.com/eugegm01-dev/metrics/internal/handler"
	"github.com/eugegm01-dev/metrics/internal/middleware"
	"github.com/eugegm01-dev/metrics/internal/repository"
	"github.com/go-chi/chi/v5"
	chiMiddleware "github.com/go-chi/chi/v5/middleware"
	"go.uber.org/zap"
)

func main() {
	// Создаем логгер zap в режиме разработки
	logger, err := zap.NewDevelopment()
	if err != nil {
		panic(err)
	}
	defer logger.Sync()

	cfg, err := config.ParseServerConfig()
	if err != nil {
		logger.Fatal("Failed to parse config", zap.Error(err))
	}

	// Логируем параметры конфигурации
	logger.Info("Server configuration",
		zap.String("addr", cfg.Addr),
		zap.Duration("store_interval", cfg.StoreInterval),
		zap.String("file_path", cfg.FileStoragePath),
		zap.Bool("restore", cfg.Restore),
	)

	// Создаем хранилище с поддержкой файлов
	storage, err := repository.NewStorage(cfg.FileStoragePath, cfg.StoreInterval, cfg.Restore)
	if err != nil {
		logger.Fatal("Failed to create storage", zap.Error(err))
	}
	defer storage.Close()

	r := chi.NewRouter()

	// Используем стандартные middleware от Chi
	r.Use(chiMiddleware.Recoverer)
	r.Use(middleware.GzipMiddleware)

	// Подключаем НАШ middleware для логирования
	r.Use(middleware.LoggingMiddleware(logger))

	r.Get("/", handler.IndexHTMLHandler(storage))
	r.Post("/update/{type}/{name}/{value}", handler.UpdateHandler(storage))
	r.Get("/value/{type}/{name}", handler.GetMetricHandler(storage))
	r.Post("/update", handler.UpdateJSONHandler(storage))
	r.Post("/value", handler.ValueJSONHandler(storage))
	r.Post("/update/", handler.UpdateJSONHandler(storage))
	r.Post("/value/", handler.ValueJSONHandler(storage))

	// Добавляем эндпоинт для принудительного сохранения
	r.Post("/save", func(w http.ResponseWriter, r *http.Request) {
		if err := storage.SaveToFile(); err != nil {
			http.Error(w, fmt.Sprintf("Failed to save: %v", err), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Metrics saved successfully"))
	})

	serverAddr := cfg.Addr

	// Настраиваем HTTP сервер с таймаутами
	server := &http.Server{
		Addr:         serverAddr,
		Handler:      r,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  30 * time.Second,
	}

	// Канал для graceful shutdown
	stopChan := make(chan os.Signal, 1)
	signal.Notify(stopChan, os.Interrupt, syscall.SIGTERM)

	// Запускаем сервер в отдельной горутине
	go func() {
		logger.Info("Starting server",
			zap.String("addr", serverAddr),
		)

		fmt.Printf("Starting server on %s\n", serverAddr)
		fmt.Println("Available endpoints:")
		fmt.Println("  GET  /                    - HTML страница со всеми метриками")
		fmt.Println("  POST /update/{type}/{name}/{value} - Обновление метрики (текстовый формат)")
		fmt.Println("  GET  /value/{type}/{name} - Получение значения метрики (текстовый формат)")
		fmt.Println("  POST /update              - Обновление метрики (JSON формат)")
		fmt.Println("  POST /value               - Получение значения метрики (JSON формат)")
		fmt.Println("  POST /save                - Принудительное сохранение метрик на диск")

		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal("Server failed",
				zap.String("addr", serverAddr),
				zap.Error(err),
			)
		}
	}()

	// Ожидаем сигнал завершения
	<-stopChan
	logger.Info("Shutting down server...")

	// Graceful shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		logger.Error("Server shutdown failed", zap.Error(err))
	}

	logger.Info("Server stopped")
}
