// Package pipeline orchestrates the conversion stages:
// Source -> Filter -> Group -> Route -> ValidateGraph.
//
// Implemented (M2):
//   - SS URI parsing (ParseSSURI)
//   - Source stage: subscription fetching, dedup, custom proxy conversion (Source)
//   - Filter stage: exclude regex filtering (Filter)
//
// To be implemented (M3-M4):
//   - Region group and chain group construction
//   - Service group and route assembly
//   - Graph-level reference validation
//
// Design reference: docs/design/pipeline.md
package pipeline
