// Package apiv2 provides API endpoints for the application.
package apiv2

import (
	"crypto/rand"
	"crypto/rsa"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"go.uber.org/mock/gomock"
	"gorm.io/gorm"

	protobuf "github.com/TUM-Dev/gocast/apiv2/protobuf/server"
	"github.com/TUM-Dev/gocast/dao"
	"github.com/TUM-Dev/gocast/mock_dao"
	"github.com/TUM-Dev/gocast/model"
	"github.com/TUM-Dev/gocast/tools"
)

// testJWTKey is the RSA key used for signing in EP2 tests.
// It is initialised once at package level so all tests share a stable key.
var testJWTKey *rsa.PrivateKey

func init() {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		panic("failed to generate test JWT key: " + err.Error())
	}
	testJWTKey = key
	// Wire the key into the tools package so that tools.SignPlaylistURL works.
	tools.SetTestJWTKey(key)
}

// parsePlaybackClaims parses a JWT string from a signed playlist URL and
// returns the embedded JWTPlaylistClaims, verifying the signature.
func parsePlaybackClaims(t *testing.T, signedURL string) *tools.JWTPlaylistClaims {
	t.Helper()
	const needle = "jwt="
	idx := strings.Index(signedURL, needle)
	if idx == -1 {
		t.Fatalf("no jwt= parameter in URL %q", signedURL)
	}
	jwtStr := signedURL[idx+len(needle):]
	if amp := strings.Index(jwtStr, "&"); amp != -1 {
		jwtStr = jwtStr[:amp]
	}
	claims := &tools.JWTPlaylistClaims{}
	tok, err := jwt.ParseWithClaims(jwtStr, claims, func(_ *jwt.Token) (interface{}, error) {
		return testJWTKey.Public(), nil
	})
	if err != nil {
		t.Fatalf("jwt.ParseWithClaims: %v", err)
	}
	if !tok.Valid {
		t.Fatal("JWT is not valid")
	}
	return claims
}

// ---------------------------------------------------------------------------
// clampTTL (pure function)
// ---------------------------------------------------------------------------

// resetTTLConfig zeroes out the three playback-token TTL config fields and
// returns a function that restores the original values. Used in TTL tests to
// ensure a clean config baseline without mutating the package-level singleton
// permanently.
func resetTTLConfig(t *testing.T) func() {
	t.Helper()
	orig := struct{ def, min, max int }{
		tools.Cfg.PlaybackTokenDefaultTTLSeconds,
		tools.Cfg.PlaybackTokenMinTTLSeconds,
		tools.Cfg.PlaybackTokenMaxTTLSeconds,
	}
	tools.Cfg.PlaybackTokenDefaultTTLSeconds = 0
	tools.Cfg.PlaybackTokenMinTTLSeconds = 0
	tools.Cfg.PlaybackTokenMaxTTLSeconds = 0
	return func() {
		tools.Cfg.PlaybackTokenDefaultTTLSeconds = orig.def
		tools.Cfg.PlaybackTokenMinTTLSeconds = orig.min
		tools.Cfg.PlaybackTokenMaxTTLSeconds = orig.max
	}
}

func TestClampTTL(t *testing.T) {
	// Sub-group 1: fallback path — all config fields unset (zero).
	t.Run("fallback (config unset)", func(t *testing.T) {
		defer resetTTLConfig(t)()

		cases := []struct {
			name string
			in   int
			want int
		}{
			{"zero → default 7200", 0, 7200},
			{"negative → default 7200", -1, 7200},
			{"below min → clamped to 300", 100, 300},
			{"above max → clamped to 86400", 999999, 86400},
			{"exact min", 300, 300},
			{"exact max", 86400, 86400},
			{"in range", 1800, 1800},
		}
		for _, tc := range cases {
			tc := tc
			t.Run(tc.name, func(t *testing.T) {
				if got := clampTTL(tc.in); got != tc.want {
					t.Errorf("clampTTL(%d) = %d, want %d", tc.in, got, tc.want)
				}
			})
		}
	})

	// Sub-group 2: config-driven path — all three fields are set.
	t.Run("config-driven", func(t *testing.T) {
		defer resetTTLConfig(t)()
		tools.Cfg.PlaybackTokenDefaultTTLSeconds = 3600 // 1 hour default
		tools.Cfg.PlaybackTokenMinTTLSeconds = 600      // 10 minutes min
		tools.Cfg.PlaybackTokenMaxTTLSeconds = 43200    // 12 hours max

		cases := []struct {
			name string
			in   int
			want int
		}{
			{"zero → config default 3600", 0, 3600},
			{"negative → config default 3600", -5, 3600},
			{"below config min → clamped to 600", 100, 600},
			{"above config max → clamped to 43200", 99999, 43200},
			{"exact config min", 600, 600},
			{"exact config max", 43200, 43200},
			{"within config range", 7200, 7200},
		}
		for _, tc := range cases {
			tc := tc
			t.Run(tc.name, func(t *testing.T) {
				if got := clampTTL(tc.in); got != tc.want {
					t.Errorf("clampTTL(%d) = %d, want %d", tc.in, got, tc.want)
				}
			})
		}
	})
}

// ---------------------------------------------------------------------------
// Helpers shared by EP1 tests
// ---------------------------------------------------------------------------

// setupServiceAccount wires the token + users mocks so that getServiceAccount
// succeeds. Returns the mock token and users daoWrapper partial for reuse.
func setupServiceTokenAndUser(ctrl *gomock.Controller, rawToken string, userID uint) (*mock_dao.MockTokenDao, *mock_dao.MockUsersDao, model.Token, model.User) {
	tokenMock := mock_dao.NewMockTokenDao(ctrl)
	usersMock := mock_dao.NewMockUsersDao(ctrl)

	tok := model.Token{UserID: userID, Scope: model.TokenScopeService}
	svcUser := model.User{Role: model.ServiceType}
	svcUser.ID = userID

	tokenMock.EXPECT().GetToken(rawToken).Return(tok, nil)
	tokenMock.EXPECT().TokenUsed(tok).Return(nil)
	usersMock.EXPECT().GetUserByID(gomock.Any(), userID).Return(svcUser, nil)

	return tokenMock, usersMock, tok, svcUser
}

// ---------------------------------------------------------------------------
// EP1 ListAdministeredCourses
// ---------------------------------------------------------------------------

func TestListAdministeredCourses_MapsCoursesCorrectly(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	tokenMock, usersMock, _, _ := setupServiceTokenAndUser(ctrl, "svctoken", 7)

	target := model.User{LrzID: "ab12cde"}
	target.ID = 42
	usersMock.EXPECT().GetUserByLrzID("ab12cde").Return(target, nil)

	c1 := model.Course{Name: "Linear Algebra", Slug: "linalg", Year: 2026, TeachingTerm: "W", VODEnabled: true, Visibility: "public"}
	c1.ID = 1
	c2 := model.Course{Name: "Analysis 2", Slug: "ana2", Year: 2026, TeachingTerm: "W", VODEnabled: false, Visibility: "loggedin"}
	c2.ID = 2

	coursesMock := mock_dao.NewMockCoursesDao(ctrl)
	coursesMock.EXPECT().
		GetDirectlyAdministeredCoursesByUserId(gomock.Any(), uint(42), "W", 2026).
		Return([]model.Course{c1, c2}, nil)

	api := buildAPI(dao.DaoWrapper{TokenDao: tokenMock, UsersDao: usersMock, CoursesDao: coursesMock})
	ctx := incomingCtx("authorization", "Bearer svctoken")

	resp, err := api.ListAdministeredCourses(ctx, &protobuf.ListAdministeredCoursesRequest{
		LrzId: "ab12cde",
		Year:  2026,
		Term:  "W",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.Courses) != 2 {
		t.Fatalf("expected 2 courses, got %d", len(resp.Courses))
	}

	// Verify first course mapping.
	got := resp.Courses[0]
	if got.Id != 1 {
		t.Errorf("courses[0].Id = %d, want 1", got.Id)
	}
	if got.Name != "Linear Algebra" {
		t.Errorf("courses[0].Name = %q, want %q", got.Name, "Linear Algebra")
	}
	if got.Slug != "linalg" {
		t.Errorf("courses[0].Slug = %q, want %q", got.Slug, "linalg")
	}
	if got.Year != 2026 {
		t.Errorf("courses[0].Year = %d, want 2026", got.Year)
	}
	if got.TeachingTerm != "W" {
		t.Errorf("courses[0].TeachingTerm = %q, want %q", got.TeachingTerm, "W")
	}
	if !got.VodEnabled {
		t.Errorf("courses[0].VodEnabled = false, want true")
	}
	if got.Visibility != "public" {
		t.Errorf("courses[0].Visibility = %q, want %q", got.Visibility, "public")
	}

	// Spot-check second course.
	got2 := resp.Courses[1]
	if got2.Id != 2 {
		t.Errorf("courses[1].Id = %d, want 2", got2.Id)
	}
	if got2.VodEnabled {
		t.Errorf("courses[1].VodEnabled = true, want false")
	}
}

func TestListAdministeredCourses_MissingAuthHeader_Unauthenticated(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	api := buildAPI(dao.DaoWrapper{
		TokenDao: mock_dao.NewMockTokenDao(ctrl),
		UsersDao: mock_dao.NewMockUsersDao(ctrl),
	})
	// context with no metadata at all
	_, err := api.ListAdministeredCourses(incomingCtx(), &protobuf.ListAdministeredCoursesRequest{LrzId: "ab12cde"})
	if err == nil {
		t.Fatal("expected error for missing auth header, got nil")
	}
	assertGRPCStatus(t, err, http.StatusUnauthorized)
}

func TestListAdministeredCourses_InvalidToken_Unauthenticated(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	tokenMock := mock_dao.NewMockTokenDao(ctrl)
	tokenMock.EXPECT().GetToken("badtoken").Return(model.Token{}, errors.New("not found"))

	api := buildAPI(dao.DaoWrapper{
		TokenDao: tokenMock,
		UsersDao: mock_dao.NewMockUsersDao(ctrl),
	})
	ctx := incomingCtx("authorization", "Bearer badtoken")

	_, err := api.ListAdministeredCourses(ctx, &protobuf.ListAdministeredCoursesRequest{LrzId: "ab12cde"})
	if err == nil {
		t.Fatal("expected Unauthenticated error, got nil")
	}
	assertGRPCStatus(t, err, http.StatusUnauthorized)
}

func TestListAdministeredCourses_WrongScope_PermissionDenied(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	tokenMock := mock_dao.NewMockTokenDao(ctrl)
	tok := model.Token{UserID: 7, Scope: model.TokenScopeAdmin} // not service scope
	tokenMock.EXPECT().GetToken("admintoken").Return(tok, nil)

	api := buildAPI(dao.DaoWrapper{
		TokenDao: tokenMock,
		UsersDao: mock_dao.NewMockUsersDao(ctrl),
	})
	ctx := incomingCtx("authorization", "Bearer admintoken")

	_, err := api.ListAdministeredCourses(ctx, &protobuf.ListAdministeredCoursesRequest{LrzId: "ab12cde"})
	if err == nil {
		t.Fatal("expected PermissionDenied error, got nil")
	}
	assertGRPCStatus(t, err, http.StatusForbidden)
}

func TestListAdministeredCourses_UnknownLrzId_NotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	tokenMock, usersMock, _, _ := setupServiceTokenAndUser(ctrl, "svctoken", 7)
	// Use gorm.ErrRecordNotFound so the handler maps it to 404 (not 500).
	usersMock.EXPECT().GetUserByLrzID("unknown99").Return(model.User{}, gorm.ErrRecordNotFound)

	api := buildAPI(dao.DaoWrapper{
		TokenDao:   tokenMock,
		UsersDao:   usersMock,
		CoursesDao: mock_dao.NewMockCoursesDao(ctrl),
	})
	ctx := incomingCtx("authorization", "Bearer svctoken")

	_, err := api.ListAdministeredCourses(ctx, &protobuf.ListAdministeredCoursesRequest{LrzId: "unknown99"})
	if err == nil {
		t.Fatal("expected NotFound error, got nil")
	}
	assertGRPCStatus(t, err, http.StatusNotFound)
}

func TestListAdministeredCourses_LrzIdBackendError_InternalError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	tokenMock, usersMock, _, _ := setupServiceTokenAndUser(ctrl, "svctoken", 7)
	// A non-record-not-found error (e.g. DB connection failure) must map to 500.
	usersMock.EXPECT().GetUserByLrzID("ab12cde").Return(model.User{}, errors.New("connection refused"))

	api := buildAPI(dao.DaoWrapper{
		TokenDao:   tokenMock,
		UsersDao:   usersMock,
		CoursesDao: mock_dao.NewMockCoursesDao(ctrl),
	})
	ctx := incomingCtx("authorization", "Bearer svctoken")

	_, err := api.ListAdministeredCourses(ctx, &protobuf.ListAdministeredCoursesRequest{LrzId: "ab12cde"})
	if err == nil {
		t.Fatal("expected InternalError, got nil")
	}
	assertGRPCStatus(t, err, http.StatusInternalServerError)
}

func TestListAdministeredCourses_EmptyResult(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	tokenMock, usersMock, _, _ := setupServiceTokenAndUser(ctrl, "svctoken", 7)

	target := model.User{LrzID: "ab12cde"}
	target.ID = 42
	usersMock.EXPECT().GetUserByLrzID("ab12cde").Return(target, nil)

	coursesMock := mock_dao.NewMockCoursesDao(ctrl)
	coursesMock.EXPECT().
		GetDirectlyAdministeredCoursesByUserId(gomock.Any(), uint(42), "S", 2026).
		Return([]model.Course{}, nil)

	api := buildAPI(dao.DaoWrapper{TokenDao: tokenMock, UsersDao: usersMock, CoursesDao: coursesMock})
	ctx := incomingCtx("authorization", "Bearer svctoken")

	resp, err := api.ListAdministeredCourses(ctx, &protobuf.ListAdministeredCoursesRequest{
		LrzId: "ab12cde",
		Year:  2026,
		Term:  "S",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.Courses) != 0 {
		t.Fatalf("expected 0 courses, got %d", len(resp.Courses))
	}
}

// ---------------------------------------------------------------------------
// EP8 ListCourseStreams
// ---------------------------------------------------------------------------

// setupServiceCourseAdmin wires mock DAOs so that requireServiceCourseAdmin
// succeeds for the given courseID. The returned course has svcUser as an
// explicit admin so IsAdminOfCourse returns true.
func setupServiceCourseAdmin(ctrl *gomock.Controller, rawToken string, userID, courseID uint, course model.Course) (
	*mock_dao.MockTokenDao, *mock_dao.MockUsersDao, *mock_dao.MockCoursesDao,
) {
	tokenMock := mock_dao.NewMockTokenDao(ctrl)
	usersMock := mock_dao.NewMockUsersDao(ctrl)
	coursesMock := mock_dao.NewMockCoursesDao(ctrl)

	tok := model.Token{UserID: userID, Scope: model.TokenScopeService}
	// svcUser is listed in course.Admins via AdministeredCourses so
	// IsAdminOfCourse reports true.
	svcUser := model.User{Role: model.ServiceType, AdministeredCourses: []model.Course{course}}
	svcUser.ID = userID

	tokenMock.EXPECT().GetToken(rawToken).Return(tok, nil)
	tokenMock.EXPECT().TokenUsed(tok).Return(nil)
	usersMock.EXPECT().GetUserByID(gomock.Any(), userID).Return(svcUser, nil)
	coursesMock.EXPECT().GetCourseById(gomock.Any(), courseID).Return(course, nil)

	return tokenMock, usersMock, coursesMock
}

func TestListCourseStreams_MapsStreamsCorrectly(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	course := model.Course{Name: "Analysis II"}
	course.ID = 10

	s1 := model.Stream{Name: "Lecture 01", Private: false}
	s1.ID = 101
	s2 := model.Stream{Name: "", Private: true} // empty name → GetName() returns "Lecture: ..."
	s2.ID = 102

	course.Streams = []model.Stream{s1, s2}

	tokenMock, usersMock, coursesMock := setupServiceCourseAdmin(ctrl, "svctoken", 7, 10, course)
	api := buildAPI(dao.DaoWrapper{TokenDao: tokenMock, UsersDao: usersMock, CoursesDao: coursesMock})
	ctx := incomingCtx("authorization", "Bearer svctoken")

	resp, err := api.ListCourseStreams(ctx, &protobuf.ListCourseStreamsRequest{CourseId: 10})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.Streams) != 2 {
		t.Fatalf("expected 2 streams, got %d", len(resp.Streams))
	}

	got1 := resp.Streams[0]
	if got1.StreamId != uint32(s1.ID) {
		t.Errorf("streams[0].StreamId = %d, want %d", got1.StreamId, uint32(s1.ID))
	}
	if got1.Name != s1.GetName() {
		t.Errorf("streams[0].Name = %q, want %q", got1.Name, s1.GetName())
	}
	if got1.Private {
		t.Errorf("streams[0].Private = true, want false")
	}
	if got1.Start == nil {
		t.Error("streams[0].Start is nil")
	}
	if got1.End == nil {
		t.Error("streams[0].End is nil")
	}

	got2 := resp.Streams[1]
	if got2.StreamId != uint32(s2.ID) {
		t.Errorf("streams[1].StreamId = %d, want %d", got2.StreamId, uint32(s2.ID))
	}
	if got2.Name != s2.GetName() {
		t.Errorf("streams[1].Name = %q, want %q", got2.Name, s2.GetName())
	}
	if !got2.Private {
		t.Errorf("streams[1].Private = false, want true")
	}
}

func TestListCourseStreams_EmptyStreams(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	course := model.Course{Name: "Empty Course"}
	course.ID = 20
	course.Streams = []model.Stream{}

	tokenMock, usersMock, coursesMock := setupServiceCourseAdmin(ctrl, "svctoken", 7, 20, course)
	api := buildAPI(dao.DaoWrapper{TokenDao: tokenMock, UsersDao: usersMock, CoursesDao: coursesMock})
	ctx := incomingCtx("authorization", "Bearer svctoken")

	resp, err := api.ListCourseStreams(ctx, &protobuf.ListCourseStreamsRequest{CourseId: 20})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.Streams) != 0 {
		t.Fatalf("expected 0 streams, got %d", len(resp.Streams))
	}
}

func TestListCourseStreams_NotAdmin_PermissionDenied(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	tokenMock := mock_dao.NewMockTokenDao(ctrl)
	usersMock := mock_dao.NewMockUsersDao(ctrl)
	coursesMock := mock_dao.NewMockCoursesDao(ctrl)

	const rawToken = "svctoken"
	const userID = uint(7)
	const courseID = uint(30)

	tok := model.Token{UserID: userID, Scope: model.TokenScopeService}
	// svcUser has NO administered courses → IsAdminOfCourse returns false.
	svcUser := model.User{Role: model.ServiceType}
	svcUser.ID = userID

	course := model.Course{Name: "Restricted Course"}
	course.ID = courseID

	tokenMock.EXPECT().GetToken(rawToken).Return(tok, nil)
	tokenMock.EXPECT().TokenUsed(tok).Return(nil)
	usersMock.EXPECT().GetUserByID(gomock.Any(), userID).Return(svcUser, nil)
	coursesMock.EXPECT().GetCourseById(gomock.Any(), courseID).Return(course, nil)

	api := buildAPI(dao.DaoWrapper{TokenDao: tokenMock, UsersDao: usersMock, CoursesDao: coursesMock})
	ctx := incomingCtx("authorization", "Bearer "+rawToken)

	_, err := api.ListCourseStreams(ctx, &protobuf.ListCourseStreamsRequest{CourseId: uint32(courseID)})
	if err == nil {
		t.Fatal("expected PermissionDenied error, got nil")
	}
	assertGRPCStatus(t, err, http.StatusForbidden)
}

func TestListCourseStreams_UnknownCourse_NotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	tokenMock := mock_dao.NewMockTokenDao(ctrl)
	usersMock := mock_dao.NewMockUsersDao(ctrl)
	coursesMock := mock_dao.NewMockCoursesDao(ctrl)

	const rawToken = "svctoken"
	const userID = uint(7)

	tok := model.Token{UserID: userID, Scope: model.TokenScopeService}
	svcUser := model.User{Role: model.ServiceType}
	svcUser.ID = userID

	tokenMock.EXPECT().GetToken(rawToken).Return(tok, nil)
	tokenMock.EXPECT().TokenUsed(tok).Return(nil)
	usersMock.EXPECT().GetUserByID(gomock.Any(), userID).Return(svcUser, nil)
	// Real GetCourseById uses Find (not First): missing course → nil error + zero-value course.
	coursesMock.EXPECT().GetCourseById(gomock.Any(), uint(999)).Return(model.Course{}, nil)

	api := buildAPI(dao.DaoWrapper{TokenDao: tokenMock, UsersDao: usersMock, CoursesDao: coursesMock})
	ctx := incomingCtx("authorization", "Bearer "+rawToken)

	_, err := api.ListCourseStreams(ctx, &protobuf.ListCourseStreamsRequest{CourseId: 999})
	if err == nil {
		t.Fatal("expected NotFound error, got nil")
	}
	assertGRPCStatus(t, err, http.StatusNotFound)
}

func TestListCourseStreams_CourseBackendError_InternalError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	tokenMock := mock_dao.NewMockTokenDao(ctrl)
	usersMock := mock_dao.NewMockUsersDao(ctrl)
	coursesMock := mock_dao.NewMockCoursesDao(ctrl)

	const rawToken = "svctoken"
	const userID = uint(7)

	tok := model.Token{UserID: userID, Scope: model.TokenScopeService}
	svcUser := model.User{Role: model.ServiceType}
	svcUser.ID = userID

	tokenMock.EXPECT().GetToken(rawToken).Return(tok, nil)
	tokenMock.EXPECT().TokenUsed(tok).Return(nil)
	usersMock.EXPECT().GetUserByID(gomock.Any(), userID).Return(svcUser, nil)
	// A non-nil error from GetCourseById is a genuine backend/DB failure → 500
	// (not 404, which is reserved for the zero-course case).
	coursesMock.EXPECT().GetCourseById(gomock.Any(), uint(42)).Return(model.Course{}, errors.New("db connection lost"))

	api := buildAPI(dao.DaoWrapper{TokenDao: tokenMock, UsersDao: usersMock, CoursesDao: coursesMock})
	ctx := incomingCtx("authorization", "Bearer "+rawToken)

	_, err := api.ListCourseStreams(ctx, &protobuf.ListCourseStreamsRequest{CourseId: 42})
	if err == nil {
		t.Fatal("expected InternalError, got nil")
	}
	assertGRPCStatus(t, err, http.StatusInternalServerError)
}

func TestListCourseStreams_MissingAuthHeader_Unauthenticated(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	api := buildAPI(dao.DaoWrapper{
		TokenDao:   mock_dao.NewMockTokenDao(ctrl),
		UsersDao:   mock_dao.NewMockUsersDao(ctrl),
		CoursesDao: mock_dao.NewMockCoursesDao(ctrl),
	})

	// No metadata in context.
	_, err := api.ListCourseStreams(incomingCtx(), &protobuf.ListCourseStreamsRequest{CourseId: 10})
	if err == nil {
		t.Fatal("expected Unauthenticated error, got nil")
	}
	assertGRPCStatus(t, err, http.StatusUnauthorized)
}

func TestListCourseStreams_InvalidToken_Unauthenticated(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	tokenMock := mock_dao.NewMockTokenDao(ctrl)
	tokenMock.EXPECT().GetToken("badtoken").Return(model.Token{}, errors.New("not found"))

	api := buildAPI(dao.DaoWrapper{
		TokenDao:   tokenMock,
		UsersDao:   mock_dao.NewMockUsersDao(ctrl),
		CoursesDao: mock_dao.NewMockCoursesDao(ctrl),
	})
	ctx := incomingCtx("authorization", "Bearer badtoken")

	_, err := api.ListCourseStreams(ctx, &protobuf.ListCourseStreamsRequest{CourseId: 10})
	if err == nil {
		t.Fatal("expected Unauthenticated error, got nil")
	}
	assertGRPCStatus(t, err, http.StatusUnauthorized)
}

// ---------------------------------------------------------------------------
// EP2 GetPlaybackToken
// ---------------------------------------------------------------------------

// setupPlaybackTokenMocks wires the common DAOs for a GetPlaybackToken test:
// service account is an admin of the given course, OBO user is set up, and
// the stream is returned by GetStreamByID.
func setupPlaybackTokenMocks(
	ctrl *gomock.Controller,
	rawToken string,
	userID, courseID uint,
	course model.Course,
	oboLrzID string,
	oboUser model.User,
	stream model.Stream,
) (*mock_dao.MockTokenDao, *mock_dao.MockUsersDao, *mock_dao.MockCoursesDao, *mock_dao.MockStreamsDao) {
	tokenMock, usersMock, coursesMock := setupServiceCourseAdmin(ctrl, rawToken, userID, courseID, course)
	usersMock.EXPECT().GetUserByLrzID(oboLrzID).Return(oboUser, nil)
	streamsMock := mock_dao.NewMockStreamsDao(ctrl)
	streamsMock.EXPECT().GetStreamByID(gomock.Any(), fmt.Sprintf("%d", stream.ID)).Return(stream, nil)
	return tokenMock, usersMock, coursesMock, streamsMock
}

func TestGetPlaybackToken_HappyPath(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	const (
		rawToken = "svctoken"
		userID   = uint(7)
		courseID = uint(100)
		streamID = uint(200)
	)

	course := model.Course{Visibility: "public", DownloadsEnabled: true}
	course.ID = courseID

	// OBO user is eligible to watch the course (public → always eligible).
	oboUser := model.User{LrzID: "ob01tum", Role: model.StudentType}
	oboUser.ID = 55

	stream := model.Stream{
		Model:           gorm.Model{ID: streamID},
		CourseID:        courseID,
		PlaylistUrl:     "https://gocast.example.com/comb.m3u8",
		PlaylistUrlPRES: "https://gocast.example.com/pres.m3u8",
		PlaylistUrlCAM:  "https://gocast.example.com/cam.m3u8",
	}

	tokenMock, usersMock, coursesMock, streamsMock := setupPlaybackTokenMocks(
		ctrl, rawToken, userID, courseID, course,
		"ob01tum", oboUser, stream,
	)

	api := buildAPI(dao.DaoWrapper{
		TokenDao:   tokenMock,
		UsersDao:   usersMock,
		CoursesDao: coursesMock,
		StreamsDao: streamsMock,
	})
	ctx := incomingCtx("authorization", "Bearer "+rawToken, "x-on-behalf-of", "ob01tum")

	before := time.Now()
	resp, err := api.GetPlaybackToken(ctx, &protobuf.GetPlaybackTokenRequest{
		CourseId:   uint32(courseID),
		StreamId:   uint32(streamID),
		TtlSeconds: 3600,
	})
	after := time.Now()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// ExpiresIn must equal the requested (clamped) TTL.
	if resp.ExpiresIn != 3600 {
		t.Errorf("ExpiresIn = %d, want 3600", resp.ExpiresIn)
	}

	// All three variants must be signed URLs.
	for variant, url := range map[string]string{
		"comb": resp.PlaylistUrl,
		"pres": resp.PlaylistUrlPres,
		"cam":  resp.PlaylistUrlCam,
	} {
		if url == "" {
			t.Errorf("%s: expected non-empty signed URL", variant)
			continue
		}
		claims := parsePlaybackClaims(t, url)
		if claims.UserID != 55 {
			t.Errorf("%s: UserID=%d, want 55", variant, claims.UserID)
		}
		if claims.StreamID != fmt.Sprintf("%d", streamID) {
			t.Errorf("%s: StreamID=%q, want %q", variant, claims.StreamID, fmt.Sprintf("%d", streamID))
		}
		if claims.CourseID != fmt.Sprintf("%d", courseID) {
			t.Errorf("%s: CourseID=%q, want %q", variant, claims.CourseID, fmt.Sprintf("%d", courseID))
		}
		if !claims.Download {
			t.Errorf("%s: Download=false, want true (course has DownloadsEnabled)", variant)
		}
		// exp ≈ now + 3600s (±2s for JWT second-level truncation)
		exp := claims.ExpiresAt.Time
		if exp.Before(before.Add(3600*time.Second-2*time.Second)) || exp.After(after.Add(3600*time.Second+2*time.Second)) {
			t.Errorf("%s: exp %v not in expected window", variant, exp)
		}
	}
}

func TestGetPlaybackToken_ServiceAccountNotAdmin_PermissionDenied(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	const (
		rawToken = "svctoken"
		userID   = uint(7)
		courseID = uint(100)
	)

	tok := model.Token{UserID: userID, Scope: model.TokenScopeService}
	// Service user has NO administered courses → not an admin.
	svcUser := model.User{Role: model.ServiceType}
	svcUser.ID = userID
	course := model.Course{}
	course.ID = courseID

	tokenMock := mock_dao.NewMockTokenDao(ctrl)
	usersMock := mock_dao.NewMockUsersDao(ctrl)
	coursesMock := mock_dao.NewMockCoursesDao(ctrl)

	tokenMock.EXPECT().GetToken(rawToken).Return(tok, nil)
	tokenMock.EXPECT().TokenUsed(tok).Return(nil)
	usersMock.EXPECT().GetUserByID(gomock.Any(), userID).Return(svcUser, nil)
	coursesMock.EXPECT().GetCourseById(gomock.Any(), courseID).Return(course, nil)

	api := buildAPI(dao.DaoWrapper{
		TokenDao:   tokenMock,
		UsersDao:   usersMock,
		CoursesDao: coursesMock,
		StreamsDao: mock_dao.NewMockStreamsDao(ctrl),
	})
	ctx := incomingCtx("authorization", "Bearer "+rawToken, "x-on-behalf-of", "ob01tum")

	_, err := api.GetPlaybackToken(ctx, &protobuf.GetPlaybackTokenRequest{
		CourseId: uint32(courseID),
		StreamId: 200,
	})
	if err == nil {
		t.Fatal("expected PermissionDenied, got nil")
	}
	assertGRPCStatus(t, err, http.StatusForbidden)
}

func TestGetPlaybackToken_StreamNotInCourse_NotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	const (
		rawToken = "svctoken"
		userID   = uint(7)
		courseID = uint(100)
		streamID = uint(200)
	)

	course := model.Course{Visibility: "public"}
	course.ID = courseID

	oboUser := model.User{LrzID: "ob01tum", Role: model.StudentType}
	oboUser.ID = 55

	// Stream belongs to a DIFFERENT course.
	stream := model.Stream{
		Model:       gorm.Model{ID: streamID},
		CourseID:    999, // not courseID
		PlaylistUrl: "https://gocast.example.com/comb.m3u8",
	}

	tokenMock, usersMock, coursesMock, streamsMock := setupPlaybackTokenMocks(
		ctrl, rawToken, userID, courseID, course,
		"ob01tum", oboUser, stream,
	)

	api := buildAPI(dao.DaoWrapper{
		TokenDao:   tokenMock,
		UsersDao:   usersMock,
		CoursesDao: coursesMock,
		StreamsDao: streamsMock,
	})
	ctx := incomingCtx("authorization", "Bearer "+rawToken, "x-on-behalf-of", "ob01tum")

	_, err := api.GetPlaybackToken(ctx, &protobuf.GetPlaybackTokenRequest{
		CourseId: uint32(courseID),
		StreamId: uint32(streamID),
	})
	if err == nil {
		t.Fatal("expected NotFound, got nil")
	}
	// A cross-course stream must return NotFound (not BadRequest) so the handler
	// does not act as a stream-ID oracle for service tokens bound to one course.
	assertGRPCStatus(t, err, http.StatusNotFound)
}

func TestGetPlaybackToken_IneligibleOBO_PermissionDenied(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	const (
		rawToken = "svctoken"
		userID   = uint(7)
		courseID = uint(100)
		streamID = uint(200)
	)

	// Enrolled course: requires explicit enrollment.
	course := model.Course{Visibility: "enrolled"}
	course.ID = courseID

	// OBO user has no courses → NOT eligible.
	oboUser := model.User{LrzID: "ob01tum", Role: model.StudentType, Courses: nil}
	oboUser.ID = 55

	stream := model.Stream{
		Model:       gorm.Model{ID: streamID},
		CourseID:    courseID,
		PlaylistUrl: "https://gocast.example.com/comb.m3u8",
	}

	tokenMock, usersMock, coursesMock, streamsMock := setupPlaybackTokenMocks(
		ctrl, rawToken, userID, courseID, course,
		"ob01tum", oboUser, stream,
	)

	api := buildAPI(dao.DaoWrapper{
		TokenDao:   tokenMock,
		UsersDao:   usersMock,
		CoursesDao: coursesMock,
		StreamsDao: streamsMock,
	})
	ctx := incomingCtx("authorization", "Bearer "+rawToken, "x-on-behalf-of", "ob01tum")

	_, err := api.GetPlaybackToken(ctx, &protobuf.GetPlaybackTokenRequest{
		CourseId: uint32(courseID),
		StreamId: uint32(streamID),
	})
	if err == nil {
		t.Fatal("expected PermissionDenied, got nil")
	}
	assertGRPCStatus(t, err, http.StatusForbidden)
}

func TestGetPlaybackToken_MissingOBOHeader_BadRequest(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	const (
		rawToken = "svctoken"
		userID   = uint(7)
		courseID = uint(100)
	)

	course := model.Course{Visibility: "public"}
	course.ID = courseID
	tokenMock, usersMock, coursesMock := setupServiceCourseAdmin(ctrl, rawToken, userID, courseID, course)

	api := buildAPI(dao.DaoWrapper{
		TokenDao:   tokenMock,
		UsersDao:   usersMock,
		CoursesDao: coursesMock,
		StreamsDao: mock_dao.NewMockStreamsDao(ctrl),
	})
	// No x-on-behalf-of header.
	ctx := incomingCtx("authorization", "Bearer "+rawToken)

	_, err := api.GetPlaybackToken(ctx, &protobuf.GetPlaybackTokenRequest{
		CourseId: uint32(courseID),
		StreamId: 200,
	})
	if err == nil {
		t.Fatal("expected BadRequest, got nil")
	}
	assertGRPCStatus(t, err, http.StatusBadRequest)
}

func TestGetPlaybackToken_TTLClamping(t *testing.T) {
	// Each subtest creates its own gomock controller (ctrl2); no outer
	// controller is needed here.
	const (
		rawToken = "svctoken"
		userID   = uint(7)
		courseID = uint(100)
		streamID = uint(200)
	)

	course := model.Course{Visibility: "public"}
	course.ID = courseID
	oboUser := model.User{LrzID: "ob01tum", Role: model.StudentType}
	oboUser.ID = 55
	stream := model.Stream{
		Model:       gorm.Model{ID: streamID},
		CourseID:    courseID,
		PlaylistUrl: "https://gocast.example.com/stream.m3u8",
	}

	cases := []struct {
		name        string
		ttlInput    int32
		wantExpires int32
	}{
		{"zero → default 7200", 0, 7200},
		{"negative → default 7200", -1, 7200},
		{"below min → clamped to 300", 100, 300},
		{"above max → clamped to 86400", 999999, 86400},
		{"exact min", 300, 300},
		{"exact max", 86400, 86400},
		{"within range", 1800, 1800},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			ctrl2 := gomock.NewController(t)
			defer ctrl2.Finish()

			tokenMock, usersMock, coursesMock, streamsMock := setupPlaybackTokenMocks(
				ctrl2, rawToken, userID, courseID, course,
				"ob01tum", oboUser, stream,
			)
			api := buildAPI(dao.DaoWrapper{
				TokenDao:   tokenMock,
				UsersDao:   usersMock,
				CoursesDao: coursesMock,
				StreamsDao: streamsMock,
			})
			ctx := incomingCtx("authorization", "Bearer "+rawToken, "x-on-behalf-of", "ob01tum")

			resp, err := api.GetPlaybackToken(ctx, &protobuf.GetPlaybackTokenRequest{
				CourseId:   uint32(courseID),
				StreamId:   uint32(streamID),
				TtlSeconds: tc.ttlInput,
			})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if resp.ExpiresIn != tc.wantExpires {
				t.Errorf("ExpiresIn = %d, want %d", resp.ExpiresIn, tc.wantExpires)
			}
		})
	}
}

func TestGetPlaybackToken_PrivateStreamIneligible_PermissionDenied(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	const (
		rawToken = "svctoken"
		userID   = uint(7)
		courseID = uint(100)
		streamID = uint(200)
	)

	course := model.Course{Visibility: "public"}
	course.ID = courseID

	// OBO user is eligible for the course but NOT an admin → denied for private streams.
	oboUser := model.User{LrzID: "ob01tum", Role: model.StudentType}
	oboUser.ID = 55

	stream := model.Stream{
		Model:       gorm.Model{ID: streamID},
		CourseID:    courseID,
		Private:     true, // private!
		PlaylistUrl: "https://gocast.example.com/comb.m3u8",
	}

	tokenMock, usersMock, coursesMock, streamsMock := setupPlaybackTokenMocks(
		ctrl, rawToken, userID, courseID, course,
		"ob01tum", oboUser, stream,
	)

	api := buildAPI(dao.DaoWrapper{
		TokenDao:   tokenMock,
		UsersDao:   usersMock,
		CoursesDao: coursesMock,
		StreamsDao: streamsMock,
	})
	ctx := incomingCtx("authorization", "Bearer "+rawToken, "x-on-behalf-of", "ob01tum")

	_, err := api.GetPlaybackToken(ctx, &protobuf.GetPlaybackTokenRequest{
		CourseId: uint32(courseID),
		StreamId: uint32(streamID),
	})
	if err == nil {
		t.Fatal("expected PermissionDenied for non-admin accessing private stream, got nil")
	}
	assertGRPCStatus(t, err, http.StatusForbidden)
}

func TestGetPlaybackToken_StreamNotFound_NotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	const (
		rawToken = "svctoken"
		userID   = uint(7)
		courseID = uint(100)
		streamID = uint(200)
	)

	course := model.Course{Visibility: "public"}
	course.ID = courseID
	oboUser := model.User{LrzID: "ob01tum", Role: model.StudentType}
	oboUser.ID = 55

	tokenMock, usersMock, coursesMock := setupServiceCourseAdmin(ctrl, rawToken, userID, courseID, course)
	usersMock.EXPECT().GetUserByLrzID("ob01tum").Return(oboUser, nil)
	streamsMock := mock_dao.NewMockStreamsDao(ctrl)
	// Use gorm.ErrRecordNotFound so the handler maps it to 404 (not 500).
	streamsMock.EXPECT().GetStreamByID(gomock.Any(), fmt.Sprintf("%d", streamID)).Return(model.Stream{}, gorm.ErrRecordNotFound)

	api := buildAPI(dao.DaoWrapper{
		TokenDao:   tokenMock,
		UsersDao:   usersMock,
		CoursesDao: coursesMock,
		StreamsDao: streamsMock,
	})
	ctx := incomingCtx("authorization", "Bearer "+rawToken, "x-on-behalf-of", "ob01tum")

	_, err := api.GetPlaybackToken(ctx, &protobuf.GetPlaybackTokenRequest{
		CourseId: uint32(courseID),
		StreamId: uint32(streamID),
	})
	if err == nil {
		t.Fatal("expected NotFound, got nil")
	}
	assertGRPCStatus(t, err, http.StatusNotFound)
}

func TestGetPlaybackToken_StreamBackendError_InternalError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	const (
		rawToken = "svctoken"
		userID   = uint(7)
		courseID = uint(100)
		streamID = uint(200)
	)

	course := model.Course{Visibility: "public"}
	course.ID = courseID
	oboUser := model.User{LrzID: "ob01tum", Role: model.StudentType}
	oboUser.ID = 55

	tokenMock, usersMock, coursesMock := setupServiceCourseAdmin(ctrl, rawToken, userID, courseID, course)
	usersMock.EXPECT().GetUserByLrzID("ob01tum").Return(oboUser, nil)
	streamsMock := mock_dao.NewMockStreamsDao(ctrl)
	// A non-record-not-found error must map to 500.
	streamsMock.EXPECT().GetStreamByID(gomock.Any(), fmt.Sprintf("%d", streamID)).Return(model.Stream{}, errors.New("db timeout"))

	api := buildAPI(dao.DaoWrapper{
		TokenDao:   tokenMock,
		UsersDao:   usersMock,
		CoursesDao: coursesMock,
		StreamsDao: streamsMock,
	})
	ctx := incomingCtx("authorization", "Bearer "+rawToken, "x-on-behalf-of", "ob01tum")

	_, err := api.GetPlaybackToken(ctx, &protobuf.GetPlaybackTokenRequest{
		CourseId: uint32(courseID),
		StreamId: uint32(streamID),
	})
	if err == nil {
		t.Fatal("expected InternalError for backend DB error, got nil")
	}
	assertGRPCStatus(t, err, http.StatusInternalServerError)
}

func TestGetPlaybackToken_NoPlayableVariants_NotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	const (
		rawToken = "svctoken"
		userID   = uint(7)
		courseID = uint(100)
		streamID = uint(200)
	)

	course := model.Course{Visibility: "public"}
	course.ID = courseID
	oboUser := model.User{LrzID: "ob01tum", Role: model.StudentType}
	oboUser.ID = 55

	// Stream has no playlist URLs → no playable variants.
	stream := model.Stream{
		Model:    gorm.Model{ID: streamID},
		CourseID: courseID,
	}

	tokenMock, usersMock, coursesMock, streamsMock := setupPlaybackTokenMocks(
		ctrl, rawToken, userID, courseID, course,
		"ob01tum", oboUser, stream,
	)

	api := buildAPI(dao.DaoWrapper{
		TokenDao:   tokenMock,
		UsersDao:   usersMock,
		CoursesDao: coursesMock,
		StreamsDao: streamsMock,
	})
	ctx := incomingCtx("authorization", "Bearer "+rawToken, "x-on-behalf-of", "ob01tum")

	_, err := api.GetPlaybackToken(ctx, &protobuf.GetPlaybackTokenRequest{
		CourseId: uint32(courseID),
		StreamId: uint32(streamID),
	})
	if err == nil {
		t.Fatal("expected NotFound for stream with no variants, got nil")
	}
	assertGRPCStatus(t, err, http.StatusNotFound)
}

// ---------------------------------------------------------------------------
// EP7 GetBindingStatus
// ---------------------------------------------------------------------------

func TestGetBindingStatus_IsAdmin_BoundTrue(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	const (
		rawToken = "svctoken"
		userID   = uint(7)
		courseID = uint(50)
	)

	course := model.Course{Name: "Algorithms"}
	course.ID = courseID

	// Service account has the course in AdministeredCourses → IsAdminOfCourse returns true.
	tokenMock := mock_dao.NewMockTokenDao(ctrl)
	usersMock := mock_dao.NewMockUsersDao(ctrl)
	coursesMock := mock_dao.NewMockCoursesDao(ctrl)

	tok := model.Token{UserID: userID, Scope: model.TokenScopeService}
	svcUser := model.User{Role: model.ServiceType, AdministeredCourses: []model.Course{course}}
	svcUser.ID = userID

	tokenMock.EXPECT().GetToken(rawToken).Return(tok, nil)
	tokenMock.EXPECT().TokenUsed(tok).Return(nil)
	usersMock.EXPECT().GetUserByID(gomock.Any(), userID).Return(svcUser, nil)
	coursesMock.EXPECT().GetCourseById(gomock.Any(), courseID).Return(course, nil)

	api := buildAPI(dao.DaoWrapper{TokenDao: tokenMock, UsersDao: usersMock, CoursesDao: coursesMock})
	ctx := incomingCtx("authorization", "Bearer "+rawToken)

	resp, err := api.GetBindingStatus(ctx, &protobuf.GetBindingStatusRequest{CourseId: uint32(courseID)})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !resp.Bound {
		t.Error("expected Bound=true when service account is admin of course, got false")
	}
}

func TestGetBindingStatus_NotAdmin_BoundFalse(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	const (
		rawToken = "svctoken"
		userID   = uint(7)
		courseID = uint(50)
	)

	course := model.Course{Name: "Algorithms"}
	course.ID = courseID

	// Service account has NO administered courses → IsAdminOfCourse returns false.
	tokenMock := mock_dao.NewMockTokenDao(ctrl)
	usersMock := mock_dao.NewMockUsersDao(ctrl)
	coursesMock := mock_dao.NewMockCoursesDao(ctrl)

	tok := model.Token{UserID: userID, Scope: model.TokenScopeService}
	svcUser := model.User{Role: model.ServiceType} // no AdministeredCourses
	svcUser.ID = userID

	tokenMock.EXPECT().GetToken(rawToken).Return(tok, nil)
	tokenMock.EXPECT().TokenUsed(tok).Return(nil)
	usersMock.EXPECT().GetUserByID(gomock.Any(), userID).Return(svcUser, nil)
	coursesMock.EXPECT().GetCourseById(gomock.Any(), courseID).Return(course, nil)

	api := buildAPI(dao.DaoWrapper{TokenDao: tokenMock, UsersDao: usersMock, CoursesDao: coursesMock})
	ctx := incomingCtx("authorization", "Bearer "+rawToken)

	resp, err := api.GetBindingStatus(ctx, &protobuf.GetBindingStatusRequest{CourseId: uint32(courseID)})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Bound {
		t.Error("expected Bound=false when service account is not admin of course, got true")
	}
}

func TestGetBindingStatus_UnknownCourse_NotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	const (
		rawToken = "svctoken"
		userID   = uint(7)
		courseID = uint(999)
	)

	tokenMock := mock_dao.NewMockTokenDao(ctrl)
	usersMock := mock_dao.NewMockUsersDao(ctrl)
	coursesMock := mock_dao.NewMockCoursesDao(ctrl)

	tok := model.Token{UserID: userID, Scope: model.TokenScopeService}
	svcUser := model.User{Role: model.ServiceType}
	svcUser.ID = userID

	tokenMock.EXPECT().GetToken(rawToken).Return(tok, nil)
	tokenMock.EXPECT().TokenUsed(tok).Return(nil)
	usersMock.EXPECT().GetUserByID(gomock.Any(), userID).Return(svcUser, nil)
	// Real GetCourseById uses Find (not First): a missing course returns nil error
	// and a zero-value course (ID == 0). The handler must detect this and return NotFound.
	coursesMock.EXPECT().GetCourseById(gomock.Any(), courseID).Return(model.Course{}, nil)

	api := buildAPI(dao.DaoWrapper{TokenDao: tokenMock, UsersDao: usersMock, CoursesDao: coursesMock})
	ctx := incomingCtx("authorization", "Bearer "+rawToken)

	_, err := api.GetBindingStatus(ctx, &protobuf.GetBindingStatusRequest{CourseId: uint32(courseID)})
	if err == nil {
		t.Fatal("expected NotFound error for unknown course, got nil")
	}
	assertGRPCStatus(t, err, http.StatusNotFound)
}

func TestGetBindingStatus_CourseBackendError_InternalError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	const (
		rawToken = "svctoken"
		userID   = uint(7)
		courseID = uint(50)
	)

	tokenMock := mock_dao.NewMockTokenDao(ctrl)
	usersMock := mock_dao.NewMockUsersDao(ctrl)
	coursesMock := mock_dao.NewMockCoursesDao(ctrl)

	tok := model.Token{UserID: userID, Scope: model.TokenScopeService}
	svcUser := model.User{Role: model.ServiceType}
	svcUser.ID = userID

	tokenMock.EXPECT().GetToken(rawToken).Return(tok, nil)
	tokenMock.EXPECT().TokenUsed(tok).Return(nil)
	usersMock.EXPECT().GetUserByID(gomock.Any(), userID).Return(svcUser, nil)
	// A non-nil error from GetCourseById is a genuine backend/DB failure → 500,
	// not 404 (which is reserved for the zero-course case).
	coursesMock.EXPECT().GetCourseById(gomock.Any(), courseID).Return(model.Course{}, errors.New("db connection lost"))

	api := buildAPI(dao.DaoWrapper{TokenDao: tokenMock, UsersDao: usersMock, CoursesDao: coursesMock})
	ctx := incomingCtx("authorization", "Bearer "+rawToken)

	_, err := api.GetBindingStatus(ctx, &protobuf.GetBindingStatusRequest{CourseId: uint32(courseID)})
	if err == nil {
		t.Fatal("expected InternalError for backend DB error, got nil")
	}
	assertGRPCStatus(t, err, http.StatusInternalServerError)
}

func TestGetBindingStatus_MissingAuthHeader_Unauthenticated(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	api := buildAPI(dao.DaoWrapper{
		TokenDao:   mock_dao.NewMockTokenDao(ctrl),
		UsersDao:   mock_dao.NewMockUsersDao(ctrl),
		CoursesDao: mock_dao.NewMockCoursesDao(ctrl),
	})

	// No metadata in context → no authorization header.
	_, err := api.GetBindingStatus(incomingCtx(), &protobuf.GetBindingStatusRequest{CourseId: 50})
	if err == nil {
		t.Fatal("expected Unauthenticated error for missing auth header, got nil")
	}
	assertGRPCStatus(t, err, http.StatusUnauthorized)
}

func TestGetBindingStatus_InvalidToken_Unauthenticated(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	tokenMock := mock_dao.NewMockTokenDao(ctrl)
	tokenMock.EXPECT().GetToken("badtoken").Return(model.Token{}, errors.New("not found"))

	api := buildAPI(dao.DaoWrapper{
		TokenDao:   tokenMock,
		UsersDao:   mock_dao.NewMockUsersDao(ctrl),
		CoursesDao: mock_dao.NewMockCoursesDao(ctrl),
	})
	ctx := incomingCtx("authorization", "Bearer badtoken")

	_, err := api.GetBindingStatus(ctx, &protobuf.GetBindingStatusRequest{CourseId: 50})
	if err == nil {
		t.Fatal("expected Unauthenticated error for invalid token, got nil")
	}
	assertGRPCStatus(t, err, http.StatusUnauthorized)
}
