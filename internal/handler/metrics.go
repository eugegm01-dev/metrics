package handler

import (
	"fmt"
	"html"
	"net/http"
	"strconv"

	"github.com/eugegm01-dev/metrics/internal/repository"
	"github.com/go-chi/chi/v5"
)

func UpdateHandler(storage repository.Storage) http.HandlerFunc {
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

func GetMetricHandler(storage repository.Storage) http.HandlerFunc {
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

func IndexHTMLHandler(storage repository.Storage) http.HandlerFunc {
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

		w.Write([]byte(htmlContent))
	}
}
