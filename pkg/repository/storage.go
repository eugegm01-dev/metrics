package repository

import "github.com/eugegm01-dev/metrics/pkg/model"

// Storage описывает общие операции хранилища
type Storage interface {
	UpdateGauge(name string, value float64)
	UpdateCounter(name string, value int64)
	GetGauge(name string) (float64, bool)
	GetCounter(name string) (int64, bool)
	GetAllMetrics() string
	UpdateBatch(metrics []model.Metrics) error
	Close()
}

// Saver описывает хранилище, которое умеет сохраняться на диск.
type Saver interface {
	SaveToFile() error
}
