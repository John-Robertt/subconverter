// Package errtype defines the four error categories used throughout the
// subconverter pipeline.
//
// Each category maps to a specific HTTP status code (applied in server layer):
//
//   - ConfigError  → 400 (invalid YAML, missing fields, bad regex)
//   - FetchError   → 502 (subscription fetch failures)
//   - BuildError   → 500 (group/route construction failures)
//   - RenderError  → 500 (output generation failures)
//
// Design reference: docs/design/validation.md
package errtype
