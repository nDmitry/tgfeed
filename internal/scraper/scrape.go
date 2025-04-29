package scraper

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gocolly/colly/v2"
	"github.com/nDmitry/tgfeed/internal/app"
	"github.com/nDmitry/tgfeed/internal/entity"
)

const tmpPath = "/tmp"
const tgDomain = "t.me"
const userAgentDefault = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/125.0.0.0 Safari/537.36"

// Scrape fetches channel data from Telegram
func Scrape(ctx context.Context, username string) (*entity.Channel, error) {
	logger := app.Logger()

	channel := &entity.Channel{
		Username: username,
		URL:      fmt.Sprintf("https://%s/s/%s", tgDomain, username),
	}

	ua := os.Getenv("USER_AGENT")

	if ua != "" {
		ua = userAgentDefault
	}

	c := colly.NewCollector(
		colly.AllowedDomains(tgDomain),
		colly.UserAgent(ua),
		colly.StdlibContext(ctx),
	)

	c.OnHTML(".tgme_channel_info_header", func(e *colly.HTMLElement) {
		channel.Title = e.ChildText(".tgme_channel_info_header_title")
		channel.ImageURL = e.ChildAttr("img", "src")
	})

	c.OnHTML(".tgme_widget_message", func(e *colly.HTMLElement) {
		var err error
		var post = entity.Post{}

		if post.ID, err = extractPostIDFromPath(e.Attr("data-post")); err != nil {
			logger.Error("Could not get post ID",
				"path", e.Attr("data-post"),
				"error", err)
			return
		}

		post.URL = fmt.Sprintf("https://%s/%s/%d", tgDomain, username, post.ID)
		post.Title = extractTitle(e)
		post.ContentHTML, err = e.DOM.Find(".tgme_widget_message_text").Html()

		if err != nil {
			logger.Error("Could not get HTML post content",
				"url", post.URL,
				"error", err)
			return
		}

		post.Images = extractImages(e)

		if len(post.Images) > 0 {
			post.Preview = &post.Images[0]
		} else {
			post.Preview = extractPreview(e)
		}

		dtText, exists := e.DOM.Find(".tgme_widget_message_date").Find("time").Attr("datetime")

		if !exists {
			logger.Error("Could not find datetime", "url", post.URL)
			return
		}

		dt, err := time.Parse(time.RFC3339, dtText)

		if err != nil {
			logger.Error("Could not parse post datetime",
				"url", post.URL,
				"datetime", dtText,
				"error", err)
			return
		}

		post.Datetime = dt

		// Display post deep link in case message content
		// is unsupported by t.me or this scraper
		if post.ContentHTML == "" {
			post.Title = "Message content is unsupported"

			postDeepLink := fmt.Sprintf(
				"tg://resolve?domain=%s&post=%d",
				username, post.ID,
			)

			unsupportedMsgHTML := os.Getenv("UNSUPPORTED_MESSAGE_HTML")

			if unsupportedMsgHTML == "" {
				unsupportedMsgHTML = fmt.Sprintf(
					`<p>Message content is unsupported, try opening it in Telegram mobile app or at t.me using the links below.</p><br><br><a href="%s">[Open in Telegram]</a>&nbsp;&bull;&nbsp;<a href="%s">[Open at t.me]</a>`,
					postDeepLink, post.URL,
				)
			} else {
				unsupportedMsgHTML = strings.ReplaceAll(
					unsupportedMsgHTML, "{postDeepLink}", postDeepLink,
				)

				unsupportedMsgHTML = strings.ReplaceAll(
					unsupportedMsgHTML, "{postURL}", post.URL,
				)
			}

			post.ContentHTML = unsupportedMsgHTML
		}

		channel.Posts = append(channel.Posts, post)
	})

	c.OnError(func(r *colly.Response, err error) {
		logger.Error("Request error",
			"url", channel.URL,
			"status", r.StatusCode,
			"error", err)
	})

	if err := c.Visit(channel.URL); err != nil {
		return nil, fmt.Errorf("could not visit %s: %w", channel.URL, err)
	}

	return channel, nil
}

// extractIDFromPath extracts the numeric ID from a string in the format "prefix/id".
func extractPostIDFromPath(path string) (int, error) {
	parts := strings.Split(path, "/")

	if len(parts) != 2 {
		return 0, fmt.Errorf("invalid path format: expected 'prefix/id', got '%s'", path)
	}

	if parts[1] == "" {
		return 0, fmt.Errorf("empty ID in path: '%s'", path)
	}

	id, err := strconv.Atoi(parts[1])

	if err != nil {
		return 0, fmt.Errorf("invalid numeric ID: %w", err)
	}

	return id, nil
}
