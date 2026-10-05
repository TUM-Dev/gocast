// Package auth authenticates the gRPC links between gocast and its runners.
//
// Both directions use the same shared secret: gocast proves itself to a runner before the
// runner accepts an ffmpeg job, and a runner proves itself to gocast before gocast accepts a
// registration or a notification. The secret travels as a bearer token in the request
// metadata, next to the runner's hostname, so gocast can tie a notification to the runner
// that sent it. Without this, anyone who can reach either port can hand out stream jobs,
// overwrite playlist URLs or crash the process (see the 2026-10 security audit).
//
// The package lives in the runner module so the main module imports one implementation
// instead of keeping a copy in sync.
package auth

import (
	"context"
	"crypto/subtle"
	"log/slog"
	"runtime/debug"
	"strings"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

const (
	// authorizationKey is the metadata key that carries "Bearer <token>".
	authorizationKey = "authorization"
	// hostnameKey is the metadata key that carries the sending runner's hostname. It is only
	// meaningful once the token check passed; a caller without the token can't set it.
	hostnameKey = "x-gocast-runner"

	bearerPrefix = "bearer "
)

type hostnameCtxKey struct{}

// CallerHostname returns the hostname the authenticated peer sent along with its token.
// It is only set in contexts that passed UnaryServerInterceptor with requireHostname.
func CallerHostname(ctx context.Context) (string, bool) {
	h, ok := ctx.Value(hostnameCtxKey{}).(string)
	return h, ok && h != ""
}

// UnaryServerInterceptor rejects every call that doesn't carry the shared token. With
// requireHostname, the call must also name the runner it comes from, which is then
// available through CallerHostname. An empty token makes every call fail: a server
// without a configured secret must not be reachable, so the callers refuse to start
// instead of passing "".
func UnaryServerInterceptor(token string, requireHostname bool) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, _ *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		md, ok := metadata.FromIncomingContext(ctx)
		if !ok {
			return nil, status.Error(codes.Unauthenticated, "missing credentials")
		}
		if !tokenMatches(token, md.Get(authorizationKey)) {
			return nil, status.Error(codes.Unauthenticated, "invalid token")
		}
		if requireHostname {
			hosts := md.Get(hostnameKey)
			if len(hosts) != 1 || hosts[0] == "" {
				return nil, status.Error(codes.Unauthenticated, "missing runner hostname")
			}
			ctx = context.WithValue(ctx, hostnameCtxKey{}, hosts[0])
		}
		return handler(ctx, req)
	}
}

// tokenMatches compares in constant time so the secret can't be recovered byte by byte
// through response timing. Exactly one authorization value is accepted; a second one is
// an attempt to smuggle a header past a proxy, not a client bug worth tolerating.
func tokenMatches(token string, values []string) bool {
	if token == "" || len(values) != 1 {
		return false
	}
	v := values[0]
	if len(v) < len(bearerPrefix) || !strings.EqualFold(v[:len(bearerPrefix)], bearerPrefix) {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(v[len(bearerPrefix):]), []byte(token)) == 1
}

// UnaryClientInterceptor attaches the shared token, and the hostname when given, to every
// outgoing call.
func UnaryClientInterceptor(token, hostname string) grpc.UnaryClientInterceptor {
	return func(ctx context.Context, method string, req, reply any, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		pairs := []string{authorizationKey, "Bearer " + token}
		if hostname != "" {
			pairs = append(pairs, hostnameKey, hostname)
		}
		ctx = metadata.AppendToOutgoingContext(ctx, pairs...)
		return invoker(ctx, method, req, reply, cc, opts...)
	}
}

// RecoveryInterceptor turns a panicking handler into an Internal error instead of letting
// it take the whole process down. grpc-go deliberately doesn't recover handler panics, and
// a single malformed message (an unset optional field dereferenced by the handler) used to
// be enough to stop every stream on the server.
func RecoveryInterceptor(logger *slog.Logger) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp any, err error) {
		defer func() {
			if r := recover(); r != nil {
				logger.Error("grpc handler panicked", "method", info.FullMethod, "panic", r, "stack", string(debug.Stack()))
				err = status.Errorf(codes.Internal, "internal error")
			}
		}()
		return handler(ctx, req)
	}
}
