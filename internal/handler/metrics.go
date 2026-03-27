package handler

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"html"
	"net/http"
	"strconv"

	models "github.com/eugegm01-dev/metrics/internal/model"
	"github.com/eugegm01-dev/metrics/internal/repository"
	"github.com/go-chi/chi/v5"
)

// UpdateHandler обрабатывает обновление метрики через URL параметры
func UpdateHandler(storage repository.Storage, key string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		metricType := chi.URLParam(r, "type")
		metricName := chi.URLParam(r, "name")
		metricValue := chi.URLParam(r, "value")

		if metricName == "" {
			http.Error(w, "Metric name is required", http.StatusNotFound)
			return
		}

		if metricType != "gauge" && metricType != "counter" {
			http.Error(w, "Invalid metric type", http.StatusBadRequest)
			return
		}

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

		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}
}

// UpdatesHandler обрабатывает обновление множества метрик за один запрос
func UpdatesHandler(storage repository.Storage, key string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		if r.Header.Get("Content-Type") != "application/json" {
			http.Error(w, "Content-Type must be application/json", http.StatusBadRequest)
			return
		}

		var metrics []models.Metrics
		dec := json.NewDecoder(r.Body)
		if err := dec.Decode(&metrics); err != nil {
			http.Error(w, "Invalid JSON", http.StatusBadRequest)
			return
		}

		if len(metrics) == 0 {
			w.WriteHeader(http.StatusOK)
			return
		}

		// Проверяем, поддерживает ли хранилище пакетное обновление
		if batchUpdater, ok := storage.(repository.BatchUpdater); ok {
			if err := batchUpdater.UpdateBatch(metrics); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
		} else {
			// Атомарное обновление всех метрик
			for _, metric := range metrics {
				if metric.ID == "" {
					http.Error(w, "Metric name (id) is required", http.StatusBadRequest)
					return
				}

				if metric.MType != models.Gauge && metric.MType != models.Counter {
					http.Error(w, "Invalid metric type", http.StatusBadRequest)
					return
				}

				switch metric.MType {
				case models.Gauge:
					if metric.Value == nil {
						http.Error(w, "Value is required for gauge", http.StatusBadRequest)
						return
					}
					storage.UpdateGauge(metric.ID, *metric.Value)

				case models.Counter:
					if metric.Delta == nil {
						http.Error(w, "Delta is required for counter", http.StatusBadRequest)
						return
					}
					storage.UpdateCounter(metric.ID, *metric.Delta)
				}
			}
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		// Возвращаем статус OK (можно также вернуть обновленные метрики)
		enc := json.NewEncoder(w)
		enc.Encode(map[string]string{"status": "ok"})
	}
}

// UpdateJSONHandler обрабатывает обновление метрики через JSON
func UpdateJSONHandler(storage repository.Storage, key string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Проверка метода удалена — chi/Post уже гарантирует POST

		if r.Header.Get("Content-Type") != "application/json" {
			http.Error(w, "Content-Type must be application/json", http.StatusBadRequest)
			return
		}

		var metric models.Metrics
		dec := json.NewDecoder(r.Body)
		if err := dec.Decode(&metric); err != nil {
			http.Error(w, "Invalid JSON", http.StatusBadRequest)
			return
		}

		if metric.ID == "" {
			http.Error(w, "Metric name (id) is required", http.StatusBadRequest)
			return
		}

		// Исправлено: models вместо model
		if metric.MType != models.Gauge && metric.MType != models.Counter {
			http.Error(w, "Invalid metric type. Must be 'gauge' or 'counter'", http.StatusBadRequest)
			return
		}

		switch metric.MType {
		case models.Gauge: // Исправлено
			if metric.Value == nil {
				http.Error(w, "Value is required for gauge", http.StatusBadRequest)
				return
			}
			storage.UpdateGauge(metric.ID, *metric.Value)

		case models.Counter: // Исправлено
			if metric.Delta == nil {
				http.Error(w, "Delta is required for counter", http.StatusBadRequest)
				return
			}
			storage.UpdateCounter(metric.ID, *metric.Delta)

		default:
			http.Error(w, "Invalid metric type", http.StatusBadRequest)
			return
		}

		var response models.Metrics
		response.ID = metric.ID
		response.MType = metric.MType

		switch metric.MType {
		case models.Gauge: // Исправлено
			val, _ := storage.GetGauge(metric.ID)
			response.Value = &val
		case models.Counter: // Исправлено
			val, _ := storage.GetCounter(metric.ID)
			response.Delta = &val
		}

		// Кодируем ответ
		var buf bytes.Buffer
		enc := json.NewEncoder(&buf)
		if err := enc.Encode(response); err != nil {
			http.Error(w, "Failed to encode response", http.StatusInternalServerError)
			return
		}

		// Вычисляем хеш, если ключ задан
		if key != "" {
			hash := computeHash(buf.Bytes(), key)
			w.Header().Set("HashSHA256", hash)
		}

		// Устанавливаем заголовки и пишем ответ
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(buf.Bytes())
	}
}

// computeHash вычисляет HMAC-SHA256 хеш
func computeHash(data []byte, key string) string {
	if key == "" {
		return ""
	}

	h := hmac.New(sha256.New, []byte(key))
	h.Write(data)
	return hex.EncodeToString(h.Sum(nil))
}

func GetMetricHandler(storage repository.Storage, key string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		metricType := chi.URLParam(r, "type")
		metricName := chi.URLParam(r, "name")

		switch metricType {
		case "gauge":
			value, exists := storage.GetGauge(metricName)
			if !exists {
				http.Error(w, "Metric not found", http.StatusNotFound)
				return
			}
			w.Header().Set("Content-Type", "text/plain")
			w.WriteHeader(http.StatusOK)
			fmt.Fprintf(w, "%v", value)

		case "counter":
			value, exists := storage.GetCounter(metricName)
			if !exists {
				http.Error(w, "Metric not found", http.StatusNotFound)
				return
			}
			w.Header().Set("Content-Type", "text/plain")
			w.WriteHeader(http.StatusOK)
			fmt.Fprintf(w, "%d", value)

		default:
			http.Error(w, "Invalid metric type", http.StatusBadRequest)
			return
		}
	}
}

// ValueJSONHandler возвращает значение метрики в формате JSON
func ValueJSONHandler(storage repository.Storage, key string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Проверка метода удалена — chi/Post уже гарантирует POST

		if r.Header.Get("Content-Type") != "application/json" {
			http.Error(w, "Content-Type must be application/json", http.StatusBadRequest)
			return
		}

		var metric models.Metrics
		dec := json.NewDecoder(r.Body)
		if err := dec.Decode(&metric); err != nil {
			http.Error(w, "Invalid JSON", http.StatusBadRequest)
			return
		}

		if metric.ID == "" {
			http.Error(w, "Metric name (id) is required", http.StatusBadRequest)
			return
		}

		var response models.Metrics
		response.ID = metric.ID
		response.MType = metric.MType

		switch metric.MType {
		case models.Gauge:
			value, exists := storage.GetGauge(metric.ID)
			if !exists {
				http.Error(w, "Metric not found", http.StatusNotFound)
				return
			}
			response.Value = &value

		case models.Counter:
			value, exists := storage.GetCounter(metric.ID)
			if !exists {
				http.Error(w, "Metric not found", http.StatusNotFound)
				return
			}
			response.Delta = &value

		default:
			http.Error(w, "Invalid metric type", http.StatusBadRequest)
			return
		}
		// Кодируем ответ в буфер
		var buf bytes.Buffer
		enc := json.NewEncoder(&buf)
		if err := enc.Encode(response); err != nil {
			http.Error(w, "Failed to encode response", http.StatusInternalServerError)
			return
		}

		// Вычисляем хеш, если ключ задан
		if key != "" {
			hash := computeHash(buf.Bytes(), key)
			w.Header().Set("HashSHA256", hash)
		}

		// Устанавливаем заголовки и пишем ответ
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(buf.Bytes())
	}
}

func IndexHTMLHandler(storage repository.Storage, key string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusOK)

		metricsText := storage.GetAllMetrics()

		htmlContent := `
<!DOCTYPE html>
<html>
<head>
    <title>Metrics Server</title>
    <style>
        body { font-family: Arial, sans-serif; margin: 40px; }
        h1 { color: #333; }
        pre { background: #f5f5f5; padding: 15px; border-radius: 5px; }
        table { border-collapse: collapse; width: 100%; }
        th, td { border: 1px solid #ddd; padding: 8px; text-align: left; }
        th { background-color: #f2f2f2; }
    </style>
</head>
<body>
    <h1>Metrics Server</h1>
    <h2>All Metrics</h2>
    <pre>` + html.EscapeString(metricsText) + `</pre>

    <h2>Quick Links</h2>
    <ul>
        <li><a href="/update/counter/test/1">Update counter test</a></li>
        <li><a href="/update/gauge/temperature/23.5">Update gauge temperature</a></li>
        <li><a href="/value/counter/test">Get counter test value</a></li>
        <li><a href="/value/gauge/temperature">Get gauge temperature value</a></li>
    </ul>
</body>
</html>`

		// Вычисляем хеш для ответа
		data := []byte(htmlContent)
		if key != "" {
			hash := computeHash(data, key)
			w.Header().Set("HashSHA256", hash)
		}

		w.Write([]byte(htmlContent))
	}
}
