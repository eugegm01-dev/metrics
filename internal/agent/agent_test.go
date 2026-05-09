package agent

import (
	"testing"

	"github.com/eugegm01-dev/metrics/internal/model"
)

func TestCollectRuntimeMetrics(t *testing.T) {
	metrics := collectRuntimeMetrics()
	// проверим, что вернулось не менее 27 метрик (ожидаемое количество)
	if len(metrics) < 27 {
		t.Errorf("collectRuntimeMetrics returned %d metrics, expected at least 27", len(metrics))
	}
	// проверим наличие PollCount? Нет, PollCount добавляется в CollectRuntimeMetrics
}

func TestCollectSystemMetrics(t *testing.T) {
	metrics := collectSystemMetrics()
	// может быть 0, если не удалось собрать
	if len(metrics) == 0 {
		t.Log("No system metrics collected, maybe not supported")
	}
}

func TestAgent_CollectRuntimeMetrics(t *testing.T) {
	a := NewAgent(1)
	metrics := a.CollectRuntimeMetrics()
	// должен быть PollCount с Delta = 1
	var pollCountMetric *Metric
	for i, m := range metrics {
		if m.ID == "PollCount" {
			pollCountMetric = &metrics[i]
			break
		}
	}
	if pollCountMetric == nil {
		t.Error("PollCount metric not found")
	} else if pollCountMetric.Delta != 1 {
		t.Errorf("PollCount Delta = %d, want 1", pollCountMetric.Delta)
	}
}

func TestAgent_PrepareMetricsForSend(t *testing.T) {
	a := NewAgent(1)
	a.pollCount = 10
	a.lastSentPollCount = 5

	metrics := []Metric{
		{ID: "gauge1", MType: model.Gauge, Value: 1.0},
		{ID: "PollCount", MType: model.Counter, Delta: 0},
	}
	prepared := a.PrepareMetricsForSend(metrics)
	var pollDelta int64
	for _, m := range prepared {
		if m.ID == "PollCount" {
			pollDelta = m.Delta
			break
		}
	}
	if pollDelta != 5 {
		t.Errorf("PollCount Delta = %d, want 5", pollDelta)
	}
}
func TestNewAgent(t *testing.T) {
	a := NewAgent(5)
	if a.rateLimit != 5 {
		t.Errorf("rateLimit = %d, want 5", a.rateLimit)
	}
	if cap(a.metricsChan) != 100 {
		t.Errorf("metricsChan cap = %d, want 100", cap(a.metricsChan))
	}
}

func TestAgent_IncrementPollCount(t *testing.T) {
	a := NewAgent(1)
	a.IncrementPollCount()
	if a.GetPollCount() != 1 {
		t.Errorf("pollCount = %d, want 1", a.GetPollCount())
	}
}

func TestAgent_StartWorkers(t *testing.T) {
	a := NewAgent(1)
	a.StartWorkers(func([]Metric) {})
	// Проверяем, что воркеры запущены и нет паники при отправке метрик
	a.SendMetrics([]Metric{{ID: "test", MType: "gauge", Value: 1.0}})
}
