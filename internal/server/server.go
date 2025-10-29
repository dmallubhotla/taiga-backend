package server

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	_ "github.com/lib/pq"  // PostgreSQL driver
	_ "modernc.org/sqlite" // SQLite driver

	"gitea.deepak.science/deepak/trygo/internal/config"
	"gitea.deepak.science/deepak/trygo/internal/filerepo"
	"gitea.deepak.science/deepak/trygo/internal/models"
	"gitea.deepak.science/deepak/trygo/internal/routes"
	"gitea.deepak.science/deepak/trygo/internal/tokens"
)

// Server represents the HTTP server
type Server struct {
	config *config.Config
	db     *sql.DB
	model  models.Model
	server *http.Server
}

// New creates a new server instance
func New(cfg *config.Config) (*Server, error) {
	// Initialize store
	m, err := models.New(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize store: %w", err)
	}

	// create toker
	toker, err := tokens.New(*cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize toker: %w", err)
	}

	// create filerepo
	fileRepo := filerepo.NewFileRepo(*cfg)

	// Create routes handler
	routesHandler := routes.New(m, toker, fileRepo)

	// Create chi router for middleware
	r := chi.NewRouter()

	// Setup router with middleware

	// Add middleware
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Timeout(30 * time.Second))

	// CORS for development
	if cfg.App.IsDevelopment() {
		r.Use(middleware.AllowContentType("application/json", "multipart/form-data"))
		r.Use(func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Access-Control-Allow-Origin", "*")
				w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
				w.Header().Set("Access-Control-Allow-Headers", "Accept, Authorization, Content-Type, X-CSRF-Token")

				if r.Method == "OPTIONS" {
					w.WriteHeader(http.StatusOK)
					return
				}

				next.ServeHTTP(w, r)
			})
		})
	}

	// Mount the routes handler
	r.Mount("/", routesHandler)

	// Create HTTP server
	server := &http.Server{
		Addr:         ":" + cfg.App.Port,
		Handler:      r,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	return &Server{
		config: cfg,
		// db:       db,
		model:  m,
		server: server,
	}, nil
}

// Start starts the HTTP server
func (s *Server) Start() error {
	return s.server.ListenAndServe()
}

// Shutdown gracefully shuts down the server
func (s *Server) Shutdown(ctx context.Context) error {
	// Close store
	if s.model != nil {
		if err := s.model.Close(); err != nil {
			log.Printf("Error closing model: %v", err)
		}
	}

	// Shutdown HTTP server
	return s.server.Shutdown(ctx)
}
