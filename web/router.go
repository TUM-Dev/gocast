package web

import (
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

func ConfigGinRouter(router gin.IRoutes) {
	templ = template.Must(template.ParseFS(templateFS, "template/*.gohtml", "template/admin/*.gohtml"))
	configGinStaticRouter(router)
	configMainRoute(router)
	configCourseRoute(router)
	return
}

func configGinStaticRouter(router gin.IRoutes) {
	router.StaticFS("/static", http.FS(staticFS))
	router.GET("/favicon.ico", func(c *gin.Context) {
		c.FileFromFS("assets/favicon.ico", http.FS(staticFS))
	})
}

func configMainRoute(router gin.IRoutes) {
	router.GET("/about", AboutPage)
	router.GET("/admin", AdminPage)
	router.GET("/admin/create-course", CreateCoursePage)
	router.GET("/admin/course/:id", EditCoursePage)
	router.GET("/admin/units/:streamID", LectureUnitsPage)
	router.GET("/admin/cut/:streamID", LectureCutPage)
	router.POST("/admin/course/:id", UpdateCourse)
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

func configCourseRoute(router gin.IRoutes) {
	router.GET("/course/:year/:teachingTerm/:slug", CoursePage)
}

func HealthCheck(context *gin.Context) {
	context.JSON(http.StatusOK, gin.H{"version": os.Getenv("hash")})
}

type ErrorPageData struct {
	IndexData IndexData
	Status    int
	Message   string
}
