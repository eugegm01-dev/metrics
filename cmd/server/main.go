package main

import (
	"fmt"
	"net/http"

	"github.com/eugegm01-dev/metrics/internal/config"
	"github.com/eugegm01-dev/metrics/internal/handler"
	customMiddleware "github.com/eugegm01-dev/metrics/internal/middleware"
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

	storage := repository.NewMemStorage()
	r := chi.NewRouter()

	// Используем стандартные middleware от Chi
	r.Use(chiMiddleware.Recoverer)

	// Подключаем НАШ middleware для логирования (важно: до объявления роутов!)
	r.Use(customMiddleware.LoggingMiddleware(logger))

	r.Get("/", handler.IndexHTMLHandler(storage))
	r.Post("/update/{type}/{name}/{value}", handler.UpdateHandler(storage))
	r.Get("/value/{type}/{name}", handler.GetMetricHandler(storage))

	serverAddr := cfg.Addr

	// Логируем запуск сервера с использованием zap
	logger.Info("Starting server",
		zap.String("addr", serverAddr),
	)

	fmt.Printf("Starting server on %s\n", serverAddr)
	fmt.Println("Available endpoints:")
	fmt.Println("  GET  /                    - HTML страница со всеми метриками")
	fmt.Println("  POST /update/{type}/{name}/{value} - Обновление метрики")
	fmt.Println("  GET  /value/{type}/{name} - Получение значения метрики")

	if err := http.ListenAndServe(serverAddr, r); err != nil {
		logger.Fatal("Server failed",
			zap.String("addr", serverAddr),
			zap.Error(err),
		)
	}
}
