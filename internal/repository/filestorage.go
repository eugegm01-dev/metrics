package repository

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	models "github.com/eugegm01-dev/metrics/internal/model"
	"go.uber.org/zap"
)

// fileStorage — файловое хранилище, обёртка над MemStorage с периодическим сохранением на диск.
type fileStorage struct {
	*MemStorage   // встроенное поле — делегируем операции с данными
	filePath      string
	storeInterval time.Duration
	saveChan      chan struct{}
	stopChan      chan struct{}
	mu            sync.RWMutex
	lastSaved     time.Time
}

// NewFileStorage создаёт файловое хранилище.
// path — путь к файлу; если пустой, будет использован временный файл.
// interval — интервал периодического сохранения; если <= 0, периодическое сохранение не запускается.
// restore — если true, при старте попытаемся загрузить данные из файла.
func NewFileStorage(path string, interval time.Duration, restore bool) (Storage, error) {
	filePath := path
	if filePath == "" {
		filePath = filepath.Join(os.TempDir(), "metrics-db.json")
	}

	mem := NewMemStorage()

	fs := &fileStorage{
		MemStorage:    mem,
		filePath:      filePath,
		storeInterval: interval,
		saveChan:      make(chan struct{}, 1),
		stopChan:      make(chan struct{}),
		lastSaved:     time.Now(),
	}

	if restore {
		if err := fs.loadFromFile(); err != nil {
			zap.L().Warn("Failed to load metrics from file", zap.Error(err), zap.String("file", fs.filePath))
		}
	}

	if interval > 0 {
		go fs.periodicSave()
	}

	return fs, nil
}

func (s *fileStorage) loadFromFile() error {
	f, err := os.Open(s.filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("failed to open file: %w", err)
	}
	defer f.Close()

	var metrics []models.Metrics
	if err := json.NewDecoder(f).Decode(&metrics); err != nil {
		return fmt.Errorf("failed to decode metrics: %w", err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

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

	zap.L().Info("Loaded metrics from file", zap.Int("count", len(metrics)), zap.String("file", s.filePath))
	return nil
}

func (s *fileStorage) saveToFile() error {
	if s.filePath == "" {
		return nil
	}

	dir := filepath.Dir(s.filePath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	s.mu.RLock()
	metrics := s.GetAllMetricsForSave()
	s.mu.RUnlock()

	tempFile := s.filePath + ".tmp"
	f, err := os.Create(tempFile)
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}

	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	if err := enc.Encode(metrics); err != nil {
		_ = f.Close()
		_ = os.Remove(tempFile)
		return fmt.Errorf("encode metrics: %w", err)
	}

	if err := f.Close(); err != nil {
		_ = os.Remove(tempFile)
		return fmt.Errorf("close temp file: %w", err)
	}

	if err := os.Rename(tempFile, s.filePath); err != nil {
		_ = os.Remove(tempFile)
		return fmt.Errorf("rename temp file: %w", err)
	}

	s.mu.Lock()
	s.lastSaved = time.Now()
	s.mu.Unlock()

	return nil
}

func (s *fileStorage) periodicSave() {
	ticker := time.NewTicker(s.storeInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if err := s.saveToFile(); err != nil {
				zap.L().Error("Failed to save metrics", zap.Error(err), zap.String("file", s.filePath))
			}
		case <-s.saveChan:
			if err := s.saveToFile(); err != nil {
				zap.L().Error("Failed to save metrics", zap.Error(err), zap.String("file", s.filePath))
			}
		case <-s.stopChan:
			_ = s.saveToFile()
			return
		}
	}
}

// SaveToFile — публичный метод (для тестов и эндпоинта /save)
func (s *fileStorage) SaveToFile() error {
	return s.saveToFile()
}

func (s *fileStorage) Close() {
	select {
	case <-s.stopChan:
	default:
		close(s.stopChan)
	}
}

func (s *fileStorage) GetAllMetricsForSave() []models.Metrics {
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
