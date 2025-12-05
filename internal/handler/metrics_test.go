package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"

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

	// Тест существующей метрики
	resp, _ := http.Get(ts.URL + "/value/counter/test")
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected 200, got %d", resp.StatusCode)
	}

	// Тест несуществующей метрики
	resp, _ = http.Get(ts.URL + "/value/counter/nonexistent")
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("Expected 404, got %d", resp.StatusCode)
	}
}
