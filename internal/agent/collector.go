package agent

import (
	"math/rand"
	"runtime"
	"sync/atomic"

	"github.com/eugegm01-dev/metrics/internal/model"
)

var (
	pollCount int64 = 0
)

func CollectMetrics() []model.Metrics {
	var stats runtime.MemStats
	runtime.ReadMemStats(&stats)

	// Атомарно увеличиваем счетчик
	atomic.AddInt64(&pollCount, 1)
	currentPollCount := atomic.LoadInt64(&pollCount)

	metrics := []model.Metrics{
		{ID: "Alloc", MType: model.Gauge, Value: float64Ptr(float64(stats.Alloc))},
		{ID: "BuckHashSys", MType: model.Gauge, Value: float64Ptr(float64(stats.BuckHashSys))},
		{ID: "Frees", MType: model.Gauge, Value: float64Ptr(float64(stats.Frees))},
		{ID: "GCCPUFraction", MType: model.Gauge, Value: float64Ptr(float64(stats.GCCPUFraction))},
		{ID: "GCSys", MType: model.Gauge, Value: float64Ptr(float64(stats.GCSys))},
		{ID: "HeapAlloc", MType: model.Gauge, Value: float64Ptr(float64(stats.HeapAlloc))},
		{ID: "HeapIdle", MType: model.Gauge, Value: float64Ptr(float64(stats.HeapIdle))},
		{ID: "HeapInuse", MType: model.Gauge, Value: float64Ptr(float64(stats.HeapInuse))},
		{ID: "HeapObjects", MType: model.Gauge, Value: float64Ptr(float64(stats.HeapObjects))},
		{ID: "HeapReleased", MType: model.Gauge, Value: float64Ptr(float64(stats.HeapReleased))},
		{ID: "HeapSys", MType: model.Gauge, Value: float64Ptr(float64(stats.HeapSys))},
		{ID: "LastGC", MType: model.Gauge, Value: float64Ptr(float64(stats.LastGC))},
		{ID: "Lookups", MType: model.Gauge, Value: float64Ptr(float64(stats.Lookups))},
		{ID: "MCacheInuse", MType: model.Gauge, Value: float64Ptr(float64(stats.MCacheInuse))},
		{ID: "MCacheSys", MType: model.Gauge, Value: float64Ptr(float64(stats.MCacheSys))},
		{ID: "MSpanInuse", MType: model.Gauge, Value: float64Ptr(float64(stats.MSpanInuse))},
		{ID: "MSpanSys", MType: model.Gauge, Value: float64Ptr(float64(stats.MSpanSys))},
		{ID: "Mallocs", MType: model.Gauge, Value: float64Ptr(float64(stats.Mallocs))},
		{ID: "NextGC", MType: model.Gauge, Value: float64Ptr(float64(stats.NextGC))},
		{ID: "NumForcedGC", MType: model.Gauge, Value: float64Ptr(float64(stats.NumForcedGC))},
		{ID: "NumGC", MType: model.Gauge, Value: float64Ptr(float64(stats.NumGC))},
		{ID: "OtherSys", MType: model.Gauge, Value: float64Ptr(float64(stats.OtherSys))},
		{ID: "PauseTotalNs", MType: model.Gauge, Value: float64Ptr(float64(stats.PauseTotalNs))},
		{ID: "StackInuse", MType: model.Gauge, Value: float64Ptr(float64(stats.StackInuse))},
		{ID: "StackSys", MType: model.Gauge, Value: float64Ptr(float64(stats.StackSys))},
		{ID: "Sys", MType: model.Gauge, Value: float64Ptr(float64(stats.Sys))},
		{ID: "TotalAlloc", MType: model.Gauge, Value: float64Ptr(float64(stats.TotalAlloc))},

		// Counter должен накапливаться
		{ID: "PollCount", MType: model.Counter, Delta: int64Ptr(currentPollCount)},

		// Произвольная метрика
		{ID: "RandomValue", MType: model.Gauge, Value: float64Ptr(rand.Float64() * 1000)},
	}

	return metrics
}

func float64Ptr(f float64) *float64 {
	return &f
}

func int64Ptr(i int64) *int64 {
	return &i
}
