package repository

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	models "github.com/eugegm01-dev/metrics/internal/model"
	"go.uber.org/zap"
)

// FileStorage оборачивает MemStorage и сохраняет метрики в файл.
// Может сохранять периодически или по требованию.
type FileStorage struct {
	*MemStorage   // встроенное поле — делегируем все операции с данными
	filePath      string
	storeInterval time.Duration
	saveChan      chan struct{}
	stopChan      chan struct{}
	lastSaved     time.Time
}

// NewFileStorage создаёт новый FileStorage.
// Если filePath пуст, используется временный файл.
// storeInterval – интервал периодического сохранения (0 – только по требованию).
// restore – загружать ли данные из файла при запуске.
func NewFileStorage(filePath string, storeInterval time.Duration, restore bool) (*FileStorage, error) {
	mem := NewMemStorage()
	// Если путь пустой или не указан, используем временную директорию
	if filePath == "" {
		// Создаём файл во временной директории
		filePath = filepath.Join(os.TempDir(), "metrics-db.json")
	}

	storage := &FileStorage{
		MemStorage:    mem,
		filePath:      filePath,
		storeInterval: storeInterval,
		saveChan:      make(chan struct{}, 100),
		stopChan:      make(chan struct{}),
		lastSaved:     time.Now(),
	}
	// Одноразовая загрузка при старте
	if restore {
		if err := storage.loadFromFile(); err != nil {
			zap.L().Warn("Failed to load metrics from file",
				zap.String("path", filePath),
				zap.Error(err))
		}
	}

	// Периодическое сохранение
	if storeInterval > 0 {
		go storage.periodicSave()
	}

	return storage, nil
}

// loadFromFile — приватная загрузка (вызывается только при создании)
func (s *FileStorage) loadFromFile() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	file, err := os.Open(s.filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // файл отсутствует — ок
		}
		return fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	var metrics []models.Metrics
	if err := json.NewDecoder(file).Decode(&metrics); err != nil {
		return fmt.Errorf("failed to decode metrics: %w", err)
	}

	// Сбрасываем текущее состояние
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

	return nil
}

// saveToFile — приватная запись
func (s *FileStorage) saveToFile() error {
	if s.filePath == "" {
		return nil // Просто игнорируем, если путь не указан
	}

	// Создаём директорию, если её нет
	dir := filepath.Dir(s.filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	metrics := s.GetAllMetricsForSave()
	tempFile := s.filePath + ".tmp"
	f, err := os.Create(tempFile)
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	defer f.Close()

	enc := json.NewEncoder(f)
	enc.SetIndent("", " ")
	if err := enc.Encode(metrics); err != nil {
		return fmt.Errorf("encode metrics: %w", err)
	}
	f.Close()

	if err := os.Rename(tempFile, s.filePath); err != nil {
		return fmt.Errorf("rename temp file: %w", err)
	}

	s.lastSaved = time.Now()
	return nil
}

// periodicSave — фоновая горутина
func (s *FileStorage) periodicSave() {
	ticker := time.NewTicker(s.storeInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if err := s.saveToFile(); err != nil {
				// Логируем ошибку, но не прерываем работу
				_ = err // игнорируем ошибку при сохранении
			}
		case <-s.saveChan:
			if err := s.saveToFile(); err != nil {
				// Логируем ошибку, но не прерываем работу
				_ = err // игнорируем ошибку при сохранении
			}
		case <-s.stopChan:
			s.saveToFile() // финальное сохранение
			return
		}
	}
}

// SaveToFile сохраняет текущие метрики в файл.
func (s *FileStorage) SaveToFile() error {
	return s.saveToFile()
}

// Close останавливает периодическое сохранение и выполняет финальную запись.
func (s *FileStorage) Close() {
	close(s.stopChan)
}

// UpdateBatch обновляет несколько метрик за одну операцию.
func (s *FileStorage) UpdateBatch(metrics []models.Metrics) error {
	// Используем метод MemStorage
	if err := s.MemStorage.UpdateBatch(metrics); err != nil {
		return err
	}
	// Если storeInterval = 0, то сохраняем синхронно
	if s.storeInterval == 0 {
		return s.saveToFile()
	}
	// Иначе откладываем сохранение до следующего тика
	return nil
}

// GetAllMetricsForSave — приватный вспомогательный метод
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
