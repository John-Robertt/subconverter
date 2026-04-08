// Package pipeline orchestrates the conversion stages:
// Source -> Filter -> Group -> Route -> ValidateGraph.
//
// Implemented (M2):
//   - SS URI parsing (ParseSSURI)
//   - Source stage: subscription fetching, dedup, custom proxy conversion (Source)
//   - Filter stage: exclude regex filtering (Filter)
//
// Implemented (M3):
//   - Group stage: region groups, chained nodes/groups, @all computation (Group)
//   - Route stage: service groups, rulesets, rules, fallback (Route)
//
// To be implemented (M4):
//   - Graph-level reference validation
//
// Design reference: docs/design/pipeline.md
package pipeline
