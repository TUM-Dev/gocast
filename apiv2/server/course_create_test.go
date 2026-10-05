package apiv2

import (
	"context"
	"errors"
	"log/slog"
	"testing"

	"go.uber.org/mock/gomock"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"gorm.io/gorm"

	protobuf "github.com/TUM-Dev/gocast/apiv2/protobuf/server"
	"github.com/TUM-Dev/gocast/dao"
	"github.com/TUM-Dev/gocast/mock_dao"
	"github.com/TUM-Dev/gocast/model"
	"github.com/TUM-Dev/gocast/model/search"
)

// fakeTUMOnline answers searches from a fixed list and records what it was asked to
// sync.
type fakeTUMOnline struct {
	courses  []search.PrefetchedCourse
	err      error
	gotQuery string
	gotLimit int64
	synced   []model.Course
}

func (f *fakeTUMOnline) SearchCourses(q string, limit int64) ([]search.PrefetchedCourse, error) {
	f.gotQuery, f.gotLimit = q, limit
	return f.courses, f.err
}

func (f *fakeTUMOnline) SyncCourse(course model.Course) {
	f.synced = append(f.synced, course)
}

func TestCreateCourse(t *testing.T) {
	lecturer := &model.User{Model: gorm.Model{ID: 2}, Role: model.LecturerType}

	valid := func() *protobuf.CreateCourseRequest {
		return &protobuf.CreateCourseRequest{
			Name: "  Brauereiwesen  ", Slug: "brau_2", Year: 2026, Term: "W", TumOnlineId: "950", Language: "de",
		}
	}

	// setup expects the slug lookup to find nothing unless lookupErr says otherwise;
	// creation and the audit are left for the subtest to expect.
	setup := func(t *testing.T, lookupErr error) (*API, *mock_dao.MockCoursesDao, *mock_dao.MockAuditDao, *fakeTUMOnline) {
		ctrl := gomock.NewController(t)
		courses := mock_dao.NewMockCoursesDao(ctrl)
		courses.EXPECT().GetCourseBySlugYearAndTerm(gomock.Any(), "brau_2", "W", 2026).
			Return(model.Course{}, lookupErr).AnyTimes()
		audits := mock_dao.NewMockAuditDao(ctrl)
		tumOnline := &fakeTUMOnline{}

		return &API{
			dao:       dao.DaoWrapper{CoursesDao: courses, AuditDao: audits},
			log:       slog.Default(),
			tumOnline: tumOnline,
		}, courses, audits, tumOnline
	}

	t.Run("creates the course with the old form's defaults and its creator as admin", func(t *testing.T) {
		api, courses, audits, tumOnline := setup(t, gorm.ErrRecordNotFound)
		var created model.Course
		courses.EXPECT().CreateCourse(gomock.Any(), gomock.Any(), true).
			DoAndReturn(func(_ context.Context, c *model.Course, _ bool) error {
				c.ID = 42
				created = *c
				return nil
			})
		audits.EXPECT().Create(gomock.Any()).Return(nil)

		resp, err := api.CreateCourse(asCaller(lecturer), valid())
		if err != nil {
			t.Fatalf("CreateCourse: %v", err)
		}

		if resp.CourseId != 42 {
			t.Errorf("CourseId = %d, want 42", resp.CourseId)
		}
		if created.Name != "Brauereiwesen" {
			t.Errorf("name = %q, want it trimmed", created.Name)
		}
		if created.Visibility != "loggedin" || !created.VODEnabled || created.DownloadsEnabled || created.ChatEnabled {
			t.Errorf("defaults = %q vod=%v dl=%v chat=%v", created.Visibility, created.VODEnabled, created.DownloadsEnabled, created.ChatEnabled)
		}
		if created.TUMOnlineIdentifier != "950" || !created.Language.Valid || created.Language.String != "de" {
			t.Errorf("tumonline=%q language=%v", created.TUMOnlineIdentifier, created.Language)
		}
		if len(created.Admins) != 1 || created.Admins[0].ID != lecturer.ID {
			t.Errorf("admins = %v, want the creator", created.Admins)
		}
		if len(tumOnline.synced) != 1 || tumOnline.synced[0].ID != 42 {
			t.Errorf("synced = %v, want the new course", tumOnline.synced)
		}
	})

	t.Run("does not list an administrator of every course as an admin", func(t *testing.T) {
		api, courses, audits, _ := setup(t, gorm.ErrRecordNotFound)
		courses.EXPECT().CreateCourse(gomock.Any(), gomock.Any(), true).
			DoAndReturn(func(_ context.Context, c *model.Course, _ bool) error {
				if len(c.Admins) != 0 {
					t.Errorf("admins = %v, want none", c.Admins)
				}
				return nil
			})
		audits.EXPECT().Create(gomock.Any()).Return(nil)

		admin := &model.User{Model: gorm.Model{ID: 1}, Role: model.AdminType}
		if _, err := api.CreateCourse(asCaller(admin), valid()); err != nil {
			t.Fatalf("CreateCourse: %v", err)
		}
	})

	t.Run("refuses a slug the semester already has", func(t *testing.T) {
		api, _, _, _ := setup(t, nil)

		_, err := api.CreateCourse(asCaller(lecturer), valid())
		if status.Code(err) != codes.AlreadyExists {
			t.Fatalf("code = %v, want AlreadyExists (409)", status.Code(err))
		}
	})

	t.Run("does not read a failed lookup as a free slug", func(t *testing.T) {
		api, _, _, _ := setup(t, errors.New("connection reset"))

		_, err := api.CreateCourse(asCaller(lecturer), valid())
		// e.WithStatus maps a 500 to Unknown.
		if status.Code(err) != codes.Unknown {
			t.Fatalf("code = %v, want Unknown (500)", status.Code(err))
		}
	})

	for name, mutate := range map[string]func(*protobuf.CreateCourseRequest){
		"a blank name":            func(r *protobuf.CreateCourseRequest) { r.Name = "  " },
		"a slug with a space":     func(r *protobuf.CreateCourseRequest) { r.Slug = "brau 2" },
		"a slug with a slash":     func(r *protobuf.CreateCourseRequest) { r.Slug = "brau/2" },
		"an empty slug":           func(r *protobuf.CreateCourseRequest) { r.Slug = "" },
		"a two-digit year":        func(r *protobuf.CreateCourseRequest) { r.Year = 26 },
		"an unknown term":         func(r *protobuf.CreateCourseRequest) { r.Term = "Wintersemester" },
		"an unsupported language": func(r *protobuf.CreateCourseRequest) { r.Language = "fr" },
	} {
		t.Run("rejects "+name, func(t *testing.T) {
			api, _, _, _ := setup(t, gorm.ErrRecordNotFound)
			req := valid()
			mutate(req)

			_, err := api.CreateCourse(asCaller(lecturer), req)
			if status.Code(err) != codes.InvalidArgument {
				t.Fatalf("code = %v, want InvalidArgument", status.Code(err))
			}
		})
	}

	t.Run("creates without TUMOnline, just without syncing", func(t *testing.T) {
		api, courses, audits, _ := setup(t, gorm.ErrRecordNotFound)
		api.tumOnline = nil
		courses.EXPECT().CreateCourse(gomock.Any(), gomock.Any(), true).Return(nil)
		audits.EXPECT().Create(gomock.Any()).Return(nil)

		if _, err := api.CreateCourse(asCaller(lecturer), valid()); err != nil {
			t.Fatalf("CreateCourse: %v", err)
		}
	})
}

func TestSearchTumOnlineCourses(t *testing.T) {
	t.Run("answers the backend's matches", func(t *testing.T) {
		tumOnline := &fakeTUMOnline{courses: []search.PrefetchedCourse{
			{CourseID: "950", Name: "Brauereiwesen", Year: 2026, Term: "W"},
		}}
		api := &API{log: slog.Default(), tumOnline: tumOnline}

		resp, err := api.SearchTumOnlineCourses(context.Background(), &protobuf.SearchTumOnlineCoursesRequest{Q: "brau"})
		if err != nil {
			t.Fatalf("SearchTumOnlineCourses: %v", err)
		}
		if tumOnline.gotQuery != "brau" || tumOnline.gotLimit != 10 {
			t.Errorf("searched %q limit %d", tumOnline.gotQuery, tumOnline.gotLimit)
		}
		if len(resp.Courses) != 1 || resp.Courses[0].TumOnlineId != "950" || resp.Courses[0].Term != "W" {
			t.Errorf("courses = %v", resp.Courses)
		}
	})

	t.Run("is unavailable when the backend is", func(t *testing.T) {
		api := &API{log: slog.Default(), tumOnline: &fakeTUMOnline{err: errors.New("connection refused")}}

		_, err := api.SearchTumOnlineCourses(context.Background(), &protobuf.SearchTumOnlineCoursesRequest{Q: "brau"})
		if status.Code(err) != codes.Unavailable {
			t.Fatalf("code = %v, want Unavailable", status.Code(err))
		}
	})

	t.Run("is unavailable without TUMOnline", func(t *testing.T) {
		api := &API{log: slog.Default()}

		_, err := api.SearchTumOnlineCourses(context.Background(), &protobuf.SearchTumOnlineCoursesRequest{Q: "brau"})
		if status.Code(err) != codes.Unavailable {
			t.Fatalf("code = %v, want Unavailable", status.Code(err))
		}
	})
}
