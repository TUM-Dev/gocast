package web

import (
	"context"
	"net/http"
	"net/url"
	"strings"

	"github.com/TUM-Dev/gocast/dao"
	"github.com/TUM-Dev/gocast/model"
	"github.com/TUM-Dev/gocast/tools"
	"github.com/TUM-Dev/gocast/tools/tum"
)

const redirCookieName = "redirURL"

func (r mainRoutes) LoginHandler(w http.ResponseWriter, req *http.Request) {
	username := req.FormValue("username")
	password := req.FormValue("password")

	var (
		data *tools.SessionData
		err  error
	)

	if data = loginWithUserCredentials(username, password, r.UsersDao); data != nil {
		HandleValidLogin(w, req, data)
		return
	}

	if tools.Cfg.Ldap.UseForLogin {
		if data, err = loginWithTumCredentials(username, password, r.UsersDao); err == nil {
			HandleValidLogin(w, req, data)
			return
		} else if err != tum.ErrLdapBadAuth {
			logger.Error("Login error", "err", err)
		}
	}

	// Redirect rather than render the login template inline: the login page may be
	// served by the SPA, which would otherwise be replaced mid-flow by a
	// server-rendered page. The redirect URL cookie set by LoginPage survives this, so
	// the user still lands where they were headed once they get their password right.
	http.Redirect(w, req, "/login?"+loginErrorQuery+"=1", http.StatusFound)
}

// loginErrorQuery marks a failed login attempt when redirecting back to the login page.
const loginErrorQuery = "error"

// HandleValidLogin starts a session and redirects the user to the page they were trying to access.
func HandleValidLogin(w http.ResponseWriter, r *http.Request, data *tools.SessionData) {
	tools.StartSession(w, data)
	redirURL, err := tools.Cookie(r, redirCookieName)
	if err != nil {
		redirURL = "/" // Fallback in case no cookie is present: Redirect to index page
	} else {
		// Delete cookie that was used for saving the redirURL.
		tools.SetCookie(w, redirCookieName, "", -1, tools.CookieSecure)
	}
	http.Redirect(w, r, redirURL, http.StatusFound)
}

func getRedirectUrl(r *http.Request) (*url.URL, error) {
	ret := r.URL.Query().Get("return")
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
			recordLogin(usersDao, u.ID)
			return &tools.SessionData{Userid: u.ID, SamlSubjectID: nil}
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

		recordLogin(usersDao, user.ID)
		return &tools.SessionData{Userid: user.ID, SamlSubjectID: nil}, nil
	}

	return nil, err
}

// recordLogin stamps the user's last_login. A failure is logged and otherwise ignored:
// the credentials were right, and a missed stamp is not worth locking the user out.
func recordLogin(usersDao dao.UsersDao, uid uint) {
	if err := usersDao.TouchLastLogin(context.Background(), uid); err != nil {
		logger.Error("Could not record last login", "err", err, "user", uid)
	}
}

// SetLoginRedirectCookie stores where to send the user once they have logged in.
//
// It has to happen while serving the login page, before the user picks between the
// internal form and an external identity provider, because an external login leaves
// the site entirely and comes back without the original query string. It is exported
// so that the route serving the SPA login page keeps the same behaviour.
func SetLoginRedirectCookie(w http.ResponseWriter, r *http.Request) {
	redirUrlStr := "/"
	redirURL, err := getRedirectUrl(r)
	if err == nil && redirURL.Scheme == "" && redirURL.Host == "" {
		redirUrlStr = redirURL.String()
	}

	// Only set cookie if (potentially) needed.
	if !strings.HasSuffix(redirUrlStr, "/") && !strings.HasSuffix(redirUrlStr, "/login") {
		// Use 10 minutes for expiry as the user may not login immediately. The cookie is deleted after login.
		tools.SetCookie(w, redirCookieName, redirUrlStr, 600, tools.CookieSecure)
	}
}

func (r mainRoutes) LogoutPage(w http.ResponseWriter, req *http.Request) {
	tools.SetCookie(w, "jwt", "", -1, tools.CookieSecure)
	http.Redirect(w, req, "/", http.StatusFound)
}
