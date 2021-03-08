package web

import (
	"TUM-Live/api"
	"github.com/gin-gonic/gin"
	"html/template"
)

var templ *template.Template

func ConfigGinRouter(router gin.IRoutes) {
	templ, _ = template.ParseGlob("./web/template/*")
	configGinStaticRouter(router)
	configMainRoute(router)
	return
}

func configGinStaticRouter(router gin.IRoutes) {
	router.Static("/assets", "./web/assets")
	router.Static("/dist", "./node_modules")
	router.StaticFile("/favicon.ico", "./web/assets/favicon.ico")
}

func configMainRoute(router gin.IRoutes) {
	router.GET("/admin", AdminPage)
	router.GET("/login", api.ConvertHttprouterToGin(LoginPage))
	router.GET("/logout", LogoutPage)
	router.GET("/setPassword/:key", CreatePasswordPage)
	router.GET("/", MainPage)
}
