package web

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/TUM-Dev/gocast/model"
	"github.com/TUM-Dev/gocast/tools"
	"github.com/TUM-Dev/gocast/tools/testutils"
)

// mainRouter is the real route table with a signed-out visitor, which is enough for
// every route that answers without the database.
func mainRouter(t *testing.T, ctx tools.TUMLiveContext) *tools.Router {
	t.Helper()
	router := tools.NewRouter()
	router.Use(testutils.InjectContext(ctx))
	// ServeMux panics when two patterns can match one request and neither is more
	// specific, so registering the whole table is itself a test: a page added next to
	// "/{shortLink}" or "/w/{slug}/{streamID}/{version}" shows up here, not at boot.
	require.NotPanics(t, func() { configMainRoute(router) })
	tools.SetErrorPageRenderer(nil)
	return router
}

func get(router http.Handler, method, target string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(method, target, nil))
	return rec
}

func TestRouteTableRedirects(t *testing.T) {
	router := mainRouter(t, tools.TUMLiveContext{})

	// Renamed URLs people have bookmarked, answered the same for anyone: the
	// destination does the checking.
	for from, want := range map[string]string{
		"/admin/lectureHalls":     "/admin/lecture-halls",
		"/admin/lectureHalls/new": "/admin/lecture-halls/new",
		"/admin/infopages":        "/admin/info-pages",
		"/admin/course/12":        "/admin/courses/12/lectures",
		"/admin/course/12/stats":  "/admin/courses/12/stats",
		"/admin/stats/12/34":      "/admin/courses/12/lectures/34/stats",
	} {
		rec := get(router, http.MethodGet, from)
		assert.Equal(t, http.StatusMovedPermanently, rec.Code, from)
		assert.Equal(t, want, rec.Header().Get("Location"), from)
	}

	for from, want := range map[string]string{
		"/semester/2024/W":    "/?year=2024&term=W",
		"/logout":             "/",
		"/cgi-bin/streams/x":  "/",
		"/cgi-bin/streams/a/": "/",
	} {
		rec := get(router, http.MethodGet, from)
		assert.Equal(t, http.StatusFound, rec.Code, from)
		assert.Equal(t, want, rec.Header().Get("Location"), from)
	}
}

// The SPA shell checks nothing itself, so each admin page sits behind the permission
// its group names. A visitor without it gets the refusal, never the shell.
func TestRouteTablePermissionGroups(t *testing.T) {
	signedOut := mainRouter(t, tools.TUMLiveContext{})
	student := mainRouter(t, tools.TUMLiveContext{User: &model.User{Role: model.StudentType}})

	for _, path := range []string{"/admin", "/admin/users", "/admin/runners", "/admin/token"} {
		assert.Equal(t, http.StatusForbidden, get(signedOut, http.MethodGet, path).Code, "signed out: "+path)
		assert.Equal(t, http.StatusForbidden, get(student, http.MethodGet, path).Code, "student: "+path)
	}

	// /settings is behind LoggedIn: an anonymous visitor goes to the login page.
	rec := get(signedOut, http.MethodGet, "/settings?tab=x")
	assert.Equal(t, http.StatusFound, rec.Code)
	assert.Equal(t, "/login?return=%2Fsettings%3Ftab%3Dx", rec.Header().Get("Location"))
}

func TestRouteTableUnownedPaths(t *testing.T) {
	router := mainRouter(t, tools.TUMLiveContext{})

	t.Run("an unknown path is the 404 page", func(t *testing.T) {
		rec := get(router, http.MethodGet, "/no/such/page")
		assert.Equal(t, http.StatusNotFound, rec.Code)
		assert.Equal(t, tools.PageNotFoundErrMsg, rec.Body.String())
	})
	t.Run("a method no page answers is not found", func(t *testing.T) {
		assert.Equal(t, http.StatusNotFound, get(router, http.MethodPost, "/admin/users").Code)
	})
	t.Run("a trailing slash goes to the page", func(t *testing.T) {
		rec := get(router, http.MethodGet, "/privacy/")
		assert.Equal(t, http.StatusMovedPermanently, rec.Code)
		assert.Equal(t, "/privacy", rec.Header().Get("Location"))
	})
}

// Directories are not browsable: http.FileServer would list them.
func TestStaticFilesAreNotListed(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "sub"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "a.txt"), []byte("hello"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "sub", "b.txt"), []byte("nested"), 0o644))

	router := tools.NewRouter()
	router.Handle("GET /public/", serveFiles("/public", http.Dir(dir)))

	rec := get(router, http.MethodGet, "/public/a.txt")
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "hello", rec.Body.String())
	assert.Equal(t, http.StatusOK, get(router, http.MethodGet, "/public/sub/b.txt").Code)
	assert.Equal(t, http.StatusOK, get(router, http.MethodHead, "/public/a.txt").Code)

	for _, path := range []string{"/public/", "/public/sub", "/public/sub/", "/public/missing.txt"} {
		rec := get(router, http.MethodGet, path)
		assert.NotEqual(t, http.StatusOK, rec.Code, path)
		assert.NotContains(t, rec.Body.String(), "a.txt", path)
	}
}

func TestMuxPattern(t *testing.T) {
	for path, want := range map[string]string{
		"/":                              "/{$}",
		"/settings":                      "/settings",
		"/w/:slug/:streamID/:version":    "/w/{slug}/{streamID}/{version}",
		"/admin/courses/:courseID/stats": "/admin/courses/{courseID}/stats",
		"/setPassword/:resetKey":         "/setPassword/{resetKey}",
		"/integration/authorize/:id":     "/integration/authorize/{id}",
		"/course/:year/:term/:slug":      "/course/{year}/{term}/{slug}",
	} {
		assert.Equal(t, want, muxPattern(path), path)
	}
}

// Every path spaRoutes lists must be registrable, so a typo or an overlap fails here
// and not when the server starts.
func TestEverySPARouteRegisters(t *testing.T) {
	router := tools.NewRouter()
	for path := range spaRoutes {
		assert.NotPanics(t, func() { registerPage(router, http.MethodGet, path) }, path)
	}
}
