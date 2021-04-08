package web

import (
	"github.com/gin-gonic/gin"
	"html/template"
	"net/http"
	"os"
)

var templ *template.Template

func ConfigGinRouter(router gin.IRoutes) {
	templ = template.Must(template.ParseGlob("./web/template/*.gohtml"))
	template.Must(templ.ParseGlob("./web/template/admin/*"))
	configGinStaticRouter(router)
	configMainRoute(router)
	configCourseRoute(router)
	return
}

func configGinStaticRouter(router gin.IRoutes) {
	router.Static("/assets", "./web/assets")
	router.Static("/dist", "./node_modules")
	router.StaticFile("/favicon.ico", "./web/assets/favicon.ico")
}

func configMainRoute(router gin.IRoutes) {
	router.GET("/about", AboutPage)
	router.GET("/admin", AdminPage)
	router.GET("/admin/create-course", CreateCoursePage)
	router.GET("/admin/course/:id", EditCoursePage)
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
