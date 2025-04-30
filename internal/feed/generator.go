package feed

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/gorilla/feeds"
	"github.com/nDmitry/tgfeed/internal/entity"
)

type Generator struct{}

// Generate creates a feed from a channel and returns it as a byte array
func (g *Generator) Generate(channel *entity.Channel, params *entity.FeedParams) ([]byte, error) {
	feed := &feeds.Feed{
		Title: channel.Title,
		Link:  &feeds.Link{Href: channel.URL},
		Image: &feeds.Image{Url: channel.ImageURL, Title: channel.Title, Link: channel.URL},
		Items: make([]*feeds.Item, 0, len(channel.Posts)),
	}

	for _, p := range channel.Posts {
		if g.shouldExcludePost(p.ContentHTML, params.ExcludeWords, params.ExcludeCaseSensitive) {
			continue
		}

		item := &feeds.Item{
			Id:      strconv.Itoa(p.ID),
			Title:   p.Title,
			Content: p.ContentHTML,
			Link:    &feeds.Link{Href: p.URL},
			Created: p.Datetime,
		}

		if p.Preview != nil {
			item.Enclosure = &feeds.Enclosure{
				Url:    p.Preview.URL,
				Type:   p.Preview.Type,
				Length: strconv.Itoa(int(p.Preview.Size)),
			}
		}

		item.Content = g.appendGallery(item.Content, p.Images)

		feed.Add(item)

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

func (g *Generator) appendGallery(content string, images []entity.Image) string {
	if len(images) == 0 {
		return content
	}

	if content != "" {
		content += "<br><br>"
	}

	content += `<div class="image-gallery">`

	for _, img := range images {
		content += fmt.Sprintf(`<p><img src="%s" alt="Image" /></p>`, img.URL)
	}

	content += "</div>"

	return content
}

// shouldExcludePost checks if a post should be excluded based on exclude words
func (g *Generator) shouldExcludePost(content string, excludeWords []string, caseSensitive bool) bool {
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
