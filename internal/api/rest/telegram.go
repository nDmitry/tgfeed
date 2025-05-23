package rest

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/nDmitry/tgfeed/internal/app"
	"github.com/nDmitry/tgfeed/internal/cache"
	"github.com/nDmitry/tgfeed/internal/entity"
)

type Scraper interface {
	Scrape(ctx context.Context, username string) (*entity.Channel, error)
}

type Generator interface {
	Generate(channel *entity.Channel, params *entity.FeedParams) ([]byte, error)
}

// telegramHandler handles routes for Telegram feeds
type telegramHandler struct {
	cache     cache.Cache
	scraper   Scraper
	generator Generator
	logger    *slog.Logger
}

// NewTelegramHandler registers all Telegram-related handlers
func NewTelegramHandler(
	mux *http.ServeMux,
	c cache.Cache, s Scraper, g Generator,
) {
	handler := &telegramHandler{
		cache:     c,
		scraper:   s,
		generator: g,
		logger:    app.Logger(),
	}

	mux.HandleFunc("GET /telegram/channel/{username}", handler.getChannelFeed)
}

// getChannelFeed handles requests for Telegram channel feeds
func (h *telegramHandler) getChannelFeed(w http.ResponseWriter, r *http.Request) {
	params, err := entity.NewFeedParamFromRequest(r)

	if err != nil {
		h.handleError(w, err, http.StatusBadRequest)
		return
	}

	// Try to get from cache first if caching is enabled
	if params.CacheTTL > 0 {
		cacheKey := h.buildCacheKey(params)
		cachedContent, cacheErr := h.cache.Get(r.Context(), cacheKey)

		if cacheErr == nil {
			// Cache hit
			w.Header().Set("X-CACHE-STATUS", "HIT")
			h.serveContent(w, cachedContent, params.Format, params.CacheTTL)
			return
		} else if cacheErr != cache.ErrCacheMiss {
			// Real error, not just cache miss
			h.logger.Error("Cache error", "error", cacheErr)
		}
	}

	// Cache miss or caching disabled - scrape the channel
	channel, err := h.scraper.Scrape(r.Context(), params.Username)

	if err != nil {
		h.handleError(w, err, http.StatusInternalServerError)
		return
	}

	// Generate feed
	content, err := h.generator.Generate(channel, params)

	if err != nil {
		h.handleError(w, err, http.StatusInternalServerError)
		return
	}

	// Cache the result if caching is enabled
	if params.CacheTTL > 0 {
		cacheKey := h.buildCacheKey(params)
		cacheTTL := time.Duration(params.CacheTTL) * time.Minute

		// Use background context for caching to avoid cancellation
		cacheCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := h.cache.Set(cacheCtx, cacheKey, content, cacheTTL); err != nil {
			h.logger.Error("Failed to cache content", "error", err)
		}
	}

	w.Header().Set("X-CACHE-STATUS", "MISS")
	h.serveContent(w, content, params.Format, params.CacheTTL)
}

// buildCacheKey generates a cache key based on request parameters
func (h *telegramHandler) buildCacheKey(params *entity.FeedParams) string {
	excludeWords := ""

	if len(params.ExcludeWords) > 0 {
		excludeWords = strings.Join(params.ExcludeWords, "|")
	}

	caseSensitive := "0"

	if params.ExcludeCaseSensitive {
		caseSensitive = "1"
	}

	return fmt.Sprintf("telegram:channel:%s:%s:%s:%s",
		params.Username,
		params.Format,
		excludeWords,
		caseSensitive)
}

// serveContent sends the content to the client with appropriate headers
func (h *telegramHandler) serveContent(w http.ResponseWriter, content []byte, format string, cacheTTL int) {
	var contentType string
	switch format {
	case entity.FormatRSS:
		contentType = "application/rss+xml"
	case entity.FormatAtom:
		contentType = "application/atom+xml"
	default:
		contentType = "application/xml"
	}

	w.Header().Set("Content-Type", contentType+"; charset=utf-8")

	if cacheTTL > 0 {
		w.Header().Set("Cache-Control", fmt.Sprintf("public, max-age=%d", cacheTTL*60))
	} else {
		w.Header().Set("Cache-Control", "no-cache")
	}

	w.WriteHeader(http.StatusOK)

	if _, err := w.Write(content); err != nil {
		handleBadErrorResponse(err, content)
	}
}

// handleError responds with an error message
func (h *telegramHandler) handleError(w http.ResponseWriter, err error, statusCode int) {
	h.logger.Error("Request error", "error", err, "status", statusCode)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	response := map[string]string{"error": err.Error()}

	if err := json.NewEncoder(w).Encode(response); err != nil {
		handleBadErrorResponse(err, response)
	}
}

func handleBadErrorResponse(err error, resp any) {
	app.Logger().Error(
		"failed to encode an error response",
		"error", err,
		"response", resp,
	)
}
