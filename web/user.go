package web

import (
	"TUM-Live/dao"
	"TUM-Live/model"
	"TUM-Live/tools"
	"TUM-Live/tools/tum"
	"context"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt"
	log "github.com/sirupsen/logrus"
	"net/http"
	"net/url"
	"time"
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
}

func startSession(c *gin.Context, data *sessionData) {
	token, err := createToken(data.userid)
	if err != nil {
		log.WithError(err).Error("Could not create token")
		return
	}
	c.SetCookie("jwt", token, 60*60*24*7, "/", "", tools.CookieSecure, true)
}

func createToken(user uint) (string, error) {
	t := jwt.New(jwt.GetSigningMethod("RS256"))

	t.Claims = &tools.JWTClaims{
		StandardClaims: &jwt.StandardClaims{
			ExpiresAt: time.Now().Add(time.Hour * 24 * 7).Unix(), // Token expires in one week
		},
		UserID: user,
	}
	return t.SignedString(tools.Cfg.GetJWTKey())
}

// loginWithUserCredentials Try to login with non-tum credentials
// Returns pointer to sessionData object if successful or nil if not.
func loginWithUserCredentials(username, password string) *sessionData {
	if u, err := dao.GetUserByEmail(context.Background(), username); err == nil {
		// user with this email found.
		if match, err := u.ComparePasswordAndHash(password); err == nil && match {
			return &sessionData{u.ID}
		}

		return nil
	}

	return nil
}

// loginWithTumCredentials Try to login with tum credentials
// Returns pointer to sessionData if successful and nil if not
func loginWithTumCredentials(username, password string) (*sessionData, error) {
	loginResp, err := tum.LoginWithTumCredentials(username, password)
	if err == nil {
		user := model.User{
			Name:                loginResp.FirstName,
			LastName:            loginResp.LastName,
			MatriculationNumber: loginResp.UserId,
			LrzID:               loginResp.LrzIdent,
			Role:                model.GenericType,
		}
		err = dao.UpsertUser(&user)
		if err != nil {
			log.Printf("%v", err)
			return nil, err
		}

		return &sessionData{user.ID}, nil
	}

	return nil, err
}

func LoginPage(c *gin.Context) {
	_ = templ.ExecuteTemplate(c.Writer, "login.gohtml", NewLoginPageData(false))
}

func LogoutPage(c *gin.Context) {
	c.SetCookie("jwt", "", -1, "/", "", tools.CookieSecure, true)
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
