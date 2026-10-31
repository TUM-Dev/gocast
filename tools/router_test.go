package tools

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// tag records that a middleware ran, in order, in a response header.
func tag(name string) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Add("X-Ran", name)
			next.ServeHTTP(w, r)
		})
	}
}

func serve(h http.Handler, method, target string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(method, target, nil))
	return rec
}

func ok(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) }

// Middleware reaches only the routes registered after it, outermost first, and a group
// starts from its parent's: the rules gin's groups had, which web/router.go relies on
// when it moves a page between permission groups.
func TestRouterMiddlewareScope(t *testing.T) {
	router := NewRouter()
	router.HandleFunc("GET /before", ok)
	router.Use(tag("global"))
	group := router.Group(tag("group"))
	router.HandleFunc("GET /plain", ok)
	group.HandleFunc("GET /grouped", ok)
	group.Use(tag("late"))
	group.HandleFunc("GET /later", ok)
	nested := group.Group(tag("nested"))
	nested.HandleFunc("GET /nested", ok)

	tests := []struct {
		path string
		want []string
	}{
		{"/before", nil},
		{"/plain", []string{"global"}},
		{"/grouped", []string{"global", "group"}},
		{"/later", []string{"global", "group", "late"}},
		{"/nested", []string{"global", "group", "late", "nested"}},
	}
	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			rec := serve(router, http.MethodGet, tt.path)
			assert.Equal(t, http.StatusOK, rec.Code)
			assert.Equal(t, tt.want, rec.Header().Values("X-Ran"))
		})
	}
}

func TestRouterNotFound(t *testing.T) {
	router := NewRouter()
	router.Use(tag("global"))
	router.HandleFunc("GET /{$}", ok)
	router.HandleFunc("GET /admin", ok)
	router.HandleFunc("POST /login", ok)
	router.HandleFunc("GET /w/{slug}/{id}", ok)
	router.NotFound(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))

	t.Run("the root pattern does not take every path", func(t *testing.T) {
		assert.Equal(t, http.StatusOK, serve(router, http.MethodGet, "/").Code)
		assert.Equal(t, http.StatusNotFound, serve(router, http.MethodGet, "/nowhere").Code)
	})
	t.Run("a path no route owns reaches the handler behind the router's middleware", func(t *testing.T) {
		rec := serve(router, http.MethodGet, "/nowhere/at/all")
		assert.Equal(t, http.StatusNotFound, rec.Code)
		assert.Equal(t, []string{"global"}, rec.Header().Values("X-Ran"))
	})
	t.Run("a method no route answers is not found, not 405", func(t *testing.T) {
		assert.Equal(t, http.StatusNotFound, serve(router, http.MethodPost, "/admin").Code)
	})
	t.Run("a trailing slash redirects to the route, with its query", func(t *testing.T) {
		rec := serve(router, http.MethodGet, "/admin/?tab=1")
		assert.Equal(t, http.StatusMovedPermanently, rec.Code)
		assert.Equal(t, "/admin?tab=1", rec.Header().Get("Location"))

		rec = serve(router, http.MethodGet, "/w/eidi/7/")
		assert.Equal(t, http.StatusMovedPermanently, rec.Code)
		assert.Equal(t, "/w/eidi/7", rec.Header().Get("Location"))
	})
	t.Run("a form post is redirected with a method-preserving status", func(t *testing.T) {
		rec := serve(router, http.MethodPost, "/login/")
		assert.Equal(t, http.StatusTemporaryRedirect, rec.Code)
		assert.Equal(t, "/login", rec.Header().Get("Location"))
	})
	t.Run("a trailing slash with no route behind it is not found", func(t *testing.T) {
		assert.Equal(t, http.StatusNotFound, serve(router, http.MethodGet, "/nowhere/").Code)
		// The route exists for POST only: GET must not be sent to it.
		assert.Equal(t, http.StatusNotFound, serve(router, http.MethodGet, "/login/").Code)
	})
}

func TestRouterConflictingPatternsPanicAtRegistration(t *testing.T) {
	router := NewRouter()
	router.HandleFunc("GET /a/{x}/b", ok)
	assert.Panics(t, func() { router.HandleFunc("GET /a/b/{y}", ok) })
}

func TestRouterPathValuesReachMiddleware(t *testing.T) {
	router := NewRouter()
	var seen string
	router.Group(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			seen = r.PathValue("courseID")
			next.ServeHTTP(w, r)
		})
	}).HandleFunc("GET /courses/{courseID}", ok)

	serve(router, http.MethodGet, "/courses/42")
	assert.Equal(t, "42", seen)
	assert.False(t, strings.Contains(seen, "{"))
}
