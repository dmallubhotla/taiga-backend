package models

import (
	"context"
	"fmt"
	"gitea.deepak.science/deepak/trygo/internal/config"
	"gitea.deepak.science/deepak/trygo/internal/store"
)

type Model interface {
	Healthy(ctx context.Context) error
	Close() error
	CreateUser(ctx context.Context, req *CreateUserRequest) (*CreateUserResponse, error)
	VerifyUserByEmailPassword(ctx context.Context, email string, password string) (*UserNoPassword, error)
}

type storeModel struct {
	store store.Store
}

func New(cfg *config.Config) (Model, error) {

	s, err := store.GetStore(cfg)
	if err != nil {
		if s != nil {
			s.Close()
		}
		return nil, fmt.Errorf("failed to initialize database :%w", err)
	}

	return &storeModel{
		store: s,
	}, nil
}

// mostly for convenience for testing
func NewFromStore(s store.Store) Model {
	return &storeModel{
		store: s,
	}
}

func (m *storeModel) Close() error {
	return m.store.Close()
}

func (m *storeModel) Healthy(ctx context.Context) error {
	return m.store.Healthy(ctx)
}
