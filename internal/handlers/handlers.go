package handlers

import (
	"database/sql"
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
)

// Handlers holds dependencies for HTTP handlers
type Handlers struct {
	db *sql.DB
}

// New creates a new handlers instance
func New(db *sql.DB) *Handlers {
	return &Handlers{
		db: db,
	}
}

// SetupRoutes configures all application routes
func (h *Handlers) SetupRoutes() *chi.Mux {
	r := chi.NewRouter()

	// Basic routes
	r.Get("/", h.Hello)
	r.Get("/ping", h.Ping)
	r.Get("/health", h.Health)

	return r
}

// Hello handles the root endpoint with a simple greeting
func (h *Handlers) Hello(w http.ResponseWriter, r *http.Request) {
	response := map[string]any{
		"message": "Hello, World!",
		"status":  "success",
		"service": "trygo-template",
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
}

// Ping handles simple ping requests
func (h *Handlers) Ping(w http.ResponseWriter, r *http.Request) {
	response := map[string]string{
		"ping": "pong",
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
}

// Health handles health check requests with database connectivity
func (h *Handlers) Health(w http.ResponseWriter, r *http.Request) {
	var dbStatus string
	var dbHealthy bool

	// Test database connection
	if h.db != nil {
		if err := h.db.Ping(); err != nil {
			dbStatus = "unhealthy: " + err.Error()
			dbHealthy = false
		} else {
			dbStatus = "healthy"
			dbHealthy = true
		}
	} else {
		dbStatus = "no database configured"
		dbHealthy = false
	}

	response := map[string]interface{}{
		"status": "ok",
		"database": map[string]interface{}{
			"status":  dbStatus,
			"healthy": dbHealthy,
		},
		"service": "trygo-template",
	}

	status := http.StatusOK
	if !dbHealthy {
		status = http.StatusServiceUnavailable
		response["status"] = "degraded"
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
}
