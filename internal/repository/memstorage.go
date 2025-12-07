package repository

import (
	"fmt"
	"sync"
)

type MemStorage struct {
	mu       sync.RWMutex
	gauges   map[string]float64
	counters map[string]int64
}

func NewMemStorage() *MemStorage {
	return &MemStorage{
		gauges:   make(map[string]float64),
		counters: make(map[string]int64),
	}
}

func (s *MemStorage) UpdateGauge(name string, value float64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.gauges[name] = value
}

func (s *MemStorage) UpdateCounter(name string, value int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.counters[name] += value
}

func (s *MemStorage) GetGauge(name string) (float64, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	val, ok := s.gauges[name]
	return val, ok
}

func (s *MemStorage) GetCounter(name string) (int64, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	val, ok := s.counters[name]
	return val, ok
}

func (s *MemStorage) GetAllMetrics() string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result string
	result += "Gauges:\n"
	for k, v := range s.gauges {
		result += fmt.Sprintf("  %s: %f\n", k, v)
	}
	result += "Counters:\n"
	for k, v := range s.counters {
		result += fmt.Sprintf("  %s: %d\n", k, v)
	}
	return result
}
