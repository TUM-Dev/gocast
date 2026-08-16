package tools

import (
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

type SessionData struct {
	Userid        uint
	SamlSubjectID *string
}

// SessionTTL is how long a browser session cookie stays valid.
const SessionTTL = time.Hour * 24 * 7

// AccessTokenTTL is how long a bearer access token stays valid. Clients hold them in
// memory and re-mint from the session cookie, so they are far shorter-lived.
const AccessTokenTTL = time.Minute * 15

func StartSession(c *gin.Context, data *SessionData) {
	token, err := createToken(data.Userid, data.SamlSubjectID)
	if err != nil {
		logger.Error("Could not create token", "err", err)
		return
	}
	c.SetCookie("jwt", token, int(SessionTTL.Seconds()), "/", "", CookieSecure, true)
}

// CreateAccessToken mints a short-lived `Authorization: Bearer` credential for the v2
// API, signed with the same key and claims as a session cookie.
func CreateAccessToken(user uint, samlSubjectID *string) (string, error) {
	return createTokenWithTTL(user, samlSubjectID, AccessTokenTTL)
}

func createToken(user uint, samlSubjectID *string) (string, error) {
	return createTokenWithTTL(user, samlSubjectID, SessionTTL)
}

func createTokenWithTTL(user uint, samlSubjectID *string, ttl time.Duration) (string, error) {
	t := jwt.New(jwt.GetSigningMethod("RS256"))

	t.Claims = &JWTClaims{
		RegisteredClaims: &jwt.RegisteredClaims{
			ExpiresAt: &jwt.NumericDate{Time: time.Now().Add(ttl)},
		},
		UserID:        user,
		SamlSubjectID: samlSubjectID,
	}
	return t.SignedString(Cfg.GetJWTKey())
}
