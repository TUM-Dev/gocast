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

	switch c.Query("type") {
	case "serve":
		sendFileContent(c, file)
	case "download":
		fallthrough
	default:
		if !tumLiveContext.User.IsAdminOfCourse(course) {
			if !course.DownloadsEnabled || !(course.Visibility == "hidden" || course.Visibility == "public") ||
				!tumLiveContext.User.IsEligibleToWatchCourse(course) || !tumLiveContext.User.IsAdminOfCourse(course) {
				c.AbortWithStatus(http.StatusForbidden)
				return
			}
		}
		log.Info(fmt.Sprintf("Download request, user: %d, file: %d[%s]", tumLiveContext.User.ID, file.ID, file.Path))
		sendDownloadFile(c, file)
	}
}

func sendFileContent(c *gin.Context, file model.File) {
	image, err := os.ReadFile(file.Path)
	if err != nil {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}

	c.Data(http.StatusOK, "image/jpg", image)
}

func sendDownloadFile(c *gin.Context, file model.File) {
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

	var filename string
	if file.Filename != "" {
		filename = file.Filename
	} else {
		filename = file.GetDownloadFileName()
	}
	c.Header("Content-Description", "File Transfer")
	c.Header("Content-Transfer-Encoding", "binary")
	c.Header("Content-Disposition", "attachment; filename="+filename)
	c.Header("Content-Type", "application/octet-stream")
	c.Header("Content-Length", fmt.Sprintf("%d", stat.Size()))
	c.File(file.Path)
}
