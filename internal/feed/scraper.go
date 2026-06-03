package feed

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gocolly/colly/v2"
	"github.com/nDmitry/tgfeed/internal/app"
	"github.com/nDmitry/tgfeed/internal/entity"
)

const tgProtocolDefault = "https"
const tgDomainDefault = "t.me"
const userAgentDefault = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/125.0.0.0 Safari/537.36"
const tgConcurrentRequests = 3

type semaphore struct {
	c chan struct{}
}

func newSemaphore(n int) *semaphore {
	return &semaphore{c: make(chan struct{}, n)}
}

func (s *semaphore) acquire(ctx context.Context) error {
	select {
	case s.c <- struct{}{}:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (s *semaphore) release() {
	<-s.c
}

type Scraper struct {
	protocol string
	host     string
	sema     *semaphore
}

func NewDefaultScraper() *Scraper {
	return &Scraper{
		protocol: tgProtocolDefault,
		host:     tgDomainDefault,
		sema:     newSemaphore(tgConcurrentRequests),
	}
}

// Scrape fetches channel data from Telegram
func (s *Scraper) Scrape(ctx context.Context, username string) (*entity.Channel, error) {
	logger := app.Logger()

	channel := &entity.Channel{
		Username: username,
		URL:      fmt.Sprintf("%s://%s/s/%s", s.protocol, s.host, username),
	}

	c := s.createCollector(ctx)

	// Replace <tg-emoji> elements with just an emoji, e.g.:
	// <tg-emoji emoji-id="5217457156567096786"><i class="emoji"><b>1️⃣</b></i></tg-emoji> => 1️⃣
	c.OnHTML("tg-emoji", func(e *colly.HTMLElement) {
		e.DOM.ReplaceWithHtml(e.Text)
	})

	// Sometimes it's just `<i class="emoji"><b>1️⃣</b></i>` (without <tg-emoji> wrapper)
	c.OnHTML("i.emoji", func(e *colly.HTMLElement) {
		e.DOM.ReplaceWithHtml(e.Text)
	})

	c.OnHTML(".tgme_channel_info_header", func(e *colly.HTMLElement) {
		channel.Title = e.ChildText(".tgme_channel_info_header_title")
		channel.ImageURL = e.ChildAttr("img", "src")
	})

	c.OnHTML(".tgme_widget_message", func(e *colly.HTMLElement) {
		post, err := s.processPost(e, username, logger)
		if err != nil {
			return // Error already logged in processPost
		}
		channel.Posts = append(channel.Posts, post)
	})

	c.OnError(func(r *colly.Response, err error) {
		logger.Error("Request error",
			"url", channel.URL,
			"status", r.StatusCode,
			"error", err)
	})

	if err := s.sema.acquire(ctx); err != nil {
		return nil, fmt.Errorf("could not acquire semaphore: %w", err)
	}

	defer s.sema.release()

	if err := c.Visit(channel.URL); err != nil {
		return nil, fmt.Errorf("could not visit %s: %w", channel.URL, err)
	}

	return channel, nil
}

// createCollector creates and configures a colly collector
func (s *Scraper) createCollector(ctx context.Context) *colly.Collector {
	ua := os.Getenv("USER_AGENT")
	if ua == "" {
		ua = userAgentDefault
	}

	c := colly.NewCollector(
		colly.AllowedDomains(s.host),
		colly.UserAgent(ua),
		colly.StdlibContext(ctx),
	)

	c.WithTransport(httpTransport)
	c.SetClient(httpClient)

	return c
}

// processPost processes a single post element and returns the post data
func (s *Scraper) processPost(e *colly.HTMLElement, username string, logger *slog.Logger) (entity.Post, error) {
	var post entity.Post
	var err error

	if post.ID, err = s.extractPostIDFromPath(e.Attr("data-post")); err != nil {
		logger.Error("Could not get post ID",
			"path", e.Attr("data-post"),
			"error", err)
		return post, err
	}

	post.URL = fmt.Sprintf("%s://%s/%s/%d", s.protocol, s.host, username, post.ID)
	post.Title = extractTitle(e)

	if err = s.extractPostContent(e, &post, logger); err != nil {
		return post, err
	}

	post.Images = extractImages(e)
	s.setPostPreview(&post, e)

	if err = s.extractPostDatetime(e, &post, logger); err != nil {
		return post, err
	}

	s.handleUnsupportedContent(&post, username)

	return post, nil
}

// extractPostContent extracts HTML content from the post
func (s *Scraper) extractPostContent(e *colly.HTMLElement, post *entity.Post, logger *slog.Logger) error {
	msgContainer := findMessageContainer(e)

	if msgContainer != nil {
		html, err := msgContainer.Html()

		if err != nil {
			logger.Error("Could not get HTML post content",
				"url", post.URL,
				"error", err)
			return err
		}

		post.ContentHTML = html
	} else {
		post.ContentHTML = ""
	}

	return nil
}

// setPostPreview sets the preview image for the post
func (s *Scraper) setPostPreview(post *entity.Post, e *colly.HTMLElement) {
	if len(post.Images) > 0 {
		post.Preview = &post.Images[0]
	} else {
		post.Preview = extractPreview(e)
	}
}

// extractPostDatetime extracts and parses the post datetime
func (s *Scraper) extractPostDatetime(e *colly.HTMLElement, post *entity.Post, logger *slog.Logger) error {
	dtText, exists := e.DOM.Find(".tgme_widget_message_date").Find("time").Attr("datetime")

	if !exists {
		logger.Error("Could not find datetime", "url", post.URL)
		return fmt.Errorf("datetime not found")
	}

	dt, err := time.Parse(time.RFC3339, dtText)

	if err != nil {
		logger.Error("Could not parse post datetime",
			"url", post.URL,
			"datetime", dtText,
			"error", err)
		return err
	}

	post.Datetime = dt

	return nil
}

// handleUnsupportedContent handles posts with unsupported content
func (s *Scraper) handleUnsupportedContent(post *entity.Post, username string) {
	if post.ContentHTML == "" && len(post.Images) == 0 {
		post.Title = "Message content is unsupported"
		post.ContentHTML = s.generateUnsupportedMessageHTML(username, post.ID, post.URL)
	} else if post.ContentHTML == "" && len(post.Images) > 0 {
		// Image-only message - set title from environment variable, leave ContentHTML empty
		post.ImageOnly = true
		if post.Title == "" {
			post.Title = s.generateImagePostTitle()
		}
	}
}

// generateImagePostTitle returns the title for image-only posts
func (s *Scraper) generateImagePostTitle() string {
	imagePostTitle := os.Getenv("IMAGE_POST_TITLE_TEXT")
	if imagePostTitle == "" {
		imagePostTitle = "[🖼️ Image]"
	}
	return imagePostTitle
}

// generateUnsupportedMessageHTML generates HTML for unsupported messages
func (s *Scraper) generateUnsupportedMessageHTML(username string, postID int, postURL string) string {
	postDeepLink := fmt.Sprintf("tg://resolve?domain=%s&post=%d", username, postID)
	unsupportedMsgHTML := os.Getenv("UNSUPPORTED_MESSAGE_HTML")

	if unsupportedMsgHTML == "" {
		return fmt.Sprintf(
			`<p>Message content is unsupported, try opening it in Telegram mobile app or at t.me using the links below.</p><br><br><a href="%s">[Open in Telegram]</a>&nbsp;&bull;&nbsp;<a href="%s">[Open at t.me]</a>`,
			postDeepLink, postURL,
		)
	}

	unsupportedMsgHTML = strings.ReplaceAll(unsupportedMsgHTML, "{postDeepLink}", postDeepLink)
	unsupportedMsgHTML = strings.ReplaceAll(unsupportedMsgHTML, "{postURL}", postURL)

	return unsupportedMsgHTML
}

// extractPostIDFromPath extracts the numeric ID from a string in the format "prefix/id".
func (s *Scraper) extractPostIDFromPath(path string) (int, error) {
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
