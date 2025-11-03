package migration

import (
	"database/sql"
	"fmt"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/database/sqlite3"
	_ "github.com/golang-migrate/migrate/v4/source/file"
)

// Migrator handles database migrations
type Migrator struct {
	migrate *migrate.Migrate
}

// New creates a new migrator instance
func New(db *sql.DB, driverName, migrationsPath string) (*Migrator, error) {
	var driver database.Driver
	var err error

	switch driverName {
	case "postgres":
		driver, err = postgres.WithInstance(db, &postgres.Config{})
	case "sqlite3", "sqlite":
		driver, err = sqlite3.WithInstance(db, &sqlite3.Config{})
	default:
		return nil, fmt.Errorf("unsupported database driver: %s", driverName)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to create database driver: %w", err)
	}

	m, err := migrate.NewWithDatabaseInstance(
		fmt.Sprintf("file://%s", migrationsPath),
		driverName,
		driver,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create migrate instance: %w", err)
	}

	return &Migrator{migrate: m}, nil
}

// Up applies all up migrations
func (m *Migrator) Up() error {
	err := m.migrate.Up()
	if err == migrate.ErrNoChange {
		return nil // No migrations to apply
	}
	return err
}

// Down reverts all migrations
func (m *Migrator) Down() error {
	return m.migrate.Down()
}

// Steps applies n migration steps (positive for up, negative for down)
func (m *Migrator) Steps(n int) error {
	return m.migrate.Steps(n)
}

// Migrate to a specific version
func (m *Migrator) Migrate(version uint) error {
	return m.migrate.Migrate(version)
}

// Version returns the current migration version
func (m *Migrator) Version() (uint, bool, error) {
	return m.migrate.Version()
}

// Close closes the migrator
func (m *Migrator) Close() error {
	sourceErr, dbErr := m.migrate.Close()
	if sourceErr != nil {
		return sourceErr
	}
	return dbErr
}
