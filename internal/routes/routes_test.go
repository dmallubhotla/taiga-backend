package routes_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gitea.deepak.science/deepak/trygo/internal/config"
	"gitea.deepak.science/deepak/trygo/internal/models"
	"gitea.deepak.science/deepak/trygo/internal/routes"
	"gitea.deepak.science/deepak/trygo/internal/store"
	"gitea.deepak.science/deepak/trygo/internal/tokens"
	_ "modernc.org/sqlite" // SQLite driver
)

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
			TokenKey:    "test-key-for-testing",
			Environment: "test",
		},
	}
	return tokens.New(cfg)
}

func TestNew(t *testing.T) {
	m := getTestModel(t)
	defer m.Close()

	h := routes.New(m, getTestTokens())
	assert.NotNil(t, h)
}

func TestNewWithNilStore(t *testing.T) {
	h := routes.New(nil, getTestTokens())
	assert.NotNil(t, h)
}

func TestSetupRoutes(t *testing.T) {
	router := routes.New(nil, getTestTokens())
	assert.NotNil(t, router)
}

func TestHello(t *testing.T) {
	router := routes.New(nil, getTestTokens())

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
	router := routes.New(nil, getTestTokens())

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
	router := routes.New(nil, getTestTokens())

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
	defer m.Close()

	router := routes.New(m, getTestTokens())

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
	defer m.Close()

	router := routes.New(m, getTestTokens())

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
	router := routes.New(nil, getTestTokens())

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
	router := routes.New(nil, getTestTokens())

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
