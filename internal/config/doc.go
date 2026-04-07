// Package config handles YAML configuration loading, order-preserving
// map parsing, and static field validation.
//
// Responsibilities (to be implemented in M1):
//   - Config struct definitions
//   - OrderedMap for groups, routing, rulesets
//   - YAML loader
//   - Static validation (required fields, enums, regex compilation)
//
// Design reference: docs/design/config-schema.md
package config
