package web

import (
	"TUM-Live-Backend/api"
	"TUM-Live-Backend/dao"
	"TUM-Live-Backend/model"
	"context"
	"errors"
	"github.com/gin-gonic/gin"
	"html/template"
	"net/http"
)

var templ *template.Template

var errorNotLoggedIn = errors.New("not logged in")
var genericError = errors.New("something went wrong")

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
	router.GET("/logout", api.ConvertHttprouterToGin(LogoutPage))
	router.GET("/", api.ConvertHttprouterToGin(MainPage))
}

func getUser(r *http.Request, user *model.User) (err error) {
	sid, err := getSID(r)
	if err != nil {
		return errorNotLoggedIn
	}
	foundUser, err := dao.GetUserBySID(context.Background(), sid)
	if err != nil {
		// Session id invalid.
		return errorNotLoggedIn
	}
	*user = foundUser
	return nil
}

func getSID(r *http.Request) (SID string, err error) {
	cookie, err := r.Cookie("SID")
	if err != nil {
		return "", errorNotLoggedIn
	}
	return cookie.Value, nil
}
