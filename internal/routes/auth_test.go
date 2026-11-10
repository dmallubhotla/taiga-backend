package routes_test

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gitea.deepak.science/deepak/taiga/internal/config"
	"gitea.deepak.science/deepak/taiga/internal/models"
	"gitea.deepak.science/deepak/taiga/internal/routes"
	"gitea.deepak.science/deepak/taiga/internal/store"
)

// migratedModel is a file-backed SQLite model with the real schema; :memory: would give each pooled connection its own empty database.
func migratedModel(t *testing.T) models.Model {
	t.Helper()
	s, err := store.GetStore(&config.Config{
		Db: config.DBConfig{
			Driver:        "sqlite",
			FilePath:      filepath.Join(t.TempDir(), "test.db"),
			MigrationPath: "../../migrations-sqlite",
			AutoMigrateUp: true,
		},
	})
	require.NoError(t, err)
	m := models.NewFromStore(s)
	t.Cleanup(func() { _ = m.Close() })
	return m
}

func TestCreateTokenRejectsBadCredentialsWith401(t *testing.T) {
	m := migratedModel(t)
	_, err := m.CreateUser(t.Context(), &models.CreateUserRequest{Email: "a@example.com", DisplayName: "A", Password: "password123"})
	require.NoError(t, err)
	srv := httptest.NewServer(routes.New(m, &mockToker{}, getTestFileRepo()))
	t.Cleanup(srv.Close)

	post := func(body string) int {
		resp, err := http.Post(srv.URL+"/auth/tokens", "application/json", strings.NewReader(body))
		require.NoError(t, err)
		_ = resp.Body.Close()
		return resp.StatusCode
	}

	assert.Equal(t, http.StatusOK, post(`{"email":"a@example.com","password":"password123"}`))
	assert.Equal(t, http.StatusUnauthorized, post(`{"email":"a@example.com","password":"wrong-password"}`), "wrong password")
	assert.Equal(t, http.StatusUnauthorized, post(`{"email":"nobody@example.com","password":"password123"}`), "unknown email")
	assert.Equal(t, http.StatusBadRequest, post(`{"email":"a@example.com","nope":1}`), "malformed body is still a 400")
}
