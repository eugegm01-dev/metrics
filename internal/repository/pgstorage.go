package repository

import (
	"database/sql"
	"fmt"
	"sync"
	"time"

	models "github.com/eugegm01-dev/metrics/internal/model"
)

type PGStorage struct {
	mu sync.RWMutex
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

		migrator := NewMigrator(db)
		if err := migrator.Migrate("migrations"); err != nil {
			migratorErr = err
			fmt.Printf("Migration attempt %d failed: %v\n", attempt+1, err)

			// Проверяем, нужно ли повторять
			classifier := NewPostgresErrorClassifier()
			if classifier.Classify(err) != Retriable {
				break
			}

			if attempt < 3 {
				fmt.Printf("Retrying migration in %v...\n", delays[attempt])
			}
		} else {
			migratorErr = nil
			break
		}
	}

	if migratorErr != nil {
		fmt.Printf("WARNING: Failed to apply migrations: %v\n", migratorErr)
		// Продолжаем создание хранилища (возможно, БД временно недоступна)
	}

	return &PGStorage{db: db}, nil
}

func (s *PGStorage) execWithRetry(query string, args ...interface{}) (sql.Result, error) {
	delays := []time.Duration{1 * time.Second, 3 * time.Second, 5 * time.Second}

	//var result sql.Result
	var lastErr error

	classifier := NewPostgresErrorClassifier()

	for attempt := 0; attempt <= 3; attempt++ {
		if attempt > 0 {
			time.Sleep(delays[attempt-1])
		}

		res, err := s.db.Exec(query, args...)
		if err == nil {
			return res, nil
		}

		lastErr = err

		// Проверяем, нужно ли повторять
		if classifier.Classify(err) != Retriable {
			return nil, fmt.Errorf("non-retryable error: %w", err)
		}

		if attempt < 3 {
			fmt.Printf("Retryable PostgreSQL error occurred (attempt %d/3): %v. Retrying in %v...\n",
				attempt+1, err, delays[attempt])
		}
	}

	return nil, fmt.Errorf("failed after 3 retries: %w", lastErr)
}

func (s *PGStorage) queryRowWithRetry(query string, args ...interface{}) *sql.Row {
	// Для QueryRow не можем легко сделать retry, так как возвращается Row
	// Вместо этого будем использовать Query и сканировать результат
	return s.db.QueryRow(query, args...)
}

func (s *PGStorage) queryWithRetry(query string, args ...interface{}) (*sql.Rows, error) {
	delays := []time.Duration{1 * time.Second, 3 * time.Second, 5 * time.Second}

	//var rows *sql.Rows
	var lastErr error

	classifier := NewPostgresErrorClassifier()

	for attempt := 0; attempt <= 3; attempt++ {
		if attempt > 0 {
			time.Sleep(delays[attempt-1])
		}

		r, err := s.db.Query(query, args...)
		if err == nil {
			return r, nil
		}

		lastErr = err

		// Проверяем, нужно ли повторять
		if classifier.Classify(err) != Retriable {
			return nil, fmt.Errorf("non-retryable error: %w", err)
		}

		if attempt < 3 {
			fmt.Printf("Retryable PostgreSQL error occurred (attempt %d/3): %v. Retrying in %v...\n",
				attempt+1, err, delays[attempt])
		}
	}

	return nil, fmt.Errorf("failed after 3 retries: %w", lastErr)
}

func (s *PGStorage) UpdateGauge(name string, value float64) {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, err := s.execWithRetry(`
        INSERT INTO gauges (name, value)
        VALUES ($1, $2)
        ON CONFLICT (name)
        DO UPDATE SET value = $2
    `, name, value)

	if err != nil {
		fmt.Printf("Failed to update gauge after retries: %v\n", err)
	}
}

func (s *PGStorage) UpdateCounter(name string, value int64) {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, err := s.execWithRetry(`
        INSERT INTO counters (name, value)
        VALUES ($1, $2)
        ON CONFLICT (name)
        DO UPDATE SET value = counters.value + $2
    `, name, value)

	if err != nil {
		fmt.Printf("Failed to update counter after retries: %v\n", err)
	}
}

func (s *PGStorage) GetGauge(name string) (float64, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var value float64
	err := s.queryRowWithRetry("SELECT value FROM gauges WHERE name = $1", name).Scan(&value)

	if err != nil {
		if err == sql.ErrNoRows {
			return 0, false
		}
		fmt.Printf("Failed to get gauge: %v\n", err)
		return 0, false
	}
	return value, true
}

func (s *PGStorage) GetCounter(name string) (int64, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var value int64
	err := s.queryRowWithRetry("SELECT value FROM counters WHERE name = $1", name).Scan(&value)

	if err != nil {
		if err == sql.ErrNoRows {
			return 0, false
		}
		fmt.Printf("Failed to get counter: %v\n", err)
		return 0, false
	}
	return value, true
}

func (s *PGStorage) GetAllMetrics() string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result string

	// Гарантируем закрытие rows
	rows, err := s.queryWithRetry("SELECT name, value FROM gauges")
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

	rows, err = s.queryWithRetry("SELECT name, value FROM counters")
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

func (s *PGStorage) UpdateBatch(metrics []models.Metrics) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	delays := []time.Duration{1 * time.Second, 3 * time.Second, 5 * time.Second}

	var lastErr error
	classifier := NewPostgresErrorClassifier()

	for attempt := 0; attempt <= 3; attempt++ {
		if attempt > 0 {
			time.Sleep(delays[attempt-1])
		}

		tx, err := s.db.Begin()
		if err != nil {
			lastErr = err
			if classifier.Classify(err) != Retriable {
				return fmt.Errorf("non-retryable error starting transaction: %w", err)
			}
			if attempt < 3 {
				fmt.Printf("Retryable error starting transaction (attempt %d/3): %v\n",
					attempt+1, err)
			}
			continue
		}

		defer func() {
			if lastErr != nil {
				tx.Rollback()
			}
		}()

		success := true
		for _, metric := range metrics {
			switch metric.MType {
			case models.Gauge:
				_, err = tx.Exec(`
                    INSERT INTO gauges (name, value)
                    VALUES ($1, $2)
                    ON CONFLICT (name)
                    DO UPDATE SET value = $2
                `, metric.ID, *metric.Value)
			case models.Counter:
				_, err = tx.Exec(`
                    INSERT INTO counters (name, value)
                    VALUES ($1, $2)
                    ON CONFLICT (name)
                    DO UPDATE SET value = counters.value + $2
                `, metric.ID, *metric.Delta)
			}

			if err != nil {
				lastErr = fmt.Errorf("failed to update metric %s: %w", metric.ID, err)
				success = false
				break
			}
		}

		if success {
			if err := tx.Commit(); err != nil {
				lastErr = err
				if classifier.Classify(err) != Retriable {
					return fmt.Errorf("non-retryable error committing transaction: %w", err)
				}
				if attempt < 3 {
					fmt.Printf("Retryable error committing transaction (attempt %d/3): %v\n",
						attempt+1, err)
				}
				continue
			}
			return nil
		}
	}

	return fmt.Errorf("batch update failed after 3 retries: %w", lastErr)
}

func (s *PGStorage) SaveToFile() error {
	return nil
}

func (s *PGStorage) Close() {
	s.db.Close()
}
