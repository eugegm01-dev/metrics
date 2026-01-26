package repository

import (
	"database/sql"
	"time"
)

func NewStorage(filePath string, storeInterval time.Duration, restore bool, db *sql.DB) (Storage, error) {
	if db != nil {
		return NewPGStorage(db)
	}

	if filePath == "" {
		return NewMemStorage(), nil
	}

	return NewFileStorage(filePath, storeInterval, restore)
}
