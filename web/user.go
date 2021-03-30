package web

import (
	"TUM-Live/dao"
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
	if c.Request.Method == "POST" {
		p1 := c.Request.FormValue("password")
		p2 := c.Request.FormValue("passwordConfirm")
		u, err := dao.GetUserByResetKey(c.Param("key"))
		if err != nil {
			c.Redirect(http.StatusFound, "/")
			return
		}
		if p1 != p2 {
			_ = templ.ExecuteTemplate(c.Writer, "passwordreset.gohtml", PasswordResetPageData{Error: true})
			return
		}
		err = u.SetPassword(p1)
		if err != nil {
			log.Printf("error setting password.")
			_ = templ.ExecuteTemplate(c.Writer, "passwordreset.gohtml", PasswordResetPageData{Error: true})
			return
		}else {
			dao.UpdateUser(u)
			c.Redirect(http.StatusFound, "/")
		}
		return
	} else {
		_, err := dao.GetUserByResetKey(c.Param("key"))
		if err != nil {
			c.Redirect(http.StatusFound, "/")
			return
		}
		_ = templ.ExecuteTemplate(c.Writer, "passwordreset.gohtml", PasswordResetPageData{Error: false})
	}
}

type PasswordResetPageData struct {
	Error bool
}
