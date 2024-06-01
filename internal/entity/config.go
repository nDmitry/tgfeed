package entity

type Config struct {
	ScrapingInterval int      `json:"scrapingEveryMinutes"`
	Channels         []string `json:"channels"`
}
