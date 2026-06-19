// Package apiv2 provides API endpoints for the application.
package apiv2

import (
	"errors"
	"net/http"
	"testing"

	"go.uber.org/mock/gomock"

	protobuf "github.com/TUM-Dev/gocast/apiv2/protobuf/server"
	"github.com/TUM-Dev/gocast/dao"
	"github.com/TUM-Dev/gocast/mock_dao"
	"github.com/TUM-Dev/gocast/model"
)

// ---------------------------------------------------------------------------
// Helpers shared by EP1 tests
// ---------------------------------------------------------------------------

// serviceCtx returns an incoming context that carries a valid service bearer token.
func serviceCtx(token string) interface{ Done() <-chan struct{} } {
	return incomingCtx("authorization", "Bearer "+token)
}

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
	usersMock.EXPECT().GetUserByLrzID("unknown99").Return(model.User{}, errors.New("not found"))

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
	coursesMock.EXPECT().GetCourseById(gomock.Any(), uint(999)).Return(model.Course{}, errors.New("record not found"))

	api := buildAPI(dao.DaoWrapper{TokenDao: tokenMock, UsersDao: usersMock, CoursesDao: coursesMock})
	ctx := incomingCtx("authorization", "Bearer "+rawToken)

	_, err := api.ListCourseStreams(ctx, &protobuf.ListCourseStreamsRequest{CourseId: 999})
	if err == nil {
		t.Fatal("expected NotFound error, got nil")
	}
	assertGRPCStatus(t, err, http.StatusNotFound)
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
