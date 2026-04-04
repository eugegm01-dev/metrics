// Package agent collects system and runtime metrics and sends them to the server.
package agent

import (
	"math/rand"
	"runtime"
	"strconv"
	"sync"
	"sync/atomic"

	"github.com/eugegm01-dev/metrics/internal/model"
	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/mem"
)

// Metric представляет собой одну собранную метрику.
// Содержит идентификатор, тип (gauge/counter) и значение/дельта.
type Metric struct {
	ID    string
	MType string
	Value float64
	Delta int64
}

// Agent собирает системные и рантайм-метрики и отправляет их на сервер.
// Использует пул воркеров для конкурентной отправки.
type Agent struct {
	pollCount         int64
	lastSentPollCount int64
	mu                sync.RWMutex
	metricsChan       chan []Metric
	workers           []*worker
	rateLimit         int
}

type worker struct {
	id      int
	metrics <-chan []Metric
	done    chan struct{}
}

// NewAgent создаёт новый Agent с указанным количеством воркеров (rateLimit).
func NewAgent(rateLimit int) *Agent {
	return &Agent{
		pollCount:         0,
		lastSentPollCount: 0,
		metricsChan:       make(chan []Metric, 100),
		rateLimit:         rateLimit,
	}
}

func (w *worker) run(processFunc func([]Metric)) {
	for {
		select {
		case metrics := <-w.metrics:
			processFunc(metrics)
		case <-w.done:
			return
		}
	}
}

// StartWorkers запускает пул воркеров, которые обрабатывают метрики через processFunc.

func (a *Agent) StartWorkers(processFunc func([]Metric)) {
	for i := 0; i < a.rateLimit; i++ {
		w := &worker{
			id:      i,
			metrics: a.metricsChan,
			done:    make(chan struct{}),
		}
		a.workers = append(a.workers, w)
		go w.run(processFunc)
	}
}

// IncrementPollCount увеличивает счётчик опросов на 1.
func (a *Agent) IncrementPollCount() {
	atomic.AddInt64(&a.pollCount, 1)
}

// StopWorkers останавливает всех воркеров и закрывает канал метрик.
func (a *Agent) StopWorkers() {
	for _, w := range a.workers {
		close(w.done)
	}
	close(a.metricsChan)
}

// SendMetrics отправляет метрики во внутренний канал для обработки воркерами.
// Безопасно вызывать из нескольких горутин.
func (a *Agent) SendMetrics(metrics []Metric) {
	a.metricsChan <- metrics
}

// GetPollCount возвращает текущее значение счётчика опросов.
func (a *Agent) GetPollCount() int64 {
	return atomic.LoadInt64(&a.pollCount)
}
func collectRuntimeMetrics() []Metric {
	var stats runtime.MemStats
	runtime.ReadMemStats(&stats)

	// известное количество метрик (27)
	metrics := make([]Metric, 0, 27)

	metrics = append(metrics,
		Metric{ID: "Alloc", MType: model.Gauge, Value: float64(stats.Alloc)},
		Metric{ID: "BuckHashSys", MType: model.Gauge, Value: float64(stats.BuckHashSys)},
		Metric{ID: "Frees", MType: model.Gauge, Value: float64(stats.Frees)},
		Metric{ID: "GCCPUFraction", MType: model.Gauge, Value: stats.GCCPUFraction},
		Metric{ID: "GCSys", MType: model.Gauge, Value: float64(stats.GCSys)},
		Metric{ID: "HeapAlloc", MType: model.Gauge, Value: float64(stats.HeapAlloc)},
		Metric{ID: "HeapIdle", MType: model.Gauge, Value: float64(stats.HeapIdle)},
		Metric{ID: "HeapInuse", MType: model.Gauge, Value: float64(stats.HeapInuse)},
		Metric{ID: "HeapObjects", MType: model.Gauge, Value: float64(stats.HeapObjects)},
		Metric{ID: "HeapReleased", MType: model.Gauge, Value: float64(stats.HeapReleased)},
		Metric{ID: "HeapSys", MType: model.Gauge, Value: float64(stats.HeapSys)},
		Metric{ID: "LastGC", MType: model.Gauge, Value: float64(stats.LastGC)},
		Metric{ID: "Lookups", MType: model.Gauge, Value: float64(stats.Lookups)},
		Metric{ID: "MCacheInuse", MType: model.Gauge, Value: float64(stats.MCacheInuse)},
		Metric{ID: "MCacheSys", MType: model.Gauge, Value: float64(stats.MCacheSys)},
		Metric{ID: "MSpanInuse", MType: model.Gauge, Value: float64(stats.MSpanInuse)},
		Metric{ID: "MSpanSys", MType: model.Gauge, Value: float64(stats.MSpanSys)},
		Metric{ID: "Mallocs", MType: model.Gauge, Value: float64(stats.Mallocs)},
		Metric{ID: "NextGC", MType: model.Gauge, Value: float64(stats.NextGC)},
		Metric{ID: "NumForcedGC", MType: model.Gauge, Value: float64(stats.NumForcedGC)},
		Metric{ID: "NumGC", MType: model.Gauge, Value: float64(stats.NumGC)},
		Metric{ID: "OtherSys", MType: model.Gauge, Value: float64(stats.OtherSys)},
		Metric{ID: "PauseTotalNs", MType: model.Gauge, Value: float64(stats.PauseTotalNs)},
		Metric{ID: "StackInuse", MType: model.Gauge, Value: float64(stats.StackInuse)},
		Metric{ID: "StackSys", MType: model.Gauge, Value: float64(stats.StackSys)},
		Metric{ID: "Sys", MType: model.Gauge, Value: float64(stats.Sys)},
		Metric{ID: "TotalAlloc", MType: model.Gauge, Value: float64(stats.TotalAlloc)},
		Metric{ID: "RandomValue", MType: model.Gauge, Value: rand.Float64()},
	)
	return metrics
}
func collectSystemMetrics() []Metric {
	var metrics []Metric

	if vmStat, err := mem.VirtualMemory(); err == nil {
		metrics = append(metrics,
			Metric{ID: "TotalMemory", MType: model.Gauge, Value: float64(vmStat.Total)},
			Metric{ID: "FreeMemory", MType: model.Gauge, Value: float64(vmStat.Free)},
		)
	}

	if cpuPercent, err := cpu.Percent(0, true); err == nil {
		for i, percent := range cpuPercent {
			metrics = append(metrics,
				Metric{ID: "CPUutilization" + strconv.Itoa(i+1), MType: model.Gauge, Value: percent},
			)
		}
	}

	return metrics
}

// CollectRuntimeMetrics собирает runtime-метрики (memstats) и включает PollCount.
// Перед сборкой увеличивает счётчик опросов.
func (a *Agent) CollectRuntimeMetrics() []Metric {
	a.IncrementPollCount()
	metrics := collectRuntimeMetrics()
	metrics = append(metrics, Metric{
		ID:    "PollCount",
		MType: model.Counter,
		Delta: a.GetPollCount(),
	})
	return metrics
}

// CollectSystemMetrics собирает системные метрики (память, CPU) через gopsutil.
func (a *Agent) CollectSystemMetrics() []Metric {
	return collectSystemMetrics()
}

// PrepareMetricsForSend подготавливает метрики к отправке, корректируя дельту PollCount.
// Возвращает копию метрик, где дельта PollCount установлена как разница с последней отправкой.
func (a *Agent) PrepareMetricsForSend(metrics []Metric) []Metric {
	delta := a.pollCount - a.lastSentPollCount
	for i := range metrics {
		if metrics[i].ID == "PollCount" && metrics[i].MType == model.Counter {
			metrics[i].Delta = delta
			break
		}
	}
	a.lastSentPollCount = a.pollCount
	return metrics
}
