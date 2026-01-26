package app

import (
	"context"
	"database/sql"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/eugegm01-dev/metrics/internal/config"
	"github.com/eugegm01-dev/metrics/internal/repository"
	"go.uber.org/zap"
)

// InitDB открывает соединение и выполняет PingContext с retry.
// Возвращает *sql.DB или ошибку (fail early если DSN указан и не удалось подключиться).
func InitDB(cfg *config.ServerConfig, logger *zap.Logger) (*sql.DB, error) {
	if cfg.DatabaseDSN == "" {
		return nil, nil
	}

	db, err := sql.Open("pgx", cfg.DatabaseDSN)
	if err != nil {
		return nil, err
	}

	const maxAttempts = 3
	backoff := time.Second
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		err = db.PingContext(ctx)
		cancel()
		if err == nil {
			return db, nil
		}
		logger.Warn("Database ping attempt failed", zap.Int("attempt", attempt), zap.Error(err))
		if attempt < maxAttempts {
			time.Sleep(backoff)
			backoff *= 2
		}
	}

	_ = db.Close()
	return nil, err
}

// InitStorage создает repository.Storage, инкапсулирует логику выбора реализации.
func InitStorage(cfg *config.ServerConfig, db *sql.DB) (repository.Storage, error) {
	return repository.NewStorage(cfg.FileStoragePath, cfg.StoreInterval, cfg.Restore, db)
}
