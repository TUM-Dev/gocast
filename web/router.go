package web

import (
	"embed"
	"fmt"
	"html/template"
	"io/fs"
	"net/http"
	"os"
	"path"

	"github.com/Masterminds/sprig/v3"
	"github.com/gin-gonic/gin"

	"github.com/TUM-Dev/gocast/dao"
	"github.com/TUM-Dev/gocast/tools"
)

var templateExecutor tools.TemplateExecutor

//go:embed template
var templateFS embed.FS

//go:embed assets/*
//go:embed node_modules
var staticFS embed.FS

// spaFS holds the built single-page app (`npm run build` writes it to web/spa). Only
// the directory is tracked, so a checkout without a build still compiles.
//
//go:embed all:spa
var spaFS embed.FS

// spaShellPath is the SPA's entry document. Every SPA-owned route serves it; the
// client router decides what to render.
const spaShellPath = "spa/index.html"

// spaRoutes lists the paths served by the SPA instead of a template. Adding a path
// moves one page across; it must also exist in frontend/src/router/index.ts or the
// shell loads and renders nothing.
//
// Removing a path moves the page back, but only while its template handler is still
// registered — see registerPage.
var spaRoutes = map[string]bool{
	"/settings": true,
	"/login":    true,
}

// spaRouteHooks holds work a route must still do server-side, run before the shell is
// written. For things the client cannot do, such as cookies — not data fetching.
var spaRouteHooks = map[string]gin.HandlerFunc{
	// Stored server-side because an external identity provider takes the browser
	// off-site and brings it back without the original query string.
	"/login": SetLoginRedirectCookie,
}

var templatePaths = []string{
	"template/*.gohtml",
	"template/components/*.gohtml",
	"template/admin/*.gohtml",
	"template/admin/admin_tabs/*.gohtml",
	"template/partial/*.gohtml",
	"template/partial/stream/*.gohtml",
	"template/partial/course/manage/*.gohtml",
	"template/partial/course/manage/*.gohtml",
	"template/partial/course/manage/create-lecture-form-slides/*.gohtml",
}

func ConfigGinRouter(router *gin.Engine) {
	if VersionTag != "development" {
		templateExecutor = tools.ReleaseTemplateExecutor{
			Template: template.Must(template.New("base").Funcs(sprig.FuncMap()).ParseFS(templateFS, templatePaths...)),
		}
	} else {
		prefixedTemplatePaths := make([]string, len(templatePaths))
		for i, v := range templatePaths {
			prefixedTemplatePaths[i] = "web/" + v
		}
		templateExecutor = tools.DebugTemplateExecutor{
			Patterns: prefixedTemplatePaths,
		}
	}
	tools.SetTemplateExecutor(templateExecutor)

	configGinStaticRouter(router)
	configSPARouter(router)
	configSaml(router, dao.NewDaoWrapper())
	configMainRoute(router)
}

// spaAvailable reports whether a built SPA is present. Without one, pages fall back
// to their template handlers, keeping the Go and frontend builds independent.
func spaAvailable() bool {
	if VersionTag == "development" {
		_, err := os.Stat("web/" + spaShellPath)
		return err == nil
	}

	_, err := fs.Stat(spaFS, spaShellPath)
	return err == nil
}

// configSPARouter mounts the SPA's hashed assets under their own prefix, leaving
// /static untouched.
func configSPARouter(router *gin.Engine) {
	if !spaAvailable() {
		logger.Info("no SPA build found, not mounting its assets")
		return
	}

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

// registerPage registers a page route, serving the SPA where it has taken the page
// over and the given template handler everywhere else.
//
// legacy may be nil for a page whose server-rendered version has been deleted. Such a
// page has no fallback, so a missing frontend build is a broken deployment rather than
// something to work around: the panic below stops the server at boot instead of
// letting it serve 404s on a page that used to work.
func registerPage(group *gin.RouterGroup, method, path string, legacy gin.HandlerFunc) {
	handler, err := pageHandler(path, legacy, spaAvailable())
	if err != nil {
		panic(err)
	}

	group.Handle(method, path, handler)
}

// pageHandler decides which of the two frontends answers a path. Split out from
// registerPage so the decision can be tested without a build present.
func pageHandler(path string, legacy gin.HandlerFunc, spaBuilt bool) (gin.HandlerFunc, error) {
	if !spaRoutes[path] {
		if legacy == nil {
			// Either the path was removed from spaRoutes to roll a page back after its
			// template was deleted, or it was never added in the first place.
			return nil, fmt.Errorf("page %q has no template handler and is not in spaRoutes", path)
		}
		return legacy, nil
	}

	if !spaBuilt {
		if legacy == nil {
			return nil, fmt.Errorf("page %q is served by the SPA and has no template handler, but no SPA build is present: run `npm run build` in frontend/ (or `make spa`)", path)
		}
		// A developer who has not built the frontend still gets a working server.
		logger.Info("no SPA build found, serving page from its template", "path", path)
		return legacy, nil
	}

	hook, hooked := spaRouteHooks[path]
	if !hooked {
		return serveSPAShell, nil
	}

	return func(c *gin.Context) {
		hook(c)
		serveSPAShell(c)
	}, nil
}

func configGinStaticRouter(router *gin.Engine) {
	router.Static("/public", tools.Cfg.Paths.Static)

	if VersionTag != "development" {
		router.StaticFS("/static", http.FS(staticFS))
	} else {
		router.Static("/static", "web/")
	}

	defaults := getDefaultStaticBrandingFiles()
	for _, file := range defaults {
		router.GET("/"+file.Name, getFileHandler(file))
	}

	router.GET("/service-worker.js", func(c *gin.Context) {
		c.FileFromFS("assets/service-worker.js", http.FS(staticFS))
	})
}

func newStartPage(router *gin.Engine, routes *mainRoutes) {
	router.GET("/", routes.home)
	router.GET("/semester/:year/:term", routes.semesterRedirect)
	router.GET("/course/:year/:term/:slug", routes.courseRedirect)
}

func oldStartPage(router *gin.Engine, routes *mainRoutes) {
	old := router.Group("/old")
	{
		old.GET("/", routes.MainPage)
		old.GET("/semester/:year/:term", routes.MainPage)

		course := old.Group("/course")
		course.Use(tools.InitCourse(routes.DaoWrapper))
		course.GET("/:year/:teachingTerm/:slug", routes.CoursePage)
	}
}

func configMainRoute(router *gin.Engine) {
	daoWrapper := dao.NewDaoWrapper()
	routes := mainRoutes{daoWrapper}
	streamGroup := router.Group("/")

	// lecturers
	atLeastLecturerGroup := router.Group("/")
	atLeastLecturerGroup.Use(tools.AtLeastLecturer)
	atLeastLecturerGroup.GET("/admin", routes.AdminPage)
	atLeastLecturerGroup.GET("/admin/create-course", routes.AdminPage)

	// info-pages (Make sure the IDs are correct!)
	router.GET("/privacy", routes.InfoPage(1, "privacy"))
	router.GET("/imprint", routes.InfoPage(2, "imprint"))
	router.GET("/about", routes.InfoPage(3, "about"))

	// search
	router.GET("/search", routes.SearchPage)

	// admins
	adminGroup := router.Group("/")
	adminGroup.GET("/admin/users", routes.AdminPage)
	adminGroup.GET("/admin/lectureHalls", routes.AdminPage)
	adminGroup.GET("/admin/lectureHalls/new", routes.AdminPage)
	adminGroup.GET("/admin/workers", routes.AdminPage)
	adminGroup.GET("/admin/runners", routes.AdminPage)
	adminGroup.GET("/admin/server-notifications", routes.AdminPage)
	adminGroup.GET("/admin/server-stats", routes.AdminPage)
	adminGroup.GET("/admin/course-import", routes.AdminPage)
	adminGroup.GET("/admin/token", routes.AdminPage)
	adminGroup.GET("/admin/infopages", routes.AdminPage)
	adminGroup.GET("/admin/notifications", routes.AdminPage)
	adminGroup.GET("/admin/audits", routes.AdminPage)
	adminGroup.GET("/admin/maintenance", routes.AdminPage)

	courseAdminGroup := router.Group("/")
	courseAdminGroup.Use(tools.InitCourse(daoWrapper))
	courseAdminGroup.Use(tools.AdminOfCourse)
	courseAdminGroup.GET("/admin/course/:courseID", routes.EditCoursePage)
	courseAdminGroup.GET("/admin/course/:courseID/stats", routes.CourseStatsPage)
	courseAdminGroup.POST("/admin/course/:courseID", routes.UpdateCourse)

	withStream := courseAdminGroup.Group("/")
	withStream.Use(tools.InitStream(daoWrapper))
	withStream.GET("/admin/units/:courseID/:streamID", routes.LectureUnitsPage)
	withStream.GET("/admin/cut/:courseID/:streamID", routes.LectureCutPage)
	withStream.GET("/admin/stats/:courseID/:streamID", routes.LectureStatsPage)
	withStream.GET("/admin/management/:courseID/:streamID", routes.LectureLiveManagementPage)

	// login/logout/password-mgmt
	router.POST("/login", routes.LoginHandler)
	registerPage(&router.RouterGroup, http.MethodGet, "/login", nil)
	router.GET("/logout", routes.LogoutPage)
	router.GET("/setPassword/:key", routes.CreatePasswordPage)
	router.POST("/setPassword/:key", routes.CreatePasswordPage)

	// home & course pages
	oldStartPage(router, &routes)
	newStartPage(router, &routes)

	// watch
	streamGroup.Use(tools.InitStream(daoWrapper))
	streamGroup.GET("/w/:slug/:streamID", routes.WatchPage)
	streamGroup.GET("/w/:slug/:streamID/:version", routes.WatchPage)
	streamGroup.GET("/w/:slug/:streamID/chat/popup", routes.PopOutChat)

	// misc
	router.GET("/healthcheck", routes.HealthCheck)
	router.GET("/jwtPubKey", routes.JWTPubKey)

	router.GET("/:shortLink", routes.HighlightPage)
	router.GET("/edit-course", routes.editCourseByTokenPage)
	router.GET("/edit-course/opt-out", routes.optOutPage)

	loggedIn := router.Group("/")
	loggedIn.Use(tools.LoggedIn)
	// The auth middleware stays on the route even when the SPA serves it, so an
	// anonymous visitor is redirected to /login by the server rather than after the
	// shell has loaded and failed a request.
	registerPage(loggedIn, http.MethodGet, "/settings", nil)

	// redirect from old site:
	router.GET("/cgi-bin/streams/*x", func(c *gin.Context) {
		c.Redirect(http.StatusFound, "/")
	})

	router.NoRoute(func(c *gin.Context) {
		tools.RenderErrorPage(c, http.StatusNotFound, tools.PageNotFoundErrMsg)
	})
}

type mainRoutes struct {
	dao.DaoWrapper
}

func (r mainRoutes) home(c *gin.Context) {
	isFresh, err := IsFreshInstallation(c, r.UsersDao)
	if err != nil {
		_ = templateExecutor.ExecuteTemplate(c.Writer, "error.gohtml", nil)
		return
	}
	if isFresh {
		if err := templateExecutor.ExecuteTemplate(c.Writer, "onboarding.gohtml", NewIndexData()); err != nil {
			logger.Error("Could not execute template: 'onboarding.gohtml'", "err", err)
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "Failed to load page"})
		}
		return
	}

	indexData := NewIndexDataWithContext(c)

	if err := templateExecutor.ExecuteTemplate(c.Writer, "home.gohtml", indexData); err != nil {
		logger.Error("Could not execute template: 'home.gohtml'", "err", err)
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "Failed to load page"})
	}
}

func (r mainRoutes) SearchPage(c *gin.Context) {
	indexData := NewIndexDataWithContext(c)
	if err := templateExecutor.ExecuteTemplate(c.Writer, "search-page.gohtml", indexData); err != nil {
		logger.Error("Could not execute template: 'search-page.gohtml'", "err", err)
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "Failed to load page"})
	}
}

func (r mainRoutes) semesterRedirect(c *gin.Context) {
	c.Redirect(http.StatusFound,
		fmt.Sprintf("/?year=%s&term=%s", c.Param("year"), c.Param("term")))
}

func (r mainRoutes) courseRedirect(c *gin.Context) {
	c.Redirect(http.StatusFound,
		fmt.Sprintf("/?year=%s&term=%s&slug=%s&view=3",
			c.Param("year"), c.Param("term"), c.Param("slug")))
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

type ChatData struct {
	IsAdminOfCourse bool // is current user admin or lecturer who created the course associated with the chat
	IndexData       IndexData
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
