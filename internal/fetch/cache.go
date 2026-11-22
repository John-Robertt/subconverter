package fetch

import (
	"context"
	"sync"
	"time"
)

// cacheEntry 保存一条已缓存的订阅响应。
type cacheEntry struct {
	body      []byte
	fetchedAt time.Time
}

// CachedFetcher 给 Fetcher 套上一层进程内 TTL 缓存。
// 可以安全地并发使用。
type CachedFetcher struct {
	inner Fetcher
	ttl   time.Duration
	mu    sync.Mutex
	cache map[string]cacheEntry
	now   func() time.Time // 可注入的时钟，供测试使用
}

// NewCachedFetcher 用给定的 TTL 为 inner 创建一层缓存包装。
func NewCachedFetcher(inner Fetcher, ttl time.Duration) *CachedFetcher {
	return &CachedFetcher{
		inner: inner,
		ttl:   ttl,
		cache: make(map[string]cacheEntry),
		now:   time.Now,
	}
}

// Fetch 在缓存仍处于 TTL 内时返回缓存的响应，否则交给内层 fetcher 拉取。
// 错误从不缓存，下一次调用会重新拉取。
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

	// 存入和返回的都是副本，防止调用方修改缓存数据。
	stored := cloneBytes(body)

	c.mu.Lock()
	c.cache[rawURL] = cacheEntry{body: stored, fetchedAt: c.now()}
	c.mu.Unlock()

	return cloneBytes(stored), nil
}

// Invalidate 删除一个 URL 的缓存。即使原缓存项仍在 TTL 内，下一次对 rawURL
// 调用 Fetch 也会交给内层 fetcher 拉取。
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
