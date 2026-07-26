// Package model defines the format-agnostic ID graph shared across pipeline
// stages and the target-projected representation consumed by renderers.
//
// Core entities:
//   - Proxy:          one identified proxy node
//   - NodeGroup:      an ordered group of proxy identities
//   - RouteGroup:     an ordered group of typed policy references
//   - Pipeline:       the format-agnostic ID graph for one generation pass
//   - TargetPipeline: renderer input with final target-local names
//
// Design reference: docs/design/domain-model.md
package model
