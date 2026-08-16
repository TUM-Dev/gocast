package apiv2

import (
	"context"
	"errors"
	"log/slog"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"gorm.io/gorm"

	protobuf "github.com/TUM-Dev/gocast/apiv2/protobuf/server"
	"github.com/TUM-Dev/gocast/model"
)

// Authorization is only as good as its weakest endpoint, and endpoints go wrong by
// being added without anyone deciding who may call them. Walking the descriptor makes
// that unskippable: a new RPC fails here until its policy is written down.
func TestEveryMethodHasAPolicy(t *testing.T) {
	for _, m := range protobuf.API_ServiceDesc.Methods {
		full := method(m.MethodName)
		if _, ok := methodPolicies[full]; !ok {
			t.Errorf("%s has no access policy; add one to methodPolicies in policy.go", full)
		}
	}

	// The other direction: a policy for a method that no longer exists reads like a
	// rule but enforces nothing.
	declared := make(map[string]bool, len(protobuf.API_ServiceDesc.Methods))
	for _, m := range protobuf.API_ServiceDesc.Methods {
		declared[method(m.MethodName)] = true
	}
	for name := range methodPolicies {
		if !declared[name] {
			t.Errorf("%s has a policy but is not a method of the service", name)
		}
	}
}

// Public endpoints are listed explicitly rather than derived, so widening access has
// to be a deliberate edit.
func TestOnlyExpectedMethodsArePublic(t *testing.T) {
	want := map[string]bool{
		"healthCheck":            true,
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

	for _, m := range protobuf.API_ServiceDesc.Methods {
		policy := methodPolicies[method(m.MethodName)]
		if policy.anonymous != want[m.MethodName] {
			if policy.anonymous {
				t.Errorf("%s is reachable without credentials but is not in the expected list", m.MethodName)
			} else {
				t.Errorf("%s is expected to be public but requires credentials", m.MethodName)
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
			fullMethod: "/protobuf.API/methodThatDoesNotExist",
			ctx:        withUser(admin, nil),
			wantCode:   codes.PermissionDenied,
		},
		{
			name:       "a public method runs without credentials",
			fullMethod: method("getPublicCourses"),
			ctx:        withUser(nil, errors.New("no credentials")),
			wantCode:   codes.OK,
			wantCalled: true,
		},
		{
			name:       "an authenticated method refuses an anonymous caller",
			fullMethod: method("getUser"),
			ctx:        withUser(nil, errors.New("no credentials")),
			wantCode:   codes.Unauthenticated,
		},
		{
			name:       "an authenticated method runs for any signed-in user",
			fullMethod: method("getUser"),
			ctx:        withUser(student, nil),
			wantCode:   codes.OK,
			wantCalled: true,
		},
		{
			name:       "a permission-gated method refuses a user without it",
			fullMethod: method("getUser"),
			policy:     &accessPolicy{permission: model.PermAdministerServer},
			ctx:        withUser(student, nil),
			wantCode:   codes.PermissionDenied,
		},
		{
			name:       "a permission-gated method runs for a user who holds it",
			fullMethod: method("getUser"),
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
