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

type Metric struct {
	ID    string
	MType string
	Value float64
	Delta int64
}

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

func (a *Agent) IncrementPollCount() {
	atomic.AddInt64(&a.pollCount, 1)
}
func (a *Agent) StopWorkers() {
	for _, w := range a.workers {
		close(w.done)
	}
	close(a.metricsChan)
}

func (a *Agent) SendMetrics(metrics []Metric) {
	a.metricsChan <- metrics
}

func (a *Agent) GetPollCount() int64 {
	return atomic.LoadInt64(&a.pollCount)
}

func collectRuntimeMetrics() []Metric {
	var stats runtime.MemStats
	runtime.ReadMemStats(&stats)

	metrics := []Metric{
		{ID: "Alloc", MType: model.Gauge, Value: float64(stats.Alloc)},
		{ID: "BuckHashSys", MType: model.Gauge, Value: float64(stats.BuckHashSys)},
		{ID: "Frees", MType: model.Gauge, Value: float64(stats.Frees)},
		{ID: "GCCPUFraction", MType: model.Gauge, Value: stats.GCCPUFraction},
		{ID: "GCSys", MType: model.Gauge, Value: float64(stats.GCSys)},
		{ID: "HeapAlloc", MType: model.Gauge, Value: float64(stats.HeapAlloc)},
		{ID: "HeapIdle", MType: model.Gauge, Value: float64(stats.HeapIdle)},
		{ID: "HeapInuse", MType: model.Gauge, Value: float64(stats.HeapInuse)},
		{ID: "HeapObjects", MType: model.Gauge, Value: float64(stats.HeapObjects)},
		{ID: "HeapReleased", MType: model.Gauge, Value: float64(stats.HeapReleased)},
		{ID: "HeapSys", MType: model.Gauge, Value: float64(stats.HeapSys)},
		{ID: "LastGC", MType: model.Gauge, Value: float64(stats.LastGC)},
		{ID: "Lookups", MType: model.Gauge, Value: float64(stats.Lookups)},
		{ID: "MCacheInuse", MType: model.Gauge, Value: float64(stats.MCacheInuse)},
		{ID: "MCacheSys", MType: model.Gauge, Value: float64(stats.MCacheSys)},
		{ID: "MSpanInuse", MType: model.Gauge, Value: float64(stats.MSpanInuse)},
		{ID: "MSpanSys", MType: model.Gauge, Value: float64(stats.MSpanSys)},
		{ID: "Mallocs", MType: model.Gauge, Value: float64(stats.Mallocs)},
		{ID: "NextGC", MType: model.Gauge, Value: float64(stats.NextGC)},
		{ID: "NumForcedGC", MType: model.Gauge, Value: float64(stats.NumForcedGC)},
		{ID: "NumGC", MType: model.Gauge, Value: float64(stats.NumGC)},
		{ID: "OtherSys", MType: model.Gauge, Value: float64(stats.OtherSys)},
		{ID: "PauseTotalNs", MType: model.Gauge, Value: float64(stats.PauseTotalNs)},
		{ID: "StackInuse", MType: model.Gauge, Value: float64(stats.StackInuse)},
		{ID: "StackSys", MType: model.Gauge, Value: float64(stats.StackSys)},
		{ID: "Sys", MType: model.Gauge, Value: float64(stats.Sys)},
		{ID: "TotalAlloc", MType: model.Gauge, Value: float64(stats.TotalAlloc)},
		// Добавляем RandomValue
		{ID: "RandomValue", MType: model.Gauge, Value: rand.Float64()},
	}

	return metrics
}

func collectSystemMetrics() []Metric {
	var metrics []Metric

	// Сбор метрик памяти через gopsutil
	if vmStat, err := mem.VirtualMemory(); err == nil {
		metrics = append(metrics,
			Metric{ID: "TotalMemory", MType: model.Gauge, Value: float64(vmStat.Total)},
			Metric{ID: "FreeMemory", MType: model.Gauge, Value: float64(vmStat.Free)},
		)
	}

	// Сбор метрик CPU через gopsutil - по ядрам
	if cpuPercent, err := cpu.Percent(0, true); err == nil {
		for i, percent := range cpuPercent {
			metrics = append(metrics,
				Metric{ID: "CPUutilization" + strconv.Itoa(i+1), MType: model.Gauge, Value: percent},
			)
		}
	}

	return metrics
}

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

func (a *Agent) CollectSystemMetrics() []Metric {
	return collectSystemMetrics()
}

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
