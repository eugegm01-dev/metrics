package handler

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/eugegm01-dev/metrics/internal/repository"
)

// UpdateHandler создает и возвращает обработчик для пути /update/
func UpdateHandler(storage *repository.MemStorage) http.HandlerFunc {
	// Возвращаем саму функцию-обработчик
	return func(w http.ResponseWriter, r *http.Request) {
		// Проверяем метод
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		// Разбираем URL: /update/<ТИП>/<ИМЯ>/<ЗНАЧЕНИЕ>
		parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")

		// Проверяем формат
		if len(parts) != 4 || parts[0] != "update" {
			http.Error(w, "Invalid request format", http.StatusNotFound)
			return
		}

		metricType := parts[1]
		metricName := parts[2]
		metricValue := parts[3]

		// Проверяем наличие имени метрики
		if metricName == "" {
			http.Error(w, "Metric name is required", http.StatusNotFound)
			return
		}

		// Проверяем тип метрики
		if metricType != "gauge" && metricType != "counter" {
			http.Error(w, "Invalid metric type", http.StatusBadRequest)
			return
		}

		// Обрабатываем метрику
		switch metricType {
		case "gauge":
			value, err := strconv.ParseFloat(metricValue, 64)
			if err != nil {
				http.Error(w, "Invalid gauge value", http.StatusBadRequest)
				return
			}
			storage.UpdateGauge(metricName, value)

		case "counter":
			value, err := strconv.ParseInt(metricValue, 10, 64)
			if err != nil {
				http.Error(w, "Invalid counter value", http.StatusBadRequest)
				return
			}
			storage.UpdateCounter(metricName, value)
		}

		// Успешный ответ
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}
}

// RootHandler для корневого пути
func RootHandler(storage *repository.MemStorage) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		info := storage.GetAllMetrics()
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(info))
	}
}
