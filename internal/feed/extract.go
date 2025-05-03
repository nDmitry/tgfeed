package feed

import (
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/PuerkitoBio/goquery"
	"github.com/gocolly/colly/v2"
	"github.com/nDmitry/tgfeed/internal/app"
	"github.com/nDmitry/tgfeed/internal/entity"
)

const (
	maxTitleLength  = 80
	ellipsis        = "…"
	openParenthesis = '('
	punctuation     = ",.;:!? "
)

var (
	breaksRegex         = regexp.MustCompile(`(?:<br\s*/?>\s*){1,}|<p>|</p>`)
	multipleSpacesRegex = regexp.MustCompile(`\s+`)
	sentenceEndRegex    = regexp.MustCompile(`[.!?…](?:\s|$)|\.{3}`)
	imageExtRegex       = regexp.MustCompile(`\.(jpg|jpeg|png|gif)$`)
)

// extractTitle extracts a meaningful title from HTML content following the specified rules.
// It prioritizes:
// 1. First line of text separated by multiple line breaks.
// 2. First sentence or paragraph from the content.
func extractTitle(element *colly.HTMLElement) string {
	msgContainer := findMessageContainer(element)

	if msgContainer == nil {
		return ""
	}

	if title := extractFirstLine(msgContainer); title != "" {
		return formatTitle(title)
	}

	text := msgContainer.Text()
	matches := sentenceEndRegex.FindStringIndex(text)

	if matches != nil {
		return formatTitle(text[:matches[1]])
	}

	return formatTitle(text)
}

func findMessageContainer(element *colly.HTMLElement) *goquery.Selection {
	msgContainer := element.DOM.Find(".tgme_widget_message_text")

	if msgContainer.Length() == 0 {
		return nil
	}

	// Sometimes there are two inner div.tgme_widget_message_text elements
	// nested in eache other, in which case the most deep one is used.
	if msgContainer.Length() > 1 {
		deepest := msgContainer

		for {
			nestedElement := deepest.Find(".tgme_widget_message_text")

			if nestedElement.Length() == 0 {
				break
			}

			deepest = nestedElement
		}

		msgContainer = deepest
	}

	return msgContainer
}

// extractFirstLine finds the first line of text before multiple line breaks
func extractFirstLine(selection *goquery.Selection) string {
	html, err := selection.Html()

	if err != nil {
		return ""
	}

	// Split content at multiple line breaks
	parts := breaksRegex.Split(html, 2)

	if len(parts) > 1 {
		// Create a new document from the first part to extract text
		doc, err := goquery.NewDocumentFromReader(strings.NewReader(parts[0]))

		if err != nil {
			return ""
		}

		return strings.TrimSpace(doc.Text())
	}

	return ""
}

// formatTitle ensures the title follows the specified rules
func formatTitle(text string) string {
	// Clean up spaces
	text = multipleSpacesRegex.ReplaceAllString(text, " ")
	text = strings.TrimSpace(text)

	// Remove parenthetical text if it crosses the character limit
	text = removeIncompleteParens(text, maxTitleLength)

	// Ensure we don't cut words in half
	text = truncateAtWordBoundary(text, maxTitleLength)

	return text
}

// removeIncompleteParens removes parenthetical text that crosses the character limit
func removeIncompleteParens(text string, limit int) string {
	if utf8.RuneCountInString(text) <= limit {
		return text
	}

	var result strings.Builder
	inParens := false
	parenStart := 0
	runeCount := 0

	for i, r := range text {
		runeCount++

		if r == openParenthesis {
			inParens = true
			parenStart = i
		} else if r == ')' {
			inParens = false
		}

		if runeCount > limit && inParens {
			// If we cross the limit while inside parentheses,
			// remove everything from the opening paren
			return strings.TrimRight(text[:parenStart], punctuation) + ellipsis
		}

		if !inParens {
			result.WriteRune(r)
		}
	}

	return result.String()
}

// truncateAtWordBoundary truncates text at a word boundary
func truncateAtWordBoundary(text string, limit int) string {
	// Remove trailing colon if present
	hasColon := strings.HasSuffix(text, ":")

	if hasColon {
		text = strings.TrimSuffix(text, ":")
	}

	runeCount := utf8.RuneCountInString(text)

	// If text is under the limit and had a colon, add ellipsis
	if runeCount <= limit && hasColon {
		return text + ellipsis
	}

	// Otherwise just return the text
	if runeCount <= limit {
		return text
	}

	lastWordEnd := 0
	currentCount := 0

	for i, r := range text {
		currentCount++

		if unicode.IsSpace(r) {
			lastWordEnd = i
		}

		if currentCount >= limit {
			var truncated string

			if lastWordEnd > 0 {
				// Truncate at the last word boundary
				truncated = text[:lastWordEnd]
			} else {
				// If no word boundary found, just truncate at the limit
				truncated = text[:i]
			}

			// Remove trailing punctuation before adding ellipsis
			truncated = strings.TrimRight(truncated, punctuation)

			return truncated + ellipsis
		}
	}

	return text
}

// extractImages gets all images from message grouped layer
func extractImages(element *colly.HTMLElement) []entity.Image {
	var images []entity.Image

	element.ForEach(".tgme_widget_message_photo_wrap", func(_ int, el *colly.HTMLElement) {
		imageURL := extractImageURLFromStyle(el.Attr("style"))

		if imageURL == "" {
			return
		}

		imageType := extractImageTypeFromURL(imageURL)
		imageSize := getImageSize(imageURL)

		images = append(images, entity.Image{
			URL:  imageURL,
			Type: imageType,
			Size: imageSize,
		})
	})

	return images
}

// extractPreview finds an image link preview and extracts it
func extractPreview(element *colly.HTMLElement) *entity.Image {
	previewURL, exists := element.DOM.Find(".tgme_widget_message_link_preview").Attr("href")

	if exists && imageExtRegex.MatchString(previewURL) {
		preview := &entity.Image{
			URL: previewURL,
		}

		preview.Type = extractImageTypeFromURL(preview.URL)
		preview.Size = getImageSize(preview.URL)

		return preview
	}

	return nil
}

func extractImageURLFromStyle(style string) string {
	if style == "" {
		return ""
	}

	urlStart := strings.Index(style, "url(")

	if urlStart == -1 {
		return ""
	}

	urlStart += 4 // Skip "url("
	urlEnd := strings.Index(style[urlStart:], ")") + urlStart

	if urlEnd <= urlStart {
		return ""
	}

	url := style[urlStart:urlEnd]
	url = strings.Trim(url, "'\"")

	return url
}

func extractImageTypeFromURL(url string) string {
	switch filepath.Ext(url) {
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".png":
		return "image/png"
	case ".gif":
		return "image/gif"
	default:
		return "" // Skip unsupported image types
	}
}

func getImageSize(imageURL string) int64 {
	logger := app.Logger()
	// nolint: gosec
	res, err := httpClient.Get(imageURL)

	if err != nil {
		logger.Error("Could not download an image",
			"imageUrl", imageURL,
			"error", err)

		return 0
	}

	defer res.Body.Close()

	tmpFile, err := os.CreateTemp(tmpPath, "enclosure_*")

	if err != nil {
		logger.Error("Could not create a temp file",
			"imageUrl", imageURL,
			"error", err)

		return 0
	}

	defer func() {
		tmpFile.Close()
		os.Remove(tmpFile.Name())
	}()

	n, err := io.Copy(tmpFile, res.Body)

	if err != nil {
		logger.Error("Could not save an image into tmp file",
			"tmpFilename", tmpFile.Name(),
			"imageUrl", imageURL,
			"error", err)

		return 0
	}

	return n
}
