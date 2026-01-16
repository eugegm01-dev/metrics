package repository

import (
	"database/sql"
	"fmt"
	"sync"
)

type PGStorage struct {
	mu sync.RWMutex
	db *sql.DB
}

func NewPGStorage(db *sql.DB) (*PGStorage, error) {
	// Создать таблицы если их нет
	_, err := db.Exec(`
        CREATE TABLE IF NOT EXISTS gauges (
            name TEXT PRIMARY KEY,
            value DOUBLE PRECISION
        );
        CREATE TABLE IF NOT EXISTS counters (
            name TEXT PRIMARY KEY,
            value BIGINT
        );
    `)
	if err != nil {
		return nil, fmt.Errorf("failed to create tables: %w", err)
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

// ... остальные методы интерфейса Storage
