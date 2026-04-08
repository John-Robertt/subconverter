// Package render converts the format-agnostic intermediate representation
// into target client configuration text.
//
// Implemented renderers:
//   - Clash Meta YAML (render.Clash)
//   - Surge conf (render.Surge)
//
// Both renderers accept an optional base template. When provided, generated
// sections (proxies, groups, rules) are injected into the template, preserving
// all user-defined settings. When absent, only generated sections are output.
//
// Design reference: docs/design/rendering.md
package render
