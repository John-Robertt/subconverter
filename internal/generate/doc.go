// Package generate hosts the application service for one "generate config"
// use case.
//
// It owns the request-time orchestration from format-agnostic Pipeline build,
// through target projection, template loading, and final rendering. Transport
// concerns such as HTTP parameter validation stay in internal/server, while
// startup-time concerns such as loading and validating YAML stay in
// cmd/subconverter + internal/config.
package generate
