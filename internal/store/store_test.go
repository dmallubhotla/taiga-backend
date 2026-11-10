package store_test

import (
	"context"
	"errors"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gitea.deepak.science/deepak/taiga/internal/config"
	"gitea.deepak.science/deepak/taiga/internal/db"
	"gitea.deepak.science/deepak/taiga/internal/store"
)

func newSqliteStore(t *testing.T) store.Store {
	t.Helper()
	_, file, _, _ := runtime.Caller(0)
	s, err := store.GetStore(&config.Config{Db: config.DBConfig{
		Driver:        "sqlite",
		FilePath:      filepath.Join(t.TempDir(), "test.db"),
		MigrationPath: filepath.Join(filepath.Dir(file), "..", "..", "migrations-sqlite"),
		AutoMigrateUp: true,
	}})
	require.NoError(t, err)
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func TestInTxCommitsOrRollsBack(t *testing.T) {
	ctx := context.Background()
	s := newSqliteStore(t)
	boom := errors.New("boom")

	err := s.InTx(ctx, func(q db.Querier) error {
		if _, err := q.CreateUser(ctx, &db.CreateUserParams{Email: "rolled@example.com", DisplayName: "r"}); err != nil {
			return err
		}
		return boom
	})
	require.ErrorIs(t, err, boom)

	require.NoError(t, s.InTx(ctx, func(q db.Querier) error {
		_, err := q.CreateUser(ctx, &db.CreateUserParams{Email: "kept@example.com", DisplayName: "k"})
		return err
	}))

	q, err := s.GetQuerier()
	require.NoError(t, err)
	_, err = q.SelectEmailPasswordForAuth(ctx, "rolled@example.com")
	assert.Error(t, err, "the failed transaction's insert was rolled back")
	_, err = q.SelectEmailPasswordForAuth(ctx, "kept@example.com")
	assert.NoError(t, err)
}
