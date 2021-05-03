package web

import (
	"TUM-Live/middleware"
	"embed"
	"github.com/gin-gonic/gin"
	"html/template"
	"net/http"
	"os"
)

var templ *template.Template

//go:embed template
var templateFS embed.FS

//go:embed assets/*
//go:embed node_modules
var staticFS embed.FS

func ConfigGinRouter(router *gin.Engine) {
	templ = template.Must(template.ParseFS(templateFS, "template/*.gohtml", "template/admin/*.gohtml"))
	configGinStaticRouter(router)
	configMainRoute(router)
	return
}

func configGinStaticRouter(router gin.IRoutes) {
	router.StaticFS("/static", http.FS(staticFS))
	router.GET("/favicon.ico", func(c *gin.Context) {
		c.FileFromFS("assets/favicon.ico", http.FS(staticFS))
	})
}

func configMainRoute(router *gin.Engine) {
	courseGroup := router.Group("/:year/:teachingTerm/:slug")
	courseGroup.Use(middleware.RequireAtLeastViewer())
	courseGroup.GET("/", CoursePage)
	courseGroup.GET(":id/*version", WatchPage)

	router.GET("/about", AboutPage)

	adminLecturerGroup := router.Group("/admin")
	adminLecturerGroup.Use(middleware.RequireAtLeastLecturer())
	adminLecturerGroup.GET("/", AdminPage)
	adminLecturerGroup.GET("/create-course", CreateCoursePage)
	adminLecturerGroup.GET("/course/:id", EditCoursePage)
	adminLecturerGroup.GET("/units/:streamID", LectureUnitsPage)
	adminLecturerGroup.GET("/cut/:streamID", LectureCutPage)
	adminLecturerGroup.POST("/course/:id", UpdateCourse)

	router.POST("/login", LoginHandler)
	router.GET("/login", LoginPage)
	router.GET("/logout", LogoutPage)
	router.GET("/setPassword/:key", CreatePasswordPage)
	router.POST("/setPassword/:key", CreatePasswordPage)
	router.GET("/w/:slug/:id", WatchPage)
	router.GET("/w/:slug/:id/:version", WatchPage)
	router.GET("/", MainPage)
	router.GET("/semester/:year/:term", MainPage)
	router.GET("/healthcheck", HealthCheck)
}

func HealthCheck(context *gin.Context) {
	context.JSON(http.StatusOK, gin.H{"version": os.Getenv("hash")})
}

type ErrorPageData struct {
	IndexData IndexData
	Status    int
	Message   string
}
