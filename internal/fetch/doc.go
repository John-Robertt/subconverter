// Package fetch handles subscription URL retrieval with in-process
// TTL caching.
//
// Responsibilities:
//   - HTTP subscription fetching (HTTPFetcher)
//   - TTL-based in-memory cache (CachedFetcher)
//   - URL sanitization for error messages (SanitizeURL)
//
// Design reference: docs/design/caching.md
package fetch
