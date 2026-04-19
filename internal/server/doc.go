// Package server provides the HTTP interface: /generate and /healthz.
//
// Responsibilities:
//   - HTTP handler registration (server.go)
//   - Request parameter validation and response writing (handler.go)
//   - Error-to-HTTP-status-code mapping (errors.go)
//
// Business generation is delegated to a Generator injected via [New]. Runtime
// parameters (-config, -listen, -cache-ttl, -timeout, -access-token) are
// parsed in cmd/subconverter/main.go and injected during wiring.
//
// Design reference: docs/design/api.md
package server
