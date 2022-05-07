package api

import (
	"errors"
	"github.com/getsentry/sentry-go"
	"github.com/gin-gonic/gin"
	"github.com/joschahenningsen/TUM-Live/tools"
	"net/http"
	_ "time"
)

func configGinDownloadICSRouter(router gin.IRoutes) {
	router.GET("/api/download_ics", downloadICS)
}

func downloadICS(c *gin.Context) {
	foundContext, exists := c.Get("TUMLiveContext")
	if !exists {
		sentry.CaptureException(errors.New("context should exist but doesn't"))
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
	tumLiveContext := foundContext.(tools.TUMLiveContext)
	if tumLiveContext.User == nil {
		c.AbortWithStatus(http.StatusForbidden)
		return
	}
	var streams = tumLiveContext.Course.Streams

	c.JSON(http.StatusOK, streams)
}
