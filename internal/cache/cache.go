package cache

import (
	"context"
	"time"
)

// Cache defines the interface for caching data
type Cache interface {
	// Get retrieves a value from the cache
	Get(ctx context.Context, key string) ([]byte, error)

	// Set stores a value in the cache with optional expiration
	// If ttl is 0, the value will not be cached
	Set(ctx context.Context, key string, value []byte, ttl time.Duration) error

	// Close releases any resources used by the cache
	Close() error
}
