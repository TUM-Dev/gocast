package web

import (
	"context"
	"net/http"
	"net/url"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v4"
	"github.com/joschahenningsen/TUM-Live/dao"
	"github.com/joschahenningsen/TUM-Live/model"
	"github.com/joschahenningsen/TUM-Live/tools"
	"github.com/joschahenningsen/TUM-Live/tools/tum"
	log "github.com/sirupsen/logrus"
)

type userSettingsData struct {
	IndexData IndexData
}

func (r mainRoutes) settingsPage(c *gin.Context) {
	d := userSettingsData{IndexData: NewIndexData()}
	d.IndexData.TUMLiveContext = c.MustGet("TUMLiveContext").(tools.TUMLiveContext)

	err := templateExecutor.ExecuteTemplate(c, c.Writer, "user-settings.gohtml", d)
	if err != nil {
		log.Error(err)
		c.AbortWithStatus(http.StatusInternalServerError)
	}
}

func (r mainRoutes) LoginHandler(c *gin.Context) {
	username := c.Request.FormValue("username")
	password := c.Request.FormValue("password")

	var data *sessionData
	var err error

	if data = loginWithUserCredentials(username, password, r.UsersDao); data != nil {
		startSession(c, data)
		c.Redirect(http.StatusFound, getRedirectUrl(c))
		return
	}

	if tools.Cfg.Ldap.UseForLogin {
		if data, err = loginWithTumCredentials(c, username, password, r.UsersDao); err == nil {
			startSession(c, data)
			c.Redirect(http.StatusFound, getRedirectUrl(c))
			return
		} else if err != tum.ErrLdapBadAuth {
			log.WithError(err).Error("Login error")
		}
	}

	_ = templateExecutor.ExecuteTemplate(c, c.Writer, "login.gohtml", NewLoginPageData(true))
}

func getRedirectUrl(c *gin.Context) string {
	ret := c.Request.FormValue("return")
	ref := c.Request.FormValue("ref")
	if ret != "" {
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
	userid        uint
	samlSubjectID *string
}

func startSession(c *gin.Context, data *sessionData) {
	token, err := createToken(data.userid, data.samlSubjectID)
	if err != nil {
		log.WithError(err).Error("Could not create token")
		return
	}
	c.SetCookie("jwt", token, 60*60*24*7, "/", "", tools.CookieSecure, true)
}

func createToken(user uint, samlSubjectID *string) (string, error) {
	t := jwt.New(jwt.GetSigningMethod("RS256"))

	t.Claims = &tools.JWTClaims{
		RegisteredClaims: &jwt.RegisteredClaims{
			ExpiresAt: &jwt.NumericDate{Time: time.Now().Add(time.Hour * 24 * 7)}, // Token expires in one week
		},
		UserID:        user,
		SamlSubjectID: samlSubjectID,
	}
	return t.SignedString(tools.Cfg.GetJWTKey())
}

// loginWithUserCredentials Try to login with non-tum credentials
// Returns pointer to sessionData object if successful or nil if not.
func loginWithUserCredentials(username, password string, usersDao dao.UsersDao) *sessionData {
	if u, err := usersDao.GetUserByEmail(context.Background(), username); err == nil {
		// user with this email found.
		if match, err := u.ComparePasswordAndHash(password); err == nil && match {
			return &sessionData{u.ID, nil}
		}
		return nil
	}

	return nil
}

// loginWithTumCredentials Try to login with tum credentials
// Returns pointer to sessionData if successful and nil if not
func loginWithTumCredentials(c context.Context, username, password string, usersDao dao.UsersDao) (*sessionData, error) {
	loginResp, err := tum.LoginWithTumCredentials(username, password)
	if err == nil {
		user := model.User{
			Name:                loginResp.FirstName,
			LastName:            loginResp.LastName,
			MatriculationNumber: loginResp.UserId,
			LrzID:               loginResp.LrzIdent,
		}
		err = usersDao.UpsertUser(c, &user)
		if err != nil {
			log.Printf("%v", err)
			return nil, err
		}

		return &sessionData{user.ID, nil}, nil
	}

	return nil, err
}

func (r mainRoutes) LoginPage(c *gin.Context) {
	d := NewLoginPageData(false)
	d.UseSAML = tools.Cfg.Saml != nil
	if d.UseSAML {
		d.IDPName = tools.Cfg.Saml.IdpName
		d.IDPColor = tools.Cfg.Saml.IdpColor
	}
	_ = templateExecutor.ExecuteTemplate(c, c.Writer, "login.gohtml", d)
}

func (r mainRoutes) LogoutPage(c *gin.Context) {
	c.SetCookie("jwt", "", -1, "/", "", tools.CookieSecure, true)
	c.Redirect(http.StatusFound, "/")
}

func (r mainRoutes) CreatePasswordPage(c *gin.Context) {
	if c.Request.Method == "POST" {
		p1 := c.Request.FormValue("password")
		p2 := c.Request.FormValue("passwordConfirm")
		u, err := r.UsersDao.GetUserByResetKey(c, c.Param("key"))
		if err != nil {
			c.Redirect(http.StatusFound, "/")
			return
		}
		if p1 != p2 {
			_ = templateExecutor.ExecuteTemplate(c, c.Writer, "passwordreset.gohtml", NewLoginPageData(true))
			return
		}
		err = u.SetPassword(p1)
		if err != nil {
			log.WithError(err).Error("error setting password.")
			_ = templateExecutor.ExecuteTemplate(c, c.Writer, "passwordreset.gohtml", NewLoginPageData(true))
			return
		} else {
			err := r.UsersDao.UpdateUser(c, u)
			if err != nil {
				log.WithError(err).Error("CreatePasswordPage: Can't update user")
			}
			r.UsersDao.DeleteResetKey(c, c.Param("key"))
			c.Redirect(http.StatusFound, "/")
		}
		return
	} else {
		_, err := r.UsersDao.GetUserByResetKey(c, c.Param("key"))
		if err != nil {
			c.Redirect(http.StatusFound, "/")
			return
		}
		_ = templateExecutor.ExecuteTemplate(c, c.Writer, "passwordreset.gohtml", NewLoginPageData(false))
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

	UseSAML  bool
	IDPName  string
	IDPColor string
}
