// Package apiv2 provides API endpoints for the application.
package apiv2

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"

	"go.uber.org/mock/gomock"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"github.com/TUM-Dev/gocast/dao"
	"github.com/TUM-Dev/gocast/mock_dao"
	"github.com/TUM-Dev/gocast/model"
)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// buildAPI builds a minimal *API backed by the supplied DaoWrapper (for tests).
func buildAPI(dw dao.DaoWrapper) *API {
	return &API{dao: dw}
}

// incomingCtx returns a context with the given metadata already attached as an
// incoming context (i.e., as if the gRPC framework populated it).
func incomingCtx(kv ...string) context.Context {
	return metadata.NewIncomingContext(context.Background(), metadata.Pairs(kv...))
}

// assertGRPCStatus checks that err is a gRPC status error with the expected
// code (expressed as its HTTP equivalent using the same mapping as e.WithStatus).
func assertGRPCStatus(t *testing.T, err error, wantHTTPStatus int) {
	t.Helper()
	if err == nil {
		t.Fatal("expected non-nil error")
	}
	st, ok := status.FromError(err)
	if !ok {
		t.Fatalf("error is not a gRPC status error: %v", err)
	}
	wantCode := httpStatusToGRPCCode(wantHTTPStatus)
	if st.Code() != wantCode {
		t.Fatalf("expected gRPC code %v (http %d), got %v: %v", wantCode, wantHTTPStatus, st.Code(), err)
	}
}

// httpStatusToGRPCCode mirrors the mapping in apiv2/errors/errors.go.
func httpStatusToGRPCCode(httpStatus int) codes.Code {
	switch httpStatus {
	case http.StatusNotFound:
		return codes.NotFound
	case http.StatusUnauthorized:
		return codes.Unauthenticated
	case http.StatusForbidden:
		return codes.PermissionDenied
	case http.StatusBadRequest:
		return codes.InvalidArgument
	default:
		return codes.Unknown
	}
}

// ---------------------------------------------------------------------------
// integrationHeaderMatcher unit-tests
// ---------------------------------------------------------------------------

func TestIntegrationHeaderMatcher(t *testing.T) {
	cases := []struct {
		in      string
		wantOut string
		wantOk  bool
	}{
		// Custom mappings — case-insensitive
		{"Authorization", "authorization", true},
		{"authorization", "authorization", true},
		{"AUTHORIZATION", "authorization", true},
		{"X-On-Behalf-Of", "x-on-behalf-of", true},
		{"x-on-behalf-of", "x-on-behalf-of", true},
		// Cookie must be passed through via DefaultHeaderMatcher so existing
		// cookie auth is preserved. DefaultHeaderMatcher uses
		// textproto.CanonicalMIMEHeaderKey so Cookie → "grpcgateway-Cookie"
		// (capital C); gRPC metadata stores all keys lowercased, so the
		// handler reading md["grpcgateway-cookie"] still works.
		{"Cookie", "grpcgateway-Cookie", true},
	}

	for _, tc := range cases {
		tc := tc // capture
		t.Run(tc.in, func(t *testing.T) {
			got, ok := integrationHeaderMatcher(tc.in)
			if ok != tc.wantOk {
				t.Fatalf("integrationHeaderMatcher(%q): ok = %v, want %v", tc.in, ok, tc.wantOk)
			}
			if ok && got != tc.wantOut {
				t.Fatalf("integrationHeaderMatcher(%q): got %q, want %q", tc.in, got, tc.wantOut)
			}
		})
	}
}

// An arbitrary custom header that is neither Authorization, X-On-Behalf-Of nor
// a Grpc-Metadata- / Grpcgateway- / known header should be dropped by the
// default matcher (i.e. ok == false).
func TestIntegrationHeaderMatcherDropsUnknown(t *testing.T) {
	_, ok := integrationHeaderMatcher("X-Custom-Unknown-Header-Foobar")
	if ok {
		t.Fatal("expected unknown header to be dropped, but got ok=true")
	}
}

// Grpc-Metadata- prefixed headers must pass through (DefaultHeaderMatcher).
func TestIntegrationHeaderMatcherPassesGrpcMetadata(t *testing.T) {
	got, ok := integrationHeaderMatcher("Grpc-Metadata-Foo")
	if !ok {
		t.Fatal("expected Grpc-Metadata- prefixed header to be forwarded, got ok=false")
	}
	if !strings.EqualFold(got, "foo") {
		t.Fatalf("expected lowercased 'foo', got %q", got)
	}
}

// ---------------------------------------------------------------------------
// extractBearerFromMetadata
// ---------------------------------------------------------------------------

func TestExtractBearerFromMetadata(t *testing.T) {
	t.Run("valid bearer", func(t *testing.T) {
		md := metadata.New(map[string]string{"authorization": "Bearer abc123"})
		tok, err := extractBearerFromMetadata(md)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if tok != "abc123" {
			t.Fatalf("got %q, want %q", tok, "abc123")
		}
	})

	t.Run("missing header", func(t *testing.T) {
		md := metadata.New(nil)
		_, err := extractBearerFromMetadata(md)
		if err == nil {
			t.Fatal("expected error for missing authorization, got nil")
		}
	})

	t.Run("malformed bearer – no 'Bearer ' prefix", func(t *testing.T) {
		md := metadata.New(map[string]string{"authorization": "abc123"})
		_, err := extractBearerFromMetadata(md)
		if err == nil {
			t.Fatal("expected error for malformed bearer, got nil")
		}
	})
}

// ---------------------------------------------------------------------------
// getServiceAccount
// ---------------------------------------------------------------------------

func TestGetServiceAccount(t *testing.T) {
	t.Run("valid service token and ServiceType user", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		tokenMock := mock_dao.NewMockTokenDao(ctrl)
		usersMock := mock_dao.NewMockUsersDao(ctrl)

		tok := model.Token{UserID: 7, Scope: model.TokenScopeService}
		svcUser := model.User{Role: model.ServiceType}
		svcUser.ID = 7

		tokenMock.EXPECT().GetToken("mysecrettoken").Return(tok, nil)
		tokenMock.EXPECT().TokenUsed(tok).Return(nil)
		usersMock.EXPECT().GetUserByID(gomock.Any(), uint(7)).Return(svcUser, nil)

		api := buildAPI(dao.DaoWrapper{TokenDao: tokenMock, UsersDao: usersMock})
		ctx := incomingCtx("authorization", "Bearer mysecrettoken")

		user, err := api.getServiceAccount(ctx)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if user.Role != model.ServiceType {
			t.Fatalf("expected ServiceType user, got role %d", user.Role)
		}
	})

	t.Run("missing authorization header – no metadata", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		api := buildAPI(dao.DaoWrapper{
			TokenDao: mock_dao.NewMockTokenDao(ctrl),
			UsersDao: mock_dao.NewMockUsersDao(ctrl),
		})
		// context with no incoming metadata at all
		_, err := api.getServiceAccount(context.Background())
		if err == nil {
			t.Fatal("expected Unauthenticated error, got nil")
		}
		assertGRPCStatus(t, err, http.StatusUnauthorized)
	})

	t.Run("missing authorization header – metadata present but no auth key", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		api := buildAPI(dao.DaoWrapper{
			TokenDao: mock_dao.NewMockTokenDao(ctrl),
			UsersDao: mock_dao.NewMockUsersDao(ctrl),
		})
		ctx := incomingCtx() // metadata present but no "authorization" key

		_, err := api.getServiceAccount(ctx)
		if err == nil {
			t.Fatal("expected Unauthenticated error, got nil")
		}
		assertGRPCStatus(t, err, http.StatusUnauthorized)
	})

	t.Run("invalid/expired token", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		tokenMock := mock_dao.NewMockTokenDao(ctrl)
		tokenMock.EXPECT().GetToken("badtoken").Return(model.Token{}, errors.New("not found"))

		api := buildAPI(dao.DaoWrapper{
			TokenDao: tokenMock,
			UsersDao: mock_dao.NewMockUsersDao(ctrl),
		})
		ctx := incomingCtx("authorization", "Bearer badtoken")

		_, err := api.getServiceAccount(ctx)
		if err == nil {
			t.Fatal("expected Unauthenticated error, got nil")
		}
		assertGRPCStatus(t, err, http.StatusUnauthorized)
	})

	t.Run("wrong scope – admin token, not service", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		tokenMock := mock_dao.NewMockTokenDao(ctrl)
		tok := model.Token{UserID: 7, Scope: model.TokenScopeAdmin}
		tokenMock.EXPECT().GetToken("admintoken").Return(tok, nil)

		api := buildAPI(dao.DaoWrapper{
			TokenDao: tokenMock,
			UsersDao: mock_dao.NewMockUsersDao(ctrl),
		})
		ctx := incomingCtx("authorization", "Bearer admintoken")

		_, err := api.getServiceAccount(ctx)
		if err == nil {
			t.Fatal("expected PermissionDenied error, got nil")
		}
		assertGRPCStatus(t, err, http.StatusForbidden)
	})

	t.Run("service-scope token on non-ServiceType user – defense in depth", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		tokenMock := mock_dao.NewMockTokenDao(ctrl)
		usersMock := mock_dao.NewMockUsersDao(ctrl)

		tok := model.Token{UserID: 8, Scope: model.TokenScopeService}
		regularUser := model.User{Role: model.AdminType}
		regularUser.ID = 8

		tokenMock.EXPECT().GetToken("servicetoken").Return(tok, nil)
		tokenMock.EXPECT().TokenUsed(tok).Return(nil)
		usersMock.EXPECT().GetUserByID(gomock.Any(), uint(8)).Return(regularUser, nil)

		api := buildAPI(dao.DaoWrapper{TokenDao: tokenMock, UsersDao: usersMock})
		ctx := incomingCtx("authorization", "Bearer servicetoken")

		_, err := api.getServiceAccount(ctx)
		if err == nil {
			t.Fatal("expected PermissionDenied error, got nil")
		}
		assertGRPCStatus(t, err, http.StatusForbidden)
	})
}

// ---------------------------------------------------------------------------
// getOnBehalfOfUser
// ---------------------------------------------------------------------------

func TestGetOnBehalfOfUser(t *testing.T) {
	t.Run("missing x-on-behalf-of header", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		api := buildAPI(dao.DaoWrapper{UsersDao: mock_dao.NewMockUsersDao(ctrl)})
		ctx := incomingCtx() // no OBO header

		_, err := api.getOnBehalfOfUser(ctx)
		if err == nil {
			t.Fatal("expected BadRequest error, got nil")
		}
		assertGRPCStatus(t, err, http.StatusBadRequest)
	})

	t.Run("unknown lrzId", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		usersMock := mock_dao.NewMockUsersDao(ctrl)
		usersMock.EXPECT().GetUserByLrzID("ab99zzz").Return(model.User{}, errors.New("not found"))

		api := buildAPI(dao.DaoWrapper{UsersDao: usersMock})
		ctx := incomingCtx("x-on-behalf-of", "ab99zzz")

		_, err := api.getOnBehalfOfUser(ctx)
		if err == nil {
			t.Fatal("expected NotFound error, got nil")
		}
		assertGRPCStatus(t, err, http.StatusNotFound)
	})

	t.Run("known lrzId", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		usersMock := mock_dao.NewMockUsersDao(ctrl)
		expected := model.User{LrzID: "ab12cde", Role: model.StudentType}
		expected.ID = 42
		usersMock.EXPECT().GetUserByLrzID("ab12cde").Return(expected, nil)

		api := buildAPI(dao.DaoWrapper{UsersDao: usersMock})
		ctx := incomingCtx("x-on-behalf-of", "ab12cde")

		user, err := api.getOnBehalfOfUser(ctx)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if user.LrzID != "ab12cde" {
			t.Fatalf("got LrzID %q, want %q", user.LrzID, "ab12cde")
		}
	})
}

// ---------------------------------------------------------------------------
// requireServiceCourseAdmin
// ---------------------------------------------------------------------------

func TestRequireServiceCourseAdmin(t *testing.T) {
	t.Run("service account is admin of course", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		tokenMock := mock_dao.NewMockTokenDao(ctrl)
		usersMock := mock_dao.NewMockUsersDao(ctrl)
		coursesMock := mock_dao.NewMockCoursesDao(ctrl)

		course := model.Course{}
		course.ID = 10

		svcUser := model.User{Role: model.ServiceType, AdministeredCourses: []model.Course{course}}
		svcUser.ID = 7

		tok := model.Token{UserID: 7, Scope: model.TokenScopeService}

		tokenMock.EXPECT().GetToken("tok").Return(tok, nil)
		tokenMock.EXPECT().TokenUsed(tok).Return(nil)
		usersMock.EXPECT().GetUserByID(gomock.Any(), uint(7)).Return(svcUser, nil)
		coursesMock.EXPECT().GetCourseById(gomock.Any(), uint(10)).Return(course, nil)

		api := buildAPI(dao.DaoWrapper{TokenDao: tokenMock, UsersDao: usersMock, CoursesDao: coursesMock})
		ctx := incomingCtx("authorization", "Bearer tok")

		retSvc, retCourse, err := api.requireServiceCourseAdmin(ctx, 10)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if retSvc.Role != model.ServiceType {
			t.Fatalf("expected ServiceType user, got role %d", retSvc.Role)
		}
		if retCourse.ID != 10 {
			t.Fatalf("expected course ID 10, got %d", retCourse.ID)
		}
	})

	t.Run("service account is NOT admin of course", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		tokenMock := mock_dao.NewMockTokenDao(ctrl)
		usersMock := mock_dao.NewMockUsersDao(ctrl)
		coursesMock := mock_dao.NewMockCoursesDao(ctrl)

		course := model.Course{}
		course.ID = 10

		// svcUser has no administered courses (IsAdminOfCourse returns false)
		svcUser := model.User{Role: model.ServiceType}
		svcUser.ID = 7

		tok := model.Token{UserID: 7, Scope: model.TokenScopeService}

		tokenMock.EXPECT().GetToken("tok").Return(tok, nil)
		tokenMock.EXPECT().TokenUsed(tok).Return(nil)
		usersMock.EXPECT().GetUserByID(gomock.Any(), uint(7)).Return(svcUser, nil)
		coursesMock.EXPECT().GetCourseById(gomock.Any(), uint(10)).Return(course, nil)

		api := buildAPI(dao.DaoWrapper{TokenDao: tokenMock, UsersDao: usersMock, CoursesDao: coursesMock})
		ctx := incomingCtx("authorization", "Bearer tok")

		_, _, err := api.requireServiceCourseAdmin(ctx, 10)
		if err == nil {
			t.Fatal("expected PermissionDenied error, got nil")
		}
		assertGRPCStatus(t, err, http.StatusForbidden)
	})
}
