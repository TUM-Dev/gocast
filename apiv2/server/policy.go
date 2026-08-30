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

	// courseScoped means the RPC acts on one particular course, named by a
	// `course_id` field on its request, and the caller must administer that course.
	// Holding no permission at all is enough when the course was granted to them
	// personally, which is the ordinary case for a lecturer.
	courseScoped bool
}

var (
	// public is for RPCs that answer before anyone has signed in.
	public = accessPolicy{anonymous: true}

	// authenticated is for RPCs that act on behalf of a specific user.
	authenticated = accessPolicy{}
)

// requires builds a policy demanding a capability. No callers yet: every migrated
// RPC is public or personal. It is here for the administrative endpoints.
func requires(p model.Permission) accessPolicy { //nolint:unused // for the admin RPCs
	return accessPolicy{permission: p}
}

// requiresCourseAdmin builds a policy for an RPC acting on one course, which the
// caller must administer. Its request must carry the course as `course_id`; see
// CourseRequest.
//
// This is a policy rather than a line inside each handler on purpose. A helper called
// at the top of a handler is opt-in, and the failure mode of forgetting it is an
// endpoint that administers a course for anyone signed in — an omission that looks
// like nothing at all in review. Declared here, TestEveryMethodHasAPolicy sees it,
// and a handler that forgets to check is not a thing that can exist.
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
