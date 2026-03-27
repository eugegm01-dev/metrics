package handler

import (
	"bytes"
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/eugegm01-dev/metrics/internal/model"
	"github.com/eugegm01-dev/metrics/internal/repository"
)

func BenchmarkUpdateJSONHandler(b *testing.B) {
	storage := repository.NewMemStorage()
	handler := UpdateJSONHandler(storage, "")

	metric := model.Metrics{
		ID:    "test",
		MType: model.Gauge,
		Value: func() *float64 { v := 42.5; return &v }(),
	}
	body, _ := json.Marshal(metric)

	b.ResetTimer()
	for b.Loop() {
		b.StopTimer()
		req := httptest.NewRequest("POST", "/update", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		b.StartTimer()

		handler.ServeHTTP(w, req)
	}
}