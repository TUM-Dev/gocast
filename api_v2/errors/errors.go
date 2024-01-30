// Package errors provides helper functions for handling errors with specific HTTP status codes.
package errors

import (
	"fmt"
	"log/slog"
	"net/http"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// WithStatus creates a new error with a specific HTTP status code and a given error message.
// It maps the HTTP status code to a corresponding gRPC status code.
// If the HTTP status code is not recognized, it logs a warning and uses gRPC's Unknown code.
// It returns a gRPC error with the mapped gRPC status code and the original error message.
func WithStatus(httpStatus int, err error) error {
	var code codes.Code
	switch httpStatus {
	case http.StatusNotFound:
		code = codes.NotFound
	case http.StatusUnauthorized:
		code = codes.Unauthenticated
	case http.StatusForbidden:
		code = codes.PermissionDenied
	case http.StatusBadRequest:
		code = codes.InvalidArgument
	case http.StatusConflict:
		code = codes.AlreadyExists
	case http.StatusTooManyRequests:
		code = codes.ResourceExhausted
	case http.StatusNotImplemented:
		code = codes.Unimplemented
	case http.StatusServiceUnavailable:
		code = codes.Unavailable
	case http.StatusGatewayTimeout:
		code = codes.DeadlineExceeded
	case http.StatusInternalServerError:
		code = codes.Unknown // default to 500
	default:
		slog.Warn("Unknown HTTP status code: ", "httpStatus", fmt.Sprintf("%d", httpStatus))
		code = codes.Unknown
	}
	return status.Error(code, err.Error())
}
