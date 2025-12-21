package agent

import (
	"math/rand"
	"runtime"

	models "github.com/eugegm01-dev/metrics/internal/model"
)

// CollectMetrics собирает метрики из runtime
func CollectMetrics() []models.Metrics {
	var stats runtime.MemStats
	runtime.ReadMemStats(&stats)

	metrics := []models.Metrics{
		// Gauge метрики
		{ID: "Alloc", MType: models.Gauge, Value: float64Ptr(float64(stats.Alloc))},
		{ID: "BuckHashSys", MType: models.Gauge, Value: float64Ptr(float64(stats.BuckHashSys))},
		{ID: "Frees", MType: models.Gauge, Value: float64Ptr(float64(stats.Frees))},
		{ID: "GCCPUFraction", MType: models.Gauge, Value: float64Ptr(float64(stats.GCCPUFraction))},
		{ID: "GCSys", MType: models.Gauge, Value: float64Ptr(float64(stats.GCSys))},
		{ID: "HeapAlloc", MType: models.Gauge, Value: float64Ptr(float64(stats.HeapAlloc))},
		{ID: "HeapIdle", MType: models.Gauge, Value: float64Ptr(float64(stats.HeapIdle))},
		{ID: "HeapInuse", MType: models.Gauge, Value: float64Ptr(float64(stats.HeapInuse))},
		{ID: "HeapObjects", MType: models.Gauge, Value: float64Ptr(float64(stats.HeapObjects))},
		{ID: "HeapReleased", MType: models.Gauge, Value: float64Ptr(float64(stats.HeapReleased))},
		{ID: "HeapSys", MType: models.Gauge, Value: float64Ptr(float64(stats.HeapSys))},
		{ID: "LastGC", MType: models.Gauge, Value: float64Ptr(float64(stats.LastGC))},
		{ID: "Lookups", MType: models.Gauge, Value: float64Ptr(float64(stats.Lookups))},
		{ID: "MCacheInuse", MType: models.Gauge, Value: float64Ptr(float64(stats.MCacheInuse))},
		{ID: "MCacheSys", MType: models.Gauge, Value: float64Ptr(float64(stats.MCacheSys))},
		{ID: "MSpanInuse", MType: models.Gauge, Value: float64Ptr(float64(stats.MSpanInuse))},
		{ID: "MSpanSys", MType: models.Gauge, Value: float64Ptr(float64(stats.MSpanSys))},
		{ID: "Mallocs", MType: models.Gauge, Value: float64Ptr(float64(stats.Mallocs))},
		{ID: "NextGC", MType: models.Gauge, Value: float64Ptr(float64(stats.NextGC))},
		{ID: "NumForcedGC", MType: models.Gauge, Value: float64Ptr(float64(stats.NumForcedGC))},
		{ID: "NumGC", MType: models.Gauge, Value: float64Ptr(float64(stats.NumGC))},
		{ID: "OtherSys", MType: models.Gauge, Value: float64Ptr(float64(stats.OtherSys))},
		{ID: "PauseTotalNs", MType: models.Gauge, Value: float64Ptr(float64(stats.PauseTotalNs))},
		{ID: "StackInuse", MType: models.Gauge, Value: float64Ptr(float64(stats.StackInuse))},
		{ID: "StackSys", MType: models.Gauge, Value: float64Ptr(float64(stats.StackSys))},
		{ID: "Sys", MType: models.Gauge, Value: float64Ptr(float64(stats.Sys))},
		{ID: "TotalAlloc", MType: models.Gauge, Value: float64Ptr(float64(stats.TotalAlloc))},

		// Counter метрики
		{ID: "PollCount", MType: models.Counter, Delta: int64Ptr(1)},

		// Произвольная метрика
		{ID: "RandomValue", MType: models.Gauge, Value: float64Ptr(rand.Float64() * 1000)},
	}

	return metrics
}

func float64Ptr(f float64) *float64 {
	return &f
}

func int64Ptr(i int64) *int64 {
	return &i
}
