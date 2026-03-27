package repository

import (
	"fmt"
	"testing"

	models "github.com/eugegm01-dev/metrics/internal/model"
)

func BenchmarkMemStorage_UpdateGauge(b *testing.B) {
	s := NewMemStorage()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s.UpdateGauge("test", float64(i))
	}
}

func BenchmarkMemStorage_UpdateBatch(b *testing.B) {
	s := NewMemStorage()
	metrics := make([]models.Metrics, 100)
	for i := range metrics {
		val := float64(i)
		metrics[i] = models.Metrics{ID: fmt.Sprintf("g%d", i), MType: models.Gauge, Value: &val}
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = s.UpdateBatch(metrics)
	}
}