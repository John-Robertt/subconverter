// Package render serializes a target-projected Pipeline into client-specific
// configuration text.
//
// Implemented renderers:
//   - Clash Meta YAML (render.Clash)
//   - Surge conf (render.Surge)
//
// Target-specific protocol filtering and graph trimming happen in
// internal/target before a Pipeline reaches this package. Both renderers
// accept an optional base template. When provided, generated sections
// (proxies, groups, rules) are injected into the template, preserving all
// user-defined settings. When absent, only generated sections are output.
//
// Design reference: docs/design/rendering.md
package render
