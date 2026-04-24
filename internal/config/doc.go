// Package config handles YAML configuration loading, order-preserving
// map parsing, static field validation, and startup preparation.
//
// Responsibilities:
//   - Config struct definitions and YAML deserialization
//   - OrderedMap for groups, routing, rulesets (preserving declaration order)
//   - YAML loader (local file or HTTP URL via fetch.LoadResource)
//   - Static validation (required fields, enums, regex compilation)
//   - Prepare: validates config completeness (name uniqueness, routing
//     member references) and produces startup-prepared RuntimeConfig
//     (compiled regexes, parsed custom proxy URLs, expanded @auto, static
//     namespace, cycle detection). Request-time stages consume RuntimeConfig
//     as read-only input.
//
// Design reference: docs/design/config-schema.md
package config
