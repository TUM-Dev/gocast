package apiv2

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"go.uber.org/mock/gomock"
	"gorm.io/gorm"

	"github.com/TUM-Dev/gocast/dao"
	"github.com/TUM-Dev/gocast/mock_dao"
	"github.com/TUM-Dev/gocast/model"
)

func TestImpersonateIDOf(t *testing.T) {
	for _, tc := range []struct {
		method, path string
		want         uint
	}{
		{"POST", "/api/v2/users/7/impersonate", 7},
		{"GET", "/api/v2/users/7/impersonate", 0},
		{"POST", "/api/v2/users/7", 0},
		{"POST", "/api/v2/users/x/impersonate", 0},
		{"POST", "/api/v2/users/0/impersonate", 0},
		{"POST", "/api/v2/users/7/impersonate/x", 0},
		{"POST", "/api/v2/admin/users/7/impersonate", 0},
	} {
		got, ok := routed(impersonatePath, tc.method, tc.path, impersonateIDOf)
		if ok != (tc.want != 0) || got != tc.want {
			t.Errorf("%s %s: got %d, %v; want %d", tc.method, tc.path, got, ok, tc.want)
		}
	}
}

func serveImpersonate(api *API, req *http.Request) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	api.Handler().ServeHTTP(rec, req)
	return rec
}

func TestHandleImpersonate(t *testing.T) {
	useJWTKey(t)
	admin := &model.User{Model: gorm.Model{ID: 1}, Role: model.AdminType}
	lecturer := &model.User{Model: gorm.Model{ID: 2}, Role: model.LecturerType}
	student := model.User{Model: gorm.Model{ID: 3}, Role: model.StudentType}

	newAPI := func(t *testing.T) (*API, *mock_dao.MockUsersDao) {
		ctrl := gomock.NewController(t)
		users := mock_dao.NewMockUsersDao(ctrl)
		// No audit entry is expected by default: a refused call must not write one.
		audits := mock_dao.NewMockAuditDao(ctrl)
		return &API{dao: dao.DaoWrapper{UsersDao: users, AuditDao: audits}, log: slog.Default()}, users
	}
	post := func(user *model.User) *http.Request {
		req := httptest.NewRequest(http.MethodPost, "/api/v2/users/3/impersonate", nil)
		if user != nil {
			req = req.WithContext(asCaller(user))
		}
		return req
	}

	t.Run("an admin gets a session cookie for the other account", func(t *testing.T) {
		api, users := newAPI(t)
		users.EXPECT().GetUserByID(gomock.Any(), uint(3)).Return(student, nil)
		api.dao.AuditDao.(*mock_dao.MockAuditDao).EXPECT().Create(gomock.Any()).Return(nil)

		rec := serveImpersonate(api, post(admin))

		if rec.Code != http.StatusNoContent {
			t.Fatalf("got %d: %s", rec.Code, rec.Body)
		}
		var jwt *http.Cookie
		for _, c := range rec.Result().Cookies() {
			if c.Name == "jwt" {
				jwt = c
			}
		}
		if jwt == nil || jwt.Value == "" || !jwt.HttpOnly {
			t.Fatalf("want an HttpOnly jwt cookie, got %+v", jwt)
		}
		// The cookie must be what v1 issued: a session of the target, found by the
		// same code that reads every other session.
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.AddCookie(jwt)
		users.EXPECT().GetUserByID(gomock.Any(), uint(3)).Return(student, nil)
		got, err := api.getCurrent(incomingContext(req))
		if err != nil || got.ID != 3 {
			t.Fatalf("the cookie signs in %+v, %v; want user 3", got, err)
		}
		if rec.Header().Get("Cache-Control") != "no-store" {
			t.Error("the response must not be cached")
		}
	})

	t.Run("an impersonation is audited with the admin and the account", func(t *testing.T) {
		users := mock_dao.NewMockUsersDao(gomock.NewController(t))
		audits := mock_dao.NewMockAuditDao(gomock.NewController(t))
		api := &API{dao: dao.DaoWrapper{UsersDao: users, AuditDao: audits}, log: slog.Default()}
		users.EXPECT().GetUserByID(gomock.Any(), uint(3)).Return(student, nil)
		var got *model.Audit
		audits.EXPECT().Create(gomock.Any()).DoAndReturn(func(a *model.Audit) error { got = a; return nil })

		if rec := serveImpersonate(api, post(admin)); rec.Code != http.StatusNoContent {
			t.Fatalf("got %d: %s", rec.Code, rec.Body)
		}
		if got == nil || got.Type != model.AuditImpersonation || got.User == nil || got.User.ID != 1 {
			t.Fatalf("audit = %+v, want an impersonation by user 1", got)
		}
		if !strings.Contains(got.Message, "(1)") || !strings.Contains(got.Message, "(3)") {
			t.Errorf("message = %q, want both account ids", got.Message)
		}
	})

	t.Run("a lecturer is refused and gets no cookie", func(t *testing.T) {
		api, _ := newAPI(t)
		rec := serveImpersonate(api, post(lecturer))
		if rec.Code != http.StatusForbidden || rec.Header().Get("Set-Cookie") != "" {
			t.Fatalf("got %d, cookie %q", rec.Code, rec.Header().Get("Set-Cookie"))
		}
	})

	t.Run("nobody signed in is answered 401", func(t *testing.T) {
		api, _ := newAPI(t)
		rec := serveImpersonate(api, post(nil))
		if rec.Code != http.StatusUnauthorized || rec.Header().Get("Set-Cookie") != "" {
			t.Fatalf("got %d, cookie %q", rec.Code, rec.Header().Get("Set-Cookie"))
		}
	})

	t.Run("an unknown account is answered 404", func(t *testing.T) {
		api, users := newAPI(t)
		users.EXPECT().GetUserByID(gomock.Any(), uint(3)).Return(model.User{}, gorm.ErrRecordNotFound)
		rec := serveImpersonate(api, post(admin))
		if rec.Code != http.StatusNotFound || rec.Header().Get("Set-Cookie") != "" {
			t.Fatalf("got %d, cookie %q", rec.Code, rec.Header().Get("Set-Cookie"))
		}
	})
}
