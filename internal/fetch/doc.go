// Package fetch handles subscription URL retrieval with in-process
// TTL caching.
//
// Responsibilities (to be implemented in M2):
//   - HTTP subscription fetching
//   - TTL-based in-memory cache
//   - URL sanitization for error messages (redact query params)
//
// Design reference: docs/design/caching.md
package fetch
