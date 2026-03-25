package handler

import (
	"bytes"
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
	// Передаем пустую строку как ключ (для тестов)
	r.Get("/value/{type}/{name}", GetMetricHandler(storage, ""))

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
	// Передаем пустую строку как ключ
	r.Post("/update", UpdateJSONHandler(storage, ""))

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
	// Передаем пустую строку как ключ
	r.Post("/value", ValueJSONHandler(storage, ""))

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
func TestUpdatesHandler(t *testing.T) {
	storage := repository.NewMemStorage()
	handler := UpdatesHandler(storage, "", nil)

	// создаём запрос с батчем
	metrics := []models.Metrics{
		{ID: "g1", MType: models.Gauge, Value: func() *float64 { v := 1.1; return &v }()},
		{ID: "c1", MType: models.Counter, Delta: func() *int64 { v := int64(5); return &v }()},
	}
	body, _ := json.Marshal(metrics)

	req := httptest.NewRequest("POST", "/updates", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d", w.Code)
	}
	if val, ok := storage.GetGauge("g1"); !ok || val != 1.1 {
		t.Errorf("Gauge not updated: %v, %v", val, ok)
	}
	if val, ok := storage.GetCounter("c1"); !ok || val != 5 {
		t.Errorf("Counter not updated: %v, %v", val, ok)
	}
}
func TestIndexHTMLHandler(t *testing.T) {
	storage := repository.NewMemStorage()
	storage.UpdateGauge("test", 1.23)
	handler := IndexHTMLHandler(storage, "")

	req := httptest.NewRequest("GET", "/", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d", rr.Code)
	}
	body := rr.Body.String()
	if !strings.Contains(body, "test: 1.230000") {
		t.Error("Metric not found in HTML")
	}
}
