package store

// this package should wrap the gen results in internal/db.

import (
	"context"
	"database/sql"
	"fmt"
	"log"

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
	var store Store
	var err error
	log.Printf("Getting store with config %+v", cfg)
	switch cfg.Db.Driver {
	case "postgres":
		store, err = GetPostgresStore(cfg)
		if err != nil {
			return nil, fmt.Errorf("tried to get postgres stored and failed")
		}

	case "sqlite":
		store, err = GetSqliteStore(cfg)
		if err != nil {
			return nil, fmt.Errorf("tried to get sqlite stored and failed")
		}
	default:
		return nil, fmt.Errorf("unsupported database driver: %s", cfg.Db.Driver)
	}
	log.Printf("Obtained store, initializing...")
	// init if all good
	err = initDB(cfg)
	if err != nil {
		return nil, fmt.Errorf("Failed to initialize DB: %w", err)
	}
	return store, nil

}

// initDB initializes the database connection
func initDB(cfg *config.Config) error {
	dsn := cfg.Db.DSN()
	if dsn == "" {
		return fmt.Errorf("invalid database configuration")
	}
	// Use database/sql for SQLite (pgx doesn't support SQLite)
	db, err := sql.Open(cfg.Db.Driver, dsn)
	if err != nil {
		return fmt.Errorf("could not open database: %w", err)
	}

	// Initialize migrations
	migrator, err := migration.New(db, cfg.Db.Driver, cfg.Db.MigrationPath)
	if err != nil {
		db.Close()
		return fmt.Errorf("failed to initialize migrator: %w", err)
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
			return fmt.Errorf("failed to run migrations: %w", err)
		}

		// Log current migration version
		if version, dirty, err := migrator.Version(); err == nil {
			log.Printf("Database migration version: %d (dirty: %v)", version, dirty)
		}
	}
	return nil
}
