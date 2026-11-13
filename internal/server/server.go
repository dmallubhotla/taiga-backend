package server

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"gitea.deepak.science/deepak/taiga/internal/config"
	"gitea.deepak.science/deepak/taiga/internal/filerepo"
	"gitea.deepak.science/deepak/taiga/internal/jobs"
	"gitea.deepak.science/deepak/taiga/internal/models"
	"gitea.deepak.science/deepak/taiga/internal/oidc"
	"gitea.deepak.science/deepak/taiga/internal/routes"
	"gitea.deepak.science/deepak/taiga/internal/tokens"
)

// Server represents the HTTP server
type Server struct {
	config *config.Config
	model  models.Model
	server *http.Server
	worker *jobs.Worker

	mu         sync.Mutex
	stopWorker func()
}

// New creates a new server instance
func New(cfg *config.Config) (*Server, error) {
	registrationOpen, err := cfg.RegistrationOpen()
	if err != nil {
		return nil, err
	}
	log.Printf("Registration open: %v", registrationOpen)

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
	fileRepo, err := filerepo.NewFileRepo(*cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize filerepo: %w", err)
	}

	// The flow cookie's key comes from the token signing key, so it survives restarts and is shared by replicas without another secret to manage.
	privateKey, err := os.ReadFile(cfg.Tokens.PrivateKeyPath)
	if err != nil {
		return nil, fmt.Errorf("reading the token signing key: %w", err)
	}
	flowKey := sha256.Sum256(append([]byte("taiga oidc flow\x00"), privateKey...))
	discoverCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	providers, err := oidc.New(discoverCtx, cfg.Auth, flowKey[:])
	if err != nil {
		return nil, err
	}
	log.Printf("Password login: %v; OIDC providers: %d", cfg.Auth.PasswordLogin, len(providers.Providers()))

	// Create routes handler
	routesHandler := routes.New(m, toker, fileRepo,
		routes.WithRegistration(registrationOpen),
		routes.WithPasswordLogin(cfg.Auth.PasswordLogin),
		routes.WithOIDC(providers),
	)

	// Create chi router for middleware
	r := chi.NewRouter()

	// Setup router with middleware

	// Add middleware
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.RequestID)
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
		model:  m,
		server: server,
		worker: jobs.NewWorker(m, fileRepo),
	}, nil
}

// Start runs the job worker and serves HTTP until Shutdown.
func (s *Server) Start() error {
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	s.mu.Lock()
	s.stopWorker = func() { cancel(); <-done }
	s.mu.Unlock()
	go func() {
		defer close(done)
		s.worker.Run(ctx)
	}()

	err := s.server.ListenAndServe()
	if !errors.Is(err, http.ErrServerClosed) {
		s.haltWorker()
	}
	return err
}

func (s *Server) haltWorker() {
	s.mu.Lock()
	stop := s.stopWorker
	s.stopWorker = nil
	s.mu.Unlock()
	if stop != nil {
		stop()
	}
}

// Shutdown stops taking requests, lets the current job finish, then closes the store.
func (s *Server) Shutdown(ctx context.Context) error {
	err := s.server.Shutdown(ctx)
	s.haltWorker()
	if s.model != nil {
		if err := s.model.Close(); err != nil {
			log.Printf("Error closing model: %v", err)
		}
	}
	return err
}
