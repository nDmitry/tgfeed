package scraper

import (
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/PuerkitoBio/goquery"
	"github.com/gocolly/colly/v2"
)

const (
	maxTitleLength  = 80
	ellipsis        = "…"
	openParenthesis = '('
)

var (
	// Match multiple line breaks (2 or more)
	multipleBreaksRegex = regexp.MustCompile(`(?:<br\s*/?>\s*){2,}|<p>|</p>`)
	// Match multiple spaces
	multipleSpacesRegex = regexp.MustCompile(`\s+`)
	sentenceEndRegex    = regexp.MustCompile(`[.!?…](?:\s|$)|\.{3}`)
)

// ExtractTitle extracts a meaningful title from HTML content following the specified rules.
// It prioritizes:
// 1. First bold text if it appears at the beginning.
// 2. First line of text separated by multiple line breaks.
// 3. First sentence or paragraph from the content.
func ExtractTitle(element *colly.HTMLElement) string {
	messageTextElement := element.DOM.Find(".tgme_widget_message_text")

	if messageTextElement.Length() == 0 {
		return ""
	}

	// Try to extract a bold line as the title first (if it's at the beginning)
	if title := extractBoldTitle(messageTextElement); title != "" {
		return formatTitle(title)
	}

	// Then try to find the first line separated by multiple breaks
	if title := extractFirstLine(messageTextElement); title != "" {
		return formatTitle(title)
	}

	// Otherwise, use the first sentence or paragraph
	text := messageTextElement.Text()
	matches := sentenceEndRegex.FindStringIndex(text)

	if matches != nil {
		return formatTitle(text[:matches[1]])
	}

	return formatTitle(text)
}

// extractBoldTitle attempts to find a bold text at the beginning
func extractBoldTitle(selection *goquery.Selection) string {
	var boldText string

	selection.Find("b").Each(func(_ int, s *goquery.Selection) {
		if boldText != "" {
			return // Already found bold text
		}

		text := s.Text()

		if text == "" {
			return
		}

		// Check if this bold text is at the beginning
		html, _ := selection.Html()

		if strings.HasPrefix(html, "<b>"+text+"</b>") {
			boldText = text
		}
	})

	return boldText
}

// extractFirstLine finds the first line of text before multiple line breaks
func extractFirstLine(selection *goquery.Selection) string {
	html, err := selection.Html()

	if err != nil {
		return ""
	}

	// Split content at multiple line breaks
	parts := multipleBreaksRegex.Split(html, 2)

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
			return strings.TrimRight(text[:parenStart], ",.;:!? ") + ellipsis
		}

		if !inParens {
			result.WriteRune(r)
		}
	}

	return result.String()
}

// truncateAtWordBoundary truncates text at a word boundary
func truncateAtWordBoundary(text string, limit int) string {
	runeCount := utf8.RuneCountInString(text)

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
			truncated = strings.TrimRight(truncated, ",.;:!? ")

			return truncated + ellipsis
		}
	}

	return text
}
