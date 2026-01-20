package repository

import (
	"database/sql"
	"fmt"
	"sync"

	models "github.com/eugegm01-dev/metrics/internal/model"
)

type PGStorage struct {
	mu sync.RWMutex
	db *sql.DB
}

func NewPGStorage(db *sql.DB) (*PGStorage, error) {
	migrator := NewMigrator(db)
	if err := migrator.Migrate("migrations"); err != nil {
		// Вместо возврата ошибки просто логируем предупреждение
		fmt.Printf("WARNING: Failed to apply migrations: %v\n", err)
		// Продолжаем создание хранилища (возможно, БД временно недоступна)
	}
	return &PGStorage{db: db}, nil
}

func (s *PGStorage) UpdateGauge(name string, value float64) {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, err := s.db.Exec(`
        INSERT INTO gauges (name, value)
        VALUES ($1, $2)
        ON CONFLICT (name)
        DO UPDATE SET value = $2
    `, name, value)
	if err != nil {
		fmt.Printf("Failed to update gauge: %v\n", err)
	}
}

func (s *PGStorage) UpdateCounter(name string, value int64) {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, err := s.db.Exec(`
        INSERT INTO counters (name, value)
        VALUES ($1, $2)
        ON CONFLICT (name)
        DO UPDATE SET value = counters.value + $2
    `, name, value)
	if err != nil {
		fmt.Printf("Failed to update counter: %v\n", err)
	}
}

func (s *PGStorage) GetGauge(name string) (float64, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var value float64
	err := s.db.QueryRow("SELECT value FROM gauges WHERE name = $1", name).Scan(&value)
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
	err := s.db.QueryRow("SELECT value FROM counters WHERE name = $1", name).Scan(&value)
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

	rows, err := s.db.Query("SELECT name, value FROM gauges")
	if err != nil {
		return fmt.Sprintf("Error retrieving gauges: %v", err)
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
	// ДОБАВЬТЕ ЭТУ ПРОВЕРКУ
	if err := rows.Err(); err != nil {
		result += fmt.Sprintf("Error iterating gauges: %v\n", err)
	}

	rows, err = s.db.Query("SELECT name, value FROM counters")
	if err != nil {
		return fmt.Sprintf("Error retrieving counters: %v", err)
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
	// ДОБАВЬТЕ ЭТУ ПРОВЕРКУ
	if err := rows.Err(); err != nil {
		result += fmt.Sprintf("Error iterating counters: %v\n", err)
	}

	return result
}
func (s *PGStorage) UpdateBatch(metrics []models.Metrics) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	defer func() {
		if err != nil {
			tx.Rollback()
		}
	}()

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
			return fmt.Errorf("failed to update metric %s: %w", metric.ID, err)
		}
	}

	return tx.Commit()
}
func (s *PGStorage) SaveToFile() error {
	return nil
}

func (s *PGStorage) Close() {
	s.db.Close()
}
