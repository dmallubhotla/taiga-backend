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
	"github.com/jackc/pgx/v5/pgxpool"
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
		dsn := cfg.Db.DSN()
		if dsn == "" {
			return nil, fmt.Errorf("invalid database configuration")
		}
		// Use pgxpool for PostgreSQL
		poolConfig, err := pgxpool.ParseConfig(dsn)
		if err != nil {
			return nil, fmt.Errorf("failed to parse postgres config: %w", err)
		}

		// Configure connection pool
		poolConfig.MaxConns = 25
		poolConfig.MinConns = 5
		poolConfig.MaxConnLifetime = 5 * time.Minute
		poolConfig.MaxConnIdleTime = 1 * time.Minute

		pool, err := pgxpool.NewWithConfig(context.Background(), poolConfig)
		if err != nil {
			return nil, fmt.Errorf("failed to create postgres pool: %w", err)
		}

		// Test the connection
		if err := pool.Ping(context.Background()); err != nil {
			pool.Close()
			return nil, fmt.Errorf("failed to ping postgres database: %w", err)
		}

		s := &pgStore{
			pgPool: pool,
		}
		return s, nil

	case "sqlite":
		dsn := cfg.Db.DSN()
		if dsn == "" {
			return nil, fmt.Errorf("invalid database configuration")
		}
		// Use database/sql for SQLite (pgx doesn't support SQLite)
		dbconn, err := sql.Open(cfg.Db.Driver, dsn)
		if err != nil {
			return nil, fmt.Errorf("failed to open sqlite database: %w", err)
		}

		// Test the connection
		if err := dbconn.Ping(); err != nil {
			if closeErr := dbconn.Close(); closeErr != nil {
				log.Printf("Error closing sqlite database after ping failure: %v\n", closeErr)
			}
			return nil, fmt.Errorf("failed to ping sqlite database: %w", err)
		}

		// Configure connection pool
		dbconn.SetMaxOpenConns(25)
		dbconn.SetMaxIdleConns(25)
		dbconn.SetConnMaxLifetime(5 * time.Minute)

		s := &sqliteStore{
			db: dbconn,
		}
		return s, nil
	default:
		return nil, fmt.Errorf("unsupported database driver: %s", cfg.Db.Driver)
	}

}
