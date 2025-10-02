package store

// this package should wrap the gen results in internal/db.

import (
	"context"
	"fmt"
	"gitea.deepak.science/deepak/trygo/internal/db"
)

// interface representing a backing store
type Store interface {
	CreateUser(ctx context.Context, req *CreateUserRequest) (int, error)
}

type dbStore struct {
	querier db.Querier
}

// can move user stuff elsewhere later
type CreateUserRequest struct {
	Email string `json:"email"`
	Name  string `json:"name"`
}

func (s *dbStore) CreateUser(ctx context.Context, req *CreateUserRequest) (int32, error) {
	if req.Email == "" {
		return -1, fmt.Errorf("No email provided")
	}
	params := db.CreateUserParams{
		Email: req.Email,
		Name:  req.Name,
	}
	user, err := s.querier.CreateUser(ctx, params)
	if err != nil {
		return -1, err
	}
	return user.ID, nil
}
