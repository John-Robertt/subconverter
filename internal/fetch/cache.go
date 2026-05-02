package fetch

import (
	"context"
	"sync"
	"time"
)

// cacheEntry stores a cached subscription response.
type cacheEntry struct {
	body      []byte
	fetchedAt time.Time
}

// CachedFetcher wraps a Fetcher with an in-process TTL cache.
// It is safe for concurrent use.
type CachedFetcher struct {
	inner Fetcher
	ttl   time.Duration
	mu    sync.Mutex
	cache map[string]cacheEntry
	now   func() time.Time // injectable clock for testing
}

// NewCachedFetcher creates a caching wrapper around inner with the given TTL.
func NewCachedFetcher(inner Fetcher, ttl time.Duration) *CachedFetcher {
	return &CachedFetcher{
		inner: inner,
		ttl:   ttl,
		cache: make(map[string]cacheEntry),
		now:   time.Now,
	}
}

// Fetch returns a cached response if still within TTL, otherwise delegates to the inner fetcher.
// Errors are never cached; a subsequent call will re-fetch.
func (c *CachedFetcher) Fetch(ctx context.Context, rawURL string) ([]byte, error) {
	c.mu.Lock()
	entry, ok := c.cache[rawURL]
	if ok && c.now().Sub(entry.fetchedAt) < c.ttl {
		c.mu.Unlock()
		return cloneBytes(entry.body), nil
	}
	c.mu.Unlock()

	body, err := c.inner.Fetch(ctx, rawURL)
	if err != nil {
		return nil, err
	}

	// Store and return copies to prevent callers from mutating cached data.
	stored := cloneBytes(body)

	c.mu.Lock()
	c.cache[rawURL] = cacheEntry{body: stored, fetchedAt: c.now()}
	c.mu.Unlock()

	return cloneBytes(stored), nil
}

// Invalidate removes one cached URL. The next Fetch for rawURL will delegate
// to the inner fetcher even if the previous entry was still within TTL.
func (c *CachedFetcher) Invalidate(rawURL string) {
	c.mu.Lock()
	delete(c.cache, rawURL)
	c.mu.Unlock()
}

func cloneBytes(src []byte) []byte {
	if src == nil {
		return nil
	}
	dst := make([]byte, len(src))
	copy(dst, src)
	return dst
}
