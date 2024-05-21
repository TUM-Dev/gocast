package oauth

import (
	"errors"
	"github.com/TUM-Dev/gocast/dao"
	"github.com/TUM-Dev/gocast/tools"
	"github.com/getsentry/sentry-go"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"net/http"
	"slices"
	"strings"
)

// JWTClaims are the claims contained in a session
type JWTClaims struct {
	*jwt.RegisteredClaims
	UserID        uint
	SamlSubjectID *string // identifier of the SAML session (if any)
}

func InitContext(daoWrapper dao.DaoWrapper) gin.HandlerFunc {
	return func(c *gin.Context) {
		// no context initialisation required for static assets.
		if strings.HasPrefix(c.Request.RequestURI, "/static") ||
			strings.HasPrefix(c.Request.RequestURI, "/public") ||
			strings.HasPrefix(c.Request.RequestURI, "/favicon") {
			return
		}

		loggedIn := CheckLoggedIn(c)

		if !loggedIn {
			c.Set("TUMLiveContext", tools.TUMLiveContext{})
		} else {
			uid, err := GetUID(c)
			if uid == "" {
				c.Set("TUMLiveContext", tools.TUMLiveContext{})
				logger.Debug("UID is empty.")
				return
			}
			if err != nil {
				c.Set("TUMLiveContext", tools.TUMLiveContext{})
				logger.Debug("Error getting UID.", "err", err)
				return
			}
			user, err := daoWrapper.UsersDao.GetUserByOAuthID(c, uid)
			if err != nil || user.OAuthID == "" {
				c.Set("TUMLiveContext", tools.TUMLiveContext{})
				logger.Debug("Error getting user by OAuth ID.", "err", err)
				return
			} else {
				c.Set("TUMLiveContext", tools.TUMLiveContext{User: &user, OAuthID: &uid})
				return
			}
		}

		////get the session
		//cookie, err := c.Cookie("jwt")
		//if err != nil {
		//	c.Set("TUMLiveContext", tools.TUMLiveContext{})
		//	return
		//}
		//
		//token, err := jwt.ParseWithClaims(cookie, &JWTClaims{}, func(token *jwt.Token) (interface{}, error) {
		//	key := tools.Cfg.GetJWTKey().Public()
		//	return key, nil
		//})
		//if err != nil {
		//	logger.Info("JWT parsing error: ", "err", err)
		//	c.Set("TUMLiveContext", tools.TUMLiveContext{})
		//	c.SetCookie("jwt", "", -1, "/", "", false, true)
		//	return
		//}
		//if !token.Valid {
		//	logger.Info("JWT token is not valid")
		//	c.Set("TUMLiveContext", tools.TUMLiveContext{})
		//	c.SetCookie("jwt", "", -1, "/", "", false, true)
		//	return
		//}
		//
		//user, err := daoWrapper.UsersDao.GetUserByID(c, token.Claims.(*JWTClaims).UserID)
		//if err != nil {
		//	c.Set("TUMLiveContext", tools.TUMLiveContext{})
		//	return
		//} else {
		//	c.Set("TUMLiveContext", tools.TUMLiveContext{User: &user, SamlSubjectID: token.Claims.(*JWTClaims).SamlSubjectID})
		//	return
		//}
	}
}

func Admin(c *gin.Context) {
	foundContext, exists := c.Get("TUMLiveContext")
	if !exists {
		sentry.CaptureException(errors.New("context should exist but doesn't"))
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
	tumLiveContext := foundContext.(tools.TUMLiveContext)
	if tumLiveContext.User == nil || !slices.Contains(GetGroups(c), "/admin") {
		c.Status(http.StatusForbidden)
		tools.RenderErrorPage(c, http.StatusForbidden, tools.ForbiddenGenericErrMsg)
	}
}

func AtLeastLecturer(c *gin.Context) {
	foundContext, exists := c.Get("TUMLiveContext")
	if !exists {
		sentry.CaptureException(errors.New("context should exist but doesn't"))
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
	tumLiveContext := foundContext.(tools.TUMLiveContext)
	if tumLiveContext.User == nil || (!slices.Contains(GetGroups(c), "/admin") && !slices.Contains(GetGroups(c), "/lecturer")) {
		c.Status(http.StatusForbidden)
		tools.RenderErrorPage(c, http.StatusForbidden, tools.ForbiddenGenericErrMsg)
	}
}

func OwnerOfCourse(c *gin.Context) {
	foundContext, exists := c.Get("TUMLiveContext")
	if !exists {
		sentry.CaptureException(errors.New("context should exist but doesn't"))
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
	tumLiveContext := foundContext.(tools.TUMLiveContext)
	if tumLiveContext.User == nil || (!slices.Contains(GetGroups(c), "/admin") && tumLiveContext.User.OAuthID != tumLiveContext.Course.UserOAuthID) {
		c.Status(http.StatusForbidden)
		tools.RenderErrorPage(c, http.StatusForbidden, tools.ForbiddenGenericErrMsg)
	}
}
