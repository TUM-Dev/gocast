package apiv2

import (
	"context"
	"errors"
	"log/slog"
	"testing"

	"go.uber.org/mock/gomock"
	"google.golang.org/protobuf/types/known/emptypb"
	"gorm.io/gorm"

	protobuf "github.com/TUM-Dev/gocast/apiv2/protobuf/server"
	"github.com/TUM-Dev/gocast/dao"
	"github.com/TUM-Dev/gocast/mock_dao"
	"github.com/TUM-Dev/gocast/model"
)

// GetLiveCourses answers without credentials, so its visibility rules are all that
// stands between an anonymous caller and an unlisted course. A context with no
// metadata is what an anonymous request looks like by the time it reaches the handler.
func TestGetLiveCoursesHidesRestrictedCoursesFromAnonymousCallers(t *testing.T) {
	course := func(id uint, visibility string) model.Course {
		return model.Course{
			Model:      gorm.Model{ID: id},
			Name:       visibility + " course",
			Slug:       visibility,
			Visibility: visibility,
			// A real owner: an unset one would make the zero-value user an admin via
			// `course.UserID == u.ID`.
			UserID: 42,
		}
	}

	courses := map[uint]model.Course{
		1: course(1, "public"),
		2: course(2, "loggedin"),
		3: course(3, "enrolled"),
		4: course(4, "hidden"),
	}

	streams := []model.Stream{
		{Model: gorm.Model{ID: 11}, CourseID: 1, LiveNow: true},
		{Model: gorm.Model{ID: 12}, CourseID: 2, LiveNow: true},
		{Model: gorm.Model{ID: 13}, CourseID: 3, LiveNow: true},
		{Model: gorm.Model{ID: 14}, CourseID: 4, LiveNow: true},
		{Model: gorm.Model{ID: 15}, CourseID: 1, LiveNow: true, Private: true},
	}

	ctrl := gomock.NewController(t)

	streamsMock := mock_dao.NewMockStreamsDao(ctrl)
	streamsMock.EXPECT().GetCurrentLive(gomock.Any()).Return(streams, nil).Times(1)

	coursesMock := mock_dao.NewMockCoursesDao(ctrl)
	coursesMock.EXPECT().GetCourseById(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, id uint) (model.Course, error) {
			return courses[id], nil
		}).AnyTimes()

	api := &API{
		dao: dao.DaoWrapper{StreamsDao: streamsMock, CoursesDao: coursesMock},
		log: slog.Default(),
	}

	resp, err := api.GetLiveCourses(context.Background(), &emptypb.Empty{})
	if err != nil {
		t.Fatalf("GetLiveCourses: %v", err)
	}

	var got []string
	for _, entry := range resp.LiveCourses {
		got = append(got, entry.Course.Slug)
	}

	// Only the public course. Its private stream is dropped too: that needs course
	// admin rights of its own.
	if len(got) != 1 || got[0] != "public" {
		t.Errorf("anonymous caller saw %v, want only [public]", got)
	}
}

// GetCourseById is uncached and preloads four relations, so a course with several
// lectures live at once was fetched once per stream.
func TestGetLiveCoursesLoadsEachCourseOnce(t *testing.T) {
	ctrl := gomock.NewController(t)

	streamsMock := mock_dao.NewMockStreamsDao(ctrl)
	streamsMock.EXPECT().GetCurrentLive(gomock.Any()).Return([]model.Stream{
		{Model: gorm.Model{ID: 1}, CourseID: 1, LiveNow: true},
		{Model: gorm.Model{ID: 2}, CourseID: 1, LiveNow: true},
		{Model: gorm.Model{ID: 3}, CourseID: 2, LiveNow: true},
		{Model: gorm.Model{ID: 4}, CourseID: 1, LiveNow: true},
	}, nil).Times(1)

	public := func(id uint) model.Course {
		return model.Course{Model: gorm.Model{ID: id}, Visibility: "public", UserID: 42}
	}

	coursesMock := mock_dao.NewMockCoursesDao(ctrl)
	// Two distinct courses across four streams: exactly two lookups.
	coursesMock.EXPECT().GetCourseById(gomock.Any(), uint(1)).Return(public(1), nil).Times(1)
	coursesMock.EXPECT().GetCourseById(gomock.Any(), uint(2)).Return(public(2), nil).Times(1)

	api := &API{
		dao: dao.DaoWrapper{StreamsDao: streamsMock, CoursesDao: coursesMock},
		log: slog.Default(),
	}

	resp, err := api.GetLiveCourses(context.Background(), &emptypb.Empty{})
	if err != nil {
		t.Fatalf("GetLiveCourses: %v", err)
	}
	if len(resp.LiveCourses) != 4 {
		t.Errorf("got %d entries, want all 4 streams", len(resp.LiveCourses))
	}
}

// gorm's Find reports a missing row as a zero-value struct with a nil error, and the
// zero course has no visibility set, so every rule waves it through.
func TestGetLiveCoursesSkipsStreamsWhoseCourseIsGone(t *testing.T) {
	ctrl := gomock.NewController(t)

	streamsMock := mock_dao.NewMockStreamsDao(ctrl)
	streamsMock.EXPECT().GetCurrentLive(gomock.Any()).Return([]model.Stream{
		{Model: gorm.Model{ID: 1}, CourseID: 1, LiveNow: true},
		{Model: gorm.Model{ID: 2}, CourseID: 404, LiveNow: true},
		{Model: gorm.Model{ID: 3}, CourseID: 500, LiveNow: true},
	}, nil).Times(1)

	coursesMock := mock_dao.NewMockCoursesDao(ctrl)
	coursesMock.EXPECT().GetCourseById(gomock.Any(), uint(1)).
		Return(model.Course{Model: gorm.Model{ID: 1}, Name: "Real", Slug: "real", Visibility: "public", UserID: 42}, nil)
	// The deleted-row case: no error, but nothing came back.
	coursesMock.EXPECT().GetCourseById(gomock.Any(), uint(404)).Return(model.Course{}, nil)
	// And an outright failure, which must not be listed either.
	coursesMock.EXPECT().GetCourseById(gomock.Any(), uint(500)).Return(model.Course{}, errors.New("database is down"))

	api := &API{
		dao: dao.DaoWrapper{StreamsDao: streamsMock, CoursesDao: coursesMock},
		log: slog.Default(),
	}

	resp, err := api.GetLiveCourses(context.Background(), &emptypb.Empty{})
	if err != nil {
		t.Fatalf("GetLiveCourses: %v", err)
	}

	if len(resp.LiveCourses) != 1 {
		var got []string
		for _, entry := range resp.LiveCourses {
			got = append(got, entry.Course.Slug)
		}
		t.Fatalf("got %d entries (%v), want only the one with a real course", len(resp.LiveCourses), got)
	}
	if resp.LiveCourses[0].Course.Slug != "real" {
		t.Errorf("got course %q, want real", resp.LiveCourses[0].Course.Slug)
	}
}

// asCaller builds the context the resolveCaller interceptor would have left behind.
func asCaller(u *model.User) context.Context {
	return context.WithValue(context.Background(), callerKey{}, &caller{user: u})
}

// A lecturer's enrolled and administered courses overlap, and GetUserCourses appends
// one list to the other. The v1 start page ran the result through commons.Unique;
// without that the same course is offered twice.
func TestGetUserCoursesDeduplicates(t *testing.T) {
	shared := model.Course{
		Model: gorm.Model{ID: 1}, Name: "Shared", Slug: "shared",
		Year: 2026, TeachingTerm: "W", Visibility: "loggedin", UserID: 42,
	}

	lecturer := &model.User{
		Model: gorm.Model{ID: 7},
		Role:  model.LecturerType,
		// Enrolled in the course they also administer.
		Courses:             []model.Course{shared},
		AdministeredCourses: []model.Course{shared},
	}

	ctrl := gomock.NewController(t)
	coursesMock := mock_dao.NewMockCoursesDao(ctrl)
	coursesMock.EXPECT().GetAdministeredCoursesByUserId(gomock.Any(), uint(7), "W", 2026).
		Return([]model.Course{shared}, nil).Times(1)

	api := &API{dao: dao.DaoWrapper{CoursesDao: coursesMock}, log: slog.Default()}

	resp, err := api.GetUserCourses(asCaller(lecturer), &protobuf.GetUserCoursesRequest{Year: 2026, Term: "W"})
	if err != nil {
		t.Fatalf("GetUserCourses: %v", err)
	}

	if len(resp.Courses) != 1 {
		var got []string
		for _, c := range resp.Courses {
			got = append(got, c.Slug)
		}
		t.Fatalf("got %d courses (%v), want the shared course once", len(resp.Courses), got)
	}
}

// A pin outlives the access that created it: the course can be made enrolled-only, or
// hidden, long after someone pinned it. Listing it then names a course the caller
// cannot open.
func TestGetPinnedCoursesDropsCoursesTheCallerMayNoLongerSee(t *testing.T) {
	course := func(id uint, visibility string) model.Course {
		return model.Course{
			Model: gorm.Model{ID: id}, Name: visibility, Slug: visibility,
			Visibility: visibility, UserID: 42,
		}
	}

	user := &model.User{
		Model: gorm.Model{ID: 7},
		Role:  model.StudentType,
		PinnedCourses: []model.Course{
			course(1, "public"),
			course(2, "loggedin"),
			// Enrolled in none of them, and an administrator of none.
			course(3, "enrolled"),
			course(4, "hidden"),
		},
	}

	api := &API{log: slog.Default()}

	resp, err := api.GetPinnedCourses(asCaller(user), &emptypb.Empty{})
	if err != nil {
		t.Fatalf("GetPinnedCourses: %v", err)
	}

	var got []string
	for _, c := range resp.Courses {
		got = append(got, c.Slug)
	}

	// loggedin is reachable because they are signed in; enrolled and hidden are not.
	if len(got) != 2 || got[0] != "public" || got[1] != "loggedin" {
		t.Errorf("got %v, want [public loggedin]", got)
	}
}
