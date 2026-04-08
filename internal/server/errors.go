package server

import (
	"errors"
	"net/http"

	"github.com/John-Robertt/subconverter/internal/errtype"
)

// mapError maps a pipeline or render error to an HTTP status code and message.
// It uses errors.As which traverses errors.Join chains (Go 1.20+).
func mapError(err error) (statusCode int, message string) {
	var cfgErr *errtype.ConfigError
	var fetchErr *errtype.FetchError
	var buildErr *errtype.BuildError
	var renderErr *errtype.RenderError

	switch {
	case errors.As(err, &cfgErr):
		return http.StatusBadRequest, err.Error()
	case errors.As(err, &fetchErr):
		return http.StatusBadGateway, err.Error()
	case errors.As(err, &buildErr):
		return http.StatusInternalServerError, err.Error()
	case errors.As(err, &renderErr):
		return http.StatusInternalServerError, err.Error()
	default:
		return http.StatusInternalServerError, "internal error"
	}
}
