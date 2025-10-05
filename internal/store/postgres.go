package store

import (
	"context"
	"fmt"
	"github.com/jackc/pgx/v5/pgxpool"

	"gitea.deepak.science/deepak/trygo/internal/db"
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
