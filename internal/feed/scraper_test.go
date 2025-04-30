package feed_test

import (
	"context"
	"testing"
	"time"

	"github.com/nDmitry/tgfeed/internal/feed"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestScraper_Scrape(t *testing.T) {
	// Create a scraper instance
	scraper := feed.NewDefaultScraper()

	// Create a context with timeout to prevent test hanging
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Test channel username
	username := "tgfeedtestch"

	// Run the scraper
	channel, err := scraper.Scrape(ctx, username)

	// Basic assertions
	require.NoError(t, err)
	require.NotNil(t, channel)

	// Verify channel metadata
	assert.Equal(t, username, channel.Username)
	assert.Equal(t, "Test channel", channel.Title)
	assert.Contains(t, channel.URL, "https://t.me/s/tgfeedtestch")
	assert.Contains(t, channel.ImageURL, "cdn-telegram.org/file")

	// Verify posts collection
	require.GreaterOrEqual(t, len(channel.Posts), 6, "Expected at least 6 posts")

	// Test specific posts in a table-driven approach
	// The posts appear to be in chronological order (oldest first)
	testCases := []struct {
		name          string
		postIdx       int
		expectedID    int
		expectedTitle string
		contentCheck  func(t *testing.T, content string)
	}{
		{
			name:          "Post 1 - Channel created",
			postIdx:       0,
			expectedID:    1,
			expectedTitle: "Channel created",
			contentCheck: func(t *testing.T, content string) {
				assert.Contains(t, content, "Channel created")
			},
		},
		{
			name:          "Post 2 - Channel photo updated",
			postIdx:       1,
			expectedID:    2,
			expectedTitle: "Channel photo updated",
			contentCheck: func(t *testing.T, content string) {
				assert.Contains(t, content, "Channel photo updated")
			},
		},
		{
			name:          "Post 3 - Encryption in France",
			postIdx:       2,
			expectedID:    3,
			expectedTitle: "😲 Last month, France nearly banned encryption. A law requiring messaging apps…",
			contentCheck: func(t *testing.T, content string) {
				assert.Contains(t, content, "France nearly banned encryption")
				assert.Contains(t, content, "Telegram would rather exit a market")
				assert.Contains(t, content, "European Commission proposed a similar initiative")
			},
		},
		{
			name:          "Post 4 - Easter Greeting with Image",
			postIdx:       3,
			expectedID:    4,
			expectedTitle: "🎁🎁 Happy Easter — the day of Freedom and Rebirth!",
			contentCheck: func(t *testing.T, content string) {
				assert.Contains(t, content, "Happy Easter")
			},
		},
		{
			name:          "Post 5 - Telegram Bonds",
			postIdx:       4,
			expectedID:    5,
			expectedTitle: "📈 Telegram bonds are trading at all-time highs…",
			contentCheck: func(t *testing.T, content string) {
				assert.Contains(t, content, "Telegram bonds are trading at all-time highs")
				assert.Contains(t, content, "1 billion monthly active users")
				assert.Contains(t, content, "profit")
			},
		},
		{
			name:          "Post 6 - TON Network Update",
			postIdx:       5,
			expectedID:    6,
			expectedTitle: "📈 Good month for The Open Network: the biggest names in venture capital…",
			contentCheck: func(t *testing.T, content string) {
				assert.Contains(t, content, "Good month for The Open Network")
				assert.Contains(t, content, "<b>TON</b> has become the backbone of creator economy on Telegram")
				assert.Contains(t, content, "400M")
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Verify we have enough posts
			require.Greater(t, len(channel.Posts), tc.postIdx)

			post := channel.Posts[tc.postIdx]

			// Check post ID
			assert.Equal(t, tc.expectedID, post.ID)

			// Check post title
			assert.Equal(t, tc.expectedTitle, post.Title)

			// Check post URL
			expectedURL := "https://t.me/tgfeedtestch/" + string(rune(tc.expectedID+'0'))
			assert.Contains(t, post.URL, expectedURL)

			// Check content
			tc.contentCheck(t, post.ContentHTML)

			// Verify datetime is set
			assert.False(t, post.Datetime.IsZero(), "Post datetime should be set")
			assert.WithinDuration(t, time.Date(2025, 4, 30, 7, 27, 0, 0, time.UTC), post.Datetime, 10*time.Minute)
		})
	}

	t.Run("Post 4 has image preview", func(t *testing.T) {
		post := channel.Posts[3] // Post 4 (index 3 in chronological order)
		require.NotNil(t, post.Preview, "Post 4 should have a preview image")
		assert.Contains(t, post.Preview.URL, "cdn-telegram.org/file")
		assert.Equal(t, "image/jpeg", post.Preview.Type)
		assert.Greater(t, post.Preview.Size, int64(0))

		// Check images collection
		require.GreaterOrEqual(t, len(post.Images), 1, "Post 4 should have at least one image in the collection")
		assert.Contains(t, post.Images[0].URL, "cdn-telegram.org/file")
		assert.Equal(t, "image/jpeg", post.Images[0].Type)
		assert.Greater(t, post.Images[0].Size, int64(0))
	})
}
