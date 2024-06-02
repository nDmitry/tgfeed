package entity

import "time"

type Channel struct {
	Username string
	Title    string
	URL      string
	ImageURL string
	Posts    []Post
}

type Post struct {
	// Post ID, e.g. Channel/123.
	ID          string
	URL         string
	ContentHTML string
	ImageURL    string
	ImageType   string
	// In bytes.
	ImageSize int64
	// Date and time of the post in RFC3339 format.
	Datetime time.Time
}
