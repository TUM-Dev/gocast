package oauth

import (
	"context"
	"database/sql"
	"errors"
	"github.com/TUM-Dev/gocast/dao"
	"github.com/TUM-Dev/gocast/model"
	"github.com/TUM-Dev/gocast/tools"
	"github.com/TUM-Dev/gocast/tools/sessions"
	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/oauth2"
	"net/http"
	"net/url"
	"strings"
	"time"
)

var Auth *OAuth = &OAuth{}
var daoWrapper dao.DaoWrapper

type OAuth struct {
	LogoutURL    string
	ProviderURL  string
	Provider     *oidc.Provider        `default:"nil"`
	Verifier     *oidc.IDTokenVerifier `default:"nil"`
	OAuth2Config oauth2.Config         `default:"nil"`
	KeySet       *oidc.RemoteKeySet    `default:"nil"`
}

func (oauth *OAuth) SetupOauth() {
	daoWrapper = dao.NewDaoWrapper()
	if oauth.ProviderURL == "" && (tools.Cfg.OAuth == nil || tools.Cfg.OAuth.ProviderURL == "") {
		logger.Info("Provider URL is empty, oauth not enabled")
		return
	} else if oauth.ProviderURL == "" {
		oauth.ProviderURL = tools.Cfg.OAuth.ProviderURL
	}

	if oauth.LogoutURL == "" && (tools.Cfg.OAuth == nil || tools.Cfg.OAuth.LogoutURL == "") {
		logger.Info("Logout URL is empty, oauth not enabled")
		return
	} else if oauth.LogoutURL == "" {
		oauth.LogoutURL = tools.Cfg.OAuth.LogoutURL
	}
	ctx := context.Background()
	var err error
	oauth.Provider, err = oidc.NewProvider(ctx, oauth.ProviderURL)
	if err != nil {
		logger.Error("Error creating provider for oauth", "err", err)
	}

	oauth.OAuth2Config = oauth2.Config{
		ClientID:     tools.Cfg.OAuth.ClientID,
		ClientSecret: tools.Cfg.OAuth.ClientSecret,
		RedirectURL:  tools.Cfg.OAuth.RedirectURL,

		Endpoint: oauth.Provider.Endpoint(),

		Scopes: []string{oidc.ScopeOpenID, "profile", "email", "roles", "identity_provider", "groups"},
	}

	oauth.Verifier = oauth.Provider.Verifier(&oidc.Config{ClientID: oauth.OAuth2Config.ClientID})

	oauth.KeySet = oidc.NewRemoteKeySet(ctx, oauth.ProviderURL+"/protocol/openid-connect/certs")

	logger.Info("Successfully created OAuthConfig")
}

func GetGroups(c *gin.Context) []string {
	if !CheckLoggedIn(c) {
		tools.RenderErrorPage(c, http.StatusUnauthorized, "Unauthorized")
		return make([]string, 0)
	}

	if cookie, _ := c.Cookie(tools.Cfg.Cookie.Name); cookie == "" {
		logger.Debug("No cookie found")
		tools.RenderErrorPage(c, http.StatusUnauthorized, "Unauthorized")
		return make([]string, 0)
	}

	session, err := sessions.Store.Get(c, tools.Cfg.Cookie.Name)
	if err != nil {
		tools.RenderErrorPage(c, http.StatusUnauthorized, "Unauthorized")
		return make([]string, 0)
	}

	var claims struct {
		*jwt.RegisteredClaims
		RealmAccess struct {
			Groups []string `json:"groups"`
		} `json:"realm_access"`
	}

	_, _, err = jwt.NewParser(jwt.WithoutClaimsValidation()).ParseUnverified(session.Values["access_token"].(string), &claims)
	if err != nil {
		logger.Debug("Error parsing claims", "err", err)
		tools.RenderErrorPage(c, http.StatusUnauthorized, "Unauthorized")
		return make([]string, 0)
	}
	return claims.RealmAccess.Groups
}

func GetIdP(c *gin.Context) (string, error) {
	if !CheckLoggedIn(c) {
		tools.RenderErrorPage(c, http.StatusUnauthorized, "Unauthorized")
		return "", errors.New("unauthorized")
	}

	if cookie, _ := c.Cookie(tools.Cfg.Cookie.Name); cookie == "" {
		logger.Debug("No cookie found")
		tools.RenderErrorPage(c, http.StatusUnauthorized, "Unauthorized")
		return "", errors.New("unauthorized")
	}

	session, err := sessions.Store.Get(c, tools.Cfg.Cookie.Name)
	if err != nil {
		tools.RenderErrorPage(c, http.StatusUnauthorized, "Unauthorized")
		return "", errors.New("unauthorized")
	}

	var claims struct {
		*jwt.RegisteredClaims
		IdP string `json:"identity_provider"`
	}

	_, _, err = jwt.NewParser(jwt.WithoutClaimsValidation()).ParseUnverified(session.Values["access_token"].(string), &claims)
	if err != nil {
		logger.Debug("Error parsing claims", "err", err)
		tools.RenderErrorPage(c, http.StatusUnauthorized, "Unauthorized")
		return "", errors.New("unauthorized")
	}
	return claims.IdP, nil
}

func GetUID(c *gin.Context) (string, error) {
	if !CheckLoggedIn(c) {
		tools.RenderErrorPage(c, http.StatusUnauthorized, "Unauthorized")
		return "", errors.New("unauthorized")
	}

	if cookie, _ := c.Cookie(tools.Cfg.Cookie.Name); cookie == "" {
		logger.Debug("No cookie found")
		tools.RenderErrorPage(c, http.StatusUnauthorized, "Unauthorized")
		return "", errors.New("unauthorized")
	}

	session, err := sessions.Store.Get(c, tools.Cfg.Cookie.Name)
	if err != nil {
		tools.RenderErrorPage(c, http.StatusUnauthorized, "Unauthorized")
		return "", errors.New("unauthorized")
	}

	var claims struct {
		*jwt.RegisteredClaims
		Uid string `json:"sub"`
	}

	_, _, err = jwt.NewParser(jwt.WithoutClaimsValidation()).ParseUnverified(session.Values["access_token"].(string), &claims)
	if err != nil {
		logger.Debug("Error parsing claims", "err", err)
		tools.RenderErrorPage(c, http.StatusUnauthorized, "Unauthorized")
		return "", errors.New("unauthorized")
	}
	return claims.Uid, nil
}

func GetUsername(c *gin.Context) (string, error) {
	if !CheckLoggedIn(c) {
		tools.RenderErrorPage(c, http.StatusUnauthorized, "Unauthorized")
		return "", errors.New("unauthorized")
	}

	if cookie, _ := c.Cookie(tools.Cfg.Cookie.Name); cookie == "" {
		logger.Debug("No cookie found")
		tools.RenderErrorPage(c, http.StatusUnauthorized, "Unauthorized")
		return "", errors.New("unauthorized")
	}

	session, err := sessions.Store.Get(c, tools.Cfg.Cookie.Name)
	if err != nil {
		tools.RenderErrorPage(c, http.StatusUnauthorized, "Unauthorized")
		return "", errors.New("unauthorized")
	}

	var claims struct {
		*jwt.RegisteredClaims
		Username string `json:"preferred_username"`
	}

	_, _, err = jwt.NewParser(jwt.WithoutClaimsValidation()).ParseUnverified(session.Values["access_token"].(string), &claims)
	if err != nil {
		logger.Debug("Error parsing claims", "err", err)
		tools.RenderErrorPage(c, http.StatusUnauthorized, "Unauthorized")
		return "", errors.New("unauthorized")
	}
	return claims.Username, nil
}

func LoggedInUsersOnly() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !CheckLoggedIn(c) {
			tools.RenderErrorPage(c, http.StatusUnauthorized, "Unauthorized")
			c.Abort()
			return
		}
		c.Next()
	}
}

func CheckLoggedIn(c *gin.Context) bool {
	if cookie, _ := c.Cookie(tools.Cfg.Cookie.Name); cookie == "" {
		logger.Debug("No cookie found")
		return false
	}

	session, err := sessions.Store.Get(c, tools.Cfg.Cookie.Name)
	if err != nil {
		return false
	}

	if !session.IsNew {
		if !validateAccessToken(session.Values["id_token"].(string), session.Values["access_token"].(string)) {
			logger.Debug("Trying to get new token")

			oldToken := new(oauth2.Token)
			oldToken.AccessToken = session.Values["access_token"].(string)
			oldToken.RefreshToken = session.Values["refresh_token"].(string)
			t := session.Values["expiry"].(string)
			oldToken.Expiry, _ = time.Parse(time.RFC3339, t)
			oldToken.TokenType = session.Values["token_type"].(string)

			newtoken, err := Auth.OAuth2Config.TokenSource(c, oldToken).Token()
			if err != nil {
				logger.Debug("Error getting new token", "err", err)
				return false
			}

			rawIDToken, ok := newtoken.Extra("id_token").(string)
			if !ok {
				logger.Debug("Error getting ID Token")
				return false
			}

			session.Values["access_token"] = newtoken.AccessToken
			session.Values["refresh_token"] = newtoken.RefreshToken
			session.Values["token_type"] = newtoken.TokenType
			session.Values["expiry"] = newtoken.Expiry.Format(time.RFC3339)
			session.Values["id_token"] = rawIDToken
			err = session.Save(c)
			if err != nil {
				logger.Debug("Error saving session")
				return false
			}

			logger.Debug("Fetched new Token", "valid", newtoken.Valid(), "expiry", newtoken.Expiry)

			if !validateAccessToken(session.Values["id_token"].(string), session.Values["access_token"].(string)) {
				logger.Debug("Token not valid")
				return false
			}
		}
		logger.Debug("Successfully read sessions")
		return true
	}

	return false
}

func validateAccessToken(rawIdToken string, token string) bool {
	idToken, err := Auth.Verifier.Verify(context.Background(), rawIdToken)
	if err != nil {
		logger.Debug("Error verifying signature of idToken", "err", err)
		return false
	}
	err = idToken.VerifyAccessToken(token)
	if err != nil {
		logger.Debug("Error verifying access token")
		return false
	}
	parsed, err := jwt.Parse(token, nil)
	if parsed == nil {
		logger.Debug("Error parsing token", "err", err)
		return false
	}

	claims := parsed.Claims.(jwt.MapClaims)

	exp := time.Unix(int64(claims["exp"].(float64)), 0)
	logger.Debug("Expiration", "exp", exp)

	if time.Now().After(exp) {
		logger.Debug("Token expired")
		return false
	}
	//logger.Debug("Claims", "claims", claims)
	return true
}

// TODO: Roles not read correctly
type loginClaims struct {
	Email    string `json:"email"`
	Verified bool   `json:"email_verified"`
	IdP      string `json:"identity_provider"`
	Edu      struct {
		MatrNr string `json:"matrNr"`
		LrzId  string `json:"uid"`
	} `json:"edu"`
	Uid string `json:"sub"`

	Groups []string `json:"groups"`

	FamName   string `json:"family_name"`
	GivenName string `json:"given_name"`
}

func HandleOAuth2Callback(c *gin.Context) {
	// Handle OAuth2 callback
	oauth2Token, err := Auth.OAuth2Config.Exchange(c, c.Query("code"))
	if err != nil {
		tools.RenderErrorPage(c, http.StatusInternalServerError, "Some error occurred during login")
		logger.Debug("Error exchanging token", "err", err)
		return
	}

	// Extract the ID Token from OAuth2 token
	rawIDToken, ok := oauth2Token.Extra("id_token").(string)
	if !ok {
		tools.RenderErrorPage(c, http.StatusInternalServerError, "Some error occurred during login")
		logger.Debug("Error getting ID Token")
		return
	}

	// Parse and verify ID Token payload
	idToken, err := Auth.Verifier.Verify(c, rawIDToken)
	if err != nil {
		tools.RenderErrorPage(c, http.StatusInternalServerError, "Some error occurred during login")
		logger.Debug("Error verifying ID Token", "err", err)
		return
	}

	var claims loginClaims
	if err := idToken.Claims(&claims); err != nil {
		tools.RenderErrorPage(c, http.StatusInternalServerError, "Some error occurred during login")
		logger.Debug("Error extracting claims", "err", err)
		return
	}

	_, err = Auth.KeySet.VerifySignature(c, oauth2Token.AccessToken)
	if err != nil {
		tools.RenderErrorPage(c, http.StatusInternalServerError, "Some error occurred during login")
		logger.Debug("Error verifying signature", "err", err)
		return
	}

	session, err := sessions.Store.Get(c, tools.Cfg.Cookie.Name)
	if err != nil {
		tools.RenderErrorPage(c, http.StatusInternalServerError, "Some error occurred during login")
		logger.Debug("Error getting session", "err", err)
		return
	}
	session.Values["access_token"] = oauth2Token.AccessToken
	session.Values["refresh_token"] = oauth2Token.RefreshToken
	session.Values["token_type"] = oauth2Token.TokenType
	session.Values["expiry"] = oauth2Token.Expiry.Format(time.RFC3339)
	session.Values["id_token"] = rawIDToken
	err = session.Save(c)
	if err != nil {
		tools.RenderErrorPage(c, http.StatusInternalServerError, "Some error occurred during login")
		logger.Debug("Error saving session", "err", err)
		return
	}

	var newUser = false

	// TODO: Add OAuth Id to correct user in DB
	if claims.IdP == "" {
		// local account, verify by email
		user, err := daoWrapper.UsersDao.GetUserByEmail(c, claims.Email)
		if err != nil || user.Email.String == "" {
			// TODO: Create new user if user not exists
			createNewUser(c, &claims)
			newUser = true
			//tools.RenderErrorPage(c, http.StatusInternalServerError, "User not found in old db")
			//logger.Debug("User not found in old db", "err", err)
		}
		if user.OAuthID == "" && !newUser {
			if user.OAuthID == "" {
				user.OAuthID = claims.Uid
				err = daoWrapper.UsersDao.UpdateUser(user)
				if err != nil {
					tools.RenderErrorPage(c, http.StatusInternalServerError, "Error updating user")
				}
				// TODO: Maybe Context broken if OAuth ID newly set, users have to reload next page
			}
		}
	} else {
		// saml account, verify by matriculation number
		user, err := daoWrapper.UsersDao.GetUserByMatrNr(c, claims.Edu.MatrNr)
		if err != nil || user.MatriculationNumber == "" {
			// TODO: Create new user if user not exists
			createNewUser(c, &claims)
			newUser = true
			//tools.RenderErrorPage(c, http.StatusInternalServerError, "User not found in old db")
			//logger.Debug("User not found in old db", "err", err)
		}
		if user.OAuthID == "" && !newUser {
			user.OAuthID = claims.Uid
			err = daoWrapper.UsersDao.UpdateUser(user)
			if err != nil {
				tools.RenderErrorPage(c, http.StatusInternalServerError, "Error updating user")
			}
			// TODO: Context broken if OAuth ID newly set, users have to reload next page
		}
	}

	// Redirect to home of current host or to the host specified in the redirectURL cookie
	if cookie, _ := c.Cookie("redirectURL"); cookie != "" {
		c.SetCookie("redirectURL", "", -1, "/", "", false, true)
		c.Redirect(http.StatusFound, cookie)
	} else {
		c.Redirect(http.StatusFound, "/")
	}
}

func createNewUser(c *gin.Context, claims *loginClaims) {
	// TODO: Check role and insert correct role
	logger.Debug("Groups", "roles", claims.Groups)
	user := model.User{
		Name:                claims.GivenName,
		LastName:            &claims.FamName,
		Email:               sql.NullString{String: claims.Email, Valid: true},
		MatriculationNumber: claims.Edu.MatrNr,
		LrzID:               claims.Edu.LrzId,
		Role:                5,
		Password:            "",
		Courses:             nil,
		AdministeredCourses: nil,
		PinnedCourses:       nil,
		OAuthID:             claims.Uid,
		Settings:            nil,
		Bookmarks:           nil,
	}
	err := daoWrapper.UsersDao.CreateUser(c, &user)
	if err != nil {
		tools.RenderErrorPage(c, http.StatusInternalServerError, "Error creating user")
		logger.Error("Error creating user", "err", err)
		return
	}
}

// Logs the user out and redirects to home of current host or to the host specified in the redirectURL cookie
func HandleOAuth2Logout(c *gin.Context) {
	if cookie, _ := c.Cookie(tools.Cfg.Cookie.Name); cookie == "" {
		tools.RenderErrorPage(c, http.StatusUnauthorized, "Unauthorized")
		return
	}
	session, err := sessions.Store.Get(c, tools.Cfg.Cookie.Name)
	if err != nil {
		tools.RenderErrorPage(c, http.StatusInternalServerError, "Session invalid")
		return
	}
	accessToken := session.Values["id_token"].(string)
	err = sessions.Store.Delete(c, session)
	if err != nil {
		tools.RenderErrorPage(c, http.StatusInternalServerError, "Error deleting session")
		return
	}
	c.SetCookie(tools.Cfg.Cookie.Name, "", -1, "/", "", false, true)

	if cookie, _ := c.Cookie("redirectURL"); cookie != "" {
		c.SetCookie("redirectURL", "", -1, "/", "", false, true)
		var hostParam string
		if strings.Contains(cookie, "localhost") || strings.Contains(cookie, "127.0.0.1") {
			hostParam = url.QueryEscape(strings.Replace(cookie, "https", "http", 1))
		} else {
			hostParam = url.QueryEscape(cookie)
		}
		c.Redirect(http.StatusFound, Auth.LogoutURL+"?id_token_hint="+accessToken+"&post_logout_redirect_uri="+hostParam)
		return
	}

	var hostParam string
	if strings.Contains(c.Request.Host, "localhost") || strings.Contains(c.Request.Host, "127.0.0.1") {
		hostParam = url.QueryEscape("http://" + c.Request.Host + "/")
	} else {
		hostParam = url.QueryEscape("https://" + c.Request.Host + "/")
	}
	c.Redirect(http.StatusFound, Auth.LogoutURL+"?id_token_hint="+accessToken+"&post_logout_redirect_uri="+hostParam)
}
