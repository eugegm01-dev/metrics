package repository

import (
	"fmt"
	"strings"
	"sync"

	models "github.com/eugegm01-dev/metrics/internal/model"
)

// MemStorage – in-memory реализация Storage.
// Потокобезопасна.
type MemStorage struct {
	mu       sync.RWMutex
	Gauges   map[string]float64
	Counters map[string]int64
}

// NewMemStorage создаёт новый экземпляр MemStorage.
func NewMemStorage() *MemStorage {
	return &MemStorage{
		Gauges:   make(map[string]float64),
		Counters: make(map[string]int64),
	}
}

// UpdateGauge сохраняет gauge-значение.
func (s *MemStorage) UpdateGauge(name string, value float64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Gauges[name] = value
}

// UpdateCounter увеличивает счётчик на переданную дельту.
func (s *MemStorage) UpdateCounter(name string, value int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Counters[name] += value
}

// GetGauge возвращает gauge-значение и булев флаг существования.
func (s *MemStorage) GetGauge(name string) (float64, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	val, ok := s.Gauges[name]
	return val, ok
}

// GetCounter возвращает counter-значение и булев флаг существования.
func (s *MemStorage) GetCounter(name string) (int64, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	val, ok := s.Counters[name]
	return val, ok
}

// GetAllMetrics возвращает строковое представление всех метрик.
func (s *MemStorage) GetAllMetrics() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var sb strings.Builder
	sb.WriteString("Gauges:\n")
	for k, v := range s.Gauges {
		fmt.Fprintf(&sb, " %s: %f\n", k, v)
	}
	sb.WriteString("Counters:\n")
	for k, v := range s.Counters {
		fmt.Fprintf(&sb, " %s: %d\n", k, v)
	}
	return sb.String()
}

// UpdateBatch обновляет несколько метрик за одну операцию.
func (s *MemStorage) UpdateBatch(metrics []models.Metrics) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, metric := range metrics {
		switch metric.MType {
		case models.Gauge:
			if metric.Value == nil {
				return fmt.Errorf("value is required for gauge")
			}
			s.Gauges[metric.ID] = *metric.Value
		case models.Counter:
			if metric.Delta == nil {
				return fmt.Errorf("delta is required for counter")
			}
			s.Counters[metric.ID] += *metric.Delta
		default:
			return fmt.Errorf("unknown metric type: %s", metric.MType)
		}
	}
	return nil
}

// Пустые реализации интерфейса (FileStorage будет их переопределять)
func (s *MemStorage) SaveToFile() error   { return nil }
func (s *MemStorage) LoadFromFile() error { return nil }
func (s *MemStorage) Close()              {}
