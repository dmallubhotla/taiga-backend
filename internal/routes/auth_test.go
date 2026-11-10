package routes_test

import (
	"encoding/json"
	"fmt"
	"io"
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

func TestRegistrationCanBeClosed(t *testing.T) {
	for _, open := range []bool{true, false} {
		t.Run(fmt.Sprintf("open=%v", open), func(t *testing.T) {
			m := migratedModel(t)
			srv := httptest.NewServer(routes.New(m, &mockToker{}, getTestFileRepo(), routes.WithRegistration(open)))
			t.Cleanup(srv.Close)

			resp, err := http.Post(srv.URL+"/auth/register", "application/json", strings.NewReader(`{"email":"new@example.com","display_name":"New","password":"password123"}`))
			require.NoError(t, err)
			body, _ := io.ReadAll(resp.Body)
			_ = resp.Body.Close()
			if open {
				assert.Equal(t, http.StatusCreated, resp.StatusCode, string(body))
			} else {
				assert.Equal(t, http.StatusForbidden, resp.StatusCode)
				assert.Contains(t, string(body), "registration is closed")
			}

			// Login still works for existing accounts either way.
			_, err = m.CreateUser(t.Context(), &models.CreateUserRequest{Email: "existing@example.com", DisplayName: "E", Password: "password123"})
			require.NoError(t, err)
			resp, err = http.Post(srv.URL+"/auth/tokens", "application/json", strings.NewReader(`{"email":"existing@example.com","password":"password123"}`))
			require.NoError(t, err)
			_ = resp.Body.Close()
			assert.Equal(t, http.StatusOK, resp.StatusCode)

			resp, err = http.Get(srv.URL + "/auth/providers")
			require.NoError(t, err)
			var providers struct {
				PasswordLogin    bool     `json:"password_login"`
				RegistrationOpen bool     `json:"registration_open"`
				Providers        []string `json:"providers"`
			}
			require.NoError(t, json.NewDecoder(resp.Body).Decode(&providers))
			_ = resp.Body.Close()
			assert.True(t, providers.PasswordLogin)
			assert.Equal(t, open, providers.RegistrationOpen)
			assert.NotNil(t, providers.Providers, "an empty list, not null")
		})
	}
}
