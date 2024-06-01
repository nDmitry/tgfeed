package main

import (
	"context"
	"log"
	"log/slog"
	"os"
	"os/signal"
	"time"

	"github.com/nDmitry/tgfeed/internal/config"
	"github.com/nDmitry/tgfeed/internal/entity"
	"github.com/nDmitry/tgfeed/internal/feed"
	"github.com/nDmitry/tgfeed/internal/scraper"
)

const configPath = "config.json"
const feedsDir = "feeds"

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	config, err := config.Read(configPath)

	if err != nil {
		log.Fatal(err)
	}

	// Initial run
	run(ctx, config)

	// Periodic run
	ticker := time.NewTicker(time.Duration(config.ScrapingInterval) * time.Minute)

	for {
		select {
		case <-ticker.C:
			run(ctx, config)
		case <-ctx.Done():
			ticker.Stop()
			slog.InfoContext(ctx, "Exiting...")
			return
		}
	}
}

func run(ctx context.Context, config *entity.Config) {
	slog.InfoContext(ctx, "Scraping the channels...")

	for _, ch := range config.Channels {
		channel, err := scraper.Scrape(ch)

		if err != nil {
			slog.ErrorContext(ctx, err.Error())
			continue
		}

		if err := feed.Generate(channel, feedsDir); err != nil {
			slog.ErrorContext(ctx, err.Error())
			continue
		}
	}

	slog.InfoContext(ctx, "Scraping is done")
}
