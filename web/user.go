package web

import (
	"TUM-Live/dao"
	"TUM-Live/tools/tum"
	"context"
	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	"log"
	"net/http"
)

func LoginHandler(c *gin.Context) {
	username := c.Request.FormValue("username")
	password := c.Request.FormValue("password")
	if u, err := dao.GetUserByEmail(context.Background(), username); err == nil {
		// user with this email found.
		if match, err := u.ComparePasswordAndHash(password); err == nil && match {
			s := sessions.Default(c)
			s.Set("UserID", u.ID)
			s.Set("Name", u.Name)
			_ = s.Save()
			c.Redirect(http.StatusFound, "/")
			return
		}
	}
	if sId, name, err := tum.LoginWithTumCredentials(username, password); err == nil {
		s := sessions.Default(c)
		s.Set("StudentID", sId)
		s.Set("Name", name)
		_ = s.Save()
		c.Redirect(http.StatusFound, "/")
		return
	}
	_ = templ.ExecuteTemplate(c.Writer, "login.gohtml", true)
}

func LoginPage(c *gin.Context) {
	_ = templ.ExecuteTemplate(c.Writer, "login.gohtml", false)
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
		} else {
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
