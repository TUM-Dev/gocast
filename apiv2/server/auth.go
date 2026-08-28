// Package apiv2 provides API endpoints for the application.
package apiv2

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/TUM-Dev/gocast/tools"
)

// accessTokenResponse is returned by the token endpoint. The field names follow
// RFC 6749 so that stock OAuth-style client code can consume it unchanged.
type accessTokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	ExpiresIn   int    `json:"expires_in"`
}

// handleAuthToken exchanges the browser's session cookie for a short-lived bearer
// token, because the cookie is HttpOnly and unreadable from JavaScript. It keeps both
// frontends on one session, whichever performed the login.
//
// Not a login endpoint: it mints only from an existing session. Credentials are still
// exchanged by the redirect-based flows in web/user.go and web/saml.go.
func (a *API) handleAuthToken(c *gin.Context) {
	if c.Request.Method != http.MethodPost {
		c.AbortWithStatusJSON(http.StatusMethodNotAllowed, gin.H{"error": "use POST"})
		return
	}

	// TUMLiveContext is populated by tools.InitContext, which runs for every route.
	foundContext, exists := c.Get("TUMLiveContext")
	if !exists {
		a.log.Error("TUMLiveContext missing on token endpoint")
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "could not read session"})
		return
	}

	tumLiveContext, ok := foundContext.(tools.TUMLiveContext)
	if !ok || tumLiveContext.User == nil {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "no active session"})
		return
	}

	token, err := tools.CreateAccessToken(tumLiveContext.User.ID, tumLiveContext.SamlSubjectID)
	if err != nil {
		a.log.Error("could not create access token", "err", err)
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "could not create token"})
		return
	}

	// Short-lived and re-minted on demand, so it must not be cached anywhere.
	c.Header("Cache-Control", "no-store")
	c.JSON(http.StatusOK, accessTokenResponse{
		AccessToken: token,
		TokenType:   "Bearer",
		ExpiresIn:   int(tools.AccessTokenTTL.Seconds()),
	})
}
