package web

import (
	"context"
	"github.com/TUM-Dev/gocast/dao"
	"github.com/TUM-Dev/gocast/model"
	"github.com/TUM-Dev/gocast/tools"
	"github.com/TUM-Dev/gocast/tools/tum"
	"github.com/gin-gonic/gin"
	"net/http"
	"net/url"
	"strings"
)

type userSettingsData struct {
	IndexData IndexData
}

const redirCookieName = "redirURL"

func (r mainRoutes) settingsPage(c *gin.Context) {
	d := userSettingsData{IndexData: NewIndexData()}
	d.IndexData.TUMLiveContext = c.MustGet("TUMLiveContext").(tools.TUMLiveContext)

	err := templateExecutor.ExecuteTemplate(c.Writer, "user-settings.gohtml", d.IndexData)
	if err != nil {
		logger.Error("Error executing template user-settings.gohtml", "err", err)
		c.AbortWithStatus(http.StatusInternalServerError)
	}
}

func (r mainRoutes) LoginHandler(c *gin.Context) {
	username := c.Request.FormValue("username")
	password := c.Request.FormValue("password")

	var (
		data *tools.SessionData
		err  error
	)

	if data = loginWithUserCredentials(username, password, r.UsersDao); data != nil {
		HandleValidLogin(c, data)
		return
	}

	if tools.Cfg.Ldap.UseForLogin {
		if data, err = loginWithTumCredentials(username, password, r.UsersDao); err == nil {
			HandleValidLogin(c, data)
			return
		} else if err != tum.ErrLdapBadAuth {
			logger.Error("Login error", "err", err)
		}
	}

	_ = templateExecutor.ExecuteTemplate(c.Writer, "login.gohtml", NewLoginPageData(true))
}

// HandleValidLogin starts a session and redirects the user to the page they were trying to access.
func HandleValidLogin(c *gin.Context, data *tools.SessionData) {
	tools.StartSession(c, data)
	redirURL, err := c.Cookie(redirCookieName)
	if err != nil {
		redirURL = "/" // Fallback in case no cookie is present: Redirect to index page
	} else {
		// Delete cookie that was used for saving the redirURL.
		c.SetCookie(redirCookieName, "", -1, "/", "", tools.CookieSecure, true)
	}
	c.Redirect(http.StatusFound, redirURL)
}

func getRedirectUrl(c *gin.Context) (*url.URL, error) {
	ret := c.Query("return")
	if ret != "" {
		red, err := url.QueryUnescape(ret)
		if err == nil {
			return url.Parse(red)
		}
	}

	if ret == "" {
		return url.Parse("/")
	}

	return url.Parse(ret)
}

// loginWithUserCredentials Try to login with non-tum credentials
// Returns pointer to sessionData object if successful or nil if not.
func loginWithUserCredentials(username, password string, usersDao dao.UsersDao) *tools.SessionData {
	if u, err := usersDao.GetUserByEmail(context.Background(), username); err == nil {
		// user with this email found.
		if match, err := u.ComparePasswordAndHash(password); err == nil && match {
			return &tools.SessionData{u.ID, nil}
		}
		return nil
	}

	return nil
}

// loginWithTumCredentials Try to login with tum credentials
// Returns pointer to sessionData if successful and nil if not
func loginWithTumCredentials(username, password string, usersDao dao.UsersDao) (*tools.SessionData, error) {
	loginResp, err := tum.LoginWithTumCredentials(username, password)
	if err == nil {
		user := model.User{
			Name:                loginResp.FirstName,
			LastName:            loginResp.LastName,
			MatriculationNumber: loginResp.UserId,
			LrzID:               loginResp.LrzIdent,
		}
		err = usersDao.UpsertUser(&user)
		if err != nil {
			logger.Error("Error upserting user", "err", err)
			return nil, err
		}

		return &tools.SessionData{user.ID, nil}, nil
	}

	return nil, err
}

func (r mainRoutes) LoginPage(c *gin.Context) {
	redirUrlStr := "/"
	redirURL, err := getRedirectUrl(c)
	if err == nil && redirURL.Scheme == "" && redirURL.Host == "" {
		redirUrlStr = redirURL.String()
	}

	// Only set cookie if (potentially) needed.
	if !strings.HasSuffix(redirUrlStr, "/") && !strings.HasSuffix(redirUrlStr, "/login") {
		// We need to set the cookie here now as we don't know whether the user will choose an internal or external login.
		// Use 10 minutes for expiry as the user may not login immediately. The cookie is deleted after login.
		c.SetCookie(redirCookieName, redirUrlStr, 600, "/", "", tools.CookieSecure, true)
	}

	d := NewLoginPageData(false)
	d.UseSAML = tools.Cfg.Saml != nil
	if d.UseSAML {
		d.IDPName = tools.Cfg.Saml.IdpName
		d.IDPColor = tools.Cfg.Saml.IdpColor
	}
	_ = templateExecutor.ExecuteTemplate(c.Writer, "login.gohtml", d)
}

func (r mainRoutes) LogoutPage(c *gin.Context) {
	c.SetCookie("jwt", "", -1, "/", "", tools.CookieSecure, true)
	c.Redirect(http.StatusFound, "/")
}

func (r mainRoutes) CreatePasswordPage(c *gin.Context) {
	if c.Request.Method == "POST" {
		p1 := c.Request.FormValue("password")
		p2 := c.Request.FormValue("passwordConfirm")
		u, err := r.UsersDao.GetUserByResetKey(c.Param("key"))
		if err != nil {
			c.Redirect(http.StatusFound, "/")
			return
		}
		if p1 != p2 {
			_ = templateExecutor.ExecuteTemplate(c.Writer, "passwordreset.gohtml", NewLoginPageData(true))
			return
		}
		err = u.SetPassword(p1)
		if err != nil {
			logger.Error("error setting password.", "err", err)
			_ = templateExecutor.ExecuteTemplate(c.Writer, "passwordreset.gohtml", NewLoginPageData(true))
			return
		} else {
			err := r.UsersDao.UpdateUser(u)
			if err != nil {
				logger.Error("CreatePasswordPage: Can't update user", "err", err)
			}
			r.UsersDao.DeleteResetKey(c.Param("key"))
			c.Redirect(http.StatusFound, "/")
		}
		return
	} else {
		_, err := r.UsersDao.GetUserByResetKey(c.Param("key"))
		if err != nil {
			c.Redirect(http.StatusFound, "/")
			return
		}
		_ = templateExecutor.ExecuteTemplate(c.Writer, "passwordreset.gohtml", NewLoginPageData(false))
	}
}

// NewLoginPageData returns a new struct LoginPageData with the Error value err
func NewLoginPageData(err bool) LoginPageData {
	return LoginPageData{
		VersionTag:   VersionTag,
		Error:        err,
		Branding:     tools.BrandingCfg,
		CanonicalURL: tools.NewCanonicalURL(tools.Cfg.CanonicalURL),
	}
}

// LoginPageData contains the data for login page templates
type LoginPageData struct {
	VersionTag string
	Error      bool

	UseSAML  bool
	IDPName  string
	IDPColor string

	Branding     tools.Branding
	CanonicalURL tools.CanonicalURL
}
