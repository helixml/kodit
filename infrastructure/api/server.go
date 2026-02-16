package api

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
)

// Server represents the HTTP API server.
type Server struct {
	router     chi.Router
	httpServer *http.Server
	logger     *slog.Logger
	addr       string
}

// NewServer creates a new API Server.
func NewServer(addr string, logger *slog.Logger) Server {
	if logger == nil {
		logger = slog.Default()
	}

	router := chi.NewRouter()

	// Apply standard middleware.
	// Note: Timeout is NOT applied here because streaming endpoints (e.g. MCP)
	// are incompatible with chi's Timeout middleware which wraps the ResponseWriter.
	// Request-level timeouts are applied per route group in mountRoutes.
	router.Use(chimiddleware.RequestID)
	router.Use(chimiddleware.RealIP)
	router.Use(chimiddleware.Recoverer)

	return Server{
		router: router,
		addr:   addr,
		logger: logger,
	}
}

// Router returns the chi router for registering routes.
func (s Server) Router() chi.Router {
	return s.router
}

// Start starts the HTTP server.
func (s *Server) Start() error {
	s.httpServer = &http.Server{
		Addr:              s.addr,
		Handler:           s.router,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	s.logger.Info("starting HTTP server", "addr", s.addr)
	if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("http server error: %w", err)
	}
	return nil
}

// Shutdown gracefully shuts down the server.
func (s *Server) Shutdown(ctx context.Context) error {
	if s.httpServer == nil {
		return nil
	}

	s.logger.Info("shutting down HTTP server")
	return s.httpServer.Shutdown(ctx)
}

// Addr returns the server address.
func (s Server) Addr() string {
	return s.addr
}
