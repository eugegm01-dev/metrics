package repository

import (
	"fmt"
	"sync"
)

type MemStorage struct {
	mu       sync.RWMutex
	Gauges   map[string]float64
	Counters map[string]int64
}

func NewMemStorage() *MemStorage {
	return &MemStorage{
		Gauges:   make(map[string]float64),
		Counters: make(map[string]int64),
	}
}

func (s *MemStorage) UpdateGauge(name string, value float64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Gauges[name] = value
}

func (s *MemStorage) UpdateCounter(name string, value int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Counters[name] += value
}

func (s *MemStorage) GetGauge(name string) (float64, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	val, ok := s.Gauges[name]
	return val, ok
}

func (s *MemStorage) GetCounter(name string) (int64, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	val, ok := s.Counters[name]
	return val, ok
}

func (s *MemStorage) GetAllMetrics() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var result string
	result += "Gauges:\n"
	for k, v := range s.Gauges {
		result += fmt.Sprintf(" %s: %f\n", k, v)
	}
	result += "Counters:\n"
	for k, v := range s.Counters {
		result += fmt.Sprintf(" %s: %d\n", k, v)
	}
	return result
}

// Пустые реализации интерфейса (FileStorage будет их переопределять)
func (s *MemStorage) SaveToFile() error   { return nil }
func (s *MemStorage) LoadFromFile() error { return nil }
func (s *MemStorage) Close()              {}
