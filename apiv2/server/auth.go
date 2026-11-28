// Package apiv2 provides API endpoints for the application.
package apiv2

import (
	"net/http"

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
func (a *API) handleAuthToken(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		tools.WriteJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "use POST"})
		return
	}

	// TUMLiveContext is populated by tools.InitContext, which runs for every route.
	tumLiveContext, ok := tools.GetTUMLiveContext(r)
	if !ok {
		a.log.Error("TUMLiveContext missing on token endpoint")
		tools.WriteJSON(w, http.StatusInternalServerError, map[string]any{"error": "could not read session"})
		return
	}
	if tumLiveContext.User == nil {
		tools.WriteJSON(w, http.StatusUnauthorized, map[string]any{"error": "no active session"})
		return
	}

	token, err := tools.CreateAccessToken(tumLiveContext.User.ID, tumLiveContext.SamlSubjectID)
	if err != nil {
		a.log.Error("could not create access token", "err", err)
		tools.WriteJSON(w, http.StatusInternalServerError, map[string]any{"error": "could not create token"})
		return
	}

	// Short-lived and re-minted on demand, so it must not be cached anywhere.
	w.Header().Set("Cache-Control", "no-store")
	tools.WriteJSON(w, http.StatusOK, accessTokenResponse{
		AccessToken: token,
		TokenType:   "Bearer",
		ExpiresIn:   int(tools.AccessTokenTTL.Seconds()),
	})
}
