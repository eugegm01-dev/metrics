package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	models "github.com/eugegm01-dev/metrics/internal/model"
	"github.com/eugegm01-dev/metrics/internal/repository"
	"github.com/go-chi/chi/v5"
)

func TestGetMetricHandler(t *testing.T) {
	storage := repository.NewMemStorage()
	storage.UpdateCounter("test", 42)

	r := chi.NewRouter()
	r.Get("/value/{type}/{name}", GetMetricHandler(storage))

	ts := httptest.NewServer(r)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/value/counter/test")
	if err != nil {
		t.Fatalf("Failed to make request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected 200, got %d", resp.StatusCode)
	}

	resp, err = http.Get(ts.URL + "/value/counter/nonexistent")
	if err != nil {
		t.Fatalf("Failed to make request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("Expected 404, got %d", resp.StatusCode)
	}
}
func TestUpdateJSONHandler(t *testing.T) {
	storage := repository.NewMemStorage()

	r := chi.NewRouter()
	r.Post("/update", UpdateJSONHandler(storage))

	ts := httptest.NewServer(r)
	defer ts.Close()

	// Тест отправки gauge метрики
	jsonData := `{"id":"testGauge","type":"gauge","value":42.5}`
	resp, err := http.Post(ts.URL+"/update", "application/json", strings.NewReader(jsonData))
	if err != nil {
		t.Fatalf("Failed to make request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected 200, got %d", resp.StatusCode)
	}

	// Проверяем, что метрика сохранилась
	val, exists := storage.GetGauge("testGauge")
	if !exists {
		t.Error("Metric was not saved")
	}
	if val != 42.5 {
		t.Errorf("Expected 42.5, got %f", val)
	}
}

func TestValueJSONHandler(t *testing.T) {
	storage := repository.NewMemStorage()
	storage.UpdateGauge("testGauge", 42.5)
	storage.UpdateCounter("testCounter", 100)

	r := chi.NewRouter()
	r.Post("/value", ValueJSONHandler(storage))

	ts := httptest.NewServer(r)
	defer ts.Close()

	// Тест получения gauge метрики
	jsonData := `{"id":"testGauge","type":"gauge"}`
	resp, err := http.Post(ts.URL+"/value", "application/json", strings.NewReader(jsonData))
	if err != nil {
		t.Fatalf("Failed to make request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected 200, got %d", resp.StatusCode)
	}

	var result models.Metrics
	dec := json.NewDecoder(resp.Body)
	if err := dec.Decode(&result); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if *result.Value != 42.5 {
		t.Errorf("Expected 42.5, got %f", *result.Value)
	}
}
