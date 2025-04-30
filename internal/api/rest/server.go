package rest

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"time"

	"github.com/nDmitry/tgfeed/internal/app"
	"github.com/nDmitry/tgfeed/internal/cache"
)

// Server represents the REST API server
type Server struct {
	mux       *http.ServeMux
	server    *http.Server
	logger    *slog.Logger
	cache     cache.Cache
	scraper   Scraper
	generator Generator
	port      string
}

// NewServer creates a new REST API server
func NewServer(c cache.Cache, s Scraper, g Generator, port string) *Server {
	mux := http.NewServeMux()
	logger := app.Logger()

	server := &Server{
		mux:       mux,
		logger:    logger,
		cache:     c,
		scraper:   s,
		generator: g,
		port:      port,
		server: &http.Server{
			Addr:              ":" + port,
			Handler:           nil,               // Will be set in Run
			ReadHeaderTimeout: 10 * time.Second,  // Mitigate Slowloris
			ReadTimeout:       30 * time.Second,  // Time to read entire request (including body)
			WriteTimeout:      30 * time.Second,  // Time to write response
			IdleTimeout:       120 * time.Second, // Keep-alive timeout
		},
	}

	server.registerHandlers()

	return server
}

// registerHandlers sets up all API routes
func (s *Server) registerHandlers() {
	NewTelegramHandler(s.mux, s.cache, s.scraper, s.generator)
	// more handlers can be here
}

// Run starts the server and blocks until the context is canceled
func (s *Server) Run(ctx context.Context) error {
	// Apply middleware to the router
	handlerWithMiddleware := Logger(s.mux)

	// Set the handler with middleware
	s.server.Handler = handlerWithMiddleware

	// Set BaseContext to pass the parent context
	s.server.BaseContext = func(_ net.Listener) context.Context { return ctx }

	// Register shutdown handler
	s.server.RegisterOnShutdown(func() {
		s.logger.Info("Server is shutting down...")
	})

	// Start server in a goroutine
	errCh := make(chan error, 1)

	go func() {
		s.logger.Info("Starting HTTP server", "port", s.port)
		if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- fmt.Errorf("server error: %w", err)
		}
		close(errCh)
	}()

	// Wait for context cancellation or server error
	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		return s.Shutdown(context.Background())
	}
}

// Shutdown gracefully shuts down the server
func (s *Server) Shutdown(ctx context.Context) error {
	s.logger.Info("Shutting down server...")

	// Create a timeout for shutdown
	shutdownCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	if err := s.server.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("server forced to shutdown: %w", err)
	}

	s.logger.Info("Server exited gracefully")

	return nil
}
