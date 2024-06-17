package oauth

import (
	"errors"
	"github.com/TUM-Dev/gocast/model"
	"github.com/TUM-Dev/gocast/tools"
	"github.com/getsentry/sentry-go"
	"github.com/gin-gonic/gin"
	"net/http"
	"slices"
)

func IsAdminOfCourse(c *gin.Context, course model.Course) bool {
	foundContext, exists := c.Get("TUMLiveContext")
	if !exists {
		sentry.CaptureException(errors.New("context should exist but doesn't"))
		c.AbortWithStatus(http.StatusInternalServerError)
		return false
	}

	tumLiveContext := foundContext.(tools.TUMLiveContext)
	if tumLiveContext.User == nil {
		return false
	}

	if slices.Contains(GetGroups(c), "/admin") {
		return true
	}
	return tumLiveContext.User.IsAdminOfCourse(course)
}
