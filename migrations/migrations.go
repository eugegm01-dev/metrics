// Package repository contains database migration utilities.
package repository

import (
	"database/sql"
	"fmt"

	"github.com/pressly/goose/v3"
	"go.uber.org/zap"
)

// RunMigrations применяет все ожидающие миграции из указанной директории.
func RunMigrations(db *sql.DB, migrationsDir string) error {
	// Перенаправляем логи goose в zap
	goose.SetLogger(zap.NewStdLog(zap.L()))

	if err := goose.SetDialect("postgres"); err != nil {
		return fmt.Errorf("goose set dialect: %w", err)
	}

	if err := goose.Up(db, migrationsDir); err != nil {
		return fmt.Errorf("goose up: %w", err)
	}

	zap.L().Info("Migrations applied", zap.String("dir", migrationsDir))
	return nil
}
