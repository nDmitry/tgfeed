package rest_test

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/nDmitry/tgfeed/internal/api/rest"
	"github.com/nDmitry/tgfeed/internal/cache"
	"github.com/nDmitry/tgfeed/internal/entity"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockScraper is a mock implementation of the Scraper interface
type MockScraper struct {
	ScrapeFunc func(ctx context.Context, username string) (*entity.Channel, error)
}

func (m *MockScraper) Scrape(ctx context.Context, username string) (*entity.Channel, error) {
	return m.ScrapeFunc(ctx, username)
}

// MockGenerator is a mock implementation of the Generator interface
type MockGenerator struct {
	GenerateFunc func(channel *entity.Channel, params *entity.FeedParams) ([]byte, error)
}

func (m *MockGenerator) Generate(channel *entity.Channel, params *entity.FeedParams) ([]byte, error) {
	return m.GenerateFunc(channel, params)
}

// MockCache is a mock implementation of the Cache interface
type MockCache struct {
	GetFunc func(ctx context.Context, key string) ([]byte, error)
	SetFunc func(ctx context.Context, key string, value []byte, ttl time.Duration) error
}

func (m *MockCache) Get(ctx context.Context, key string) ([]byte, error) {
	return m.GetFunc(ctx, key)
}

func (m *MockCache) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	return m.SetFunc(ctx, key, value, ttl)
}

func (m *MockCache) Close() error {
	return nil
}

func TestTelegramHandler_GetChannelFeed(t *testing.T) {
	tests := []struct {
		name               string
		url                string
		setupMocks         func(cache *MockCache, scraper *MockScraper, generator *MockGenerator)
		expectedStatusCode int
		expectedHeaders    map[string]string
		expectedBodyPart   string
	}{
		{
			name: "Successful RSS feed generation with cache miss",
			url:  "/telegram/channel/testchannel?format=rss&cache_ttl=60",
			setupMocks: func(mockCache *MockCache, mockScraper *MockScraper, mockGenerator *MockGenerator) {
				// Cache miss
				mockCache.GetFunc = func(_ context.Context, _ string) ([]byte, error) {
					return nil, cache.ErrCacheMiss
				}

				// Scraper returns channel data
				mockScraper.ScrapeFunc = func(_ context.Context, username string) (*entity.Channel, error) {
					assert.Equal(t, "testchannel", username)
					return &entity.Channel{
						Username: "testchannel",
						Title:    "Test Channel",
						URL:      "https://t.me/s/testchannel",
						Posts: []entity.Post{
							{
								ID:          1,
								URL:         "https://t.me/testchannel/1",
								Title:       "Test Post",
								ContentHTML: "<p>Test content</p>",
								Datetime:    time.Now(),
							},
						},
					}, nil
				}

				// Generator returns feed content
				mockGenerator.GenerateFunc = func(_ *entity.Channel, params *entity.FeedParams) ([]byte, error) {
					assert.Equal(t, "testchannel", params.Username)
					assert.Equal(t, entity.FormatRSS, params.Format)
					return []byte("<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n<rss version=\"2.0\"><channel><title>Test Channel</title></channel></rss>"), nil
				}

				// Cache set
				mockCache.SetFunc = func(_ context.Context, _ string, _ []byte, _ time.Duration) error {
					return nil
				}
			},
			expectedStatusCode: http.StatusOK,
			expectedHeaders: map[string]string{
				"Content-Type":   "application/rss+xml; charset=utf-8",
				"Cache-Control":  "public, max-age=3600",
				"X-CACHE-STATUS": "MISS",
			},
			expectedBodyPart: "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n<rss",
		},
		{
			name: "Successful Atom feed generation",
			url:  "/telegram/channel/testchannel?format=atom",
			setupMocks: func(mockCache *MockCache, mockScraper *MockScraper, mockGenerator *MockGenerator) {
				// Cache miss
				mockCache.GetFunc = func(_ context.Context, _ string) ([]byte, error) {
					return nil, cache.ErrCacheMiss
				}

				// Scraper returns channel data
				mockScraper.ScrapeFunc = func(_ context.Context, username string) (*entity.Channel, error) {
					assert.Equal(t, "testchannel", username)
					return &entity.Channel{
						Username: "testchannel",
						Title:    "Test Channel",
						URL:      "https://t.me/s/testchannel",
						Posts:    []entity.Post{},
					}, nil
				}

				// Generator returns feed content
				mockGenerator.GenerateFunc = func(_ *entity.Channel, params *entity.FeedParams) ([]byte, error) {
					assert.Equal(t, "testchannel", params.Username)
					assert.Equal(t, entity.FormatAtom, params.Format)
					return []byte("<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n<feed xmlns=\"http://www.w3.org/2005/Atom\"><title>Test Channel</title></feed>"), nil
				}

				// Cache set
				mockCache.SetFunc = func(_ context.Context, _ string, _ []byte, _ time.Duration) error {
					return nil
				}
			},
			expectedStatusCode: http.StatusOK,
			expectedHeaders: map[string]string{
				"Content-Type":   "application/atom+xml; charset=utf-8",
				"Cache-Control":  "public, max-age=3600",
				"X-CACHE-STATUS": "MISS",
			},
			expectedBodyPart: "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n<feed",
		},
		{
			name: "Cache hit",
			url:  "/telegram/channel/testchannel?format=rss&cache_ttl=60",
			setupMocks: func(mockCache *MockCache, mockScraper *MockScraper, mockGenerator *MockGenerator) {
				// Cache hit
				mockCache.GetFunc = func(_ context.Context, _ string) ([]byte, error) {
					return []byte("<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n<rss version=\"2.0\"><channel><title>Cached Feed</title></channel></rss>"), nil
				}

				// Scraper and generator should not be called
				mockScraper.ScrapeFunc = func(_ context.Context, _ string) (*entity.Channel, error) {
					t.Fatal("Scraper should not be called on cache hit")
					return nil, nil
				}

				mockGenerator.GenerateFunc = func(_ *entity.Channel, _ *entity.FeedParams) ([]byte, error) {
					t.Fatal("Generator should not be called on cache hit")
					return nil, nil
				}
			},
			expectedStatusCode: http.StatusOK,
			expectedHeaders: map[string]string{
				"Content-Type":   "application/rss+xml; charset=utf-8",
				"Cache-Control":  "public, max-age=3600",
				"X-CACHE-STATUS": "HIT",
			},
			expectedBodyPart: "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n<rss",
		},
		{
			name: "Scraper error",
			url:  "/telegram/channel/testchannel",
			setupMocks: func(mockCache *MockCache, mockScraper *MockScraper, mockGenerator *MockGenerator) {
				// Cache miss
				mockCache.GetFunc = func(_ context.Context, _ string) ([]byte, error) {
					return nil, cache.ErrCacheMiss
				}

				// Scraper returns error
				mockScraper.ScrapeFunc = func(_ context.Context, _ string) (*entity.Channel, error) {
					return nil, errors.New("scraper error")
				}

				// Generator should not be called
				mockGenerator.GenerateFunc = func(_ *entity.Channel, _ *entity.FeedParams) ([]byte, error) {
					t.Fatal("Generator should not be called when scraper returns error")
					return nil, nil
				}
			},
			expectedStatusCode: http.StatusInternalServerError,
			expectedHeaders: map[string]string{
				"Content-Type": "application/json",
			},
			expectedBodyPart: "scraper error",
		},
		{
			name: "Generator error",
			url:  "/telegram/channel/testchannel",
			setupMocks: func(mockCache *MockCache, mockScraper *MockScraper, mockGenerator *MockGenerator) {
				// Cache miss
				mockCache.GetFunc = func(_ context.Context, _ string) ([]byte, error) {
					return nil, cache.ErrCacheMiss
				}

				// Scraper returns channel data
				mockScraper.ScrapeFunc = func(_ context.Context, _ string) (*entity.Channel, error) {
					return &entity.Channel{
						Username: "testchannel",
						Title:    "Test Channel",
						URL:      "https://t.me/s/testchannel",
						Posts:    []entity.Post{},
					}, nil
				}

				// Generator returns error
				mockGenerator.GenerateFunc = func(_ *entity.Channel, _ *entity.FeedParams) ([]byte, error) {
					return nil, errors.New("generator error")
				}
			},
			expectedStatusCode: http.StatusInternalServerError,
			expectedHeaders: map[string]string{
				"Content-Type": "application/json",
			},
			expectedBodyPart: "generator error",
		},
		{
			name: "No caching with cache_ttl=0",
			url:  "/telegram/channel/testchannel?cache_ttl=0",
			setupMocks: func(mockCache *MockCache, mockScraper *MockScraper, mockGenerator *MockGenerator) {
				// Cache should not be used
				mockCache.GetFunc = func(_ context.Context, _ string) ([]byte, error) {
					t.Fatal("Cache Get should not be called when cache_ttl=0")
					return nil, nil
				}

				// Scraper returns channel data
				mockScraper.ScrapeFunc = func(_ context.Context, _ string) (*entity.Channel, error) {
					return &entity.Channel{
						Username: "testchannel",
						Title:    "Test Channel",
						URL:      "https://t.me/s/testchannel",
						Posts:    []entity.Post{},
					}, nil
				}

				// Generator returns feed content
				mockGenerator.GenerateFunc = func(_ *entity.Channel, _ *entity.FeedParams) ([]byte, error) {
					return []byte("<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n<rss version=\"2.0\"><channel><title>Test Channel</title></channel></rss>"), nil
				}

				// Cache set should not be called
				mockCache.SetFunc = func(_ context.Context, _ string, _ []byte, _ time.Duration) error {
					t.Fatal("Cache Set should not be called when cache_ttl=0")
					return nil
				}
			},
			expectedStatusCode: http.StatusOK,
			expectedHeaders: map[string]string{
				"Content-Type":   "application/rss+xml; charset=utf-8",
				"Cache-Control":  "no-cache",
				"X-CACHE-STATUS": "MISS",
			},
			expectedBodyPart: "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n<rss",
		},
		{
			name: "Feed with exclude words parameter",
			url:  "/telegram/channel/testchannel?exclude=word1|word2&exclude_case_sensitive=1",
			setupMocks: func(mockCache *MockCache, mockScraper *MockScraper, mockGenerator *MockGenerator) {
				// Cache miss
				mockCache.GetFunc = func(_ context.Context, _ string) ([]byte, error) {
					return nil, cache.ErrCacheMiss
				}

				// Scraper returns channel data
				mockScraper.ScrapeFunc = func(_ context.Context, _ string) (*entity.Channel, error) {
					return &entity.Channel{
						Username: "testchannel",
						Title:    "Test Channel",
						URL:      "https://t.me/s/testchannel",
						Posts:    []entity.Post{},
					}, nil
				}

				// Generator returns feed content with correct params
				mockGenerator.GenerateFunc = func(_ *entity.Channel, params *entity.FeedParams) ([]byte, error) {
					assert.Equal(t, "testchannel", params.Username)
					assert.Len(t, params.ExcludeWords, 2)
					assert.Equal(t, "word1", params.ExcludeWords[0])
					assert.Equal(t, "word2", params.ExcludeWords[1])
					assert.True(t, params.ExcludeCaseSensitive)
					return []byte("<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n<rss version=\"2.0\"><channel><title>Test Channel</title></channel></rss>"), nil
				}

				// Cache set
				mockCache.SetFunc = func(_ context.Context, _ string, _ []byte, _ time.Duration) error {
					return nil
				}
			},
			expectedStatusCode: http.StatusOK,
			expectedHeaders: map[string]string{
				"Content-Type":   "application/rss+xml; charset=utf-8",
				"X-CACHE-STATUS": "MISS",
			},
			expectedBodyPart: "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n<rss",
		},
		{
			name: "Invalid request parameters",
			url:  "/telegram/channel/testchannel?format=invalid",
			setupMocks: func(_ *MockCache, _ *MockScraper, _ *MockGenerator) {
				// No cache, scraper, or generator calls needed
			},
			expectedStatusCode: http.StatusBadRequest,
			expectedHeaders: map[string]string{
				"Content-Type": "application/json",
			},
			expectedBodyPart: "format must be rss or atom",
		},
		{
			name: "Invalid cache TTL",
			url:  "/telegram/channel/testchannel?cache_ttl=invalid",
			setupMocks: func(_ *MockCache, _ *MockScraper, _ *MockGenerator) {
				// No cache, scraper, or generator calls needed
			},
			expectedStatusCode: http.StatusBadRequest,
			expectedHeaders: map[string]string{
				"Content-Type": "application/json",
			},
			expectedBodyPart: "cache_ttl must be a valid integer",
		},
		{
			name: "Missing username",
			url:  "/telegram/channel/",
			setupMocks: func(_ *MockCache, _ *MockScraper, _ *MockGenerator) {
				// No cache, scraper, or generator calls needed
			},
			expectedStatusCode: http.StatusNotFound,
			expectedHeaders: map[string]string{
				"Content-Type": "text/plain; charset=utf-8",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup mocks
			mockCache := &MockCache{}
			mockScraper := &MockScraper{}
			mockGenerator := &MockGenerator{}
			tt.setupMocks(mockCache, mockScraper, mockGenerator)

			// Create a new test server
			mux := http.NewServeMux()
			rest.NewTelegramHandler(mux, mockCache, mockScraper, mockGenerator)

			// Create a test request
			req := httptest.NewRequest(http.MethodGet, tt.url, nil)
			rec := httptest.NewRecorder()

			// Handle the request
			mux.ServeHTTP(rec, req)

			// Check response status code
			assert.Equal(t, tt.expectedStatusCode, rec.Code)

			// Check headers
			for key, value := range tt.expectedHeaders {
				assert.Equal(t, value, rec.Header().Get(key))
			}

			// Check response body
			body, err := io.ReadAll(rec.Body)
			require.NoError(t, err)
			assert.Contains(t, string(body), tt.expectedBodyPart)
		})
	}
}
