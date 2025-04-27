package entity

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
)

const (
	FormatAtom = "atom"
	FormatRSS  = "rss"
)

const CacheTTLDefault = 60 // minutes

// FeedParams represents validated request parameters for feed generation
type FeedParams struct {
	// Username is the Telegram channel username
	Username string

	// Format is the feed format, either "atom" or "rss"
	Format string

	// ExcludeWords is a list of words that will exclude a post if matched
	ExcludeWords []string

	// ExcludeCaseSensitive determines if exclusion matching is case-sensitive
	ExcludeCaseSensitive bool

	// CacheTTL is the cache time-to-live in minutes
	// A value of 0 means no caching
	CacheTTL int
}

// NewFeedParamFromRequest parses and validates request parameters and creates a new FeedParams
// nolint: cyclop
func NewFeedParamFromRequest(r *http.Request) (*FeedParams, error) {
	username := r.PathValue("username")

	if username == "" {
		return nil, fmt.Errorf("username is required")
	}

	qp := r.URL.Query()

	format := qp.Get("format")

	if format == "" {
		format = FormatRSS
	} else if format != FormatRSS && format != FormatAtom {
		return nil, fmt.Errorf("format must be %s or %s", FormatRSS, FormatAtom)
	}

	var excludeWords []string

	if exclude := qp.Get("exclude"); exclude != "" {
		excludeWords = strings.Split(exclude, "|")

		for i, word := range excludeWords {
			excludeWords[i] = strings.TrimSpace(word)
		}

		// Filter out empty strings
		filtered := make([]string, 0, len(excludeWords))

		for _, word := range excludeWords {
			if word != "" {
				filtered = append(filtered, word)
			}
		}

		excludeWords = filtered
	}

	excludeCaseSensitive := false

	if caseSensitive := qp.Get("exclude_case_sensitive"); caseSensitive != "" {
		if caseSensitive == "1" || strings.EqualFold(caseSensitive, "true") {
			excludeCaseSensitive = true
		}
	}

	// Parse cache TTL with default
	cacheTTL := CacheTTLDefault

	if ttlStr := qp.Get("cache_ttl"); ttlStr != "" {
		var err error
		cacheTTL, err = strconv.Atoi(ttlStr)

		if err != nil {
			return nil, fmt.Errorf("cache_ttl must be a valid integer")
		}

		if cacheTTL < 0 {
			return nil, fmt.Errorf("cache_ttl must be non-negative")
		}
	}

	return &FeedParams{
		Username:             username,
		Format:               format,
		ExcludeWords:         excludeWords,
		ExcludeCaseSensitive: excludeCaseSensitive,
		CacheTTL:             cacheTTL,
	}, nil
}
