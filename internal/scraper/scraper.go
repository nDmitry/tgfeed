package scraper

import (
	"fmt"
	"time"

	"github.com/gocolly/colly/v2"
	"github.com/nDmitry/tgfeed/internal/entity"
)

const tgDomain = "t.me"
const userAgent = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/125.0.0.0 Safari/537.36"

func Scrape(username string) (channel *entity.Channel, err error) {
	defer func() {
		if r := recover(); r != nil {
			channel = nil
			err = fmt.Errorf("recovered: %w", r.(error))
		}
	}()

	channel = &entity.Channel{
		Username: username,
		URL:      fmt.Sprintf("https://%s/s/%s", tgDomain, username),
	}

	c := colly.NewCollector(
		colly.AllowedDomains(tgDomain),
		colly.CacheDir("cache"),
		colly.UserAgent(userAgent),
	)

	c.OnHTML(".tgme_channel_info_header", func(e *colly.HTMLElement) {
		channel.Title = e.ChildText(".tgme_channel_info_header_title")
		channel.ImageURL = e.ChildAttr("img", "src")
	})

	c.OnHTML(".tgme_widget_message", func(e *colly.HTMLElement) {
		var err error
		var post = entity.Post{}

		post.ID = e.Attr("data-post")
		post.URL = fmt.Sprintf("https://%s/%s", tgDomain, post.ID)

		post.ContentHTML, err = e.DOM.Find(".tgme_widget_message_text").Html()

		if err != nil {
			panic(fmt.Errorf("could not get HTML post content for %s: %w", post.URL, err))
		}

		dtText, exists := e.DOM.Find(".tgme_widget_message_date").Find("time").Attr("datetime")

		if !exists {
			panic(fmt.Errorf("could not find datetime for %s: %w", post.URL, err))
		}

		dt, err := time.Parse(time.RFC3339, dtText)

		if err != nil {
			panic(fmt.Errorf("could not parse post datetime for %s: %w", post.URL, err))
		}

		post.Datetime = dt

		if post.ContentHTML == "" {
			// Default content in case Telegram does not show it in the web version.
			post.ContentHTML = fmt.Sprintf(
				`<a href="%s">[Open in Telegram]</a>`,
				post.URL,
			)
		}

		channel.Posts = append(channel.Posts, post)
	})

	c.OnError(func(r *colly.Response, err error) {
		panic(fmt.Errorf("request error %s: %w", channel.URL, err))
	})

	if err := c.Visit(channel.URL); err != nil {
		return nil, fmt.Errorf("could not visit %s: %w", channel.URL, err)
	}

	return channel, nil
}
