package routes

import (
	"encoding/json"
	"net/http"

	"gitea.deepak.science/deepak/taiga/internal/filerepo"
	"gitea.deepak.science/deepak/taiga/internal/models"
	"gitea.deepak.science/deepak/taiga/internal/tokens"
	"github.com/go-chi/chi/v5"
)

// Handlers holds dependencies for HTTP handlers
// type Handlers struct {
// 	db *sql.DB
// }

// New creates a new handlers instance
func New(m models.Model, tok tokens.Toker, fileRepo filerepo.FileRepo) http.Handler {
	r := chi.NewRouter()

	// Basic routes
	r.Get("/", hello)
	r.Get("/ping", ping)

	r.Mount("/health", newHealthRouter(m))
	r.Mount("/auth", newAuthRouter(m, tok))

	// authenticated routes
	r.Group(func(r chi.Router) {
		r.Use(tok.Authenticator)
		r.Mount("/hats", newHatRouter(m))
		r.Mount("/activity_files", newActivityFileRouter(m, fileRepo))
		r.Mount("/workouts", newWorkoutRouter(m, fileRepo))
	})

	return r
}

// Hello handles the root endpoint with a simple greeting
func hello(w http.ResponseWriter, r *http.Request) {
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
func ping(w http.ResponseWriter, r *http.Request) {
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
