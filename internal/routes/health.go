package routes

import (
	"encoding/json"
	"fmt"
	"net/http"

	"gitea.deepak.science/deepak/trygo/internal/models"
	"github.com/go-chi/chi/v5"
)

func newHealthRouter(m models.Model) http.Handler {
	router := chi.NewRouter()
	router.Get("/", healthFunc(m))
	return router
}

// Health handles health check requests with store connectivity
func healthFunc(m models.Model) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var dbStatus string
		var dbHealthy bool

		if m == nil {
			dbStatus = "no store configured"
			dbHealthy = false
		} else if err := m.Healthy(r.Context()); err != nil {
			dbStatus = fmt.Sprintf("unhealthy: %v", err)
			dbHealthy = false
		} else {
			dbStatus = "healthy"
			dbHealthy = true
		}

		response := map[string]any{
			"status": "ok",
			"database": map[string]any{
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
}
