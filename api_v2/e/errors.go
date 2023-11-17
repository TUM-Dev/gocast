package e

import (
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"log/slog"
	"net/http"
)

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
		slog.Warn("Unknown HTTP status code: ", httpStatus)
		code = codes.Unknown
	}
	return status.Error(code, err.Error())
}
