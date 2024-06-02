package entity

import "regexp"

type Config struct {
	// Can be "atom" or "rss".
	FeedFormat string `json:"feedFormat"`
	// Where the feeds are stored.
	FeedsPath string `json:"feedsPath"`
	// Period in minutes between channels scrapings.
	ScrapingInterval int `json:"scrapingEveryMinutes"`
	// Posts matching this regexes will be skipped in the feed.
	StopWords        []string `json:"stopWords"`
	StopWordsRegexps []regexp.Regexp
	// List of channels usernames for scraping.
	Channels []string `json:"channels"`
}
