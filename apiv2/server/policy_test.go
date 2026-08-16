package apiv2

import (
	"context"
	"errors"
	"log/slog"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/reflect/protoregistry"
	"gorm.io/gorm"

	protobuf "github.com/TUM-Dev/gocast/apiv2/protobuf/server"
	"github.com/TUM-Dev/gocast/model"
)

// Authorization is only as good as its weakest endpoint, and endpoints go wrong by
// being added without anyone deciding who may call them. Walking the descriptor makes
// that unskippable: a new RPC fails here until its policy is written down.
func TestEveryMethodHasAPolicy(t *testing.T) {
	declared := map[string]bool{}

	for _, svc := range services {
		for _, m := range svc.desc.Methods {
			full := method(svc.desc, m.MethodName)
			declared[full] = true
			if _, ok := methodPolicies[full]; !ok {
				t.Errorf("%s has no access policy; add one to methodPolicies in policy.go", full)
			}
		}
	}

	// The other direction: a policy for a method that no longer exists reads like a
	// rule but enforces nothing.
	for name := range methodPolicies {
		if !declared[name] {
			t.Errorf("%s has a policy but is not a method of any service", name)
		}
	}
}

// The tests above are exhaustive only over the services in `services`, so one added
// to the proto but not there would go unchecked. Compare against the proto itself.
func TestEveryServiceInTheProtoIsPoliced(t *testing.T) {
	fd, err := protoregistry.GlobalFiles.FindFileByPath("server/apiv2.proto")
	if err != nil {
		t.Fatalf("could not find the proto file descriptor: %v", err)
	}

	known := map[string]bool{}
	for _, svc := range services {
		known[svc.desc.ServiceName] = true
	}

	inProto := fd.Services()
	for i := 0; i < inProto.Len(); i++ {
		name := string(inProto.Get(i).FullName())
		if !known[name] {
			t.Errorf("service %s is defined in the proto but missing from `services` in policy.go", name)
		}
	}

	if inProto.Len() != len(services) {
		t.Errorf("proto defines %d services, policy.go lists %d", inProto.Len(), len(services))
	}
}

// Public endpoints are listed explicitly rather than derived, so widening access has
// to be a deliberate edit.
func TestOnlyExpectedMethodsArePublic(t *testing.T) {
	want := map[string]bool{
		"healthCheck":            true,
		"getFrontendConfig":      true,
		"getLoginOptions":        true,
		"resetPassword":          true,
		"getSemesters":           true,
		"getPublicCourses":       true,
		"getCourseBySlug":        true,
		"getLiveCourses":         true,
		"getStream":              true,
		"getVideoSections":       true,
		"getStreamPlaylist":      true,
		"getSubtitles":           true,
		"getThumbs":              true,
		"getServerNotifications": true,
	}

	for _, svc := range services {
		for _, m := range svc.desc.Methods {
			policy := methodPolicies[method(svc.desc, m.MethodName)]
			if policy.anonymous != want[m.MethodName] {
				if policy.anonymous {
					t.Errorf("%s is reachable without credentials but is not in the expected list", m.MethodName)
				} else {
					t.Errorf("%s is expected to be public but requires credentials", m.MethodName)
				}
			}
		}
	}
}

func TestAuthorize(t *testing.T) {
	api := &API{log: slog.Default()}

	called := false
	handler := func(context.Context, any) (any, error) {
		called = true
		return "ok", nil
	}

	// A context carrying a resolved caller, as the resolveCaller interceptor leaves it.
	withUser := func(u *model.User, err error) context.Context {
		return context.WithValue(context.Background(), callerKey{}, &caller{user: u, err: err})
	}

	student := &model.User{Model: gorm.Model{ID: 1}, Role: model.StudentType}
	admin := &model.User{Model: gorm.Model{ID: 2}, Role: model.AdminType}

	tests := []struct {
		name       string
		fullMethod string
		policy     *accessPolicy // overrides the declared policy when set
		ctx        context.Context
		wantCode   codes.Code
		wantCalled bool
	}{
		{
			name:       "an undeclared method is refused",
			fullMethod: "/protobuf.UserService/methodThatDoesNotExist",
			ctx:        withUser(admin, nil),
			wantCode:   codes.PermissionDenied,
		},
		{
			name:       "a public method runs without credentials",
			fullMethod: method(&protobuf.CourseService_ServiceDesc, "getPublicCourses"),
			ctx:        withUser(nil, errors.New("no credentials")),
			wantCode:   codes.OK,
			wantCalled: true,
		},
		{
			name:       "an authenticated method refuses an anonymous caller",
			fullMethod: method(&protobuf.UserService_ServiceDesc, "getUser"),
			ctx:        withUser(nil, errors.New("no credentials")),
			wantCode:   codes.Unauthenticated,
		},
		{
			name:       "an authenticated method runs for any signed-in user",
			fullMethod: method(&protobuf.UserService_ServiceDesc, "getUser"),
			ctx:        withUser(student, nil),
			wantCode:   codes.OK,
			wantCalled: true,
		},
		{
			name:       "a permission-gated method refuses a user without it",
			fullMethod: method(&protobuf.UserService_ServiceDesc, "getUser"),
			policy:     &accessPolicy{permission: model.PermAdministerServer},
			ctx:        withUser(student, nil),
			wantCode:   codes.PermissionDenied,
		},
		{
			name:       "a permission-gated method runs for a user who holds it",
			fullMethod: method(&protobuf.UserService_ServiceDesc, "getUser"),
			policy:     &accessPolicy{permission: model.PermAdministerServer},
			ctx:        withUser(admin, nil),
			wantCode:   codes.OK,
			wantCalled: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.policy != nil {
				restore := methodPolicies[tt.fullMethod]
				methodPolicies[tt.fullMethod] = *tt.policy
				defer func() { methodPolicies[tt.fullMethod] = restore }()
			}

			called = false
			_, err := api.authorize(tt.ctx, nil, &grpc.UnaryServerInfo{FullMethod: tt.fullMethod}, handler)

			if got := status.Code(err); got != tt.wantCode {
				t.Errorf("code = %v, want %v", got, tt.wantCode)
			}
			if called != tt.wantCalled {
				t.Errorf("handler called = %v, want %v", called, tt.wantCalled)
			}
		})
	}
}
