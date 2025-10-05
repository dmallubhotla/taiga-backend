package store

// this package should wrap the gen results in internal/db.

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"time"

	"gitea.deepak.science/deepak/trygo/internal/config"
	"gitea.deepak.science/deepak/trygo/internal/db"
	"gitea.deepak.science/deepak/trygo/internal/migration"
)

// interface representing a backing store
type Store interface {
	Healthy(ctx context.Context) error // ping the store's connection
	Close() error                      // cleanup resources
	GetQuerier() (db.Querier, error)
}

func GetStore(cfg *config.Config) (Store, error) {
	switch cfg.Db.Driver {
	case "postgres":
		return GetPostgresStore(cfg)

	case "sqlite":
		return GetSqliteStore(cfg)
	default:
		return nil, fmt.Errorf("unsupported database driver: %s", cfg.Db.Driver)
	}

}

// initDB initializes the database connection
func initDB(cfg *config.Config) (*sql.DB, error) {
	dsn := cfg.Db.DSN()
	if dsn == "" {
		return nil, fmt.Errorf("invalid database configuration")
	}

	db, err := sql.Open(cfg.Db.Driver, dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Test the connection
	if err := db.Ping(); err != nil {
		if closeErr := db.Close(); closeErr != nil {
			log.Printf("Error closing database after ping failure: %v\n", closeErr)
		}
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	// Configure connection pool
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(25)
	db.SetConnMaxLifetime(5 * time.Minute)

	var migrator *migration.Migrator

	// Initialize migrations
	migrator, err = migration.New(db, cfg.Db.Driver, cfg.Db.MigrationPath)
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to initialize migrator: %w", err)
	}

	// Run down migrations if auto_down is enabled and in development
	if cfg.Db.DropOnStart && cfg.App.IsDevelopment() {
		log.Println("Running down migrations...")
		if err := migrator.Down(); err != nil {
			log.Printf("Warning: failed to run down migrations: %v", err)
		}
	}

	// Run migrations if auto_up is enabled
	if cfg.Db.AutoMigrateUp {
		log.Println("Running database migrations...")
		if err := migrator.Up(); err != nil {
			migrator.Close()
			db.Close()
			return nil, fmt.Errorf("failed to run migrations: %w", err)
		}

		// Log current migration version
		if version, dirty, err := migrator.Version(); err == nil {
			log.Printf("Database migration version: %d (dirty: %v)", version, dirty)
		}
	}
	return db, nil
}
