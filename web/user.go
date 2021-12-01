package web

import (
	"TUM-Live/dao"
	"TUM-Live/model"
	"TUM-Live/tools/tum"
	"context"
	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
	"net/http"
	"net/url"
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
			if c.Request.FormValue("return") != "" {
				red, err := url.QueryUnescape(c.Request.FormValue("return"))
				if err == nil {
					c.Redirect(http.StatusFound, red)
					return
				}
			}
			c.Redirect(http.StatusFound, "/")
			return
		}
	}
	sId, lrzID, name, err := tum.LoginWithTumCredentials(username, password)
	if err == nil {
		user := model.User{
			Name:                name,
			MatriculationNumber: sId,
			LrzID:               lrzID,
			Role:                model.GenericType,
		}
		err = dao.UpsertUser(&user)
		if err != nil {
			log.Printf("%v", err)
			return
		}
		s := sessions.Default(c)
		s.Set("UserID", user.ID)
		s.Set("Name", user.Name)
		_ = s.Save()
		if c.Request.FormValue("return") != "" {
			red, err := url.QueryUnescape(c.Request.FormValue("return"))
			if err == nil {
				c.Redirect(http.StatusFound, red)
				return
			}
		}
		c.Redirect(http.StatusFound, "/")
		return
	} else if err != tum.ErrLdapBadAuth {
		log.WithError(err).Error("Login error")
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
			log.WithError(err).Error("error setting password.")
			_ = templ.ExecuteTemplate(c.Writer, "passwordreset.gohtml", PasswordResetPageData{Error: true})
			return
		} else {
			err := dao.UpdateUser(u)
			if err != nil {
				log.WithError(err).Error("CreatePasswordPage: Can't update user")
			}
			dao.DeleteResetKey(c.Param("key"))
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
