// Package fetch 负责拉取订阅 URL，并提供进程内 TTL 缓存。
//
// 职责：
//   - 通过 HTTP 拉取订阅（HTTPFetcher）
//   - 基于 TTL 的内存缓存（CachedFetcher）
//   - 为错误信息清理 URL 中的敏感部分（SanitizeURL）
//
// 设计参考：docs/design/caching.md
package fetch
