package repository

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"

	models "github.com/eugegm01-dev/metrics/internal/model"
)

type FileStorage struct {
	mu            sync.RWMutex
	gauges        map[string]float64
	counters      map[string]int64
	filePath      string
	storeInterval time.Duration
	saveChan      chan struct{}
	stopChan      chan struct{}
	lastSaved     time.Time
}

func NewFileStorage(filePath string, storeInterval time.Duration, restore bool) (*FileStorage, error) {
	storage := &FileStorage{
		gauges:        make(map[string]float64),
		counters:      make(map[string]int64),
		filePath:      filePath,
		storeInterval: storeInterval,
		saveChan:      make(chan struct{}, 100),
		stopChan:      make(chan struct{}),
		lastSaved:     time.Now(),
	}

	// Загружаем метрики при старте, если требуется
	if restore {
		if err := storage.LoadFromFile(); err != nil {
			// Логируем ошибку, но не падаем
			fmt.Printf("WARNING: Failed to load metrics from file: %v\n", err)
		}
	}

	// Запускаем горутину для периодического сохранения
	if storeInterval > 0 {
		go storage.periodicSave()
	}

	return storage, nil
}

func (s *FileStorage) UpdateGauge(name string, value float64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.gauges[name] = value

	// Если интервал сохранения = 0, сохраняем синхронно
	if s.storeInterval == 0 {
		s.saveToFile()
	}
}

func (s *FileStorage) UpdateCounter(name string, value int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.counters[name] += value

	// Если интервал сохранения = 0, сохраняем синхронно
	if s.storeInterval == 0 {
		s.saveToFile()
	}
}

func (s *FileStorage) GetGauge(name string) (float64, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	val, ok := s.gauges[name]
	return val, ok
}

func (s *FileStorage) GetCounter(name string) (int64, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	val, ok := s.counters[name]
	return val, ok
}

func (s *FileStorage) GetAllMetrics() string {
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

// GetAllMetricsForSave возвращает все метрики в формате JSON
func (s *FileStorage) GetAllMetricsForSave() []models.Metrics {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var metrics []models.Metrics

	// Добавляем gauge метрики
	for name, value := range s.gauges {
		val := value
		metrics = append(metrics, models.Metrics{
			ID:    name,
			MType: models.Gauge,
			Value: &val,
		})
	}

	// Добавляем counter метрики
	for name, delta := range s.counters {
		deltaVal := delta
		metrics = append(metrics, models.Metrics{
			ID:    name,
			MType: models.Counter,
			Delta: &deltaVal,
		})
	}

	return metrics
}

// SaveToFile сохраняет метрики в файл
func (s *FileStorage) SaveToFile() error {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.saveToFile()
}

func (s *FileStorage) saveToFile() error {
	metrics := s.GetAllMetricsForSave()

	// Создаем временный файл для атомарной записи
	tempFile := s.filePath + ".tmp"
	file, err := os.Create(tempFile)
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")

	if err := encoder.Encode(metrics); err != nil {
		return fmt.Errorf("failed to encode metrics: %w", err)
	}

	// Закрываем файл перед переименованием
	file.Close()

	// Атомарно заменяем старый файл новым
	if err := os.Rename(tempFile, s.filePath); err != nil {
		return fmt.Errorf("failed to rename temp file: %w", err)
	}

	s.lastSaved = time.Now()
	return nil
}

// LoadFromFile загружает метрики из файла
func (s *FileStorage) LoadFromFile() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	file, err := os.Open(s.filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // Файл не существует - это нормально
		}
		return fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	var metrics []models.Metrics
	decoder := json.NewDecoder(file)
	if err := decoder.Decode(&metrics); err != nil {
		return fmt.Errorf("failed to decode metrics: %w", err)
	}

	// Сбрасываем текущие метрики
	s.gauges = make(map[string]float64)
	s.counters = make(map[string]int64)

	// Загружаем метрики из файла
	for _, metric := range metrics {
		switch metric.MType {
		case models.Gauge:
			if metric.Value != nil {
				s.gauges[metric.ID] = *metric.Value
			}
		case models.Counter:
			if metric.Delta != nil {
				s.counters[metric.ID] = *metric.Delta
			}
		}
	}

	fmt.Printf("Loaded %d metrics from %s\n", len(metrics), s.filePath)
	return nil
}

// periodicSave периодически сохраняет метрики на диск
func (s *FileStorage) periodicSave() {
	ticker := time.NewTicker(s.storeInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if err := s.SaveToFile(); err != nil {
				fmt.Printf("ERROR: Failed to save metrics: %v\n", err)
			} else {
				fmt.Printf("Metrics saved to %s\n", s.filePath)
			}
		case <-s.saveChan:
			if err := s.SaveToFile(); err != nil {
				fmt.Printf("ERROR: Failed to save metrics: %v\n", err)
			}
		case <-s.stopChan:
			// Сохраняем при завершении
			if err := s.SaveToFile(); err != nil {
				fmt.Printf("ERROR: Failed to save metrics on shutdown: %v\n", err)
			}
			return
		}
	}
}

// Close останавливает периодическое сохранение
func (s *FileStorage) Close() {
	close(s.stopChan)
}
