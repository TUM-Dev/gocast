// Package apiv2 provides API endpoints for the application.
package apiv2

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"go.uber.org/mock/gomock"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"gorm.io/gorm"

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

// TestIntegrationHeaderMatcher_GatewayPath is the httptest-level gateway path
// test mandated by the LOW review item. It drives the integrationHeaderMatcher
// through all the combinations that matter for the integration mux:
//
//   - Authorization header  → "authorization" metadata key (bearer token)
//   - X-On-Behalf-Of header → "x-on-behalf-of" metadata key (OBO LRZ ID)
//   - Cookie header         → "grpcgateway-Cookie" metadata key
//     (grpc metadata is lowercased, so the handler reads md["grpcgateway-cookie"])
//
// This is a unit-level test of the shared header-matcher function used by the
// single gateway mux registered in Proxy(). A full end-to-end mux test would
// require a running gRPC server; all handler-level assertions for bearer and
// OBO propagation are covered by TestGetServiceAccount and TestGetOnBehalfOfUser.
func TestIntegrationHeaderMatcher_GatewayPath(t *testing.T) {
	cases := []struct {
		name    string
		in      string
		wantOut string
		wantOk  bool
	}{
		// Bearer-token path: Authorization must land on "authorization" so
		// extractBearerFromMetadata can find it via md.Get("authorization").
		{"bearer lowercase", "authorization", "authorization", true},
		{"bearer title-case", "Authorization", "authorization", true},
		{"bearer upper-case", "AUTHORIZATION", "authorization", true},
		// OBO path: X-On-Behalf-Of must land on "x-on-behalf-of" so
		// getOnBehalfOfUser can find it via md.Get("x-on-behalf-of").
		{"obo title-case", "X-On-Behalf-Of", "x-on-behalf-of", true},
		{"obo lowercase", "x-on-behalf-of", "x-on-behalf-of", true},
		// Cookie path: must go through DefaultHeaderMatcher so the existing
		// jwt-cookie-based auth continues to work. DefaultHeaderMatcher maps
		// Cookie → "grpcgateway-Cookie"; gRPC stores all metadata keys
		// lowercased, so the handler reads it as md["grpcgateway-cookie"].
		{"cookie", "Cookie", "grpcgateway-Cookie", true},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
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
// grpc-gateway mux header-propagation test (Fix 4)
// ---------------------------------------------------------------------------

// TestIntegrationHeaderMatcher_ViaGRPCGatewayMux drives an HTTP request through
// an actual runtime.ServeMux wired with integrationHeaderMatcher and asserts that
// all three header paths survive the full grpc-gateway AnnotateIncomingContext
// path:
//
//   - Authorization   → "authorization" metadata key
//   - X-On-Behalf-Of → "x-on-behalf-of" metadata key
//   - Cookie          → "grpcgateway-cookie" metadata key
//     (DefaultHeaderMatcher maps Cookie → "grpcgateway-Cookie"; gRPC stores keys
//     lower-cased, so the downstream handler reads md["grpcgateway-cookie"])
//
// Implementation note: WithMetadata annotators and the header matcher are only
// invoked via runtime.AnnotateIncomingContext, which generated gRPC-gateway code
// calls during handler dispatch — NOT by ServeHTTP itself for unregistered paths.
// To exercise the full path without a live gRPC backend, this test registers a
// stub handler via mux.HandlePath that explicitly calls AnnotateIncomingContext
// (exactly as generated handlers do) and captures the resulting metadata.
func TestIntegrationHeaderMatcher_ViaGRPCGatewayMux(t *testing.T) {
	// capturedMD is written by the stub HandlePath handler.
	var capturedMD metadata.MD

	mux := runtime.NewServeMux(
		runtime.WithIncomingHeaderMatcher(integrationHeaderMatcher),
	)

	// Register a stub handler on a path the test will hit.
	// The handler calls AnnotateIncomingContext — exactly what generated
	// gRPC-gateway handlers do — so the header matcher runs in full.
	if err := mux.HandlePath("GET", "/test/headers", func(w http.ResponseWriter, r *http.Request, _ map[string]string) {
		// AnnotateIncomingContext iterates r.Header, calls integrationHeaderMatcher
		// for each key, and returns a context whose incoming metadata contains the
		// mapped pairs (plus hardcoded "authorization" for the Authorization header).
		annotated, err := runtime.AnnotateIncomingContext(r.Context(), mux, r, "/test.Service/Method")
		if err != nil {
			http.Error(w, "annotate error: "+err.Error(), http.StatusInternalServerError)
			return
		}
		md, _ := metadata.FromIncomingContext(annotated)
		capturedMD = md.Copy()
		w.WriteHeader(http.StatusOK)
	}); err != nil {
		t.Fatalf("HandlePath: %v", err)
	}

	ts := httptest.NewServer(mux)
	defer ts.Close()

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, ts.URL+"/test/headers", nil)
	if err != nil {
		t.Fatalf("http.NewRequest: %v", err)
	}
	req.Header.Set("Authorization", "Bearer testtoken")
	req.Header.Set("X-On-Behalf-Of", "ab12cde")
	req.Header.Set("Cookie", "jwt=testcookie")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("HTTP request failed: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("stub handler returned %d, expected 200", resp.StatusCode)
	}

	// Verify Authorization → "authorization" (integrationHeaderMatcher maps it).
	if vals := capturedMD.Get("authorization"); len(vals) == 0 || !strings.Contains(vals[0], "testtoken") {
		t.Errorf("authorization metadata missing or wrong: %v", capturedMD.Get("authorization"))
	}
	// Verify X-On-Behalf-Of → "x-on-behalf-of" (integrationHeaderMatcher maps it).
	if vals := capturedMD.Get("x-on-behalf-of"); len(vals) == 0 || vals[0] != "ab12cde" {
		t.Errorf("x-on-behalf-of metadata missing or wrong: %v", capturedMD.Get("x-on-behalf-of"))
	}
	// Verify Cookie → "grpcgateway-cookie" (DefaultHeaderMatcher via integrationHeaderMatcher's
	// default branch; gRPC metadata lowercases all keys).
	if vals := capturedMD.Get("grpcgateway-cookie"); len(vals) == 0 || !strings.Contains(vals[0], "jwt=testcookie") {
		t.Errorf("grpcgateway-cookie metadata missing or wrong: %v", capturedMD.Get("grpcgateway-cookie"))
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

	t.Run("unknown lrzId – record not found → 404", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		usersMock := mock_dao.NewMockUsersDao(ctrl)
		// Use gorm.ErrRecordNotFound so the handler maps it to 404.
		usersMock.EXPECT().GetUserByLrzID("ab99zzz").Return(model.User{}, gorm.ErrRecordNotFound)

		api := buildAPI(dao.DaoWrapper{UsersDao: usersMock})
		ctx := incomingCtx("x-on-behalf-of", "ab99zzz")

		_, err := api.getOnBehalfOfUser(ctx)
		if err == nil {
			t.Fatal("expected NotFound error, got nil")
		}
		assertGRPCStatus(t, err, http.StatusNotFound)
	})

	t.Run("lrzId backend error → 500", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		usersMock := mock_dao.NewMockUsersDao(ctrl)
		// A non-record-not-found error (e.g. DB connection failure) must map to 500.
		usersMock.EXPECT().GetUserByLrzID("ab99zzz").Return(model.User{}, errors.New("connection refused"))

		api := buildAPI(dao.DaoWrapper{UsersDao: usersMock})
		ctx := incomingCtx("x-on-behalf-of", "ab99zzz")

		_, err := api.getOnBehalfOfUser(ctx)
		if err == nil {
			t.Fatal("expected InternalError, got nil")
		}
		assertGRPCStatus(t, err, http.StatusInternalServerError)
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
