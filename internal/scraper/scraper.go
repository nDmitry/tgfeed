package scraper

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/gocolly/colly/v2"
	"github.com/nDmitry/tgfeed/internal/entity"
)

const cachePath = "cache"
const tgDomain = "t.me"
const userAgent = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/125.0.0.0 Safari/537.36"

var imageExtRe = regexp.MustCompile(`\.(jpg|jpeg|png)$`)

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
		colly.CacheDir(cachePath),
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

		style, exists := e.DOM.Find(".tgme_widget_message_photo_wrap").Attr("style")

		if exists {
			post.ImageURL = style[strings.Index(style, "url(")+4 : strings.Index(style, ")")-1]
			post.ImageURL = strings.Trim(post.ImageURL, "'")
		}

		if post.ImageURL == "" {
			preview, _ := e.DOM.Find(".tgme_widget_message_link_preview").Attr("href")

			if imageExtRe.MatchString(preview) {
				post.ImageURL = preview
			}
		}

		switch filepath.Ext(post.ImageURL) {
		case ".jpg", ".jpeg":
			post.ImageType = "image/jpeg"
		case ".png":
			post.ImageType = "image/png"
		}

		if post.ImageURL != "" {
			if post.ImageSize, err = getImageSize(post.ImageURL); err != nil {
				panic(fmt.Errorf("could not get image size for %s: %w", post.URL, err))
			}
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

func getImageSize(imageURL string) (int64, error) {
	res, err := http.Get(imageURL)

	if err != nil {
		return 0, fmt.Errorf("could not download an image: %w", err)
	}

	defer res.Body.Close()

	tmpFile, err := os.CreateTemp(cachePath, "enclosure_*")

	if err != nil {
		return 0, fmt.Errorf("could not create a temp file: %w", err)
	}

	defer func() {
		tmpFile.Close()
		os.Remove(tmpFile.Name())
	}()

	n, err := io.Copy(tmpFile, res.Body)

	if err != nil {
		return 0, fmt.Errorf("could not save an image into %s: %w", tmpFile.Name(), err)
	}

	return n, nil
}
