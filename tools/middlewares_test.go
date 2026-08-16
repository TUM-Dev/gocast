package tools

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

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
