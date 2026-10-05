package database

import (
	"database/sql"
	"embed"
	"fmt"
	"log/slog"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	_ "github.com/lib/pq"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

//go:embed *.sql
var migrationFiles embed.FS

func InitConnection(connectionString string) (*gorm.DB, error) {
	if err := InitMigration(connectionString); err != nil {
		return nil, fmt.Errorf("aplicar migrations: %w", err)
	}
	db, err := gorm.Open(postgres.Open(connectionString), &gorm.Config{Logger: gormlogger.Default.LogMode(gormlogger.Silent), DisableForeignKeyConstraintWhenMigrating: true})
	if err != nil {
		return nil, fmt.Errorf("conectar PostgreSQL: %w", err)
	}
	return db, nil
}
func InitMigration(connectionString string) error {
	source, err := iofs.New(migrationFiles, ".")
	if err != nil {
		return err
	}
	m, err := migrate.NewWithSourceInstance("embedded-migrations", source, connectionString)
	if err != nil {
		_ = source.Close()
		return err
	}
	defer func() {
		sourceErr, databaseErr := m.Close()
		if sourceErr != nil {
			slog.Error("erro ao fechar origem de migration", "error", sourceErr)
		}
		if databaseErr != nil {
			slog.Error("erro ao fechar conexão de migration", "error", databaseErr)
		}
	}()
	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("migration rejeitada; estado preservado: %w", err)
	}
	return nil
}

// OpenIsolatedSQL supports the migration verifier without exposing connection secrets.
func OpenIsolatedSQL(connectionString string) (*sql.DB, error) {
	return sql.Open("postgres", connectionString)
}
