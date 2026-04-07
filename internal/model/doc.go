// Package model defines the format-agnostic intermediate representation
// shared across all pipeline stages and renderers.
//
// Core entities (to be implemented in M1):
//   - Proxy:      a single proxy node (subscription, custom, or chained)
//   - ProxyGroup: a named group of proxies (node-scope or route-scope)
//   - Ruleset:    a set of remote rule URLs bound to a policy
//   - Rule:       a single inline rule entry
//   - Pipeline:   aggregation of all above for one generation pass
//
// Design reference: docs/design/domain-model.md
package model
