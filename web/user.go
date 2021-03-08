package web

import (
	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	"github.com/julienschmidt/httprouter"
	"log"
	"net/http"
)

func LoginPage(writer http.ResponseWriter, request *http.Request, params httprouter.Params) {
	err := templ.ExecuteTemplate(writer, "login.gohtml", nil)
	if err != nil {
		log.Printf("couldn't render template: %v\n", err)
	}
}

func LogoutPage(c *gin.Context) {
	s := sessions.Default(c)
	s.Clear()
	_ = s.Save()
	c.Redirect(http.StatusFound, "/")
}

func CreatePasswordPage(c *gin.Context) {
	key := c.Param("key")
	err := templ.ExecuteTemplate(c.Writer, "passwordreset.gohtml", key)
	if err != nil {
		log.Printf("couldn't render template: %v\n", err)
	}
}
