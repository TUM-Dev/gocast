package tools

import (
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v4"
)

type SessionData struct {
	Userid        uint
	SamlSubjectID *string
}

func StartSession(c *gin.Context, data *SessionData, rememberMe bool) {
	maxAgeInDays := 7 // by default, log-in status expires in one week
	if rememberMe {
		maxAgeInDays = 30 * 6 // if user chooses "remember me", let log-in status be valid for 6 months
	}

	token, err := createToken(data.Userid, data.SamlSubjectID, maxAgeInDays)
	if err != nil {
		logger.Error("Could not create token", "err", err)
		return
	}
	c.SetCookie("jwt", token, 60*60*24*maxAgeInDays, "/", "", CookieSecure, true)
}

func createToken(user uint, samlSubjectID *string, maxAgeInDays int) (string, error) {
	t := jwt.New(jwt.GetSigningMethod("RS256"))

	t.Claims = &JWTClaims{
		RegisteredClaims: &jwt.RegisteredClaims{
			ExpiresAt: &jwt.NumericDate{Time: time.Now().Add(time.Hour * 24 * time.Duration(maxAgeInDays))}, // Token expires in one week
		},
		UserID:        user,
		SamlSubjectID: samlSubjectID,
	}
	return t.SignedString(Cfg.GetJWTKey())
}
