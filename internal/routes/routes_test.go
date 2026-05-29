package routes_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gitea.deepak.science/deepak/taiga/internal/config"
	"gitea.deepak.science/deepak/taiga/internal/filerepo"
	"gitea.deepak.science/deepak/taiga/internal/models"
	"gitea.deepak.science/deepak/taiga/internal/routes"
	"gitea.deepak.science/deepak/taiga/internal/store"
	"gitea.deepak.science/deepak/taiga/internal/tokens"
	_ "modernc.org/sqlite" // SQLite driver
)

// mockToker is a test implementation of tokens.Toker that doesn't require files
type mockToker struct{}

// mockFileRepo is a test implementation of filerepo.FileRepo
type mockFileRepo struct{}

func (m *mockFileRepo) Store(ctx context.Context, r io.Reader) (hash string, err error) {
	return "mock-hash", nil
}

func (m *mockFileRepo) Exists(ctx context.Context, hash string) (exists bool, err error) {
	return true, nil
}

func (m *mockFileRepo) Fetch(ctx context.Context, hash string) (rc io.ReadCloser, err error) {
	return io.NopCloser(strings.NewReader("mock file content")), nil
}

func (m *mockToker) EncodeUser(userToken *tokens.UserToken) (string, error) {
	return "mock-token-" + userToken.Email, nil
}

func (m *mockToker) DecodeTokenString(tokenString string) (*tokens.UserToken, error) {
	return &tokens.UserToken{
		ID:    123,
		Email: "test@example.com",
	}, nil
}

func (m *mockToker) Authenticator(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		next.ServeHTTP(w, r)
	})
}

// getTestModel returns an in-memory model for testing
func getTestModel(t *testing.T) models.Model {
	cfg := &config.Config{
		Db: config.DBConfig{
			Driver:   "sqlite",
			FilePath: ":memory:",
		},
	}
	s, err := store.GetStore(cfg)
	require.NoError(t, err)
	return models.NewFromStore(s)
}

// getTestTokens returns a test tokens instance
func getTestTokens() tokens.Toker {
	cfg := config.Config{
		App: config.AppConfig{
			Environment: "test",
		},
	}
	toker, err := tokens.New(cfg)
	if err != nil {
		// Return a mock toker for testing when file-based tokens fail
		return &mockToker{}
	}
	return toker
}

// getTestFileRepo returns a test filerepo instance
func getTestFileRepo() filerepo.FileRepo {
	return &mockFileRepo{}
}

func TestNew(t *testing.T) {
	m := getTestModel(t)
	defer func() { _ = m.Close() }()

	h := routes.New(m, getTestTokens(), getTestFileRepo())
	assert.NotNil(t, h)
}

func TestNewWithNilStore(t *testing.T) {
	h := routes.New(nil, getTestTokens(), getTestFileRepo())
	assert.NotNil(t, h)
}

func TestSetupRoutes(t *testing.T) {
	router := routes.New(nil, getTestTokens(), getTestFileRepo())
	assert.NotNil(t, router)
}

func TestHello(t *testing.T) {
	router := routes.New(nil, getTestTokens(), getTestFileRepo())

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, "Hello, World!", response["message"])
	assert.Equal(t, "success", response["status"])
	assert.Equal(t, "trygo-template", response["service"])
}

func TestPing(t *testing.T) {
	router := routes.New(nil, getTestTokens(), getTestFileRepo())

	req := httptest.NewRequest(http.MethodGet, "/ping", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

	var response map[string]string
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, "pong", response["ping"])
}

func TestHealthWithNilStore(t *testing.T) {
	router := routes.New(nil, getTestTokens(), getTestFileRepo())

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, "degraded", response["status"])
	assert.Equal(t, "trygo-template", response["service"])

	database, ok := response["database"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "no store configured", database["status"])
	assert.False(t, database["healthy"].(bool))
}

func TestHealthWithHealthyStore(t *testing.T) {
	m := getTestModel(t)
	defer func() { _ = m.Close() }()

	router := routes.New(m, getTestTokens(), getTestFileRepo())

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, "ok", response["status"])
	assert.Equal(t, "trygo-template", response["service"])

	database, ok := response["database"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "healthy", database["status"])
	assert.True(t, database["healthy"].(bool))
}

func TestHealthWithUnhealthyStore(t *testing.T) {
	s := store.NewErrorStore()
	m := models.NewFromStore(s)
	defer func() { _ = m.Close() }()

	router := routes.New(m, getTestTokens(), getTestFileRepo())

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, "degraded", response["status"])
	assert.Equal(t, "trygo-template", response["service"])

	database, ok := response["database"].(map[string]interface{})
	require.True(t, ok)
	assert.Contains(t, database["status"].(string), "unhealthy:")
	assert.False(t, database["healthy"].(bool))
}

func TestRoutesExist(t *testing.T) {
	router := routes.New(nil, getTestTokens(), getTestFileRepo())

	tests := []struct {
		method string
		path   string
	}{
		{http.MethodGet, "/"},
		{http.MethodGet, "/ping"},
		{http.MethodGet, "/health"},
	}

	for _, tt := range tests {
		t.Run(tt.method+"_"+tt.path, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.path, nil)
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			// Should not return 404
			assert.NotEqual(t, http.StatusNotFound, w.Code)
		})
	}
}

func TestInvalidRoutes(t *testing.T) {
	router := routes.New(nil, getTestTokens(), getTestFileRepo())

	tests := []struct {
		method string
		path   string
	}{
		{http.MethodGet, "/nonexistent"},
		{http.MethodPost, "/"},
		{http.MethodPut, "/ping"},
		{http.MethodDelete, "/health"},
	}

	for _, tt := range tests {
		t.Run(tt.method+"_"+tt.path, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.path, nil)
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			if tt.path == "/nonexistent" {
				assert.Equal(t, http.StatusNotFound, w.Code)
			} else {
				// POST/PUT/DELETE on existing paths should return method not allowed
				assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
			}
		})
	}
}
