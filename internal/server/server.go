package server

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	_ "github.com/lib/pq"           // PostgreSQL driver
	_ "modernc.org/sqlite"          // SQLite driver

	"github.com/user/trygo/internal/config"
	"github.com/user/trygo/internal/handlers"
)

// Server represents the HTTP server
type Server struct {
	config *config.Config
	db     *sql.DB
	server *http.Server
}

// New creates a new server instance
func New(cfg *config.Config) (*Server, error) {
	// Initialize database connection
	db, err := initDB(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize database: %w", err)
	}

	// Create handlers
	h := handlers.New(db)

	// Setup router with middleware
	r := chi.NewRouter()

	// Add middleware
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Timeout(30 * time.Second))

	// CORS for development
	if cfg.IsDevelopment() {
		r.Use(middleware.AllowContentType("application/json"))
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

	// Mount application routes
	r.Mount("/", h.SetupRoutes())

	// Create HTTP server
	server := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      r,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	return &Server{
		config: cfg,
		db:     db,
		server: server,
	}, nil
}

// Start starts the HTTP server
func (s *Server) Start() error {
	return s.server.ListenAndServe()
}

// Shutdown gracefully shuts down the server
func (s *Server) Shutdown(ctx context.Context) error {
	// Close database connection
	if s.db != nil {
		s.db.Close()
	}

	// Shutdown HTTP server
	return s.server.Shutdown(ctx)
}

// initDB initializes the database connection
func initDB(cfg *config.Config) (*sql.DB, error) {
	dsn := cfg.Database.DSN()
	if dsn == "" {
		return nil, fmt.Errorf("invalid database configuration")
	}

	db, err := sql.Open(cfg.Database.Driver, dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Test the connection
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	// Configure connection pool
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(25)
	db.SetConnMaxLifetime(5 * time.Minute)

	return db, nil
}