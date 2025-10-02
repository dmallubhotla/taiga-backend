package routes_test

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	_ "modernc.org/sqlite"

	"gitea.deepak.science/deepak/trygo/internal/routes"
)

func TestNew(t *testing.T) {
	db := &sql.DB{}
	h := routes.New(db)
	assert.NotNil(t, h)
}

func TestNewWithNilDB(t *testing.T) {
	h := routes.New(nil)
	assert.NotNil(t, h)
}

func TestSetupRoutes(t *testing.T) {
	h := routes.New(nil)
	router := h.SetupRoutes()
	assert.NotNil(t, router)
}

func TestHello(t *testing.T) {
	h := routes.New(nil)
	router := h.SetupRoutes()

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
	h := routes.New(nil)
	router := h.SetupRoutes()

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

func TestHealthWithNilDB(t *testing.T) {
	h := routes.New(nil)
	router := h.SetupRoutes()

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
	assert.Equal(t, "no database configured", database["status"])
	assert.False(t, database["healthy"].(bool))
}

func TestHealthWithHealthyDB(t *testing.T) {
	// Create an in-memory SQLite database for testing
	db, err := sql.Open("sqlite", ":memory:")
	require.NoError(t, err)
	defer db.Close()

	h := routes.New(db)
	router := h.SetupRoutes()

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

	var response map[string]interface{}
	err = json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, "ok", response["status"])
	assert.Equal(t, "trygo-template", response["service"])

	database, ok := response["database"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "healthy", database["status"])
	assert.True(t, database["healthy"].(bool))
}

func TestHealthWithUnhealthyDB(t *testing.T) {
	// Create a database that we'll close to make it unhealthy
	db, err := sql.Open("sqlite", ":memory:")
	require.NoError(t, err)
	db.Close() // Close immediately to make ping fail

	h := routes.New(db)
	router := h.SetupRoutes()

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

	var response map[string]interface{}
	err = json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, "degraded", response["status"])
	assert.Equal(t, "trygo-template", response["service"])

	database, ok := response["database"].(map[string]interface{})
	require.True(t, ok)
	assert.Contains(t, database["status"].(string), "unhealthy:")
	assert.False(t, database["healthy"].(bool))
}

func TestRoutesExist(t *testing.T) {
	h := routes.New(nil)
	router := h.SetupRoutes()

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
	h := routes.New(nil)
	router := h.SetupRoutes()

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
