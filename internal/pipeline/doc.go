// Package pipeline orchestrates the format-agnostic build stages:
// Source -> Filter -> Group -> Route -> ValidateGraph.
//
// Stages:
//   - Source: subscription/Snell/VLESS fetching, SS/AnyTLS/Snell/VLESS parsing,
//     original-proxy ID allocation, candidate-name preservation, custom proxy
//     conversion
//   - Filter: exclude regex filtering on fetched nodes
//   - Group: region groups, chained-node ID allocation, chained groups, @all
//     computation
//   - Route: service groups, rulesets, rules, fallback
//   - ValidateGraph: graph-level reference and namespace validation
//   - Build: full pipeline orchestration (entry point)
//
// Utilities:
//   - URI helpers: host:port splitting, base64 decoding
//   - Proxy validation: invariant checks on generated proxies
//
// Design reference: docs/design/pipeline.md
package pipeline
