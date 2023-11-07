package web

import (
	"errors"
	"github.com/TUM-Dev/gocast/tools"
	"github.com/getsentry/sentry-go"
	"github.com/gin-gonic/gin"
	"net/http"
)

func (r mainRoutes) PopOutChat(c *gin.Context) {
	foundContext, exists := c.Get("TUMLiveContext")
	if !exists {
		sentry.CaptureException(errors.New("context should exist but doesn't"))
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}

	var data ChatData

	tumLiveContext := foundContext.(tools.TUMLiveContext)
	data.IndexData = NewIndexData()
	data.IndexData.TUMLiveContext = foundContext.(tools.TUMLiveContext)
	data.IsAdminOfCourse = tumLiveContext.UserIsAdmin()

	err := templateExecutor.ExecuteTemplate(c.Writer, "popup-chat.gohtml", data)
	if err != nil {
		logger.Error("couldn't render template popup-chat.gohtml", "err", err)
	}
}
