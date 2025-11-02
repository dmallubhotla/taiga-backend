package store

import (
	"context"
	"fmt"

	"gitea.deepak.science/deepak/taiga/internal/db"
)

// errorStore is a mock store that always returns an error for health checks
type errorStore struct{}

func NewErrorStore() Store {
	return &errorStore{}
}

func (m *errorStore) Healthy(ctx context.Context) error {
	return fmt.Errorf("connection failed")
}

func (m *errorStore) Close() error {
	return nil
}

func (m *errorStore) GetQuerier() (db.Querier, error) {
	return nil, fmt.Errorf("error store can't actually query")
}
