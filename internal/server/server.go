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
	"gitea.deepak.science/deepak/trygo/internal/migration"
	"gitea.deepak.science/deepak/trygo/internal/routes"
)

// Server represents the HTTP server
type Server struct {
	config   *config.Config
	db       *sql.DB
	server   *http.Server
	migrator *migration.Migrator
}

// New creates a new server instance
func New(cfg *config.Config) (*Server, error) {
	// Initialize database connection
	db, err := initDB(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize database: %w", err)
	}

	// Initialize migrations
	migrator, err := migration.New(db, cfg.Db.Driver, cfg.Db.MigrationPath)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize migrator: %w", err)
	}

	// Run down migrations if auto_down is enabled and in development
	if cfg.Db.DropOnStart && cfg.App.IsDevelopment() {
		log.Println("Running down migrations...")
		if err := migrator.Down(); err != nil {
			log.Printf("Warning: failed to run down migrations: %v", err)
		}
	}

	// Run migrations if auto_up is enabled
	if cfg.Db.AutoMigrateUp {
		log.Println("Running database migrations...")
		if err := migrator.Up(); err != nil {
			return nil, fmt.Errorf("failed to run migrations: %w", err)
		}

		// Log current migration version
		if version, dirty, err := migrator.Version(); err == nil {
			log.Printf("Database migration version: %d (dirty: %v)", version, dirty)
		}
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
	// if cfg.IsDevelopment() {
	// 	r.Use(middleware.AllowContentType("application/json"))
	// 	r.Use(func(next http.Handler) http.Handler {
	// 		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	// 			w.Header().Set("Access-Control-Allow-Origin", "*")
	// 			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
	// 			w.Header().Set("Access-Control-Allow-Headers", "Accept, Authorization, Content-Type, X-CSRF-Token")
	//
	// 			if r.Method == "OPTIONS" {
	// 				w.WriteHeader(http.StatusOK)
	// 				return
	// 			}
	//
	// 			next.ServeHTTP(w, r)
	// 		})
	// 	})
	// }

	// Mount application routes
	r.Mount("/", h.SetupRoutes())

	// Create HTTP server
	server := &http.Server{
		Addr:         ":" + cfg.App.Port,
		Handler:      r,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	return &Server{
		config:   cfg,
		db:       db,
		server:   server,
		migrator: migrator,
	}, nil
}

// Start starts the HTTP server
func (s *Server) Start() error {
	return s.server.ListenAndServe()
}

// Shutdown gracefully shuts down the server
func (s *Server) Shutdown(ctx context.Context) error {

	// Close migrator
	if s.migrator != nil {
		if err := s.migrator.Close(); err != nil {
			log.Printf("Error closing migrator: %v", err)
		}
	}

	// Close database connection
	if s.db != nil {
		if err := s.db.Close(); err != nil {
			// Log error but don't return it since we're shutting down
			log.Printf("Error closing database connection: %v\n", err)
		}
	}

	// Shutdown HTTP server
	return s.server.Shutdown(ctx)
}

// initDB initializes the database connection
func initDB(cfg *config.Config) (*sql.DB, error) {
	dsn := cfg.Db.DSN()
	if dsn == "" {
		return nil, fmt.Errorf("invalid database configuration")
	}

	db, err := sql.Open(cfg.Db.Driver, dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Test the connection
	if err := db.Ping(); err != nil {
		if closeErr := db.Close(); closeErr != nil {
			log.Printf("Error closing database after ping failure: %v\n", closeErr)
		}
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	// Configure connection pool
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(25)
	db.SetConnMaxLifetime(5 * time.Minute)

	return db, nil
}
