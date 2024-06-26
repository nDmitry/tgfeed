package feed

import (
	"fmt"
	"log/slog"
	"os"
	"strconv"

	"github.com/gorilla/feeds"
	"github.com/nDmitry/tgfeed/internal/entity"
)

// Generate creates a feed and saves it to a file.
func Generate(channel *entity.Channel, config *entity.Config) error {
	feed := &feeds.Feed{
		Title: channel.Title,
		Link:  &feeds.Link{Href: channel.URL},
		Image: &feeds.Image{Url: channel.ImageURL, Title: channel.Title, Link: channel.URL},
	}

	for _, p := range channel.Posts {
		skip := false

		for _, sw := range config.StopWordsRegexps {
			if sw.MatchString(p.ContentHTML) {
				skip = true
				break
			}
		}

		if skip {
			slog.Info("skipping post with a stop word", "ID", p.ID)
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

	out, err := feed.ToRss()

	if err != nil {
		return fmt.Errorf("could not marshal channel %s to feed: %w", channel.Username, err)
	}

	if err := os.MkdirAll(config.FeedsPath, 0755); err != nil {
		return fmt.Errorf("could not make the feeds dir: %w", err)
	}

	if err := os.WriteFile(fmt.Sprintf("%s/%s.%s", config.FeedsPath, channel.Username, config.FeedFormat), []byte(out), 0644); err != nil {
		return fmt.Errorf("could not save the feed %s to a file: %w", channel.Username, err)
	}

	return nil
}
