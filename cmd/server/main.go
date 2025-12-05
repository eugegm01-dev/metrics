package main

import (
	"fmt"
	"log"
	"net/http"

	"github.com/eugegm01-dev/metrics/internal/handler"
	"github.com/eugegm01-dev/metrics/internal/repository"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

func main() {
	// Инициализируем хранилище
	storage := repository.NewMemStorage()

	// Создаём роутер Chi
	r := chi.NewRouter()

	// Добавляем middleware (по желанию)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	// Настраиваем маршруты
	r.Get("/", handler.IndexHTMLHandler(storage))                    // HTML страница
	r.Post("/update/*", handler.UpdateHandler(storage))              // Обновление метрик
	r.Get("/value/{type}/{name}", handler.GetMetricHandler(storage)) // Получение значения

	// Запускаем сервер
	serverAddr := ":8080"
	fmt.Printf("Starting server on %s\n", serverAddr)
	fmt.Println("Available endpoints:")
	fmt.Println("  GET  /                    - HTML страница со всеми метриками")
	fmt.Println("  POST /update/{type}/{name}/{value} - Обновление метрики")
	fmt.Println("  GET  /value/{type}/{name} - Получение значения метрики")

	if err := http.ListenAndServe(serverAddr, r); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
