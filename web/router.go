package web

import (
	"embed"
	"fmt"
	"html/template"
	"net/http"
	"os"
	"path"

	"github.com/getsentry/sentry-go"

	"github.com/Masterminds/sprig/v3"
	"github.com/TUM-Dev/gocast/dao"
	"github.com/TUM-Dev/gocast/tools"
	"github.com/gin-gonic/gin"
)

var templateExecutor tools.TemplateExecutor

//go:embed template
var templateFS embed.FS

//go:embed assets/*
//go:embed node_modules
var staticFS embed.FS

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
	configSaml(router, dao.NewDaoWrapper())
	configMainRoute(router)
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

	// login/logout/password-mgmt
	router.POST("/login", routes.LoginHandler)
	router.GET("/login", routes.LoginPage)
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
	loggedIn.GET("/settings", routes.settingsPage)

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
	tName := sentry.WithTransactionSource("GET /")
	spanMain := sentry.StartSpan(c.Request.Context(), "HomePageHandler", tName)
	defer spanMain.Finish()

	isFresh, err := IsFreshInstallation(c, r.UsersDao)
	if err != nil {
		_ = templateExecutor.ExecuteTemplate(c.Writer, "error.gohtml", nil)
		return
	}
	if isFresh {
		_ = templateExecutor.ExecuteTemplate(c.Writer, "onboarding.gohtml", NewIndexData())
		return
	}

	indexData := NewIndexDataWithContext(c)

	if err := templateExecutor.ExecuteTemplate(c.Writer, "home.gohtml", indexData); err != nil {
		logger.Error("Could not execute template: 'home.gohtml'", "err", err)
	}
}

func (r mainRoutes) SearchPage(c *gin.Context) {
	indexData := NewIndexDataWithContext(c)
	if err := templateExecutor.ExecuteTemplate(c.Writer, "search-page.gohtml", indexData); err != nil {
		logger.Error("Could not execute template: 'search.gohtml'", "err", err)
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
	} else {
		// Use Default with embedded FS
		// p := file.Path // Copy bc. file is pointer
		return func(c *gin.Context) {
			c.FileFromFS(file.Path, http.FS(staticFS))
		}
	}
}
