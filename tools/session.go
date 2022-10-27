package tools

import (
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v4"
	log "github.com/sirupsen/logrus"

	"time"
)

type SessionData struct {
	Userid        uint
	SamlSubjectID *string
}

func StartSession(c *gin.Context, data *SessionData) {
	token, err := createToken(data.Userid, data.SamlSubjectID)
	if err != nil {
		log.WithError(err).Error("Could not create token")
		return
	}
	c.SetCookie("jwt", token, 60*60*24*7, "/", "", CookieSecure, true)
}

func createToken(user uint, samlSubjectID *string) (string, error) {
	t := jwt.New(jwt.GetSigningMethod("RS256"))

	t.Claims = &JWTClaims{
		RegisteredClaims: &jwt.RegisteredClaims{
			ExpiresAt: &jwt.NumericDate{Time: time.Now().Add(time.Hour * 24 * 7)}, // Token expires in one week
		},
		UserID:        user,
		SamlSubjectID: samlSubjectID,
	}
	return t.SignedString(Cfg.GetJWTKey())
}
