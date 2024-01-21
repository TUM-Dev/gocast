package tools

import (
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v4"
)

const (
	MaxTokenAgeInDays = 7
)

type SessionData struct {
	Userid        uint
	SamlSubjectID *string
}

func StartSession(c *gin.Context, data *SessionData, rememberMe bool) {
	token, err := createToken(data.Userid, data.SamlSubjectID, rememberMe)
	if err != nil {
		logger.Error("Could not create token", "err", err)
		return
	}
	c.SetCookie("jwt", token, 60*60*24*MaxTokenAgeInDays, "/", "", CookieSecure, true)
}

func createToken(user uint, samlSubjectID *string, rememberMe bool) (string, error) {
	t := jwt.New(jwt.GetSigningMethod("RS256"))

	t.Claims = &JWTClaims{
		RegisteredClaims: &jwt.RegisteredClaims{
			IssuedAt:  &jwt.NumericDate{Time: time.Now()},
			ExpiresAt: &jwt.NumericDate{Time: time.Now().Add(time.Hour * 24 * MaxTokenAgeInDays)}, // Token expires in one week
		},
		UpdatedAt:     &jwt.NumericDate{Time: time.Now()},
		UserID:        user,
		SamlSubjectID: samlSubjectID,
		RememberMe:    rememberMe,
	}
	return t.SignedString(Cfg.GetJWTKey())
}
