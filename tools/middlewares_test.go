package tools

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"go.uber.org/mock/gomock"
	"gorm.io/gorm"

	"github.com/TUM-Dev/gocast/dao"
	"github.com/TUM-Dev/gocast/mock_dao"
	"github.com/TUM-Dev/gocast/model"
)

func TestLoggedIn(t *testing.T) {
	gin.SetMode(gin.TestMode)

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

// serveWithContext runs LoggedIn against one request with the given context already
// set, standing in for InitContext.
func serveWithContext(t *testing.T, target string, ctx TUMLiveContext) *httptest.ResponseRecorder {
	t.Helper()

	router := gin.New()
	router.Use(func(c *gin.Context) { c.Set("TUMLiveContext", ctx) })
	router.GET("/settings", LoggedIn, func(c *gin.Context) { c.Status(http.StatusOK) })

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, target, nil))
	return rec
}

// RequirePermission is the single gate in front of thirteen route groups.
func TestRequirePermission(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// The forbidden path renders the error page, which needs an executor installed.
	SetTemplateExecutor(stubTemplateExecutor{})

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
			router := gin.New()
			router.Use(func(c *gin.Context) { c.Set("TUMLiveContext", tt.ctx) })
			router.GET("/gated", RequirePermission(tt.required), func(c *gin.Context) {
				c.Status(http.StatusOK)
			})

			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/gated", nil))

			if rec.Code != tt.want {
				t.Errorf("status = %d, want %d", rec.Code, tt.want)
			}
		})
	}
}

type stubTemplateExecutor struct{}

func (stubTemplateExecutor) ExecuteTemplate(w io.Writer, _ string, _ interface{}) error {
	_, err := w.Write([]byte("forbidden"))
	return err
}

// InitStream behind InitCourse: the course from the URL is already in the context, and
// AdminOfCourse vouched only for it, so a stream of any other course must look missing.
func TestInitStreamCourseScope(t *testing.T) {
	gin.SetMode(gin.TestMode)
	SetTemplateExecutor(stubTemplateExecutor{})

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

			router := gin.New()
			router.Use(func(c *gin.Context) {
				c.Set("TUMLiveContext", TUMLiveContext{User: &model.User{Role: model.AdminType}, Course: &course})
			})
			router.GET("/:streamID", InitStream(dao.DaoWrapper{StreamsDao: streams}), func(c *gin.Context) {
				c.Status(http.StatusOK)
			})

			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/"+tt.streamID, nil))
			if rec.Code != tt.want {
				t.Errorf("status = %d, want %d", rec.Code, tt.want)
			}
		})
	}
}
