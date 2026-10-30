package web

import (
	"io/fs"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/TUM-Dev/gocast/tools"
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

	// Vite's `base` and the router mount are configured independently; changing one
	// without the other leaves the shell asking for assets nothing serves.
	if !strings.Contains(string(shell), "/spa-assets/") {
		t.Errorf("SPA shell does not reference /spa-assets/; check `base` in frontend/vite.config.ts")
	}
}

func TestRegisterPageServesSPAForMigratedRoutes(t *testing.T) {
	spaIsBuilt(t)

	router := tools.NewRouter()
	registerPage(router, http.MethodGet, "/settings")

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/settings", nil))

	if rec.Code != http.StatusOK {
		t.Errorf("serving SPA shell: got status %d, want 200", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "/spa-assets/") {
		t.Errorf("SPA shell was not returned for /settings, got: %q", truncate(rec.Body.String()))
	}
}

// The consent page hands a one-time code to a third party, so it must not be framable.
func TestIntegrationAuthorizationPageDeniesFraming(t *testing.T) {
	spaIsBuilt(t)

	router := tools.NewRouter()
	registerPage(router, http.MethodGet, "/integration/authorize/:id")

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/integration/authorize/1", nil))

	if got := rec.Header().Get("X-Frame-Options"); got != "DENY" {
		t.Errorf("X-Frame-Options = %q, want DENY", got)
	}
	if got := rec.Header().Get("Content-Security-Policy"); got != "frame-ancestors 'none'" {
		t.Errorf("Content-Security-Policy = %q, want frame-ancestors 'none'", got)
	}
}

// The login page is SPA-served but still records where to return to, because an
// identity provider drops the original query string.
func TestLoginRouteKeepsSettingTheRedirectCookie(t *testing.T) {
	spaIsBuilt(t)

	if !spaRoutes["/login"] {
		t.Skip("/login is not migrated to the SPA")
	}

	router := tools.NewRouter()
	registerPage(router, http.MethodGet, "/login")

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/login?return=%2Fw%2Fslug%2F42", nil))

	if !strings.Contains(rec.Body.String(), "/spa-assets/") {
		t.Error("the SPA shell was not served for /login")
	}

	var redirect string
	for _, cookie := range rec.Result().Cookies() {
		if cookie.Name == redirCookieName {
			// Cookie values are escaped on write, so compare what tools.Cookie returns.
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

// A path the client router does not know would render nothing, so registering one that
// spaRoutes does not list stops the server at boot.
func TestPageHandlerRefusesAPathOutsideSpaRoutes(t *testing.T) {
	if _, err := pageHandler("/not-migrated"); err == nil {
		t.Fatal("expected an error, got none")
	}
	if _, err := pageHandler("/settings"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// A hook may answer the request instead of preparing for the shell: the start page's
// fresh-installation check redirects to the onboarding page and aborts. Appending the shell
// to that would send both.
func TestPageHandlerLetsAHookAnswerTheRequest(t *testing.T) {
	spaIsBuilt(t)

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

	spaRouteHooks[path] = func(w http.ResponseWriter, _ *http.Request) bool {
		tools.WriteText(w, http.StatusOK, "answered by the hook")
		return true
	}

	handler, err := pageHandler(path)
	if err != nil {
		t.Fatalf("pageHandler: %v", err)
	}

	router := tools.NewRouter()
	router.HandleFunc("GET "+path, handler)

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

	handler, err := pageHandler("/login")
	if err != nil {
		t.Fatalf("pageHandler: %v", err)
	}

	router := tools.NewRouter()
	router.HandleFunc("GET /login", handler)

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

// A refused request gets the shell with its status, and the message in a meta tag the
// client renders; under /api/ it is plain text, which scripts can read.
func TestRenderErrorPage(t *testing.T) {
	spaIsBuilt(t)

	tools.SetErrorPageRenderer(serveErrorPage)
	t.Cleanup(func() { tools.SetErrorPageRenderer(nil) })

	router := tools.NewRouter()
	router.NotFound(http.HandlerFunc(serveNotFound))
	router.HandleFunc("GET /forbidden", func(w http.ResponseWriter, r *http.Request) {
		tools.RenderErrorPage(w, r, http.StatusForbidden, `no <entry> "here"`)
	})

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/forbidden", nil))
	body := rec.Body.String()
	if rec.Code != http.StatusForbidden || !strings.Contains(body, "/spa-assets/") {
		t.Errorf("page error: status %d, body %q", rec.Code, truncate(body))
	}
	if !strings.Contains(body, `<meta name="gocast-error-status" content="403">`) ||
		!strings.Contains(body, `content="no &lt;entry&gt; &#34;here&#34;"`) {
		t.Errorf("error meta tags missing or unescaped in %q", truncate(body))
	}

	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/no/such/page", nil))
	if rec.Code != http.StatusNotFound || !strings.Contains(rec.Body.String(), "/spa-assets/") {
		t.Errorf("unknown page: status %d", rec.Code)
	}

	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/no/such/route", nil))
	if rec.Code != http.StatusNotFound || rec.Body.String() != tools.PageNotFoundErrMsg {
		t.Errorf("unknown API route: status %d, body %q", rec.Code, rec.Body.String())
	}
}

// An unknown watch view gets the shell with a 404 status rather than a 200 the client
// then contradicts; the four known views, in any case, and the chat pages still get 200.
func TestUnknownWatchViewAnswers404WithTheShell(t *testing.T) {
	spaIsBuilt(t)

	// serveNotFound renders through the error page renderer ConfigRouter installs.
	tools.SetErrorPageRenderer(serveErrorPage)
	t.Cleanup(func() { tools.SetErrorPageRenderer(nil) })
	group := tools.NewRouter()
	registerPage(group, http.MethodGet, "/w/:slug/:streamID/:version")
	registerPage(group, http.MethodGet, "/w/:slug/:streamID/chat")
	registerPage(group, http.MethodGet, "/w/:slug/:streamID/chat/popup")

	for path, want := range map[string]int{
		"/w/ws/1/COMB":       http.StatusOK,
		"/w/ws/1/PRES":       http.StatusOK,
		"/w/ws/1/CAM":        http.StatusOK,
		"/w/ws/1/SPLIT":      http.StatusOK,
		"/w/ws/1/pres":       http.StatusOK,
		"/w/ws/1/Split":      http.StatusOK,
		"/w/ws/1/chat":       http.StatusOK,
		"/w/ws/1/chat/popup": http.StatusOK,
		"/w/ws/1/NOPE":       http.StatusNotFound,
		"/w/ws/1/COMBINED":   http.StatusNotFound,
	} {
		rec := httptest.NewRecorder()
		group.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		if rec.Code != want {
			t.Errorf("%s: status %d, want %d", path, rec.Code, want)
		}
		if !strings.Contains(rec.Body.String(), "/spa-assets/") {
			t.Errorf("%s: the SPA shell was not served, got: %q", path, truncate(rec.Body.String()))
		}
	}
}
