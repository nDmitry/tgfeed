package entity

type Config struct {
	FeedFormat       string   `json:"feedFormat"`
	FeedsPath        string   `json:"feedsPath"`
	ScrapingInterval int      `json:"scrapingEveryMinutes"`
	Channels         []string `json:"channels"`
}
