package web

import (
	"TUM-Live-Backend/api"
	"github.com/gin-gonic/gin"
	"github.com/julienschmidt/httprouter"
	"html/template"
	"net/http"
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
}

func configMainRoute(router gin.IRoutes) {
	router.GET("/", api.ConverHttprouterToGin(MainPage))
}

func MainPage(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	_ = templ.ExecuteTemplate(w, "index.html", "")
}
