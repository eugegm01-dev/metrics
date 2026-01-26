package repository

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"go.uber.org/zap"

	models "github.com/eugegm01-dev/metrics/pkg/model"
)

type PGStorage struct {
	db *sql.DB
}

func NewPGStorage(db *sql.DB) (*PGStorage, error) {
	// Пытаемся применить миграции с retry
	delays := []time.Duration{1 * time.Second, 3 * time.Second, 5 * time.Second}

	var migratorErr error
	for attempt := 0; attempt <= 3; attempt++ {
		if attempt > 0 {
			time.Sleep(delays[attempt-1])
		}

		if err := RunMigrations(db, "migrations"); err != nil {
			migratorErr = err
			zap.L().Warn("Migration attempt failed", zap.Int("attempt", attempt+1), zap.Error(err))
			classifier := NewPostgresErrorClassifier()
			if classifier.Classify(err) != Retriable {
				break
			}
			if attempt < 3 {
				zap.L().Debug("Retrying migration", zap.Duration("delay", delays[attempt]))
			}
		} else {
			migratorErr = nil
			break
		}
	}

	if migratorErr != nil {
		zap.L().Warn("Failed to apply migrations", zap.Error(migratorErr))
	}

	return &PGStorage{db: db}, nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// execWithRetry выполняет одиночный ExecContext с retry и уважением ctx.
func (s *PGStorage) execWithRetry(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	delays := []time.Duration{1 * time.Second, 3 * time.Second, 5 * time.Second}
	var lastErr error
	classifier := NewPostgresErrorClassifier()

	for attempt := 0; attempt <= 3; attempt++ {
		// respect context cancellation
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		if attempt > 0 {
			var delay time.Duration
			if attempt-1 < len(delays) {
				delay = delays[attempt-1]
			} else {
				delay = delays[len(delays)-1]
			}
			timer := time.NewTimer(delay)
			select {
			case <-ctx.Done():
				if !timer.Stop() {
					<-timer.C
				}
				return nil, ctx.Err()
			case <-timer.C:
			}
		}

		res, err := s.db.ExecContext(ctx, query, args...)
		if err == nil {
			return res, nil
		}
		lastErr = err

		if classifier.Classify(err) != Retriable {
			return nil, fmt.Errorf("non-retryable error: %w", err)
		}

		if attempt < 3 {
			zap.L().Sugar().Debugf("Retryable PostgreSQL error occurred (attempt %d/3): %v. Retrying in %v",
				attempt+1, err, delays[min(attempt, len(delays)-1)])
		}
	}

	return nil, fmt.Errorf("failed after 3 retries: %w", lastErr)
}

// queryRowWithRetry возвращает Row, вызывающий код должен использовать ctx и Scan.
func (s *PGStorage) queryRowWithRetry(ctx context.Context, query string, args ...interface{}) (*sql.Row, error) {
	return s.db.QueryRowContext(ctx, query, args...), nil
}

// queryWithRetry выполняет QueryContext с retry и уважением ctx.
func (s *PGStorage) queryWithRetry(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
	delays := []time.Duration{1 * time.Second, 3 * time.Second, 5 * time.Second}
	var lastErr error
	classifier := NewPostgresErrorClassifier()

	for attempt := 0; attempt <= 3; attempt++ {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		if attempt > 0 {
			var delay time.Duration
			if attempt-1 < len(delays) {
				delay = delays[attempt-1]
			} else {
				delay = delays[len(delays)-1]
			}
			timer := time.NewTimer(delay)
			select {
			case <-ctx.Done():
				if !timer.Stop() {
					<-timer.C
				}
				return nil, ctx.Err()
			case <-timer.C:
			}
		}

		r, err := s.db.QueryContext(ctx, query, args...)
		if err == nil {
			return r, nil
		}
		lastErr = err

		if classifier.Classify(err) != Retriable {
			return nil, fmt.Errorf("non-retryable error: %w", err)
		}

		if attempt < 3 {
			zap.L().Sugar().Debugf("Retryable PostgreSQL error occurred (attempt %d/3): %v. Retrying in %v",
				attempt+1, err, delays[attempt])
		}
	}

	return nil, fmt.Errorf("failed after 3 retries: %w", lastErr)
}

// UpdateGauge соответствует интерфейсу Storage (без context).
func (s *PGStorage) UpdateGauge(name string, value float64) {
	ctx := context.Background()
	_, err := s.execWithRetry(ctx, `
        INSERT INTO gauges (name, value)
        VALUES ($1, $2)
        ON CONFLICT (name)
        DO UPDATE SET value = $2
    `, name, value)

	if err != nil {
		zap.L().Error("Failed to update gauge after retries", zap.Error(err))
	}
}

// UpdateCounter соответствует интерфейсу Storage (без context).
func (s *PGStorage) UpdateCounter(name string, value int64) {
	ctx := context.Background()
	_, err := s.execWithRetry(ctx, `
        INSERT INTO counters (name, value)
        VALUES ($1, $2)
        ON CONFLICT (name)
        DO UPDATE SET value = counters.value + $2
    `, name, value)

	if err != nil {
		zap.L().Error("Failed to update counter after retries", zap.Error(err))
	}
}

func (s *PGStorage) GetGauge(name string) (float64, bool) {
	ctx := context.Background()
	var value float64
	row, _ := s.queryRowWithRetry(ctx, "SELECT value FROM gauges WHERE name = $1", name)
	err := row.Scan(&value)

	if err != nil {
		if err == sql.ErrNoRows {
			return 0, false
		}
		zap.L().Error("Failed to get gauge", zap.Error(err))
		return 0, false
	}
	return value, true
}

func (s *PGStorage) GetCounter(name string) (int64, bool) {
	ctx := context.Background()
	var value int64
	row, _ := s.queryRowWithRetry(ctx, "SELECT value FROM counters WHERE name = $1", name)
	err := row.Scan(&value)

	if err != nil {
		if err == sql.ErrNoRows {
			return 0, false
		}
		zap.L().Error("Failed to get counter", zap.Error(err))
		return 0, false
	}
	return value, true
}

func (s *PGStorage) GetAllMetrics() string {
	ctx := context.Background()

	var result string

	// Gauges
	rows, err := s.queryWithRetry(ctx, "SELECT name, value FROM gauges")
	if err != nil {
		result += fmt.Sprintf("Error retrieving gauges: %v\n", err)
	} else {
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
	}

	// Counters
	rows, err = s.queryWithRetry(ctx, "SELECT name, value FROM counters")
	if err != nil {
		result += fmt.Sprintf("Error retrieving counters: %v\n", err)
	} else {
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
	}

	return result
}

// withTxRetry выполняет fn внутри транзакции с retry.
// fn получает *sql.Tx и должен вернуть nil при успехе или ошибку.
// Если fn возвращает retryable ошибку, обёртка повторит попытку.
func (s *PGStorage) withTxRetry(ctx context.Context, attempts int, delays []time.Duration, fn func(*sql.Tx) error) error {
	classifier := NewPostgresErrorClassifier()
	var lastErr error

	for attempt := 0; attempt < attempts; attempt++ {
		// respect context cancellation before starting attempt
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		tx, err := s.db.BeginTx(ctx, nil)
		if err != nil {
			lastErr = err
			if classifier.Classify(err) != Retriable {
				return fmt.Errorf("non-retryable error starting transaction: %w", err)
			}
			if attempt+1 < attempts {
				delay := delays[min(attempt, len(delays)-1)]
				timer := time.NewTimer(delay)
				select {
				case <-ctx.Done():
					if !timer.Stop() {
						<-timer.C
					}
					return ctx.Err()
				case <-timer.C:
				}
			}
			continue
		}

		// call user function
		err = fn(tx)
		if err != nil {
			_ = tx.Rollback()
			lastErr = err
			if classifier.Classify(err) != Retriable {
				return fmt.Errorf("non-retryable error in tx function: %w", err)
			}
			if attempt+1 < attempts {
				delay := delays[min(attempt, len(delays)-1)]
				timer := time.NewTimer(delay)
				select {
				case <-ctx.Done():
					if !timer.Stop() {
						<-timer.C
					}
					return ctx.Err()
				case <-timer.C:
				}
				continue
			}
			break
		}

		// try commit
		if err := tx.Commit(); err != nil {
			lastErr = err
			if classifier.Classify(err) != Retriable {
				_ = tx.Rollback()
				return fmt.Errorf("non-retryable error committing transaction: %w", err)
			}
			if attempt+1 < attempts {
				delay := delays[min(attempt, len(delays)-1)]
				timer := time.NewTimer(delay)
				select {
				case <-ctx.Done():
					if !timer.Stop() {
						<-timer.C
					}
					return ctx.Err()
				case <-timer.C:
				}
				continue
			}
			break
		}

		// success
		return nil
	}

	if lastErr != nil {
		return fmt.Errorf("transaction failed after %d attempts: %w", attempts, lastErr)
	}
	return fmt.Errorf("transaction failed after %d attempts", attempts)
}

// UpdateBatch через withTxRetry
func (s *PGStorage) UpdateBatch(metrics []models.Metrics) error {
	ctx := context.Background()
	attempts := 3
	delays := []time.Duration{1 * time.Second, 3 * time.Second, 5 * time.Second}

	fn := func(tx *sql.Tx) error {
		for _, metric := range metrics {
			switch metric.MType {
			case models.Gauge:
				if _, err := tx.ExecContext(ctx, `
                    INSERT INTO gauges (name, value)
                    VALUES ($1, $2)
                    ON CONFLICT (name)
                    DO UPDATE SET value = $2
                `, metric.ID, *metric.Value); err != nil {
					return fmt.Errorf("failed to update gauge %s: %w", metric.ID, err)
				}
			case models.Counter:
				if _, err := tx.ExecContext(ctx, `
                    INSERT INTO counters (name, value)
                    VALUES ($1, $2)
                    ON CONFLICT (name)
                    DO UPDATE SET value = counters.value + $2
                `, metric.ID, *metric.Delta); err != nil {
					return fmt.Errorf("failed to update counter %s: %w", metric.ID, err)
				}
			default:
				return fmt.Errorf("unknown metric type for %s", metric.ID)
			}
		}
		return nil
	}

	if err := s.withTxRetry(ctx, attempts, delays, fn); err != nil {
		zap.L().Error("UpdateBatch failed", zap.Error(err))
		return err
	}
	return nil
}

// Close возвращает ошибку (соответствует интерфейсу Storage)
func (s *PGStorage) Close() {
	if err := s.db.Close(); err != nil {
		zap.L().Warn("Failed to close DB", zap.Error(err))
	}
}
