package feed

import (
	"fmt"
	"os"

	"github.com/gorilla/feeds"
	"github.com/nDmitry/tgfeed/internal/entity"
)

// Generate creates an Atom feed and saves it to a file.
func Generate(channel *entity.Channel, outPath string) error {
	feed := &feeds.Feed{
		Title: channel.Title,
		Link:  &feeds.Link{Href: channel.URL},
	}

	for _, p := range channel.Posts {
		feed.Items = append(feed.Items, &feeds.Item{
			Id:      p.ID,
			Content: p.ContentHTML,
			Link:    &feeds.Link{Href: p.URL},
			Created: p.Datetime,
		})

		if feed.Created.IsZero() || p.Datetime.After(feed.Created) {
			feed.Created = p.Datetime
		}
	}

	atom, err := feed.ToAtom()

	if err != nil {
		return fmt.Errorf("could not marshal channel %s to Atom: %w", channel.Username, err)
	}

	if err = save(atom, channel.Username, outPath); err != nil {
		return fmt.Errorf("could not save the feed %s to a file: %w", channel.Username, err)
	}

	return nil
}

func save(feed string, username string, outPath string) error {
	if err := os.MkdirAll(outPath, 0644); err != nil {
		return err
	}

	if err := os.WriteFile(fmt.Sprintf("%s/%s.atom", outPath, username), []byte(feed), 0644); err != nil {
		return err
	}

	return nil
}
