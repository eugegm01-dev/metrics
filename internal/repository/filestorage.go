package repository

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	models "github.com/eugegm01-dev/metrics/internal/model"
)

// FileStorage — обёртка над MemStorage с сохранением в файл
type FileStorage struct {
	*MemStorage   // встроенное поле — все операции с метриками делегируются сюда
	filePath      string
	storeInterval time.Duration
	saveChan      chan struct{}
	stopChan      chan struct{}
	lastSaved     time.Time
}

// NewFileStorage создаёт файловое хранилище на основе in-memory
func NewFileStorage(filePath string, storeInterval time.Duration, restore bool) (*FileStorage, error) {
	mem := NewMemStorage()
	storage := &FileStorage{
		MemStorage:    mem,
		filePath:      filePath,
		storeInterval: storeInterval,
		saveChan:      make(chan struct{}, 100),
		stopChan:      make(chan struct{}),
		lastSaved:     time.Now(),
	}

	// Загружаем данные один раз при создании (если restore == true)
	if restore {
		if err := storage.loadFromFile(); err != nil {
			fmt.Printf("WARNING: Failed to load metrics from file: %v\n", err)
			// Можно также вернуть ошибку, если загрузка обязательна:
			// return nil, fmt.Errorf("failed to load metrics: %w", err)
		}
	}

	// Запускаем периодическое сохранение
	if storeInterval > 0 {
		go storage.periodicSave()
	}

	return storage, nil
}

// loadFromFile — внутренняя загрузка (не экспортируется)
func (s *FileStorage) loadFromFile() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	file, err := os.Open(s.filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // файл не существует — нормальная ситуация
		}
		return fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	var metrics []models.Metrics
	decoder := json.NewDecoder(file)
	if err := decoder.Decode(&metrics); err != nil {
		return fmt.Errorf("failed to decode metrics: %w", err)
	}

	// Очищаем текущее состояние
	s.Gauges = make(map[string]float64)
	s.Counters = make(map[string]int64)

	for _, m := range metrics {
		switch m.MType {
		case models.Gauge:
			if m.Value != nil {
				s.Gauges[m.ID] = *m.Value
			}
		case models.Counter:
			if m.Delta != nil {
				s.Counters[m.ID] = *m.Delta
			}
		}
	}

	fmt.Printf("Loaded %d metrics from %s\n", len(metrics), s.filePath)
	return nil
}

// saveToFile — внутренняя запись (не экспортируется)
func (s *FileStorage) saveToFile() error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	metrics := s.GetAllMetricsForSave()
	tempFile := s.filePath + ".tmp"
	file, err := os.Create(tempFile)
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", " ")
	if err := encoder.Encode(metrics); err != nil {
		return fmt.Errorf("failed to encode metrics: %w", err)
	}
	file.Close()

	if err := os.Rename(tempFile, s.filePath); err != nil {
		return fmt.Errorf("failed to rename temp file: %w", err)
	}

	s.lastSaved = time.Now()
	return nil
}

// periodicSave — горутина для периодического сохранения
func (s *FileStorage) periodicSave() {
	ticker := time.NewTicker(s.storeInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if err := s.saveToFile(); err != nil {
				fmt.Printf("ERROR: Failed to save metrics: %v\n", err)
			}
		case <-s.saveChan:
			if err := s.saveToFile(); err != nil {
				fmt.Printf("ERROR: Failed to save metrics: %v\n", err)
			}
		case <-s.stopChan:
			// финальное сохранение при закрытии
			s.saveToFile()
			return
		}
	}
}

// Close — завершает работу и делает финальное сохранение
func (s *FileStorage) Close() {
	close(s.stopChan)
}

// GetAllMetricsForSave — вспомогательный метод для сериализации (остаётся приватным)
func (s *FileStorage) GetAllMetricsForSave() []models.Metrics {
	var metrics []models.Metrics
	for name, value := range s.Gauges {
		val := value
		metrics = append(metrics, models.Metrics{
			ID:    name,
			MType: models.Gauge,
			Value: &val,
		})
	}
	for name, delta := range s.Counters {
		deltaVal := delta
		metrics = append(metrics, models.Metrics{
			ID:    name,
			MType: models.Counter,
			Delta: &deltaVal,
		})
	}
	return metrics
}
