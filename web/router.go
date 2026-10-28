package web

import (
	"bytes"
	"embed"
	"fmt"
	"html"
	"io/fs"
	"net/http"
	"net/url"
	"os"
	"path"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/TUM-Dev/gocast/dao"
	"github.com/TUM-Dev/gocast/model"
	"github.com/TUM-Dev/gocast/tools"
)

// staticFS holds the default branding files, served at the root (see
// getDefaultStaticBrandingFiles). Nothing else lives in assets/: the frontend bundles
// its own scripts, styles and fonts.
//
//go:embed assets/favicon.ico assets/manifest.json assets/img
var staticFS embed.FS

// spaFS holds the built single-page app (`pnpm --dir frontend run build` writes it to
// web/spa). Only the directory is tracked, so a checkout without a build still
// compiles; ConfigGinRouter refuses to start without one.
//
//go:embed all:spa
var spaFS embed.FS

// spaShellPath is the SPA's entry document. Every SPA-owned route serves it; the
// client router decides what to render.
const spaShellPath = "spa/index.html"

// The URL grammar for /admin. gin panics when two parameters with different names
// share a position, so these shapes are effectively permanent once a sibling exists.
//
//  1. Kebab-case: /admin/lecture-halls. The old spellings redirect.
//  2. Plural collections: /admin/courses/:courseID.
//  3. Lectures nest: /admin/courses/:courseID/lectures/:streamID/{stats,live}.
//  4. In-page tabs are child routes, not component state.
//
// Rules 2 and 3 do not hold for the server-rendered pages yet; each is normalized as
// it migrates, gaining a redirect beside the two below.

// spaRoutes lists the paths served by the SPA. A path here must also exist in
// frontend/src/router/index.ts or the browser reloads forever; spa-routes.test.ts
// enforces that. registerPage refuses a path that is not listed.
var spaRoutes = map[string]bool{
	"/settings":                             true,
	"/login":                                true,
	"/search":                               true,
	"/setPassword/:resetKey":                true,
	"/edit-course":                          true,
	"/edit-course/opt-out":                  true,
	"/onboarding":                           true,
	"/":                                     true,
	"/courses/mine":                         true,
	"/courses/public":                       true,
	"/course/:year/:term/:slug":             true,
	"/admin/runners":                        true,
	"/admin/integrations":                   true,
	"/admin/users":                          true,
	"/admin/info-pages":                     true,
	"/admin/maintenance":                    true,
	"/admin/course-import":                  true,
	"/admin/token":                          true,
	"/privacy":                              true,
	"/imprint":                              true,
	"/about":                                true,
	"/admin/server-notifications":           true,
	"/admin/notifications":                  true,
	"/admin/audits":                         true,
	"/admin/server-stats":                   true,
	"/admin/lecture-halls":                  true,
	"/admin/lecture-halls/new":              true,
	"/admin/courses/:courseID/stats":        true,
	"/admin/courses/:courseID/settings":     true,
	"/admin/courses/:courseID/participants": true,
	"/admin/courses/:courseID/lectures":     true,
	"/admin/courses/:courseID/lectures/:streamID/stats": true,
	"/admin/create-course":                              true,
	"/admin":                                            true,
	"/w/:slug/:streamID":                                true,
	"/w/:slug/:streamID/:version":                       true,
	"/w/:slug/:streamID/chat":                           true,
	"/w/:slug/:streamID/chat/popup":                     true,
	"/admin/courses/:courseID/lectures/:streamID/live":  true,
	"/integration/authorize/:id":                        true,
}

// spaRouteHooks holds work a route must still do server-side, run before the shell is
// written. For things the client cannot do, such as cookies — not data fetching.
//
// A hook may also answer the request itself; see pageHandler. Hooks that need a
// database are registered in configMainRoute rather than here.
var spaRouteHooks = map[string]gin.HandlerFunc{
	// Stored server-side because an external identity provider takes the browser
	// off-site and brings it back without the original query string.
	"/login": SetLoginRedirectCookie,
	// Approving hands a one-time code to a third party, so the page must not be
	// clickable through a frame; see denyFraming.
	"/integration/authorize/:id": denyFraming,
	// The shell would answer 200 for a view the client then renders as not-found; the
	// status is what crawlers, link checkers and curl read.
	"/w/:slug/:streamID/:version": notFoundForUnknownWatchView,
}

// watchViews are the `:version` segments the watch page knows, the same four as
// LAYOUT_SEGMENTS in frontend/src/lib/watch.ts.
var watchViews = map[string]bool{"COMB": true, "PRES": true, "CAM": true, "SPLIT": true}

// notFoundForUnknownWatchView answers 404 (with the SPA shell, like any unowned path)
// for a view the watch page does not have. Matched case-insensitively because the
// client uppercases the segment before looking it up, so /w/x/1/pres plays.
func notFoundForUnknownWatchView(c *gin.Context) {
	if !watchViews[strings.ToUpper(c.Param("version"))] {
		serveNotFound(c)
	}
}

// denyFraming forbids embedding the response in another page, so a framing page cannot
// overlay the consent buttons and have the user click them unknowingly.
// X-Frame-Options is for browsers that predate CSP's frame-ancestors.
func denyFraming(c *gin.Context) {
	c.Header("X-Frame-Options", "DENY")
	c.Header("Content-Security-Policy", "frame-ancestors 'none'")
}

// ConfigGinRouter mounts every page. The SPA is the only frontend, so a missing build
// is a broken deployment: it stops the server here instead of answering every page
// with an error.
func ConfigGinRouter(router *gin.Engine) {
	if _, err := readSPAShell(); err != nil {
		panic(fmt.Errorf("no SPA build found (%w): run `make spa` (or `pnpm --dir frontend run build`) before building or starting the server", err))
	}
	tools.SetErrorPageRenderer(serveErrorPage)

	configGinStaticRouter(router)
	configSPARouter(router)
	configSaml(router, dao.NewDaoWrapper())
	configMainRoute(router)
}

// configSPARouter mounts the SPA's hashed assets under their own prefix.
func configSPARouter(router *gin.Engine) {
	if VersionTag != "development" {
		assets, err := fs.Sub(spaFS, "spa")
		if err != nil {
			logger.Error("can't mount SPA assets", "err", err)
			return
		}
		router.StaticFS("/spa-assets", http.FS(assets))
		return
	}

	router.Static("/spa-assets", "web/spa")
}

// readSPAShell returns the SPA's entry document, from disk in development so a
// rebuild is picked up without a restart.
func readSPAShell() ([]byte, error) {
	if VersionTag == "development" {
		return os.ReadFile("web/" + spaShellPath)
	}

	return fs.ReadFile(spaFS, spaShellPath)
}

// serveSPAShell responds with the SPA's entry document. Written directly rather than
// through http.FileServer, which would redirect /index.html to its directory.
func serveSPAShell(c *gin.Context) {
	shell, err := readSPAShell()
	if err != nil {
		logger.Error("can't read SPA shell", "err", err)
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}

	// The shell names content-hashed assets from its own build, so a cached copy
	// would ask a new binary for files it no longer has. Assets stay cacheable.
	c.Header("Cache-Control", "no-store")
	c.Data(http.StatusOK, "text/html; charset=utf-8", shell)
}

// registerPage registers a page route served by the SPA. A path missing from spaRoutes
// panics at boot: the client router would not know it, and the shell would render
// nothing.
func registerPage(group *gin.RouterGroup, method, path string) {
	handler, err := pageHandler(path)
	if err != nil {
		panic(err)
	}

	group.Handle(method, path, handler)
}

// pageHandler builds the handler for a page path. Split out from registerPage so the
// decision can be tested.
func pageHandler(path string) (gin.HandlerFunc, error) {
	if !spaRoutes[path] {
		return nil, fmt.Errorf("page %q is not in spaRoutes; add it there and to frontend/src/router/index.ts", path)
	}

	hook, hooked := spaRouteHooks[path]
	if !hooked {
		return serveSPAShell, nil
	}

	return func(c *gin.Context) {
		hook(c)
		// A hook may answer the request instead of preparing for the shell: the
		// fresh-installation check on "/" redirects to the onboarding page and aborts.
		// Writing the shell after that would append it to a finished response.
		if c.IsAborted() {
			return
		}
		serveSPAShell(c)
	}, nil
}

func configGinStaticRouter(router *gin.Engine) {
	router.Static("/public", tools.Cfg.Paths.Static)

	defaults := getDefaultStaticBrandingFiles()
	for _, file := range defaults {
		router.GET("/"+file.Name, getFileHandler(file))
	}

	// Browsers that visited before the server-rendered pages were removed still have
	// the service worker they registered, which caches those pages' bundles. Nothing
	// registers one any more, so this answers the update check with a worker that
	// removes itself and its caches. Do not delete the route: without it the old worker
	// would never be replaced.
	router.GET("/service-worker.js", func(c *gin.Context) {
		c.Header("Cache-Control", "no-store")
		c.Data(http.StatusOK, "text/javascript; charset=utf-8", []byte(retireServiceWorker))
	})
}

const retireServiceWorker = `self.addEventListener("install", () => self.skipWaiting());
self.addEventListener("activate", (event) => {
  event.waitUntil(
    caches.keys()
      .then((keys) => Promise.all(keys.map((key) => caches.delete(key))))
      .then(() => self.registration.unregister()),
  );
});
`

// newStartPage registers the four routes of the start page, all served by the SPA,
// plus the semester redirect that predates it.
func newStartPage(router *gin.Engine, routes *mainRoutes) {
	registerPage(&router.RouterGroup, http.MethodGet, "/")
	registerPage(&router.RouterGroup, http.MethodGet, "/courses/mine")
	registerPage(&router.RouterGroup, http.MethodGet, "/courses/public")
	// Kept as a path rather than a query parameter because it is the URL people have
	// bookmarked; the client router matches the same shape.
	registerPage(&router.RouterGroup, http.MethodGet, "/course/:year/:term/:slug")

	// Not a page of its own: it redirects into the semester query the start page uses.
	router.GET("/semester/:year/:term", routes.semesterRedirect)
}

// redirectTo permanently redirects a renamed path, for bookmarks and shared links.
func redirectTo(path string) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Redirect(http.StatusMovedPermanently, path)
	}
}

func configMainRoute(router *gin.Engine) {
	daoWrapper := dao.NewDaoWrapper()
	routes := mainRoutes{daoWrapper}
	// Registered here rather than in the package-level table, which holds only hooks
	// that need nothing but the request. Must precede the registerPage calls below.
	spaRouteHooks["/"] = routes.onboardingIfFresh
	streamGroup := router.Group("/")

	// lecturers
	atLeastLecturerGroup := router.Group("/")
	atLeastLecturerGroup.Use(tools.RequirePermission(model.PermLecture))
	registerPage(atLeastLecturerGroup, http.MethodGet, "/admin")
	registerPage(atLeastLecturerGroup, http.MethodGet, "/admin/create-course")

	// info-pages. Public, so no middleware; the rows are getInfoPage's business now.
	registerPage(&router.RouterGroup, http.MethodGet, "/privacy")
	registerPage(&router.RouterGroup, http.MethodGet, "/imprint")
	registerPage(&router.RouterGroup, http.MethodGet, "/about")

	// Public: anonymous callers search public courses; the page itself asks v2.
	registerPage(&router.RouterGroup, http.MethodGet, "/search")

	// admins
	//
	// The SPA shell checks nothing itself, so without these any student could load
	// every page. (The template these replaced only checked that someone was signed
	// in, and hid the links, which is not a guard.)
	//
	// Split by what each page administers. Both permissions belong to admins today;
	// the distinction is what makes an operator role a change to the role table.
	serverAdminGroup := router.Group("/")
	serverAdminGroup.Use(tools.RequirePermission(model.PermAdministerServer))
	registerPage(serverAdminGroup, http.MethodGet, "/admin/lecture-halls")
	registerPage(serverAdminGroup, http.MethodGet, "/admin/lecture-halls/new")
	registerPage(serverAdminGroup, http.MethodGet, "/admin/runners")
	registerPage(serverAdminGroup, http.MethodGet, "/admin/server-notifications")
	registerPage(serverAdminGroup, http.MethodGet, "/admin/course-import")
	registerPage(serverAdminGroup, http.MethodGet, "/admin/server-stats")
	registerPage(serverAdminGroup, http.MethodGet, "/admin/info-pages")
	registerPage(serverAdminGroup, http.MethodGet, "/admin/notifications")
	registerPage(serverAdminGroup, http.MethodGet, "/admin/integrations")
	registerPage(serverAdminGroup, http.MethodGet, "/admin/maintenance")
	registerPage(serverAdminGroup, http.MethodGet, "/admin/audits")

	// Accounts and their API tokens. dao.GetAllTokens already scopes its rows on the
	// same permission.
	userAdminGroup := router.Group("/")
	userAdminGroup.Use(tools.RequirePermission(model.PermManageUsers))
	registerPage(userAdminGroup, http.MethodGet, "/admin/users")
	registerPage(userAdminGroup, http.MethodGet, "/admin/token")

	// Outside the permission groups: a redirect reveals nothing, and the destination
	// does the checking.
	for from, to := range map[string]string{
		"/admin/lectureHalls":     "/admin/lecture-halls",
		"/admin/lectureHalls/new": "/admin/lecture-halls/new",
		"/admin/infopages":        "/admin/info-pages",
	} {
		router.GET(from, redirectTo(to))
	}

	courseAdminGroup := router.Group("/")
	courseAdminGroup.Use(tools.InitCourse(daoWrapper))
	courseAdminGroup.Use(tools.AdminOfCourse)
	// The first page on rule 2's plural path. The edit-course page's statistics tab
	// shows the same thing; that tab becomes this route when the page migrates.
	registerPage(courseAdminGroup, http.MethodGet, "/admin/courses/:courseID/stats")
	// The settings, external-participants and lectures tabs of the same page, as
	// rule 4 child routes.
	registerPage(courseAdminGroup, http.MethodGet, "/admin/courses/:courseID/settings")
	registerPage(courseAdminGroup, http.MethodGet, "/admin/courses/:courseID/participants")
	registerPage(courseAdminGroup, http.MethodGet, "/admin/courses/:courseID/lectures")

	// Outside the course group, like the redirects above: the destination checks.
	// The server-rendered course page opened on its lectures tab; a fragment such as
	// #lecture-li-7 never reaches the server, so it is lost here.
	router.GET("/admin/course/:courseID", func(c *gin.Context) {
		c.Redirect(http.StatusMovedPermanently, "/admin/courses/"+url.PathEscape(c.Param("courseID"))+"/lectures")
	})
	router.GET("/admin/course/:courseID/stats", func(c *gin.Context) {
		c.Redirect(http.StatusMovedPermanently, "/admin/courses/"+url.PathEscape(c.Param("courseID"))+"/stats")
	})
	// The lecture cut editor was dropped (it may return later), so old bookmarks and
	// emailed links go to the course's lectures tab. This must
	// stay behind courseAdminGroup's middleware so the redirect doesn't reveal anything
	// to people who couldn't open the page before. Stored cuts (StartOffset/EndOffset)
	// still apply on playback.
	courseAdminGroup.GET("/admin/cut/:courseID/:streamID", func(c *gin.Context) {
		c.Redirect(http.StatusMovedPermanently, "/admin/courses/"+url.PathEscape(c.Param("courseID"))+"/lectures")
	})
	// Lecture units were dropped too; both of the page's URLs redirect the same way,
	// behind the same middleware and for the same reason.
	for _, path := range []string{
		"/admin/units/:courseID/:streamID",
		"/admin/courses/:courseID/lectures/:streamID/units",
	} {
		courseAdminGroup.GET(path, func(c *gin.Context) {
			c.Redirect(http.StatusMovedPermanently, "/admin/courses/"+url.PathEscape(c.Param("courseID"))+"/lectures")
		})
	}
	router.GET("/admin/stats/:courseID/:streamID", func(c *gin.Context) {
		c.Redirect(http.StatusMovedPermanently, "/admin/courses/"+url.PathEscape(c.Param("courseID"))+
			"/lectures/"+url.PathEscape(c.Param("streamID"))+"/stats")
	})

	withStream := courseAdminGroup.Group("/")
	withStream.Use(tools.InitStream(daoWrapper))
	// Rule 3's nested path; the old one redirects below.
	registerPage(withStream, http.MethodGet, "/admin/courses/:courseID/lectures/:streamID/stats")
	registerPage(withStream, http.MethodGet, "/admin/courses/:courseID/lectures/:streamID/live")
	// The old management URL, which lecturers have bookmarked and the old watch page
	// linked to. Behind the same middleware as the page, so it reveals nothing the page
	// would not.
	withStream.GET("/admin/management/:courseID/:streamID", func(c *gin.Context) {
		c.Redirect(http.StatusMovedPermanently, "/admin/courses/"+url.PathEscape(c.Param("courseID"))+
			"/lectures/"+url.PathEscape(c.Param("streamID"))+"/live")
	})

	// login/logout/password-mgmt
	router.POST("/login", routes.LoginHandler)
	registerPage(&router.RouterGroup, http.MethodGet, "/login")
	// Public: the page itself finds out from getFrontendConfig whether it still applies.
	registerPage(&router.RouterGroup, http.MethodGet, "/onboarding")
	router.GET("/logout", routes.LogoutPage)
	// The key in the link is the credential; the page asks v2 whether it is still good.
	registerPage(&router.RouterGroup, http.MethodGet, "/setPassword/:resetKey")

	// home & course pages
	newStartPage(router, &routes)

	// watch
	streamGroup.Use(tools.InitStream(daoWrapper))
	// InitStream runs before the shell so a private stream answers with the login
	// redirect or 403 rather than an app that then fails to load. `?video_only=1`
	// (Artemis embeds it) is the same URL: the client decides.
	// `/:version` accepts anything at the router; notFoundForUnknownWatchView answers
	// 404 for all but COMB, PRES, CAM and SPLIT.
	registerPage(streamGroup, http.MethodGet, "/w/:slug/:streamID")
	registerPage(streamGroup, http.MethodGet, "/w/:slug/:streamID/:version")
	registerPage(streamGroup, http.MethodGet, "/w/:slug/:streamID/chat/popup")
	registerPage(streamGroup, http.MethodGet, "/w/:slug/:streamID/chat")

	// misc
	router.GET("/healthcheck", routes.HealthCheck)
	router.GET("/jwtPubKey", routes.JWTPubKey)

	router.GET("/:shortLink", routes.shortLinkOrInfoPage)
	// The course token in the mailed link is the credential; both pages ask v2 for it.
	registerPage(&router.RouterGroup, http.MethodGet, "/edit-course")
	registerPage(&router.RouterGroup, http.MethodGet, "/edit-course/opt-out")

	loggedIn := router.Group("/")
	loggedIn.Use(tools.LoggedIn)
	// The consent page; the decision is the v2 decideIntegrationAuthorization RPC.
	registerPage(loggedIn, http.MethodGet, "/integration/authorize/:id")
	// The auth middleware stays on the route even when the SPA serves it, so an
	// anonymous visitor is redirected to /login by the server rather than after the
	// shell has loaded and failed a request.
	registerPage(loggedIn, http.MethodGet, "/settings")

	// redirect from old site:
	router.GET("/cgi-bin/streams/*x", func(c *gin.Context) {
		c.Redirect(http.StatusFound, "/")
	})

	router.NoRoute(serveNotFound)
}

// serveNotFound is the 404 for a path nothing owns.
func serveNotFound(c *gin.Context) {
	tools.RenderErrorPage(c, http.StatusNotFound, tools.PageNotFoundErrMsg)
}

// serveErrorPage answers a page request with the SPA shell and the given status. The
// status and message ride along in meta tags, which the client reads to render the
// error instead of the route (frontend/src/lib/server-error.ts). The shell alone would
// render whatever page the URL names, for a request the server just refused.
func serveErrorPage(c *gin.Context, status int, message string) {
	shell, err := readSPAShell()
	if err != nil {
		logger.Error("can't read SPA shell", "err", err)
		c.String(status, message)
		return
	}
	meta := fmt.Sprintf(`<meta name="gocast-error-status" content="%d"><meta name="gocast-error-message" content="%s">`,
		status, html.EscapeString(message))
	shell = bytes.Replace(shell, []byte("</head>"), []byte(meta+"</head>"), 1)
	c.Header("Cache-Control", "no-store")
	c.Data(status, "text/html; charset=utf-8", shell)
}

type mainRoutes struct {
	dao.DaoWrapper
}

// onboardingIfFresh sends "/" to the onboarding page while the deployment has no
// users, where the first account is created, instead of a start page nobody can sign
// in to.
//
// This stays server-side because the shell would otherwise render the start page for
// a moment before finding out. The onboarding page itself asks GetFrontendConfig,
// which reports the same flag, so it can refuse once an account exists.
func (r mainRoutes) onboardingIfFresh(c *gin.Context) {
	isFresh, err := IsFreshInstallation(c, r.UsersDao)
	if err != nil {
		logger.Error("can't check for a fresh installation", "err", err)
		tools.RenderErrorPage(c, http.StatusInternalServerError, "Something went wrong. Please try again.")
		return
	}
	if !isFresh {
		return
	}

	c.Redirect(http.StatusFound, "/onboarding")
	c.Abort()
}

func (r mainRoutes) semesterRedirect(c *gin.Context) {
	c.Redirect(http.StatusFound,
		fmt.Sprintf("/?year=%s&term=%s", c.Param("year"), c.Param("term")))
}

func (r mainRoutes) HealthCheck(context *gin.Context) {
	resp := HealthCheckData{
		Version:      VersionTag,
		CacheMetrics: CacheMetrics{Hits: dao.Cache.Metrics.Hits(), Misses: dao.Cache.Metrics.Misses(), KeysAdded: dao.Cache.Metrics.KeysAdded()},
	}
	context.JSON(http.StatusOK, resp)
}

func (r mainRoutes) JWTPubKey(c *gin.Context) {
	c.JSON(http.StatusOK, tools.Cfg.GetJWTKey().PublicKey)
}

type HealthCheckData struct {
	Version      string       `json:"version"`
	CacheMetrics CacheMetrics `json:"cacheMetrics"`
}

type CacheMetrics struct {
	Hits      uint64 `json:"hits"`
	Misses    uint64 `json:"misses"`
	KeysAdded uint64 `json:"keysAdded"`
}

type staticFile struct {
	Name string
	Path string
}

func getDefaultStaticBrandingFiles() []staticFile {
	return []staticFile{
		{Name: "logo.svg", Path: "assets/img/logo.svg"},
		{Name: "manifest.json", Path: "assets/manifest.json"},
		{Name: "favicon.ico", Path: "assets/favicon.ico"},
		{Name: "icons-192.png", Path: "assets/img/icons-192.png"},
		{Name: "icons-512.png", Path: "assets/img/icons-512.png"},
		{Name: "thumb-fallback.png", Path: "assets/img/thumb-fallback.png"},
	}
}

func getFileHandler(file staticFile) gin.HandlerFunc {
	pathToFile := path.Join(tools.Cfg.Paths.Branding, file.Name)
	_, err := os.Stat(pathToFile)
	if tools.Cfg.Paths.Branding != "" && err == nil {
		// Use customized file without embedded FS
		return func(c *gin.Context) {
			c.File(pathToFile)
		}
	}
	// Use Default with embedded FS
	// p := file.Path // Copy bc. file is pointer
	return func(c *gin.Context) {
		c.FileFromFS(file.Path, http.FS(staticFS))
	}
}
