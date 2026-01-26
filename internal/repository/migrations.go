// internal/repository/migrations.go
package repository

import (
	"database/sql"
	"fmt"

	"github.com/pressly/goose/v3"
	"go.uber.org/zap"
)

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
