package store

import (
	"context"
	"fmt"
	"github.com/jackc/pgx/v5/pgxpool"
	"time"

	"gitea.deepak.science/deepak/taiga/internal/config"
	"gitea.deepak.science/deepak/taiga/internal/db"
)

type pgStore struct {
	pgPool *pgxpool.Pool
}

// func (s *pgStore) CreateUser(ctx context.Context, params db.CreateUserParams) (int32, error) {
// 	querier := db.New(s.pgPool)
// 	user, err := querier.CreateUser(ctx, params)
// 	if err != nil {
// 		return -1, err
// 	}
// 	return user.ID, nil
// }

func GetPostgresStore(cfg *config.Config) (Store, error) {
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
}

func (s *pgStore) GetQuerier() (db.Querier, error) {
	return db.New(s.pgPool), nil
}

func (s *pgStore) Healthy(ctx context.Context) error {
	if s.pgPool == nil {
		return fmt.Errorf("pool unavailable")
	}
	return s.pgPool.Ping(ctx)
}

func (s *pgStore) Close() error {
	if s.pgPool != nil {
		s.pgPool.Close()
	}
	return nil
}
