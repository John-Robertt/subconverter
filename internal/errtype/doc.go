// Package errtype defines the six error categories used throughout the
// subconverter pipeline.
//
// Each category maps to a specific HTTP status code (applied in server layer):
//
//   - ConfigError  → 400 (invalid YAML, missing fields, bad regex)
//   - FetchError   → 502 (remote resource fetch failures)
//   - ResourceError → 500 (local resource read failures)
//   - BuildError   → 400 (group/route/config semantic failures)
//   - TargetError  → 500 (target-format projection failures)
//   - RenderError  → 500 (output generation failures)
//
// Design reference: docs/design/validation.md
package errtype
