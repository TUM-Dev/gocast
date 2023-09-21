package web

import (
	"errors"
	"github.com/getsentry/sentry-go"
	"github.com/gin-gonic/gin"
	"github.com/TUM-Dev/gocast/tools"
	"log"
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
		log.Printf("couldn't render template: %v\n", err)
	}
}
