// Package repository contains the code of application's repo
package repository

import models "github.com/eugegm01-dev/metrics/internal/model"

// Storage определяет интерфейс для хранилища метрик.
type Storage interface {
	UpdateGauge(name string, value float64) error
	UpdateCounter(name string, value int64) error
	GetGauge(name string) (float64, bool, error)
	GetCounter(name string) (int64, bool, error)
	GetAllMetrics() (string, error)
	Close() error
}

// FileSaver – опциональный интерфейс для хранилищ, поддерживающих сохранение в файл.
type FileSaver interface {
	SaveToFile() error
}

// BatchUpdater – опциональный интерфейс для хранилищ, поддерживающих пакетное обновление.
type BatchUpdater interface {
	UpdateBatch(metrics []models.Metrics) error
}
