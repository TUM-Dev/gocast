package api

import (
	"errors"
	"fmt"
	"github.com/getsentry/sentry-go"
	"github.com/gin-gonic/gin"
	"github.com/joschahenningsen/TUM-Live/dao"
	"github.com/joschahenningsen/TUM-Live/model"
	"github.com/joschahenningsen/TUM-Live/tools"
	log "github.com/sirupsen/logrus"
	"net/http"
	"os"
)

func configGinDownloadRouter(router *gin.Engine, daoWrapper dao.DaoWrapper) {
	routes := downloadRoutes{daoWrapper}
	router.GET("/api/download/:id", routes.download)
}

type downloadRoutes struct {
	dao.DaoWrapper
}

func (r downloadRoutes) download(c *gin.Context) {
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
	file, err := r.FileDao.GetFileById(c.Param("id"))
	if err != nil {
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}

	stream, err := r.StreamsDao.GetStreamByID(c, fmt.Sprintf("%d", file.StreamID))
	if err != nil {
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
	course, err := r.CoursesDao.GetCourseById(c, stream.CourseID)
	if err != nil {
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
	log.Info(fmt.Sprintf("Download request, user: %d, file: %d[%s]", tumLiveContext.User.ID, file.ID, file.Path))
	if tumLiveContext.User.IsAdminOfCourse(course) {
		sendFile(c, file)
		return
	}
	if course.DownloadsEnabled {
		if course.Visibility == "hidden" || course.Visibility == "public" {
			sendFile(c, file)
			return
		}
		if tumLiveContext.User.IsEligibleToWatchCourse(course) {
			sendFile(c, file)
			return
		}
	}
	c.AbortWithStatus(http.StatusForbidden)
}

func sendFile(c *gin.Context, file model.File) {
	f, err := os.Open(file.Path)
	if err != nil {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}
	defer f.Close()
	stat, err := f.Stat()
	if err != nil {
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
	c.Header("Content-Description", "File Transfer")
	c.Header("Content-Transfer-Encoding", "binary")
	c.Header("Content-Disposition", "attachment; filename="+file.GetDownloadFileName())
	c.Header("Content-Type", "application/octet-stream")
	c.Header("Content-Length", fmt.Sprintf("%d", stat.Size()))
	c.File(file.Path)
}
