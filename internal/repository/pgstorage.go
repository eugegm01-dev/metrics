package repository

import (
	"context"
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"path/filepath"
	"runtime"
	"time"

	models "github.com/eugegm01-dev/metrics/internal/model"
	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/pressly/goose/v3"
)

var migrationFiles embed.FS

type PGStorage struct {
	db *sql.DB
}

func NewPGStorage(db *sql.DB) (*PGStorage, error) {
	if db == nil {
		return nil, fmt.Errorf("database connection is nil")
	}

	// Определяем путь к миграциям
	_, filename, _, _ := runtime.Caller(0)
	currentDir := filepath.Dir(filename)
	migrationsDir := filepath.Join(currentDir, "../../migrations")

	goose.SetBaseFS(migrationFiles) // если используете embed
	if err := goose.SetDialect("postgres"); err != nil {
		return nil, fmt.Errorf("goose set dialect: %w", err)
	}

	if err := goose.Up(db, migrationsDir); err != nil {
		return nil, fmt.Errorf("failed to apply migrations: %w", err)
	}

	return &PGStorage{db: db}, nil
}

// executeWithRetry выполняет операцию с ретраями для retryable ошибок
func (s *PGStorage) executeWithRetry(ctx context.Context, operation func() error) error {
	delays := []time.Duration{1 * time.Second, 3 * time.Second, 5 * time.Second}

	for attempt := 0; attempt <= len(delays); attempt++ {
		err := operation()
		if err == nil {
			return nil
		}

		// Проверяем, является ли ошибка retryable
		if !isRetryableError(err) {
			return fmt.Errorf("non-retryable error: %w", err)
		}

		if attempt == len(delays) {
			break
		}

		// Используем Timer с поддержкой контекста
		timer := time.NewTimer(delays[attempt])
		select {
		case <-ctx.Done():
			if !timer.Stop() {
				<-timer.C
			}
			return ctx.Err()
		case <-timer.C:
			// Продолжаем со следующей попыткой
		}
	}

	return fmt.Errorf("operation failed after %d retries", len(delays))
}

// isRetryableError проверяет, является ли ошибка PostgreSQL retryable
func isRetryableError(err error) bool {
	var pgErr *pgconn.PgError
	if ok := errors.As(err, &pgErr); ok {
		switch pgErr.Code {
		case pgerrcode.ConnectionException,
			pgerrcode.ConnectionDoesNotExist,
			pgerrcode.ConnectionFailure,
			pgerrcode.SQLClientUnableToEstablishSQLConnection,
			pgerrcode.SQLServerRejectedEstablishmentOfSQLConnection,
			pgerrcode.TransactionResolutionUnknown,
			pgerrcode.SerializationFailure,
			pgerrcode.DeadlockDetected:
			return true
		}
	}
	return false
}

// executeInTransaction выполняет функцию в транзакции с ретраями
func (s *PGStorage) executeInTransaction(ctx context.Context, fn func(tx *sql.Tx) error) error {
	return s.executeWithRetry(ctx, func() error {
		tx, err := s.db.BeginTx(ctx, nil)
		if err != nil {
			return err
		}

		defer func() {
			if p := recover(); p != nil {
				tx.Rollback()
				panic(p)
			}
		}()

		if err := fn(tx); err != nil {
			tx.Rollback()
			return err
		}

		return tx.Commit()
	})
}

func (s *PGStorage) UpdateGauge(name string, value float64) {
	ctx := context.Background()
	s.executeWithRetry(ctx, func() error {
		_, err := s.db.ExecContext(ctx, `
			INSERT INTO gauges (name, value)
			VALUES ($1, $2)
			ON CONFLICT (name)
			DO UPDATE SET value = $2
		`, name, value)
		return err
	})
}

func (s *PGStorage) UpdateCounter(name string, value int64) {
	ctx := context.Background()
	s.executeWithRetry(ctx, func() error {
		_, err := s.db.ExecContext(ctx, `
			INSERT INTO counters (name, value)
			VALUES ($1, $2)
			ON CONFLICT (name)
			DO UPDATE SET value = counters.value + $2
		`, name, value)
		return err
	})
}

func (s *PGStorage) GetGauge(name string) (float64, bool) {
	var value float64
	ctx := context.Background()
	err := s.executeWithRetry(ctx, func() error {
		return s.db.QueryRowContext(ctx,
			"SELECT value FROM gauges WHERE name = $1", name).Scan(&value)
	})

	if err != nil {
		if err == sql.ErrNoRows {
			return 0, false
		}
		return 0, false
	}
	return value, true
}

func (s *PGStorage) GetCounter(name string) (int64, bool) {
	var value int64
	ctx := context.Background()
	err := s.executeWithRetry(ctx, func() error {
		return s.db.QueryRowContext(ctx,
			"SELECT value FROM counters WHERE name = $1", name).Scan(&value)
	})

	if err != nil {
		if err == sql.ErrNoRows {
			return 0, false
		}
		return 0, false
	}
	return value, true
}

func (s *PGStorage) GetAllMetrics() string {
	var result string

	ctx := context.Background()
	err := s.executeWithRetry(ctx, func() error {
		rows, err := s.db.QueryContext(ctx, "SELECT name, value FROM gauges")
		if err != nil {
			result += fmt.Sprintf("Error retrieving gauges: %v\n", err)
			return nil
		}
		defer rows.Close()

		result += "Gauges:\n"
		for rows.Next() {
			var name string
			var value float64
			if err := rows.Scan(&name, &value); err != nil {
				continue
			}
			result += fmt.Sprintf(" %s: %f\n", name, value)
		}
		if err := rows.Err(); err != nil {
			result += fmt.Sprintf("Error iterating gauges: %v\n", err)
		}

		rows, err = s.db.QueryContext(ctx, "SELECT name, value FROM counters")
		if err != nil {
			result += fmt.Sprintf("Error retrieving counters: %v\n", err)
			return nil
		}
		defer rows.Close()

		result += "Counters:\n"
		for rows.Next() {
			var name string
			var value int64
			if err := rows.Scan(&name, &value); err != nil {
				continue
			}
			result += fmt.Sprintf(" %s: %d\n", name, value)
		}
		if err := rows.Err(); err != nil {
			result += fmt.Sprintf("Error iterating counters: %v\n", err)
		}

		return nil
	})

	if err != nil {
		result += fmt.Sprintf("Error executing query: %v\n", err)
	}

	return result
}

func (s *PGStorage) UpdateBatch(metrics []models.Metrics) error {
	ctx := context.Background()
	return s.executeInTransaction(ctx, func(tx *sql.Tx) error {
		for _, metric := range metrics {
			var err error
			switch metric.MType {
			case models.Gauge:
				_, err = tx.ExecContext(ctx, `
                    INSERT INTO gauges (name, value)
                    VALUES ($1, $2)
                    ON CONFLICT (name)
                    DO UPDATE SET value = $2
                `, metric.ID, *metric.Value)
			case models.Counter:
				_, err = tx.ExecContext(ctx, `
                    INSERT INTO counters (name, value)
                    VALUES ($1, $2)
                    ON CONFLICT (name)
                    DO UPDATE SET value = counters.value + $2
                `, metric.ID, *metric.Delta)
			}

			if err != nil {
				return fmt.Errorf("failed to update metric %s: %w", metric.ID, err)
			}
		}
		return nil
	})
}

func (s *PGStorage) Close() {
	s.db.Close()
}
