package apiv2

import (
	"context"
	"errors"
	"log/slog"
	"testing"

	"go.uber.org/mock/gomock"
	"google.golang.org/grpc/codes"
	"gorm.io/gorm"

	"github.com/TUM-Dev/gocast/api"
	protobuf "github.com/TUM-Dev/gocast/apiv2/protobuf/server"
	"github.com/TUM-Dev/gocast/dao"
	"github.com/TUM-Dev/gocast/mock_dao"
	"github.com/TUM-Dev/gocast/model"
)

/* Search. */

// fakeSearch records what the handler asked for and answers with canned hits.
type fakeSearch struct {
	user  *model.User
	query string
	limit int64
	scope api.SearchScope
	hits  api.SearchHits
	err   error
}

func (f *fakeSearch) Search(_ context.Context, user *model.User, query string, limit int64, scope api.SearchScope) (api.SearchHits, error) {
	f.user, f.query, f.limit, f.scope = user, query, limit, scope
	return f.hits, f.err
}

func TestSearch(t *testing.T) {
	student := &model.User{Model: gorm.Model{ID: 4}, Role: model.StudentType}

	t.Run("passes the query, a default limit and the caller along and maps the hits", func(t *testing.T) {
		search := &fakeSearch{hits: api.SearchHits{
			Courses: []api.SearchCourseDTO{{Name: "Brauereiwesen", Slug: "brauereiwesen", Year: 2022, TeachingTerm: "S"}},
			Streams: []api.SearchStreamDTO{{ID: 7, Name: "Hopfen", CourseSlug: "brauereiwesen", Year: 2022, TeachingTerm: "S"}},
		}}
		a := &API{search: search, log: slog.Default()}

		resp, err := a.Search(asCaller(student), &protobuf.SearchRequest{Query: "  hopfen "})
		if err != nil {
			t.Fatalf("Search: %v", err)
		}
		if search.user != student || search.query != "hopfen" || search.limit != searchDefaultLimit {
			t.Errorf("asked for %q limit %d as %v", search.query, search.limit, search.user)
		}
		if len(resp.Courses) != 1 || resp.Courses[0].Slug != "brauereiwesen" || len(resp.Streams) != 1 || resp.Streams[0].Id != 7 {
			t.Errorf("resp = %v", resp)
		}
	})

	t.Run("serves an anonymous caller", func(t *testing.T) {
		search := &fakeSearch{}
		a := &API{search: search, log: slog.Default()}

		if _, err := a.Search(context.Background(), &protobuf.SearchRequest{Query: "bier", Limit: 500}); err != nil {
			t.Fatalf("Search: %v", err)
		}
		if search.user != nil || search.limit != searchMaxLimit {
			t.Errorf("user = %v, limit = %d", search.user, search.limit)
		}
	})

	t.Run("narrows by semesters and a range", func(t *testing.T) {
		search := &fakeSearch{}
		a := &API{search: search, log: slog.Default()}

		_, err := a.Search(context.Background(), &protobuf.SearchRequest{
			Query: "bier", Semesters: []string{"2022S", "2021W"}, FirstSemester: "2020W", LastSemester: "2022S", CoursesOnly: true,
		})
		if err != nil {
			t.Fatalf("Search: %v", err)
		}
		s := search.scope
		if len(s.Semesters) != 2 || s.Semesters[1].Year != 2021 || s.FirstSemester == nil || s.FirstSemester.Year != 2020 ||
			s.LastSemester == nil || s.LastSemester.TeachingTerm != "S" || !s.CoursesOnly {
			t.Errorf("scope = %+v", s)
		}
	})

	t.Run("searches inside a course the caller may watch", func(t *testing.T) {
		courses := mock_dao.NewMockCoursesDao(gomock.NewController(t))
		courses.EXPECT().GetCourseBySlugYearAndTerm(gomock.Any(), "brauereiwesen", "S", 2022).
			Return(model.Course{Model: gorm.Model{ID: 1}, Slug: "brauereiwesen", Visibility: "public"}, nil)
		search := &fakeSearch{}
		a := &API{search: search, dao: dao.DaoWrapper{CoursesDao: courses}, log: slog.Default()}

		if _, err := a.Search(context.Background(), &protobuf.SearchRequest{Query: "bier", Courses: []string{"brauereiwesen2022S"}}); err != nil {
			t.Fatalf("Search: %v", err)
		}
		if len(search.scope.Courses) != 1 || search.scope.Courses[0].ID != 1 {
			t.Errorf("scope = %+v", search.scope)
		}
	})

	t.Run("tells an anonymous caller a loggedin course does not exist", func(t *testing.T) {
		courses := mock_dao.NewMockCoursesDao(gomock.NewController(t))
		courses.EXPECT().GetCourseBySlugYearAndTerm(gomock.Any(), "geheim", "S", 2022).
			Return(model.Course{Model: gorm.Model{ID: 5}, Visibility: "loggedin"}, nil)
		a := &API{search: &fakeSearch{}, dao: dao.DaoWrapper{CoursesDao: courses}, log: slog.Default()}

		_, err := a.Search(context.Background(), &protobuf.SearchRequest{Query: "bier", Courses: []string{"geheim2022S"}})
		wantCode(t, err, codes.NotFound)
	})

	for name, req := range map[string]*protobuf.SearchRequest{
		"no query":               {Query: "  "},
		"a malformed semester":   {Query: "bier", Semesters: []string{"2022X"}},
		"half a range":           {Query: "bier", FirstSemester: "2022S"},
		"too many courses":       {Query: "bier", Courses: []string{"a2022S", "b2022S", "c2022S", "d2022S"}},
		"a malformed course key": {Query: "bier", Courses: []string{"nonsense"}},
	} {
		t.Run("refuses "+name, func(t *testing.T) {
			a := &API{search: &fakeSearch{}, log: slog.Default()}
			_, err := a.Search(context.Background(), req)
			if name == "a malformed course key" {
				wantCode(t, err, codes.NotFound)
			} else {
				wantCode(t, err, codes.InvalidArgument)
			}
		})
	}

	t.Run("answers 503 without Meilisearch", func(t *testing.T) {
		a := &API{search: &fakeSearch{err: api.ErrSearchUnavailable}, log: slog.Default()}
		_, err := a.Search(context.Background(), &protobuf.SearchRequest{Query: "bier"})
		wantCode(t, err, codes.Unavailable)

		none := &API{log: slog.Default()}
		_, err = none.Search(context.Background(), &protobuf.SearchRequest{Query: "bier"})
		wantCode(t, err, codes.Unavailable)
	})
}

/* Course tokens. */

func TestCourseByToken(t *testing.T) {
	course := model.Course{Model: gorm.Model{ID: 3}, Name: "Golang", Slug: "godev", Year: 2021, TeachingTerm: "W"}

	setup := func(t *testing.T) (*API, *mock_dao.MockCoursesDao, *mock_dao.MockAuditDao) {
		ctrl := gomock.NewController(t)
		courses := mock_dao.NewMockCoursesDao(ctrl)
		audits := mock_dao.NewMockAuditDao(ctrl)
		return &API{dao: dao.DaoWrapper{CoursesDao: courses, AuditDao: audits}, log: slog.Default()}, courses, audits
	}

	t.Run("names the course, opted out or not", func(t *testing.T) {
		a, courses, _ := setup(t)
		out := course
		out.DeletedAt = gorm.DeletedAt{Valid: true}
		courses.EXPECT().GetCourseByToken("secret").Return(out, nil)

		resp, err := a.GetCourseByToken(context.Background(), &protobuf.CourseTokenRequest{Token: " secret "})
		if err != nil {
			t.Fatalf("GetCourseByToken: %v", err)
		}
		if resp.Id != 3 || resp.Slug != "godev" || resp.Year != 2021 || !resp.OptedOut {
			t.Errorf("resp = %v", resp)
		}
	})

	t.Run("opts in: undeletes and saves visibility and recordings", func(t *testing.T) {
		a, courses, audits := setup(t)
		courses.EXPECT().GetCourseByToken("secret").Return(course, nil)
		courses.EXPECT().UnDeleteCourse(gomock.Any(), gomock.Any()).Return(nil)
		courses.EXPECT().UpdateCourseColumns(gomock.Any(), gomock.Any(), "vod_enabled", "visibility").
			DoAndReturn(func(_ context.Context, c model.Course, _ ...string) error {
				if !c.VODEnabled || c.Visibility != "loggedin" {
					t.Errorf("saved %+v", c)
				}
				return nil
			})
		audits.EXPECT().Create(gomock.Any()).Return(nil)

		if _, err := a.OptInCourseByToken(context.Background(), &protobuf.CourseTokenRequest{Token: "secret"}); err != nil {
			t.Fatalf("OptInCourseByToken: %v", err)
		}
	})

	t.Run("opts out: deletes once", func(t *testing.T) {
		a, courses, audits := setup(t)
		courses.EXPECT().GetCourseByToken("secret").Return(course, nil)
		audits.EXPECT().Create(gomock.Any()).Return(nil)
		courses.EXPECT().DeleteCourse(gomock.Any())

		if _, err := a.OptOutCourseByToken(context.Background(), &protobuf.CourseTokenRequest{Token: "secret"}); err != nil {
			t.Fatalf("OptOutCourseByToken: %v", err)
		}

		// Already out: nothing to delete again.
		out := course
		out.DeletedAt = gorm.DeletedAt{Valid: true}
		courses.EXPECT().GetCourseByToken("secret").Return(out, nil)
		if _, err := a.OptOutCourseByToken(context.Background(), &protobuf.CourseTokenRequest{Token: "secret"}); err != nil {
			t.Fatalf("OptOutCourseByToken again: %v", err)
		}
	})

	t.Run("refuses an unknown or missing token", func(t *testing.T) {
		a, courses, _ := setup(t)
		courses.EXPECT().GetCourseByToken("nope").Return(model.Course{}, gorm.ErrRecordNotFound)

		_, err := a.GetCourseByToken(context.Background(), &protobuf.CourseTokenRequest{Token: "nope"})
		wantCode(t, err, codes.NotFound)
		_, err = a.OptInCourseByToken(context.Background(), &protobuf.CourseTokenRequest{})
		wantCode(t, err, codes.InvalidArgument)
	})
}

/* Password reset. */

func TestPasswordReset(t *testing.T) {
	user := model.User{Model: gorm.Model{ID: 2}, Name: "Prof"}

	setup := func(t *testing.T) (*API, *mock_dao.MockUsersDao) {
		users := mock_dao.NewMockUsersDao(gomock.NewController(t))
		return &API{dao: dao.DaoWrapper{UsersDao: users}, log: slog.Default()}, users
	}

	t.Run("checks a key", func(t *testing.T) {
		a, users := setup(t)
		users.EXPECT().GetUserByResetKey("key").Return(user, nil)
		users.EXPECT().GetUserByResetKey("used").Return(model.User{}, gorm.ErrRecordNotFound)

		if _, err := a.CheckPasswordResetKey(context.Background(), &protobuf.PasswordResetKeyRequest{Key: "key"}); err != nil {
			t.Fatalf("CheckPasswordResetKey: %v", err)
		}
		_, err := a.CheckPasswordResetKey(context.Background(), &protobuf.PasswordResetKeyRequest{Key: "used"})
		wantCode(t, err, codes.NotFound)
	})

	t.Run("sets the password and spends the key", func(t *testing.T) {
		a, users := setup(t)
		users.EXPECT().GetUserByResetKey("key").Return(user, nil)
		users.EXPECT().UpdateUser(gomock.Any()).DoAndReturn(func(u model.User) error {
			if ok, _ := u.ComparePasswordAndHash("correct horse"); !ok {
				t.Errorf("password not set")
			}
			return nil
		})
		users.EXPECT().DeleteResetKey("key")

		_, err := a.SetPasswordByResetKey(context.Background(), &protobuf.SetPasswordByResetKeyRequest{Key: "key", Password: "correct horse"})
		if err != nil {
			t.Fatalf("SetPasswordByResetKey: %v", err)
		}
	})

	t.Run("refuses a short password without touching the key", func(t *testing.T) {
		a, users := setup(t)
		users.EXPECT().GetUserByResetKey("key").Return(user, nil)

		_, err := a.SetPasswordByResetKey(context.Background(), &protobuf.SetPasswordByResetKeyRequest{Key: "key", Password: "short"})
		wantCode(t, err, codes.InvalidArgument)
	})
}

/* Onboarding. */

func TestCreateFirstUser(t *testing.T) {
	setup := func(t *testing.T, fresh bool) (*API, *mock_dao.MockUsersDao) {
		users := mock_dao.NewMockUsersDao(gomock.NewController(t))
		users.EXPECT().AreUsersEmpty(gomock.Any()).Return(fresh, nil)
		return &API{dao: dao.DaoWrapper{UsersDao: users}, log: slog.Default()}, users
	}
	req := &protobuf.CreateFirstUserRequest{Name: " Root ", Email: "root@example.com", Password: "correct horse"}

	t.Run("creates an administrator while there are no users", func(t *testing.T) {
		a, users := setup(t, true)
		users.EXPECT().CreateUser(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, u *model.User) error {
			if u.Name != "Root" || u.Email.String != "root@example.com" || u.Role != model.AdminType || u.Password == "" {
				t.Errorf("created %+v", u)
			}
			return nil
		})

		if _, err := a.CreateFirstUser(context.Background(), req); err != nil {
			t.Fatalf("CreateFirstUser: %v", err)
		}
	})

	t.Run("refuses once a user exists", func(t *testing.T) {
		a, _ := setup(t, false)
		_, err := a.CreateFirstUser(context.Background(), req)
		wantCode(t, err, codes.PermissionDenied)
	})

	for name, bad := range map[string]*protobuf.CreateFirstUserRequest{
		"no name":          {Email: "root@example.com", Password: "correct horse"},
		"a bad email":      {Name: "Root", Email: "root", Password: "correct horse"},
		"a short password": {Name: "Root", Email: "root@example.com", Password: "short"},
	} {
		t.Run("refuses "+name, func(t *testing.T) {
			a, _ := setup(t, true)
			_, err := a.CreateFirstUser(context.Background(), bad)
			wantCode(t, err, codes.InvalidArgument)
		})
	}

	t.Run("reports a database it cannot ask", func(t *testing.T) {
		users := mock_dao.NewMockUsersDao(gomock.NewController(t))
		users.EXPECT().AreUsersEmpty(gomock.Any()).Return(false, errors.New("down"))
		a := &API{dao: dao.DaoWrapper{UsersDao: users}, log: slog.Default()}
		_, err := a.CreateFirstUser(context.Background(), req)
		wantCode(t, err, codes.Unknown)
	})
}
