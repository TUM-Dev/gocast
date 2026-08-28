package web

import (
	"io/fs"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

// The SPA is wired across two places that must agree: the embed and asset mount here,
// and the client router in frontend/src/router/index.ts. These cover the Go half.

func spaIsBuilt(t *testing.T) {
	t.Helper()
	if _, err := fs.Stat(spaFS, spaShellPath); err != nil {
		t.Skip("no SPA build present; run `npm run build` in frontend/")
	}
}

func TestSPAShellReferencesAssetPrefix(t *testing.T) {
	spaIsBuilt(t)

	shell, err := fs.ReadFile(spaFS, spaShellPath)
	if err != nil {
		t.Fatalf("reading SPA shell: %v", err)
	}

	// Vite's `base` and the gin mount are configured independently; changing one
	// without the other leaves the shell asking for assets nothing serves.
	if !strings.Contains(string(shell), "/spa-assets/") {
		t.Errorf("SPA shell does not reference /spa-assets/; check `base` in frontend/vite.config.ts")
	}
}

func TestRegisterPageServesSPAForMigratedRoutes(t *testing.T) {
	spaIsBuilt(t)

	gin.SetMode(gin.TestMode)
	router := gin.New()
	group := router.Group("/")

	legacyCalled := false
	legacy := func(c *gin.Context) {
		legacyCalled = true
		c.String(http.StatusOK, "legacy template")
	}

	// A synthetic path, so this does not need updating as real pages migrate.
	const legacyPath = "/not-migrated"
	if spaRoutes[legacyPath] {
		t.Fatalf("%s is unexpectedly listed in spaRoutes", legacyPath)
	}

	registerPage(group, http.MethodGet, "/settings", legacy)
	registerPage(group, http.MethodGet, legacyPath, legacy)

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/settings", nil))

	if legacyCalled {
		t.Error("/settings is listed in spaRoutes but was served by the template handler")
	}
	if rec.Code != http.StatusOK {
		t.Errorf("serving SPA shell: got status %d, want 200", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "/spa-assets/") {
		t.Errorf("SPA shell was not returned for /settings, got: %q", truncate(rec.Body.String()))
	}

	legacyCalled = false
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, legacyPath, nil))

	if !legacyCalled {
		t.Errorf("%s is not in spaRoutes but was not served by the template handler", legacyPath)
	}
}

// The login page is SPA-served but still records where to return to, because an
// identity provider drops the original query string.
func TestLoginRouteKeepsSettingTheRedirectCookie(t *testing.T) {
	spaIsBuilt(t)

	if !spaRoutes["/login"] {
		t.Skip("/login is not migrated to the SPA")
	}

	gin.SetMode(gin.TestMode)
	router := gin.New()
	registerPage(&router.RouterGroup, http.MethodGet, "/login", func(c *gin.Context) {
		t.Error("the template handler ran for a migrated route")
	})

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/login?return=%2Fw%2Fslug%2F42", nil))

	if !strings.Contains(rec.Body.String(), "/spa-assets/") {
		t.Error("the SPA shell was not served for /login")
	}

	var redirect string
	for _, cookie := range rec.Result().Cookies() {
		if cookie.Name == redirCookieName {
			// gin escapes cookie values on write, so compare what c.Cookie returns.
			decoded, err := url.QueryUnescape(cookie.Value)
			if err != nil {
				t.Fatalf("redirect cookie is not a valid escaped value: %v", err)
			}
			redirect = decoded
		}
	}
	if redirect != "/w/slug/42" {
		t.Errorf("redirect cookie = %q, want %q", redirect, "/w/slug/42")
	}
}

// Once a template is deleted the SPA build stops being optional for that page.
func TestPageHandlerRequiresABuildForPagesWithNoTemplate(t *testing.T) {
	gin.SetMode(gin.TestMode)

	const migrated = "/settings"
	const notMigrated = "/not-migrated"
	if !spaRoutes[migrated] {
		t.Fatalf("%s is expected to be listed in spaRoutes", migrated)
	}
	if spaRoutes[notMigrated] {
		t.Fatalf("%s is unexpectedly listed in spaRoutes", notMigrated)
	}

	legacy := func(c *gin.Context) {}

	tests := []struct {
		name     string
		path     string
		legacy   gin.HandlerFunc
		spaBuilt bool
		wantErr  bool
	}{
		{
			name:     "a fully migrated page without a build stops the server",
			path:     migrated,
			legacy:   nil,
			spaBuilt: false,
			wantErr:  true,
		},
		{
			name:     "a page still holding its template falls back instead",
			path:     migrated,
			legacy:   legacy,
			spaBuilt: false,
		},
		{
			// The rollback that no longer works: nothing is left to serve the path.
			name:     "a rolled-back page with no template left stops the server",
			path:     notMigrated,
			legacy:   nil,
			spaBuilt: true,
			wantErr:  true,
		},
		{
			name:     "a migrated page with a build is served the shell",
			path:     migrated,
			legacy:   nil,
			spaBuilt: true,
		},
		{
			name:     "an unmigrated page is served its template",
			path:     notMigrated,
			legacy:   legacy,
			spaBuilt: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler, err := pageHandler(tt.path, tt.legacy, tt.spaBuilt)

			if tt.wantErr {
				if err == nil {
					t.Fatal("expected an error, got none")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if handler == nil {
				t.Error("no handler returned")
			}
		})
	}
}

// A hook may answer the request instead of preparing for the shell: the start page's
// fresh-installation check renders the onboarding page and aborts. Appending the shell
// to that would send both.
func TestPageHandlerLetsAHookAnswerTheRequest(t *testing.T) {
	spaIsBuilt(t)

	gin.SetMode(gin.TestMode)

	const path = "/settings"
	if !spaRoutes[path] {
		t.Fatalf("%s is expected to be listed in spaRoutes", path)
	}

	original, had := spaRouteHooks[path]
	t.Cleanup(func() {
		if had {
			spaRouteHooks[path] = original
		} else {
			delete(spaRouteHooks, path)
		}
	})

	spaRouteHooks[path] = func(c *gin.Context) {
		c.String(http.StatusOK, "answered by the hook")
		c.Abort()
	}

	handler, err := pageHandler(path, nil, true)
	if err != nil {
		t.Fatalf("pageHandler: %v", err)
	}

	router := gin.New()
	router.GET(path, handler)

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))

	if body := rec.Body.String(); strings.Contains(body, "/spa-assets/") {
		t.Errorf("the shell was written after an aborting hook, got: %q", truncate(body))
	}
	if !strings.Contains(rec.Body.String(), "answered by the hook") {
		t.Errorf("the hook's response was not sent, got: %q", truncate(rec.Body.String()))
	}
}

// A hook that only prepares — the login page's redirect cookie — must still let the
// shell through.
func TestPageHandlerServesTheShellAfterANonAbortingHook(t *testing.T) {
	spaIsBuilt(t)

	gin.SetMode(gin.TestMode)

	handler, err := pageHandler("/login", nil, true)
	if err != nil {
		t.Fatalf("pageHandler: %v", err)
	}

	router := gin.New()
	router.GET("/login", handler)

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/login", nil))

	if !strings.Contains(rec.Body.String(), "/spa-assets/") {
		t.Errorf("the shell was not served, got: %q", truncate(rec.Body.String()))
	}
}

func TestSPAAssetsAreEmbedded(t *testing.T) {
	spaIsBuilt(t)

	assets, err := fs.Sub(spaFS, "spa")
	if err != nil {
		t.Fatalf("sub FS: %v", err)
	}

	entries, err := fs.ReadDir(assets, "assets")
	if err != nil {
		t.Fatalf("reading embedded assets: %v", err)
	}
	if len(entries) == 0 {
		t.Error("no hashed assets were embedded alongside the SPA shell")
	}
}

func truncate(s string) string {
	if len(s) > 120 {
		return s[:120] + "…"
	}
	return s
}
