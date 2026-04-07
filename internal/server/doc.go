// Package server provides the HTTP interface: /generate and /healthz.
//
// Responsibilities (to be implemented in M5):
//   - HTTP handler registration
//   - Request parameter validation
//   - Error-to-HTTP-status-code mapping
//   - Runtime parameter handling (-config, -listen, -cache-ttl, -timeout)
//
// Design reference: docs/design/api.md
package server
