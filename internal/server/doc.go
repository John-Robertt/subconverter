// Package server provides the HTTP interface: /generate and /healthz.
//
// Responsibilities:
//   - HTTP handler registration (server.go)
//   - Request parameter validation and response rendering (handler.go)
//   - Error-to-HTTP-status-code mapping (errors.go)
//
// Runtime parameters (-config, -listen, -cache-ttl, -timeout) are parsed
// in cmd/subconverter/main.go and injected via [New].
//
// Design reference: docs/design/api.md
package server
