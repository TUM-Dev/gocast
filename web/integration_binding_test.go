package web

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"go.uber.org/mock/gomock"
	"gorm.io/gorm"

	"github.com/TUM-Dev/gocast/dao"
	"github.com/TUM-Dev/gocast/mock_dao"
	"github.com/TUM-Dev/gocast/model"
	"github.com/TUM-Dev/gocast/tools"
)

// stubTemplateExecutor is a minimal TemplateExecutor for tests.
// For the confirm page, it writes the service account name and course name into the response so
// assertions can verify them without needing the full Alpine.js / Tailwind template setup.
type stubTemplateExecutor struct{}

func (stubTemplateExecutor) ExecuteTemplate(w io.Writer, name string, data interface{}) error {
	switch d := data.(type) {
	case IntegrationConfirmPageData:
		// Emit enough text for tests to check service account name and course name.
		_, _ = io.WriteString(w, "<html><body>")
		_, _ = io.WriteString(w, "course:"+d.CourseName)
		_, _ = io.WriteString(w, " service:"+d.ServiceUser.GetPreferredName())
		_, _ = io.WriteString(w, "</body></html>")
	case tools.ErrorPageData:
		_, _ = io.WriteString(w, "<html><body>error</body></html>")
	default:
		_, _ = io.WriteString(w, "<html><body>stub</body></html>")
	}
	return nil
}

// courseAdminContext returns a TUMLiveContext that represents a course admin.
func courseAdminContext(course *model.Course) tools.TUMLiveContext {
	admin := &model.User{
		Model:               gorm.Model{ID: 1},
		Name:                "Prof. Test",
		Role:                model.AdminType,
		AdministeredCourses: []model.Course{*course},
	}
	return tools.TUMLiveContext{User: admin, Course: course}
}

// buildRouter creates a minimal Gin router wired to the integration confirm routes and
// injects a TUMLiveContext (with course already set) via middleware.
func buildRouter(t *testing.T, daoWrapper dao.DaoWrapper, ctx tools.TUMLiveContext) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)

	// Install a stub template executor so GET handlers and error pages can render.
	stub := stubTemplateExecutor{}
	templateExecutor = stub
	tools.SetTemplateExecutor(stub)

	r := gin.New()
	// Middleware that pre-populates TUMLiveContext (simulates InitCourse + AdminOfCourse).
	r.Use(func(c *gin.Context) {
		c.Set("TUMLiveContext", ctx)
		c.Next()
	})
	routes := mainRoutes{daoWrapper}
	r.GET("/admin/course/:courseID/integration/confirm", routes.IntegrationBindingConfirmGET)
	r.POST("/admin/course/:courseID/integration/confirm", routes.IntegrationBindingConfirmPOST)
	return r
}

func TestIntegrationBindingConfirmGET_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	course := &model.Course{
		Model: gorm.Model{ID: 10},
		Name:  "Eidi",
		Slug:  "eidi",
	}
	serviceUser := model.User{
		Model: gorm.Model{ID: 99},
		Name:  "artemis-service",
		Role:  model.ServiceType,
	}

	usersDao := mock_dao.NewMockUsersDao(ctrl)
	usersDao.EXPECT().
		GetUserByID(gomock.Any(), uint(99)).
		Return(serviceUser, nil).
		Times(1)

	wrapper := dao.DaoWrapper{UsersDao: usersDao}
	ctx := courseAdminContext(course)

	r := buildRouter(t, wrapper, ctx)

	req := httptest.NewRequest(http.MethodGet, "/admin/course/10/integration/confirm?service=99", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	bodyStr := string(body)
	if !strings.Contains(bodyStr, "artemis-service") {
		t.Errorf("expected body to contain service account name 'artemis-service', got: %s", bodyStr)
	}
	if !strings.Contains(bodyStr, "Eidi") {
		t.Errorf("expected body to contain course name 'Eidi', got: %s", bodyStr)
	}
}

func TestIntegrationBindingConfirmGET_NonServiceType(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	course := &model.Course{Model: gorm.Model{ID: 10}, Name: "Eidi"}
	// User with non-service role
	regularUser := model.User{Model: gorm.Model{ID: 77}, Name: "regular", Role: model.LecturerType}

	usersDao := mock_dao.NewMockUsersDao(ctrl)
	usersDao.EXPECT().
		GetUserByID(gomock.Any(), uint(77)).
		Return(regularUser, nil).
		Times(1)

	wrapper := dao.DaoWrapper{UsersDao: usersDao}
	ctx := courseAdminContext(course)
	r := buildRouter(t, wrapper, ctx)

	req := httptest.NewRequest(http.MethodGet, "/admin/course/10/integration/confirm?service=77", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for non-ServiceType user, got %d", w.Code)
	}
}

func TestIntegrationBindingConfirmGET_MissingServiceParam(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	course := &model.Course{Model: gorm.Model{ID: 10}, Name: "Eidi"}
	wrapper := dao.DaoWrapper{UsersDao: mock_dao.NewMockUsersDao(ctrl)}
	ctx := courseAdminContext(course)
	r := buildRouter(t, wrapper, ctx)

	req := httptest.NewRequest(http.MethodGet, "/admin/course/10/integration/confirm", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for missing service param, got %d", w.Code)
	}
}

func TestIntegrationBindingConfirmPOST_SuccessAllowlistedRedirect(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	course := &model.Course{Model: gorm.Model{ID: 10}, Name: "Eidi", Slug: "eidi"}
	serviceUser := model.User{Model: gorm.Model{ID: 99}, Name: "artemis-service", Role: model.ServiceType}
	adminUser := &model.User{Model: gorm.Model{ID: 1}, Role: model.AdminType, Name: "Prof. Test"}

	usersDao := mock_dao.NewMockUsersDao(ctrl)
	usersDao.EXPECT().GetUserByID(gomock.Any(), uint(99)).Return(serviceUser, nil).Times(1)

	coursesDao := mock_dao.NewMockCoursesDao(ctrl)
	coursesDao.EXPECT().AddAdminToCourse(uint(99), uint(10)).Return(nil).Times(1)

	auditDao := mock_dao.NewMockAuditDao(ctrl)
	auditDao.EXPECT().Create(gomock.Any()).Return(nil).Times(1)

	wrapper := dao.DaoWrapper{
		UsersDao:   usersDao,
		CoursesDao: coursesDao,
		AuditDao:   auditDao,
	}
	ctx := tools.TUMLiveContext{User: adminUser, Course: course}

	// Set the allowed base URL in config.
	origBase := tools.Cfg.AllowedIntegrationRedirectBaseURL
	tools.Cfg.AllowedIntegrationRedirectBaseURL = "https://artemis.example.com"
	defer func() { tools.Cfg.AllowedIntegrationRedirectBaseURL = origBase }()

	r := buildRouter(t, wrapper, ctx)

	redirectURL := "https://artemis.example.com/courses/42/binding-confirmed"
	form := url.Values{}
	form.Set("service", "99")
	form.Set("redirect", redirectURL)

	req := httptest.NewRequest(http.MethodPost, "/admin/course/10/integration/confirm", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	// Same-origin: Origin header must match the request host.
	req.Host = "gocast.example.com"
	req.Header.Set("Origin", "https://gocast.example.com")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusFound {
		t.Fatalf("expected 302 redirect, got %d", w.Code)
	}
	location := w.Header().Get("Location")
	if location != redirectURL {
		t.Errorf("expected redirect to allowlisted URL %q, got %q", redirectURL, location)
	}
}

func TestIntegrationBindingConfirmPOST_BlockedRedirect(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	course := &model.Course{Model: gorm.Model{ID: 10}, Name: "Eidi", Slug: "eidi"}
	serviceUser := model.User{Model: gorm.Model{ID: 99}, Name: "artemis-service", Role: model.ServiceType}
	adminUser := &model.User{Model: gorm.Model{ID: 1}, Role: model.AdminType, Name: "Prof. Test"}

	usersDao := mock_dao.NewMockUsersDao(ctrl)
	usersDao.EXPECT().GetUserByID(gomock.Any(), uint(99)).Return(serviceUser, nil).Times(1)

	coursesDao := mock_dao.NewMockCoursesDao(ctrl)
	coursesDao.EXPECT().AddAdminToCourse(uint(99), uint(10)).Return(nil).Times(1)

	auditDao := mock_dao.NewMockAuditDao(ctrl)
	auditDao.EXPECT().Create(gomock.Any()).Return(nil).Times(1)

	wrapper := dao.DaoWrapper{
		UsersDao:   usersDao,
		CoursesDao: coursesDao,
		AuditDao:   auditDao,
	}
	ctx := tools.TUMLiveContext{User: adminUser, Course: course}

	// AllowedIntegrationRedirectBaseURL set to allowed base, but we send a different host.
	origBase := tools.Cfg.AllowedIntegrationRedirectBaseURL
	tools.Cfg.AllowedIntegrationRedirectBaseURL = "https://artemis.example.com"
	defer func() { tools.Cfg.AllowedIntegrationRedirectBaseURL = origBase }()

	r := buildRouter(t, wrapper, ctx)

	maliciousRedirect := "https://evil.attacker.com/steal-cookies"
	form := url.Values{}
	form.Set("service", "99")
	form.Set("redirect", maliciousRedirect)

	req := httptest.NewRequest(http.MethodPost, "/admin/course/10/integration/confirm", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	// Same-origin: provide a valid same-origin header so CSRF check passes.
	req.Host = "gocast.example.com"
	req.Header.Set("Origin", "https://gocast.example.com")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusFound {
		t.Fatalf("expected 302, got %d", w.Code)
	}
	location := w.Header().Get("Location")
	if location == maliciousRedirect {
		t.Error("open-redirect: handler must not redirect to a non-allow-listed URL")
	}
	// Should fall back to the course admin page.
	expectedFallback := "/admin/course/10"
	if location != expectedFallback {
		t.Errorf("expected fallback redirect to %q, got %q", expectedFallback, location)
	}
}

func TestIntegrationBindingConfirmPOST_NonServiceTypeUserRejected(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	course := &model.Course{Model: gorm.Model{ID: 10}, Name: "Eidi"}
	// A student account, NOT a service account.
	studentUser := model.User{Model: gorm.Model{ID: 55}, Name: "student", Role: model.StudentType}

	usersDao := mock_dao.NewMockUsersDao(ctrl)
	usersDao.EXPECT().GetUserByID(gomock.Any(), uint(55)).Return(studentUser, nil).Times(1)

	// AddAdminToCourse must NOT be called.
	coursesDao := mock_dao.NewMockCoursesDao(ctrl)
	// (no EXPECT — gomock will fail the test if AddAdminToCourse is called)

	wrapper := dao.DaoWrapper{UsersDao: usersDao, CoursesDao: coursesDao}
	adminUser := &model.User{Model: gorm.Model{ID: 1}, Role: model.AdminType, Name: "Prof. Test"}
	ctx := tools.TUMLiveContext{User: adminUser, Course: course}
	r := buildRouter(t, wrapper, ctx)

	form := url.Values{}
	form.Set("service", "55")
	form.Set("redirect", "https://artemis.example.com/done")

	req := httptest.NewRequest(http.MethodPost, "/admin/course/10/integration/confirm", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	// Same-origin: provide a valid same-origin header so CSRF check passes.
	req.Host = "gocast.example.com"
	req.Header.Set("Origin", "https://gocast.example.com")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for non-ServiceType user, got %d", w.Code)
	}
}

// ---------------------------------------------------------------------------
// CSRF / same-origin enforcement tests
// ---------------------------------------------------------------------------

func TestIntegrationBindingConfirmPOST_CrossOrigin_Forbidden(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	course := &model.Course{Model: gorm.Model{ID: 10}, Name: "Eidi", Slug: "eidi"}
	// No DAO methods should be called: CSRF check fires before any business logic.
	wrapper := dao.DaoWrapper{
		UsersDao:   mock_dao.NewMockUsersDao(ctrl),
		CoursesDao: mock_dao.NewMockCoursesDao(ctrl),
	}
	adminUser := &model.User{Model: gorm.Model{ID: 1}, Role: model.AdminType}
	ctx := tools.TUMLiveContext{User: adminUser, Course: course}
	r := buildRouter(t, wrapper, ctx)

	form := url.Values{}
	form.Set("service", "99")

	req := httptest.NewRequest(http.MethodPost, "/admin/course/10/integration/confirm", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	// Cross-origin: Origin header from a different host.
	req.Host = "gocast.example.com"
	req.Header.Set("Origin", "https://evil.attacker.com")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for cross-origin POST, got %d", w.Code)
	}
}

func TestIntegrationBindingConfirmPOST_SameOriginViaReferer(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	course := &model.Course{Model: gorm.Model{ID: 10}, Name: "Eidi", Slug: "eidi"}
	serviceUser := model.User{Model: gorm.Model{ID: 99}, Name: "artemis-service", Role: model.ServiceType}
	adminUser := &model.User{Model: gorm.Model{ID: 1}, Role: model.AdminType, Name: "Prof. Test"}

	usersDao := mock_dao.NewMockUsersDao(ctrl)
	usersDao.EXPECT().GetUserByID(gomock.Any(), uint(99)).Return(serviceUser, nil).Times(1)
	coursesDao := mock_dao.NewMockCoursesDao(ctrl)
	coursesDao.EXPECT().AddAdminToCourse(uint(99), uint(10)).Return(nil).Times(1)
	auditDao := mock_dao.NewMockAuditDao(ctrl)
	auditDao.EXPECT().Create(gomock.Any()).Return(nil).Times(1)

	wrapper := dao.DaoWrapper{UsersDao: usersDao, CoursesDao: coursesDao, AuditDao: auditDao}
	ctx := tools.TUMLiveContext{User: adminUser, Course: course}

	origBase := tools.Cfg.AllowedIntegrationRedirectBaseURL
	tools.Cfg.AllowedIntegrationRedirectBaseURL = ""
	defer func() { tools.Cfg.AllowedIntegrationRedirectBaseURL = origBase }()

	r := buildRouter(t, wrapper, ctx)

	form := url.Values{}
	form.Set("service", "99")

	req := httptest.NewRequest(http.MethodPost, "/admin/course/10/integration/confirm", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	// No Origin header, but same-host Referer — fallback path.
	req.Host = "gocast.example.com"
	req.Header.Set("Referer", "https://gocast.example.com/admin/course/10")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// Binding should succeed (302 redirect to fallback course admin page).
	if w.Code != http.StatusFound {
		t.Fatalf("expected 302 for same-origin Referer, got %d", w.Code)
	}
}

func TestIntegrationBindingConfirmPOST_NoOriginNoReferer_Forbidden(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	course := &model.Course{Model: gorm.Model{ID: 10}, Name: "Eidi", Slug: "eidi"}
	wrapper := dao.DaoWrapper{
		UsersDao:   mock_dao.NewMockUsersDao(ctrl),
		CoursesDao: mock_dao.NewMockCoursesDao(ctrl),
	}
	adminUser := &model.User{Model: gorm.Model{ID: 1}, Role: model.AdminType}
	ctx := tools.TUMLiveContext{User: adminUser, Course: course}
	r := buildRouter(t, wrapper, ctx)

	form := url.Values{}
	form.Set("service", "99")

	// No Origin, no Referer → should be rejected.
	req := httptest.NewRequest(http.MethodPost, "/admin/course/10/integration/confirm", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Host = "gocast.example.com"
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403 when neither Origin nor Referer is set, got %d", w.Code)
	}
}

// ---------------------------------------------------------------------------
// Service-account restriction tests (MEDIUM-2)
// ---------------------------------------------------------------------------

func TestIntegrationBindingConfirmPOST_WrongServiceAccountID_Forbidden(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	course := &model.Course{Model: gorm.Model{ID: 10}, Name: "Eidi", Slug: "eidi"}
	// Operator has configured account 99 as the allowed service account.
	// The request presents account 42 → must be rejected.
	configuredServiceUser := model.User{Model: gorm.Model{ID: 42}, Name: "other-service", Role: model.ServiceType}

	usersDao := mock_dao.NewMockUsersDao(ctrl)
	usersDao.EXPECT().GetUserByID(gomock.Any(), uint(42)).Return(configuredServiceUser, nil).Times(1)

	// AddAdminToCourse must NOT be called.
	coursesDao := mock_dao.NewMockCoursesDao(ctrl)

	wrapper := dao.DaoWrapper{UsersDao: usersDao, CoursesDao: coursesDao}
	adminUser := &model.User{Model: gorm.Model{ID: 1}, Role: model.AdminType}
	ctx := tools.TUMLiveContext{User: adminUser, Course: course}

	// Configure the allowed service account to a DIFFERENT ID.
	origID := tools.Cfg.IntegrationServiceAccountID
	tools.Cfg.IntegrationServiceAccountID = 99
	defer func() { tools.Cfg.IntegrationServiceAccountID = origID }()

	r := buildRouter(t, wrapper, ctx)

	form := url.Values{}
	form.Set("service", "42") // not the configured account
	req := httptest.NewRequest(http.MethodPost, "/admin/course/10/integration/confirm", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Host = "gocast.example.com"
	req.Header.Set("Origin", "https://gocast.example.com")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403 when non-configured service account is presented, got %d", w.Code)
	}
}

func TestIntegrationBindingConfirmPOST_CorrectServiceAccountID_Succeeds(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	course := &model.Course{Model: gorm.Model{ID: 10}, Name: "Eidi", Slug: "eidi"}
	// The configured account AND the request account are both 99 → allowed.
	serviceUser := model.User{Model: gorm.Model{ID: 99}, Name: "artemis-service", Role: model.ServiceType}
	adminUser := &model.User{Model: gorm.Model{ID: 1}, Role: model.AdminType, Name: "Prof. Test"}

	usersDao := mock_dao.NewMockUsersDao(ctrl)
	usersDao.EXPECT().GetUserByID(gomock.Any(), uint(99)).Return(serviceUser, nil).Times(1)
	coursesDao := mock_dao.NewMockCoursesDao(ctrl)
	coursesDao.EXPECT().AddAdminToCourse(uint(99), uint(10)).Return(nil).Times(1)
	auditDao := mock_dao.NewMockAuditDao(ctrl)
	auditDao.EXPECT().Create(gomock.Any()).Return(nil).Times(1)

	wrapper := dao.DaoWrapper{UsersDao: usersDao, CoursesDao: coursesDao, AuditDao: auditDao}
	ctx := tools.TUMLiveContext{User: adminUser, Course: course}

	origID := tools.Cfg.IntegrationServiceAccountID
	tools.Cfg.IntegrationServiceAccountID = 99 // exactly matches the request
	defer func() { tools.Cfg.IntegrationServiceAccountID = origID }()

	origBase := tools.Cfg.AllowedIntegrationRedirectBaseURL
	tools.Cfg.AllowedIntegrationRedirectBaseURL = ""
	defer func() { tools.Cfg.AllowedIntegrationRedirectBaseURL = origBase }()

	r := buildRouter(t, wrapper, ctx)

	form := url.Values{}
	form.Set("service", "99")
	req := httptest.NewRequest(http.MethodPost, "/admin/course/10/integration/confirm", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Host = "gocast.example.com"
	req.Header.Set("Origin", "https://gocast.example.com")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusFound {
		t.Fatalf("expected 302 when configured service account matches, got %d", w.Code)
	}
}

func TestIntegrationBindingConfirmPOST_ZeroConfigID_AnyServiceTypeAllowed(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	course := &model.Course{Model: gorm.Model{ID: 10}, Name: "Eidi", Slug: "eidi"}
	// Any ServiceType user should be accepted when IntegrationServiceAccountID is 0.
	serviceUser := model.User{Model: gorm.Model{ID: 77}, Name: "any-service", Role: model.ServiceType}
	adminUser := &model.User{Model: gorm.Model{ID: 1}, Role: model.AdminType, Name: "Prof. Test"}

	usersDao := mock_dao.NewMockUsersDao(ctrl)
	usersDao.EXPECT().GetUserByID(gomock.Any(), uint(77)).Return(serviceUser, nil).Times(1)
	coursesDao := mock_dao.NewMockCoursesDao(ctrl)
	coursesDao.EXPECT().AddAdminToCourse(uint(77), uint(10)).Return(nil).Times(1)
	auditDao := mock_dao.NewMockAuditDao(ctrl)
	auditDao.EXPECT().Create(gomock.Any()).Return(nil).Times(1)

	wrapper := dao.DaoWrapper{UsersDao: usersDao, CoursesDao: coursesDao, AuditDao: auditDao}
	ctx := tools.TUMLiveContext{User: adminUser, Course: course}

	origID := tools.Cfg.IntegrationServiceAccountID
	tools.Cfg.IntegrationServiceAccountID = 0 // unset → backward-compatible: any ServiceType accepted
	defer func() { tools.Cfg.IntegrationServiceAccountID = origID }()

	origBase := tools.Cfg.AllowedIntegrationRedirectBaseURL
	tools.Cfg.AllowedIntegrationRedirectBaseURL = ""
	defer func() { tools.Cfg.AllowedIntegrationRedirectBaseURL = origBase }()

	r := buildRouter(t, wrapper, ctx)

	form := url.Values{}
	form.Set("service", "77")
	req := httptest.NewRequest(http.MethodPost, "/admin/course/10/integration/confirm", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Host = "gocast.example.com"
	req.Header.Set("Origin", "https://gocast.example.com")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusFound {
		t.Fatalf("expected 302 when IntegrationServiceAccountID=0 (any service type allowed), got %d", w.Code)
	}
}

func TestSafeRedirectTarget(t *testing.T) {
	cases := []struct {
		name     string
		base     string
		target   string
		wantSafe bool
	}{
		{"empty base blocks all", "", "https://artemis.example.com/done", false},
		{"matching prefix allowed", "https://artemis.example.com", "https://artemis.example.com/courses/1/confirm", true},
		{"non-matching host blocked", "https://artemis.example.com", "https://evil.com/steal", false},
		{"empty target", "https://artemis.example.com", "", false},
		{"subdomain not a prefix match", "https://artemis.example.com", "https://artemis.example.com.evil.com/x", false},
	}

	origBase := tools.Cfg.AllowedIntegrationRedirectBaseURL
	defer func() { tools.Cfg.AllowedIntegrationRedirectBaseURL = origBase }()

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tools.Cfg.AllowedIntegrationRedirectBaseURL = tc.base
			got := safeRedirectTarget(tc.target)
			isSafe := got != ""
			if isSafe != tc.wantSafe {
				t.Errorf("safeRedirectTarget(%q) safe=%v, want %v (returned %q)", tc.target, isSafe, tc.wantSafe, got)
			}
		})
	}
}
