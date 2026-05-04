// Package server provides the HTTP interface for the public generator, admin
// API, health check, and optional embedded Web UI.
//
// Responsibilities:
//   - HTTP handler registration (server.go)
//   - Request parameter validation and response writing (handler.go)
//   - Error-to-HTTP-status-code mapping (errors.go)
//   - Embedded SPA fallback and static asset cache headers (webui.go)
//
// Business generation is delegated to a Generator injected via [New]. Admin API
// handling, session validation, and the optional Web filesystem are also wired
// in through Options. Runtime parameters (-config, -listen, -cache-ttl,
// -timeout, -access-token) are parsed in cmd/subconverter/main.go and injected
// during wiring.
//
// Design reference: docs/design/api.md
package server
