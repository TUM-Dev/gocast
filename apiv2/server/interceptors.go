package apiv2

import (
	"context"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/TUM-Dev/gocast/model"
)

// callerKey is the context key for the resolved caller. Unexported so nothing
// outside this package can collide with it or forge an entry.
type callerKey struct{}

// caller is the result of resolving a request's credentials. The error is kept so
// handlers can tell "not signed in" from "token rejected".
type caller struct {
	user *model.User
	err  error
}

// interceptors returns the chain every unary RPC passes through, outermost first.
// Resolution must precede authorization, which needs to know who is calling.
func (a *API) interceptors() grpc.ServerOption {
	return grpc.ChainUnaryInterceptor(
		a.logRequest,
		a.resolveCaller,
		a.authorize,
	)
}

// logRequest records one line per RPC with its outcome and duration.
func (a *API) logRequest(
	ctx context.Context,
	req any,
	info *grpc.UnaryServerInfo,
	handler grpc.UnaryHandler,
) (any, error) {
	start := time.Now()
	resp, err := handler(ctx, req)

	code := status.Code(err)
	log := a.log.With(
		"method", info.FullMethod,
		"code", code.String(),
		"duration", time.Since(start),
	)

	// Only a fault on this side is worth an error line: rejected credentials are
	// the most common failure and would bury the ones needing attention.
	switch code {
	case codes.OK:
		log.Info("rpc")
	case codes.Unknown, codes.Internal, codes.DataLoss, codes.Unavailable:
		log.Error("rpc failed", "err", err)
	default:
		log.Info("rpc rejected", "err", err)
	}

	return resp, err
}

// resolveCaller authenticates once per request and puts the result in the context,
// so later lookups cost nothing. It rejects nothing: which RPCs allow anonymous
// callers is declared in policy.go.
func (a *API) resolveCaller(
	ctx context.Context,
	req any,
	info *grpc.UnaryServerInfo,
	handler grpc.UnaryHandler,
) (any, error) {
	user, err := a.resolveCurrent(ctx)
	return handler(context.WithValue(ctx, callerKey{}, &caller{user: user, err: err}), req)
}
