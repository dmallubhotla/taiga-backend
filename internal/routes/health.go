package routes

import (
	"encoding/json"
	"fmt"
	"net/http"

	"gitea.deepak.science/deepak/trygo/internal/store"
	"github.com/go-chi/chi/v5"
)

func newHealthRouter(s store.Store) http.Handler {
	router := chi.NewRouter()
	router.Get("/health", healthFunc(s))
	return router
}

// Health handles health check requests with database connectivity
func healthFunc(s store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var dbErr error
		var dbHealthy bool

		// Test database connection
		if err := s.Healthy(r.Context()); err != nil {
			dbErr = fmt.Errorf("unhealthy database", err)
			dbHealthy = false
		}

		response := map[string]any{
			"status": "ok",
			"database": map[string]any{
				"status":  dbErr,
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
}
