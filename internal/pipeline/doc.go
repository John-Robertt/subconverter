// Package pipeline orchestrates the conversion stages:
// Source -> Filter -> Group -> Route -> ValidateGraph.
//
// Responsibilities (to be implemented in M2-M4):
//   - SS URI parsing
//   - Subscription node filtering
//   - Region group and chain group construction
//   - Service group and route assembly
//   - Graph-level reference validation
//
// Design reference: docs/design/pipeline.md
package pipeline
