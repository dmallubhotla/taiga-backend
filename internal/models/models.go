package models

import (
	"context"
	"database/sql"
	"fmt"
	"gitea.deepak.science/deepak/trygo/internal/config"
	"gitea.deepak.science/deepak/trygo/internal/db"
	"gitea.deepak.science/deepak/trygo/internal/store"
	"log"
)

type Model interface {
	Healthy(ctx context.Context) error
	CreateUser(ctx context.Context, req *CreateUserRequest) (int32, error)
	Close() error
}

type storeModel struct {
	store store.Store
}

func New(cfg *config.Config) (Model, error) {

	s, err := store.GetStore(cfg)
	if err != nil {
		s.Close()
		return nil, fmt.Errorf("failed to initialize database :%w", err)
	}

	return &storeModel{
		store: s,
	}, nil
}

func initializeStore(s store.Store) {
	var db *sql.DB
}

type CreateUserRequest struct {
	Email string `json:"email"`
	Name  string `json:"name"`
}

func (m *storeModel) Close() error {
	return m.store.Close()
}

func (m *storeModel) CreateUser(ctx context.Context, req *CreateUserRequest) (int32, error) {
	if req.Email == "" {
		return -1, fmt.Errorf("No email provided")
	}

	params := &db.CreateUserParams{
		Email: req.Email,
		Name:  req.Name,
	}
	querier, err := m.store.GetQuerier()
	if err != nil {
		log.Printf("Could not get a querier")
		return -1, err
	}

	user, err := querier.CreateUser(ctx, params)
	if err != nil {
		log.Printf("Error creating user")
	}

	return user.ID, nil

}

func (m *storeModel) Healthy(ctx context.Context) error {
	return m.store.Healthy(ctx)
}
