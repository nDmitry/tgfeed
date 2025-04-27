package feed

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/gorilla/feeds"
	"github.com/nDmitry/tgfeed/internal/entity"
)

// Generate creates a feed from a channel and returns it as a byte array
func Generate(channel *entity.Channel, params *entity.FeedParams) ([]byte, error) {
	feed := &feeds.Feed{
		Title: channel.Title,
		Link:  &feeds.Link{Href: channel.URL},
		Image: &feeds.Image{Url: channel.ImageURL, Title: channel.Title, Link: channel.URL},
	}

	for _, p := range channel.Posts {
		if shouldExcludePost(p.ContentHTML, params.ExcludeWords, params.ExcludeCaseSensitive) {
			continue
		}

		feed.Items = append(feed.Items, &feeds.Item{
			Id:      p.ID,
			Content: p.ContentHTML,
			Link:    &feeds.Link{Href: p.URL},
			Created: p.Datetime,
			Enclosure: &feeds.Enclosure{
				Url:    p.ImageURL,
				Type:   p.ImageType,
				Length: strconv.Itoa(int(p.ImageSize)),
			},
		})

		if feed.Created.IsZero() || p.Datetime.After(feed.Created) {
			feed.Created = p.Datetime
		}
	}

	var content string
	var err error

	switch params.Format {
	case entity.FormatRSS:
		content, err = feed.ToRss()
	case entity.FormatAtom:
		content, err = feed.ToAtom()
	default:
		return nil, fmt.Errorf("unsupported feed format: %s", params.Format)
	}

	if err != nil {
		return nil, fmt.Errorf("could not marshal channel %s to feed: %w", channel.Username, err)
	}

	return []byte(content), nil
}

// shouldExcludePost checks if a post should be excluded based on exclude words
func shouldExcludePost(content string, excludeWords []string, caseSensitive bool) bool {
	if len(excludeWords) == 0 {
		return false
	}

	if !caseSensitive {
		content = strings.ToLower(content)
	}

	for _, word := range excludeWords {
		if !caseSensitive {
			word = strings.ToLower(word)
		}
		if strings.Contains(content, word) {
			return true
		}
	}

	return false
}
