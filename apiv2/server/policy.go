package apiv2

import (
	"context"
	"errors"
	"net/http"

	"google.golang.org/grpc"

	e "github.com/TUM-Dev/gocast/apiv2/errors"
	protobuf "github.com/TUM-Dev/gocast/apiv2/protobuf/server"
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

// methodPolicies maps every RPC to its policy, keyed by full gRPC method name.
//
// Every method in the service must appear here; TestEveryMethodHasAPolicy walks the
// service descriptor and fails when one does not. That test is the guarantee: adding
// an RPC without deciding who may call it does not compile into a silently public
// endpoint, it breaks the build.
//
// A policy declared here is a floor, not the whole rule. Handlers still narrow what
// a caller may see — a public course listing shows less to an anonymous caller than
// to an enrolled one — and this only settles whether the call is allowed to run.
var methodPolicies = map[string]accessPolicy{
	method("healthCheck"): public,

	// Reachable before signing in, by definition: they render or drive the login
	// page itself.
	method("getLoginOptions"): public,
	method("resetPassword"):   public,

	// Course and stream browsing. Anonymous callers see public and hidden courses;
	// the handlers filter the rest by visibility.
	method("getSemesters"):           public,
	method("getPublicCourses"):       public,
	method("getCourseBySlug"):        public,
	method("getLiveCourses"):         public,
	method("getStream"):              public,
	method("getVideoSections"):       public,
	method("getStreamPlaylist"):      public,
	method("getSubtitles"):           public,
	method("getThumbs"):              public,
	method("getServerNotifications"): public,

	// Everything below acts on, or reveals, one particular account.
	method("getUser"):            authenticated,
	method("updateUserSettings"): authenticated,
	method("exportPersonalData"): authenticated,
	method("getUserCourses"):     authenticated,
	method("getPinnedCourses"):   authenticated,
	method("getPinForCourse"):    authenticated,
	method("pinCourse"):          authenticated,
	method("getProgressBatch"):   authenticated,
	method("updateProgress"):     authenticated,
	method("addBookmark"):        authenticated,
	method("getBookmarks"):       authenticated,
	method("updateBookmark"):     authenticated,
	method("deleteBookmark"):     authenticated,
	method("getNotifications"):   authenticated,
}

// method builds the full gRPC method name the interceptor is given, so the table
// above reads as the proto does rather than repeating the service prefix 27 times.
func method(name string) string {
	return "/" + protobuf.API_ServiceDesc.ServiceName + "/" + name
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

	return handler(ctx, req)
}
