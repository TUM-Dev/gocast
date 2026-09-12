package apiv2

import (
	"context"
	"errors"
	"net/http"

	"google.golang.org/grpc"

	e "github.com/TUM-Dev/gocast/apiv2/errors"
	"github.com/TUM-Dev/gocast/model"
)

// accessPolicy says what a caller needs in order to reach an RPC. The zero value
// grants nothing, so an unlisted method fails closed.
type accessPolicy struct {
	// anonymous allows the RPC to proceed without credentials. The handler still
	// decides what an anonymous caller may see.
	anonymous bool

	// permission, when set, must be held by the caller. Empty means any signed-in
	// user will do.
	permission model.Permission

	// courseScoped requires the caller to administer the course named by the
	// request's `course_id`. A lecturer needs no permission beyond that grant.
	courseScoped bool
}

var (
	// public is for RPCs that answer before anyone has signed in.
	public = accessPolicy{anonymous: true}

	// authenticated is for RPCs that act on behalf of a specific user.
	authenticated = accessPolicy{}
)

// requires builds a policy demanding a capability.
func requires(p model.Permission) accessPolicy {
	return accessPolicy{permission: p}
}

// requiresCourseAdmin gates an RPC on administering the course its request names.
// A policy rather than a handler-side check, so forgetting one cannot open an endpoint.
func requiresCourseAdmin() accessPolicy { //nolint:unused // for the admin RPCs
	return accessPolicy{courseScoped: true}
}

// methodPolicies maps every RPC to its policy, keyed by full gRPC method name,
// derived from the per-service tables in services.go. TestEveryMethodHasAPolicy
// fails when a method is missing, so a new RPC cannot default to public.
//
// A policy is a floor: handlers still narrow what a caller may see.
var methodPolicies = buildPolicies()

// buildPolicies flattens the per-service tables in services.go into the full method
// names the interceptor is handed.
func buildPolicies() map[string]accessPolicy {
	out := make(map[string]accessPolicy)
	for _, svc := range services {
		for name, p := range svc.policies {
			out[method(svc.desc, name)] = p
		}
	}
	return out
}

// method builds the full gRPC method name, as it appears in UnaryServerInfo.
func method(desc *grpc.ServiceDesc, name string) string {
	return "/" + desc.ServiceName + "/" + name
}

// authorize enforces the declared policy before a handler runs. An RPC with no
// policy is refused, so forgetting one breaks the endpoint rather than opening it.
func (a *API) authorize(
	ctx context.Context,
	req any,
	info *grpc.UnaryServerInfo,
	handler grpc.UnaryHandler,
) (any, error) {
	policy, declared := methodPolicies[info.FullMethod]
	if !declared {
		a.log.Error("no access policy declared for method, refusing", "method", info.FullMethod)
		return nil, e.WithStatus(http.StatusForbidden, errors.New("no access policy declared for this endpoint"))
	}

	if policy.anonymous {
		return handler(ctx, req)
	}

	user, err := a.getCurrent(ctx)
	if err != nil {
		return nil, e.WithStatus(http.StatusUnauthorized, err)
	}

	if policy.permission != "" && !user.Can(policy.permission) {
		return nil, e.WithStatus(http.StatusForbidden, errors.New("insufficient permissions"))
	}

	if policy.courseScoped {
		if err := a.authorizeCourseAdmin(ctx, user, req, info.FullMethod); err != nil {
			return nil, err
		}
	}

	return handler(ctx, req)
}
