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

	var data *sessionData
	var err error

	if data = loginWithUserCredentials(username, password); data != nil {
		startSession(c, data)
		c.Redirect(http.StatusFound, getRedirectUrl(c))
		return
	}

	if data, err = loginWithTumCredentials(username, password); err == nil {
		startSession(c, data)
		c.Redirect(http.StatusFound, getRedirectUrl(c))
		return
	} else if err != tum.ErrLdapBadAuth {
		log.WithError(err).Error("Login error")
	}

	_ = templ.ExecuteTemplate(c.Writer, "login.gohtml", NewLoginPageData(true))
}

func getRedirectUrl(c *gin.Context) string {
	retur := c.Request.FormValue("return")
	ref := c.Request.FormValue("ref")
	if retur != "" {
		red, err := url.QueryUnescape(c.Request.FormValue("return"))
		if err == nil {
			return red
		}
	}

	if ref == "" {
		return "/"
	}

	return ref
}

type sessionData struct {
	userid uint
	name   string
}

func startSession(c *gin.Context, data *sessionData) {
	s := sessions.Default(c)
	s.Set("UserID", data.userid)
	s.Set("Name", data.name)
	_ = s.Save()
}

// loginWithUserCredentials Try to login with non-tum credentials
// Returns pointer to sessionData object if successful or nil if not.
func loginWithUserCredentials(username, password string) *sessionData {
	if u, err := dao.GetUserByEmail(context.Background(), username); err == nil {
		// user with this email found.
		if match, err := u.ComparePasswordAndHash(password); err == nil && match {
			return &sessionData{u.ID, u.Name}
		}

		return nil
	}

	return nil
}

// loginWithTumCredentials Try to login with tum credentials
// Returns pointer to sessionData if successful and nil if not
func loginWithTumCredentials(username, password string) (*sessionData, error) {
	sId, lrzID, name, err := tum.LoginWithTumCredentials(username, password)
	if err == nil {
		user := model.User{
			Name:                name,
			MatriculationNumber: sId,
			LrzID:               lrzID,
		}
		err = dao.UpsertUser(&user)
		if err != nil {
			log.Printf("%v", err)
			return nil, err
		}

		return &sessionData{user.ID, user.Name}, nil
	}

	return nil, err
}

func LoginPage(c *gin.Context) {
	_ = templ.ExecuteTemplate(c.Writer, "login.gohtml", NewLoginPageData(false))
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
			_ = templ.ExecuteTemplate(c.Writer, "passwordreset.gohtml", NewLoginPageData(true))
			return
		}
		err = u.SetPassword(p1)
		if err != nil {
			log.WithError(err).Error("error setting password.")
			_ = templ.ExecuteTemplate(c.Writer, "passwordreset.gohtml", NewLoginPageData(true))
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
		_ = templ.ExecuteTemplate(c.Writer, "passwordreset.gohtml", NewLoginPageData(false))
	}
}

// NewLoginPageData returns a new struct LoginPageData with the Error value err
func NewLoginPageData(err bool) LoginPageData {
	return LoginPageData{
		VersionTag: VersionTag,
		Error:      err,
	}
}

// LoginPageData contains the data for login page templates
type LoginPageData struct {
	VersionTag string
	Error      bool
}
