package apiv2

import (
	"context"
	"errors"
	"log/slog"
	"testing"

	"go.uber.org/mock/gomock"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
	"gorm.io/gorm"

	protobuf "github.com/TUM-Dev/gocast/apiv2/protobuf/server"
	"github.com/TUM-Dev/gocast/dao"
	"github.com/TUM-Dev/gocast/mock_dao"
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

// A course-scoped request that cannot name a course leaves nothing to check, and the
// interceptor can only refuse every call. The loop is empty until the first such RPC
// exists; the resolution check below keeps this from being a test that never ran.
func TestCourseScopedMethodsCarryACourseID(t *testing.T) {
	fd, err := protoregistry.GlobalFiles.FindFileByPath("server/apiv2.proto")
	if err != nil {
		t.Fatalf("could not find the proto file descriptor: %v", err)
	}

	// The request message of a method, as the handler is handed it.
	requestType := func(t *testing.T, serviceName, methodName string) any {
		t.Helper()

		services := fd.Services()
		for i := 0; i < services.Len(); i++ {
			svc := services.Get(i)
			if string(svc.FullName()) != serviceName {
				continue
			}
			md := svc.Methods().ByName(protoreflect.Name(methodName))
			if md == nil {
				t.Fatalf("%s has no method %s", serviceName, methodName)
			}
			mt, err := protoregistry.GlobalTypes.FindMessageByName(md.Input().FullName())
			if err != nil {
				t.Fatalf("could not resolve %s: %v", md.Input().FullName(), err)
			}
			return mt.New().Interface()
		}

		t.Fatalf("no service named %s in the proto", serviceName)
		return nil
	}

	// Otherwise this passes just as happily when the descriptor walk finds nothing.
	if _, ok := requestType(t, "protobuf.StreamService", "getStream").(StreamRequest); !ok {
		t.Error("GetStreamRequest does not implement StreamRequest; the lookup is broken")
	}

	for _, svc := range services {
		for _, m := range svc.desc.Methods {
			if !methodPolicies[method(svc.desc, m.MethodName)].courseScoped {
				continue
			}
			if _, ok := requestType(t, svc.desc.ServiceName, m.MethodName).(CourseRequest); !ok {
				t.Errorf(
					"%s is course-scoped but its request has no course_id field, so the "+
						"interceptor cannot tell which course it is about",
					m.MethodName,
				)
			}
		}
	}
}

// courseRequest stands in for a generated request carrying a `course_id`.
type courseRequest struct{ id uint32 }

func (c courseRequest) GetCourseId() uint32 { return c.id }

// A lecturer holds no permission and still administers what they were granted; an
// admin administers everything through the wildcard. These separate the two.
func TestAuthorizeCourseScoped(t *testing.T) {
	const granted, other, missing = 1, 2, 404

	courses := map[uint]model.Course{
		granted: {Model: gorm.Model{ID: granted}, UserID: 99},
		other:   {Model: gorm.Model{ID: other}, UserID: 99},
	}

	admin := &model.User{Model: gorm.Model{ID: 1}, Role: model.AdminType}
	lecturer := &model.User{
		Model: gorm.Model{ID: 2},
		Role:  model.LecturerType,
		// What `course_admins` grants; dao.GetUserByID preloads it.
		AdministeredCourses: []model.Course{courses[granted]},
	}
	owner := &model.User{Model: gorm.Model{ID: 99}, Role: model.LecturerType}
	student := &model.User{Model: gorm.Model{ID: 3}, Role: model.StudentType}

	tests := []struct {
		name     string
		user     *model.User
		req      any
		wantCode codes.Code
	}{
		{
			name:     "the wildcard permission administers any course",
			user:     admin,
			req:      courseRequest{granted},
			wantCode: codes.OK,
		},
		{
			name:     "a lecturer administers a course granted to them",
			user:     lecturer,
			req:      courseRequest{granted},
			wantCode: codes.OK,
		},
		{
			name:     "a lecturer is refused a course granted to someone else",
			user:     lecturer,
			req:      courseRequest{other},
			wantCode: codes.NotFound,
		},
		{
			name:     "the course's owner administers it without a grant",
			user:     owner,
			req:      courseRequest{granted},
			wantCode: codes.OK,
		},
		{
			name:     "a signed-in student administers nothing",
			user:     student,
			req:      courseRequest{granted},
			wantCode: codes.NotFound,
		},
		{
			// Same answer as one they may not administer, so codes cannot enumerate.
			name:     "a course that does not exist is refused",
			user:     admin,
			req:      courseRequest{missing},
			wantCode: codes.NotFound,
		},
		{
			// A mistake in the proto, not the caller's. Unknown is what
			// errors.WithStatus maps a 500 to.
			name:     "a request that names no course is refused as misconfigured",
			user:     admin,
			req:      struct{}{},
			wantCode: codes.Unknown,
		},
	}

	// getUser stands in for a course-scoped method; the policy is overridden below.
	const fullMethod = "/protobuf.UserService/getUser"

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			coursesMock := mock_dao.NewMockCoursesDao(ctrl)
			coursesMock.EXPECT().GetCourseById(gomock.Any(), gomock.Any()).
				DoAndReturn(func(_ context.Context, id uint) (model.Course, error) {
					// gorm's Find, which the real implementation uses, reports a missing
					// row as a zero-value course and no error rather than as ErrNotFound.
					return courses[id], nil
				}).AnyTimes()

			api := &API{dao: dao.DaoWrapper{CoursesDao: coursesMock}, log: slog.Default()}

			restore := methodPolicies[fullMethod]
			methodPolicies[fullMethod] = accessPolicy{courseScoped: true}
			defer func() { methodPolicies[fullMethod] = restore }()

			called := false
			handler := func(context.Context, any) (any, error) {
				called = true
				return "ok", nil
			}

			ctx := context.WithValue(context.Background(), callerKey{}, &caller{user: tt.user})
			_, err := api.authorize(ctx, tt.req, &grpc.UnaryServerInfo{FullMethod: fullMethod}, handler)

			if got := status.Code(err); got != tt.wantCode {
				t.Errorf("code = %v, want %v", got, tt.wantCode)
			}
			if want := tt.wantCode == codes.OK; called != want {
				t.Errorf("handler called = %v, want %v", called, want)
			}
		})
	}
}
