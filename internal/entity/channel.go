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
	// Post ID, e.g. Channel/123
	ID          string
	URL         string
	ContentHTML string
	// Date and time of the post in RFC3339 format.
	Datetime time.Time
}
