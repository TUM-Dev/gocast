package web

import (
	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	"log"
	"net/http"
)

func LoginPage(c *gin.Context) {
	err := templ.ExecuteTemplate(c.Writer, "login.gohtml", nil)
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
