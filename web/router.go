package web

import (
	"embed"
	"github.com/Masterminds/sprig/v3"
	"github.com/gin-gonic/gin"
	"github.com/joschahenningsen/TUM-Live/dao"
	"github.com/joschahenningsen/TUM-Live/tools"
	"html/template"
	"net/http"
)

var templateExecutor tools.TemplateExecutor

//go:embed template
var templateFS embed.FS

//go:embed assets/*
//go:embed node_modules
var staticFS embed.FS

var templatePaths = []string{
	"template/*.gohtml",
	"template/admin/*.gohtml",
	"template/admin/admin_tabs/*.gohtml",
	"template/partial/*.gohtml",
	"template/partial/stream/*.gohtml",
	"template/partial/course/manage/*.gohtml",
	"template/partial/admin/*.gohtml",
	"template/partial/stream/chat/*.gohtml",
	"template/partial/course/manage/*.gohtml",
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
	configCourseRoute(router)
}

func configGinStaticRouter(router gin.IRoutes) {
	router.Static("/public", tools.Cfg.Paths.Static)
	if VersionTag != "development" {
		router.StaticFS("/static", http.FS(staticFS))
		router.GET("/favicon.ico", func(c *gin.Context) {
			c.FileFromFS("assets/favicon.ico", http.FS(staticFS))
		})
	} else {
		router.Static("/static", "web/")
		router.GET("/favicon.ico", func(c *gin.Context) {
			c.File("web/assets/favicon.ico")
		})
	}
}

//todo: un-export functions
func configMainRoute(router *gin.Engine) {
	daoWrapper := dao.NewDaoWrapper()
	routes := mainRoutes{daoWrapper}
	streamGroup := router.Group("/")

	atLeastLecturerGroup := router.Group("/")
	atLeastLecturerGroup.Use(tools.AtLeastLecturer)
	atLeastLecturerGroup.GET("/admin", routes.AdminPage)
	atLeastLecturerGroup.GET("/admin/create-course", routes.AdminPage)
	router.GET("/about", routes.AboutPage)
	router.GET("/privacy", routes.PrivacyPage)
	router.GET("/imprint", routes.ImprintPage)

	adminGroup := router.Group("/")
	adminGroup.GET("/admin/users", routes.AdminPage)
	adminGroup.GET("/admin/lectureHalls", routes.AdminPage)
	adminGroup.GET("/admin/lectureHalls/new", routes.AdminPage)
	adminGroup.GET("/admin/workers", routes.AdminPage)
	adminGroup.GET("/admin/server-notifications", routes.AdminPage)
	adminGroup.GET("/admin/server-stats", routes.AdminPage)
	adminGroup.GET("/admin/course-import", routes.AdminPage)
	adminGroup.GET("/admin/token", routes.AdminPage)
	adminGroup.GET("/admin/texts", routes.AdminPage)
	adminGroup.GET("/admin/notifications", routes.AdminPage)

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

	router.POST("/login", routes.LoginHandler)
	router.GET("/login", routes.LoginPage)
	router.GET("/logout", routes.LogoutPage)
	router.GET("/setPassword/:key", routes.CreatePasswordPage)
	router.POST("/setPassword/:key", routes.CreatePasswordPage)
	streamGroup.Use(tools.InitStream(daoWrapper))
	streamGroup.GET("/w/:slug/:streamID", routes.WatchPage)
	streamGroup.GET("/w/:slug/:streamID/:version", routes.WatchPage)
	streamGroup.GET("/w/:slug/:streamID/chat/popup", routes.PopUpChat)
	router.GET("/", routes.MainPage)
	router.GET("/semester/:year/:term", routes.MainPage)
	router.GET("/healthcheck", routes.HealthCheck)

	router.GET("/:shortLink", routes.HighlightPage)
	router.GET("/edit-course", routes.editCourseByTokenPage)

	// redirect from old site:
	router.GET("/cgi-bin/streams/*x", func(c *gin.Context) {
		c.Redirect(http.StatusFound, "/")
	})
	router.NoRoute(func(c *gin.Context) {
		tools.RenderErrorPage(c, http.StatusNotFound, tools.PageNotFoundErrMsg)
	})

	loggedIn := router.Group("/")
	loggedIn.Use(tools.LoggedIn)
	loggedIn.GET("/settings", routes.settingsPage)
}

type mainRoutes struct {
	dao.DaoWrapper
}

func configCourseRoute(router *gin.Engine) {
	daoWrapper := dao.NewDaoWrapper()
	routes := mainRoutes{daoWrapper}
	g := router.Group("/course")
	g.Use(tools.InitCourse(daoWrapper))
	g.GET("/:year/:teachingTerm/:slug", routes.CoursePage)
}

func (r mainRoutes) HealthCheck(context *gin.Context) {
	resp := HealthCheckData{
		Version:      VersionTag,
		CacheMetrics: CacheMetrics{Hits: dao.Cache.Metrics.Hits(), Misses: dao.Cache.Metrics.Misses(), KeysAdded: dao.Cache.Metrics.KeysAdded()},
	}
	context.JSON(http.StatusOK, resp)
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
	IsPopUp         bool
}
