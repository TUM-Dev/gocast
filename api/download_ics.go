package api

import (
	"errors"
	"github.com/getsentry/sentry-go"
	"github.com/gin-gonic/gin"
	"github.com/joschahenningsen/TUM-Live/dao"
	"github.com/joschahenningsen/TUM-Live/model"
	"github.com/joschahenningsen/TUM-Live/tools"
	"net/http"
	"strconv"
)

func configGinDownloadICSRouter(router *gin.Engine) {
	router.GET("/api/download_ics/:slug/:term/:year", downloadICS)
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

	slug, term := c.Param("slug"), c.Param("term")
	year, err := strconv.Atoi(c.Param("year"))
	if err != nil {
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}

	course, err := dao.GetCourseBySlugYearAndTerm(c, slug, term, year)
	if err != nil {
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}

	c.Header("content-type", "text/calendar")

	c.JSON(http.StatusOK, streamsToICS(course.Streams))
}

func streamsToICS(streams []model.Stream) []model.Stream {
	return streams
}
