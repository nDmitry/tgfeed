package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/nDmitry/tgfeed/internal/api/rest"
	"github.com/nDmitry/tgfeed/internal/app"
	"github.com/nDmitry/tgfeed/internal/cache"
	"github.com/nDmitry/tgfeed/internal/feed"
)

func main() {
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

	scraper := feed.NewDefaultScraper()
	generator := &feed.Generator{}

	// Initialize and run the HTTP server
	server := rest.NewServer(redisClient, scraper, generator, port)

	if err := server.Run(ctx); err != nil {
		logger.Error("Server error", "error", err)
		os.Exit(1)
	}

	logger.Info("Server exited gracefully")
}
