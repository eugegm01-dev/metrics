package main

import (
	"fmt"
	"log"
	"net/http"

	"github.com/eugegm01-dev/metrics/internal/config"
	"github.com/eugegm01-dev/metrics/internal/handler"
	"github.com/eugegm01-dev/metrics/internal/repository"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

func main() {
	// 1. Парсим конфигурацию (флаги)
	cfg, err := config.ParseServerConfig()
	if err != nil {
		log.Fatalf("Failed to parse config: %v", err)
	}

	// 2. Инициализируем хранилище и роутер
	storage := repository.NewMemStorage()
	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	// 3. Настраиваем маршруты
	r.Get("/", handler.IndexHTMLHandler(storage))
	r.Post("/update/*", handler.UpdateHandler(storage))
	r.Get("/value/{type}/{name}", handler.GetMetricHandler(storage))

	// 4. Запускаем сервер с адресом из конфига
	serverAddr := cfg.Addr
	fmt.Printf("Starting server on %s\n", serverAddr)
	fmt.Println("Available endpoints:")
	fmt.Println("  GET  /                    - HTML страница со всеми метриками")
	fmt.Println("  POST /update/{type}/{name}/{value} - Обновление метрики")
	fmt.Println("  GET  /value/{type}/{name} - Получение значения метрики")

	if err := http.ListenAndServe(serverAddr, r); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
