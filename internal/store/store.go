package store

// this package should wrap the gen results in internal/db.

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"gitea.deepak.science/deepak/trygo/internal/config"
)

// interface representing a backing store
type Store interface {
	Healthy(ctx context.Context) error // ping the store's connection
	CreateUser(ctx context.Context, req *CreateUserRequest) (int32, error)
	Close() error // cleanup resources
}

func GetStore(cfg *config.Config) (Store, error) {
	dsn := cfg.Db.DSN()
	if dsn == "" {
		return nil, fmt.Errorf("invalid database configuration")
	}

	switch cfg.Db.Driver {
	case "postgres":
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

// can move user stuff elsewhere later
type CreateUserRequest struct {
	Email string `json:"email"`
	Name  string `json:"name"`
}
