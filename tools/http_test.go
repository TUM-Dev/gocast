package tools

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/TUM-Dev/gocast/dao"
	"github.com/TUM-Dev/gocast/model"
)

func TestTUMLiveContextOnTheRequest(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	_, ok := GetTUMLiveContext(r)
	assert.False(t, ok, "a request nothing set a context on")

	user := &model.User{Name: "Anja"}
	got, ok := GetTUMLiveContext(WithTUMLiveContext(r, TUMLiveContext{User: user}))
	require.True(t, ok)
	assert.Same(t, user, got.User)

	// The later context replaces the earlier one, as InitCourse and InitStream rely on.
	replaced := WithTUMLiveContext(WithTUMLiveContext(r, TUMLiveContext{User: user}), TUMLiveContext{})
	got, _ = GetTUMLiveContext(replaced)
	assert.Nil(t, got.User)
}

func TestRequireTUMLiveContextAnswers500(t *testing.T) {
	rec := httptest.NewRecorder()
	_, ok := requireTUMLiveContext(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	assert.False(t, ok)
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestWriteHelpers(t *testing.T) {
	rec := httptest.NewRecorder()
	WriteJSON(rec, http.StatusTeapot, map[string]any{"a": 1})
	assert.Equal(t, http.StatusTeapot, rec.Code)
	assert.Equal(t, "application/json; charset=utf-8", rec.Header().Get("Content-Type"))
	assert.JSONEq(t, `{"a":1}`, rec.Body.String())

	rec = httptest.NewRecorder()
	WriteJSON(rec, http.StatusOK, make(chan int))
	assert.Equal(t, http.StatusInternalServerError, rec.Code, "an unencodable value is a 500, not a 200 with half a body")

	rec = httptest.NewRecorder()
	WriteText(rec, http.StatusNotFound, "gone")
	assert.Equal(t, "text/plain; charset=utf-8", rec.Header().Get("Content-Type"))
	assert.Equal(t, "gone", rec.Body.String())
}

// The value is query-escaped on the way out and unescaped on the way in, as the gin
// cookies were: redirURL holds a path with slashes and a query, and a session cookie
// set before the migration has to read back.
func TestCookieRoundTrip(t *testing.T) {
	rec := httptest.NewRecorder()
	SetCookie(rec, "redirURL", "/w/eidi/7?x=1&y=a b", 600, true)

	cookies := rec.Result().Cookies()
	require.Len(t, cookies, 1)
	c := cookies[0]
	assert.Equal(t, "/", c.Path)
	assert.True(t, c.HttpOnly)
	assert.True(t, c.Secure)
	assert.Equal(t, 600, c.MaxAge)
	assert.NotContains(t, c.Value, "/", "stored escaped")

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.AddCookie(c)
	got, err := Cookie(r, "redirURL")
	require.NoError(t, err)
	assert.Equal(t, "/w/eidi/7?x=1&y=a b", got)

	_, err = Cookie(httptest.NewRequest(http.MethodGet, "/", nil), "redirURL")
	assert.Error(t, err)
}

func TestSetCookieClears(t *testing.T) {
	rec := httptest.NewRecorder()
	SetCookie(rec, "jwt", "", -1, false)
	assert.Contains(t, rec.Header().Get("Set-Cookie"), "Max-Age=0")
}

func TestRenderErrorPage(t *testing.T) {
	var rendered int
	SetErrorPageRenderer(func(w http.ResponseWriter, _ *http.Request, status int, _ string) {
		rendered = status
		w.WriteHeader(status)
	})
	t.Cleanup(func() { SetErrorPageRenderer(nil) })

	t.Run("a page goes through the renderer", func(t *testing.T) {
		rendered = 0
		rec := httptest.NewRecorder()
		RenderErrorPage(rec, httptest.NewRequest(http.MethodGet, "/admin", nil), http.StatusForbidden, "no")
		assert.Equal(t, http.StatusForbidden, rendered)
		assert.Equal(t, http.StatusForbidden, rec.Code)
	})
	t.Run("a script under /api/ gets plain text", func(t *testing.T) {
		rendered = 0
		rec := httptest.NewRecorder()
		RenderErrorPage(rec, httptest.NewRequest(http.MethodGet, "/api/v2/x", nil), http.StatusForbidden, "no")
		assert.Zero(t, rendered)
		assert.Equal(t, "no", rec.Body.String())
		assert.Equal(t, "text/plain; charset=utf-8", rec.Header().Get("Content-Type"))
	})
}

func TestInitContext(t *testing.T) {
	var got TUMLiveContext
	var set bool
	h := InitContext(dao.DaoWrapper{})(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		got, set = GetTUMLiveContext(r)
	}))

	t.Run("a request without a session gets an empty context", func(t *testing.T) {
		set = false
		serve(h, http.MethodGet, "/settings")
		assert.True(t, set)
		assert.Nil(t, got.User)
	})
	t.Run("static assets get none", func(t *testing.T) {
		for _, target := range []string{"/public/a.png", "/favicon.ico"} {
			set = true
			serve(h, http.MethodGet, target)
			assert.False(t, set, target)
		}
	})
}
