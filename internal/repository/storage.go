package repository

import models "github.com/eugegm01-dev/metrics/internal/model"

// Storage - основной интерфейс хранилища
type Storage interface {
	UpdateGauge(name string, value float64)
	UpdateCounter(name string, value int64)
	GetGauge(name string) (float64, bool)
	GetCounter(name string) (int64, bool)
	GetAllMetrics() string
	Close()
}

// FileSaver - интерфейс для хранилищ, поддерживающих сохранение в файл
type FileSaver interface {
	SaveToFile() error
}

// BatchUpdater - интерфейс для хранилищ, поддерживающих пакетное обновление
type BatchUpdater interface {
	UpdateBatch(metrics []models.Metrics) error
}
