package web

import (
	"TUM-Live/dao"
	"TUM-Live/tools"
	"embed"
	"github.com/gin-gonic/gin"
	"html/template"
	"net/http"
)

var templ *template.Template

//go:embed template
var templateFS embed.FS

//go:embed assets/*
//go:embed node_modules
var staticFS embed.FS

func ConfigGinRouter(router *gin.Engine) {
	templ = template.Must(template.ParseFS(templateFS,
		"template/*.gohtml",
		"template/admin/*.gohtml",
		"template/admin/admin_tabs/*.gohtml",
		"template/partial/*.gohtml",
		"template/partial/stream/*.gohtml",
		"template/partial/course/manage/*.gohtml",
		"template/partial/admin/*.gohtml",
		"template/partial/stream/chat/*.gohtml",
		"template/partial/course/manage/*.gohtml"))
	tools.SetTemplates(templ)
	configGinStaticRouter(router)
	configMainRoute(router)
	configCourseRoute(router)
}

func configGinStaticRouter(router gin.IRoutes) {
	router.Static("/public", tools.Cfg.Paths.Static)
	router.StaticFS("/static", http.FS(staticFS))
	router.GET("/favicon.ico", func(c *gin.Context) {
		c.FileFromFS("assets/favicon.ico", http.FS(staticFS))
	})
}

//todo: un-export functions
func configMainRoute(router *gin.Engine) {
	streamGroup := router.Group("/")

	atLeastLecturerGroup := router.Group("/")
	atLeastLecturerGroup.Use(tools.AtLeastLecturer)
	atLeastLecturerGroup.GET("/admin", AdminPage)
	atLeastLecturerGroup.GET("/admin/create-course", AdminPage)
	router.GET("/about", AboutPage)

	adminGroup := router.Group("/")
	adminGroup.GET("/admin/users", AdminPage)
	adminGroup.GET("/admin/lectureHalls", AdminPage)
	adminGroup.GET("/admin/lectureHalls/new", AdminPage)
	adminGroup.GET("/admin/workers", AdminPage)
	adminGroup.GET("/admin/tumlive-notifications", AdminPage)
	adminGroup.GET("/admin/tumlive-stats", AdminPage)
	adminGroup.GET("/admin/course-import", AdminPage)
	adminGroup.GET("/admin/token", AdminPage)
	adminGroup.GET("/admin/notifications", AdminPage)

	courseAdminGroup := router.Group("/")
	courseAdminGroup.Use(tools.InitCourse)
	courseAdminGroup.Use(tools.AdminOfCourse)
	courseAdminGroup.GET("/admin/course/:courseID", EditCoursePage)
	courseAdminGroup.GET("/admin/course/:courseID/stats", CourseStatsPage)
	courseAdminGroup.POST("/admin/course/:courseID", UpdateCourse)

	withStream := courseAdminGroup.Group("/")
	withStream.Use(tools.InitStream)
	withStream.GET("/admin/units/:courseID/:streamID", LectureUnitsPage)
	withStream.GET("/admin/cut/:courseID/:streamID", LectureCutPage)

	router.POST("/login", LoginHandler)
	router.GET("/login", LoginPage)
	router.GET("/logout", LogoutPage)
	router.GET("/setPassword/:key", CreatePasswordPage)
	router.POST("/setPassword/:key", CreatePasswordPage)
	streamGroup.Use(tools.InitStream)
	streamGroup.GET("/w/:slug/:streamID", WatchPage)
	streamGroup.GET("/w/:slug/:streamID/:version", WatchPage)
	streamGroup.GET("/w/:slug/:streamID/chat/popup", PopUpChat)
	router.GET("/", MainPage)
	router.GET("/semester/:year/:term", MainPage)
	router.GET("/healthcheck", HealthCheck)

	router.GET("/:shortLink", HighlightPage)
	router.GET("/edit-course", editCourseByTokenPage)

	// redirect from old site:
	router.GET("/cgi-bin/streams/*x", func(c *gin.Context) {
		c.Redirect(http.StatusFound, "/")
	})
	router.NoRoute(func(c *gin.Context) {
		tools.RenderErrorPage(c, http.StatusNotFound, tools.PageNotFoundErrMsg)
	})
}

func configCourseRoute(router *gin.Engine) {
	g := router.Group("/course")
	g.Use(tools.InitCourse)
	g.GET("/:year/:teachingTerm/:slug", CoursePage)
}

func HealthCheck(context *gin.Context) {
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
