package repository

import (
	"database/sql"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

type Migrator struct {
	db *sql.DB
}

func NewMigrator(db *sql.DB) *Migrator {
	return &Migrator{db: db}
}

func (m *Migrator) Migrate(migrationsDir string) error {
	// Создаем таблицу для отслеживания миграций
	_, err := m.db.Exec(`
        CREATE TABLE IF NOT EXISTS schema_migrations (
            version BIGINT PRIMARY KEY
        );
    `)
	if err != nil {
		return fmt.Errorf("failed to create schema_migrations table: %w", err)
	}

	// Получаем список примененных миграций
	rows, err := m.db.Query("SELECT version FROM schema_migrations ORDER BY version")
	if err != nil {
		return fmt.Errorf("failed to query applied migrations: %w", err)
	}
	defer rows.Close()

	applied := make(map[int64]bool)
	for rows.Next() {
		var version int64
		if err := rows.Scan(&version); err != nil {
			return fmt.Errorf("failed to scan version: %w", err)
		}
		applied[version] = true
	}

	// Читаем файлы миграций
	files, err := ioutil.ReadDir(migrationsDir)
	if err != nil {
		return fmt.Errorf("failed to read migrations directory: %w", err)
	}

	sort.Slice(files, func(i, j int) bool {
		return files[i].Name() < files[j].Name()
	})

	for _, file := range files {
		if file.IsDir() {
			continue
		}
		filename := file.Name()
		if !strings.HasSuffix(filename, ".up.sql") {
			continue
		}

		base := strings.TrimSuffix(filename, ".up.sql")
		parts := strings.Split(base, "_")
		if len(parts) == 0 {
			continue
		}
		version, err := strconv.ParseInt(parts[0], 10, 64)
		if err != nil {
			return fmt.Errorf("invalid migration version in filename %s: %w", filename, err)
		}

		if applied[version] {
			continue
		}

		path := filepath.Join(migrationsDir, filename)
		sqlBytes, err := ioutil.ReadFile(path)
		if err != nil {
			return fmt.Errorf("failed to read migration file %s: %w", path, err)
		}
		sql := string(sqlBytes)

		tx, err := m.db.Begin()
		if err != nil {
			return fmt.Errorf("failed to begin transaction: %w", err)
		}

		_, err = tx.Exec(sql)
		if err != nil {
			tx.Rollback()
			return fmt.Errorf("failed to execute migration %s: %w", filename, err)
		}

		_, err = tx.Exec("INSERT INTO schema_migrations (version) VALUES ($1)", version)
		if err != nil {
			tx.Rollback()
			return fmt.Errorf("failed to insert migration record: %w", err)
		}

		if err := tx.Commit(); err != nil {
			return fmt.Errorf("failed to commit transaction: %w", err)
		}

		fmt.Printf("Applied migration: %s\n", filename)
	}

	return nil
}
