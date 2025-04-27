package main

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/nDmitry/tgfeed/internal/app"
	"github.com/nDmitry/tgfeed/internal/cache"
	"github.com/nDmitry/tgfeed/internal/handler"
)

func main() {
	// Setup logger
	logger := app.Logger()
	slog.SetDefault(logger)

	// Create a cancellable context for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())

	// Setup signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-sigChan
		logger.Info("Received first shutdown signal, starting graceful shutdown...")
		cancel()

		// If we receive a second signal, exit immediately
		<-sigChan
		logger.Info("Received second shutdown signal, exiting immediately...")
		os.Exit(1)
	}()

	port := os.Getenv("HTTP_SERVER_PORT")

	if port == "" {
		port = "8080"
	}

	redisHost := os.Getenv("REDIS_HOST")

	if redisHost == "" {
		redisHost = "redis"
	}

	// Initialize Redis cache
	redisClient, err := cache.NewRedisClient(ctx, fmt.Sprintf("%s:6379", redisHost))

	if err != nil {
		logger.Error("Failed to connect to Redis", "error", err)
		os.Exit(1)
	}

	defer redisClient.Close()

	telegramHandler := handler.NewTelegramHandler(redisClient)

	server := &http.Server{
		Addr:              ":" + port,
		Handler:           telegramHandler.Handler(),
		BaseContext:       func(_ net.Listener) context.Context { return ctx },
		ReadHeaderTimeout: 10 * time.Second,  // Mitigate Slowloris
		ReadTimeout:       30 * time.Second,  // Time to read entire request (including body)
		WriteTimeout:      30 * time.Second,  // Time to write response
		IdleTimeout:       120 * time.Second, // Keep-alive timeout
	}

	server.RegisterOnShutdown(cancel)

	// Start server in a goroutine so it doesn't block
	go func() {
		logger.Info("Starting server", "port", port)

		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("Server error", "error", err)
			os.Exit(1)
		}
	}()

	// Wait for context cancelation (shutdown signal)
	<-ctx.Done()
	logger.Info("Shutting down server...")

	// Create a deadline for server shutdown
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.Error("Server forced to shutdown", "error", err)
	}

	logger.Info("Server exited gracefully")
}
