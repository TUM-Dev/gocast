package web

import (
	"TUM-Live-Backend/api"
	"TUM-Live-Backend/dao"
	"context"
	"github.com/gin-gonic/gin"
	"github.com/julienschmidt/httprouter"
	"html/template"
	"log"
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
	router.Static("/dist", "./node_modules")
}

func configMainRoute(router gin.IRoutes) {
	router.GET("/admin", api.ConvertHttprouterToGin(AdminPage))
	router.GET("/login", api.ConvertHttprouterToGin(LoginPage))
	router.GET("/", api.ConvertHttprouterToGin(MainPage))
}

func MainPage(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	res, err := dao.AreUsersEmpty(context.Background())
	if err != nil {
		_ = templ.ExecuteTemplate(w, "error.html", "")
	} else if res {
		_ = templ.ExecuteTemplate(w, "onboarding.html", "")
	} else {
		_ = templ.ExecuteTemplate(w, "index.html", "")
	}
}

func AdminPage(writer http.ResponseWriter, request *http.Request, params httprouter.Params) {
	//todo authentication
	_ = templ.ExecuteTemplate(writer, "admin.html", "")
}

func LoginPage(writer http.ResponseWriter, request *http.Request, params httprouter.Params) {
	err := templ.ExecuteTemplate(writer, "login.html", "")
	if err != nil {
		log.Printf("couldn't render template: %v\n", err)
	}
}
