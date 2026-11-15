package tools

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"go.uber.org/mock/gomock"
	"gorm.io/gorm"

	"github.com/TUM-Dev/gocast/dao"
	"github.com/TUM-Dev/gocast/mock_dao"
	"github.com/TUM-Dev/gocast/model"
)

func TestLoggedIn(t *testing.T) {
	t.Run("sends an anonymous visitor to the login page with somewhere to return to", func(t *testing.T) {
		// Without the return parameter, SetLoginRedirectCookie has nothing to record and
		// the user is dropped on the start page after signing in.
		rec := serveWithContext(t, "/settings?tab=general", TUMLiveContext{})

		if rec.Code != http.StatusFound {
			t.Fatalf("status = %d, want %d", rec.Code, http.StatusFound)
		}
		want := "/login?return=%2Fsettings%3Ftab%3Dgeneral"
		if got := rec.Header().Get("Location"); got != want {
			t.Errorf("Location = %q, want %q", got, want)
		}
	})

	t.Run("lets a signed-in user through", func(t *testing.T) {
		rec := serveWithContext(t, "/settings", TUMLiveContext{User: &model.User{}})

		if rec.Code != http.StatusOK {
			t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
		}
	})
}

// withContext stands in for InitContext: the testutils package has the same helper,
// but imports this one.
func withContext(ctx TUMLiveContext) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			next.ServeHTTP(w, WithTUMLiveContext(r, ctx))
		})
	}
}

// serveWithContext runs LoggedIn against one request with the given context already
// set, standing in for InitContext.
func serveWithContext(t *testing.T, target string, ctx TUMLiveContext) *httptest.ResponseRecorder {
	t.Helper()

	router := NewRouter()
	router.Use(withContext(ctx))
	router.Group(LoggedIn).HandleFunc("GET /settings", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, target, nil))
	return rec
}

// RequirePermission is the single gate in front of thirteen route groups.
func TestRequirePermission(t *testing.T) {
	tests := []struct {
		name     string
		ctx      TUMLiveContext
		required model.Permission
		want     int
	}{
		{
			name:     "an admin holds server administration",
			ctx:      TUMLiveContext{User: &model.User{Role: model.AdminType}},
			required: model.PermAdministerServer,
			want:     http.StatusOK,
		},
		{
			name:     "a lecturer may lecture",
			ctx:      TUMLiveContext{User: &model.User{Role: model.LecturerType}},
			required: model.PermLecture,
			want:     http.StatusOK,
		},
		{
			name:     "a lecturer may not administer the server",
			ctx:      TUMLiveContext{User: &model.User{Role: model.LecturerType}},
			required: model.PermAdministerServer,
			want:     http.StatusForbidden,
		},
		{
			name:     "a lecturer may not manage users",
			ctx:      TUMLiveContext{User: &model.User{Role: model.LecturerType}},
			required: model.PermManageUsers,
			want:     http.StatusForbidden,
		},
		{
			name:     "a student may not lecture",
			ctx:      TUMLiveContext{User: &model.User{Role: model.StudentType}},
			required: model.PermLecture,
			want:     http.StatusForbidden,
		},
		{
			// The gate runs before anything establishes there is a user.
			name:     "an anonymous caller is refused, not a panic",
			ctx:      TUMLiveContext{},
			required: model.PermLecture,
			want:     http.StatusForbidden,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := NewRouter()
			router.Use(withContext(tt.ctx))
			router.Group(RequirePermission(tt.required)).HandleFunc("GET /gated", func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusOK)
			})

			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/gated", nil))

			if rec.Code != tt.want {
				t.Errorf("status = %d, want %d", rec.Code, tt.want)
			}
		})
	}
}

// A refused course page goes through the error page renderer, so the SPA can show the
// refusal; a bare 403 used to leave the browser with an empty page.
func TestAdminOfCourseRendersTheRefusal(t *testing.T) {
	var rendered int
	SetErrorPageRenderer(func(w http.ResponseWriter, _ *http.Request, status int, _ string) {
		rendered = status
		w.WriteHeader(status)
	})
	t.Cleanup(func() { SetErrorPageRenderer(nil) })

	course := model.Course{Model: gorm.Model{ID: 1}}
	router := NewRouter()
	router.Use(withContext(TUMLiveContext{
		User:   &model.User{Role: model.StudentType},
		Course: &course,
	}))
	router.Group(AdminOfCourse).HandleFunc("GET /admin/courses/1/settings", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/admin/courses/1/settings", nil))

	if rec.Code != http.StatusForbidden || rendered != http.StatusForbidden {
		t.Errorf("status = %d, rendered = %d, want 403 through the renderer", rec.Code, rendered)
	}
}

// InitStream behind InitCourse: the course from the URL is already in the context, and
// AdminOfCourse vouched only for it, so a stream of any other course must look missing.
func TestInitStreamCourseScope(t *testing.T) {
	course := model.Course{Model: gorm.Model{ID: 1}, Visibility: "public"}
	own := model.Stream{Model: gorm.Model{ID: 10}, CourseID: 1}
	foreign := model.Stream{Model: gorm.Model{ID: 20}, CourseID: 2}

	tests := []struct {
		name     string
		streamID string
		want     int
	}{
		{"stream of the context course", "10", http.StatusOK},
		{"stream of another course", "20", http.StatusNotFound},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			streams := mock_dao.NewMockStreamsDao(gomock.NewController(t))
			streams.EXPECT().GetStreamByID(gomock.Any(), "10").Return(own, nil).AnyTimes()
			streams.EXPECT().GetStreamByID(gomock.Any(), "20").Return(foreign, nil).AnyTimes()

			router := NewRouter()
			router.Use(withContext(TUMLiveContext{User: &model.User{Role: model.AdminType}, Course: &course}))
			router.Group(InitStream(dao.DaoWrapper{StreamsDao: streams})).HandleFunc("GET /{streamID}", func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusOK)
			})

			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/"+tt.streamID, nil))
			if rec.Code != tt.want {
				t.Errorf("status = %d, want %d", rec.Code, tt.want)
			}
		})
	}
}
