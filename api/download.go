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
		_ = c.Error(tools.RequestError{
			Status:        http.StatusInternalServerError,
			CustomMessage: "context should exist but doesn't",
		})
		return
	}
	tumLiveContext := foundContext.(tools.TUMLiveContext)
	if tumLiveContext.User == nil {
		_ = c.Error(tools.RequestError{
			Status:        http.StatusForbidden,
			CustomMessage: "not logged in",
		})
		return
	}
	file, err := r.FileDao.GetFileById(c, c.Param("id"))
	if err != nil {
		_ = c.Error(tools.RequestError{
			Status:        http.StatusBadRequest,
			CustomMessage: "can not find file",
			Err:           err,
		})
		return
	}
	stream, err := r.StreamsDao.GetStreamByID(c, fmt.Sprintf("%d", file.StreamID))
	if err != nil {
		_ = c.Error(tools.RequestError{
			Status:        http.StatusInternalServerError,
			CustomMessage: "can not get stream",
			Err:           err,
		})
		return
	}
	course, err := r.CoursesDao.GetCourseById(c, stream.CourseID)
	if err != nil {
		_ = c.Error(tools.RequestError{
			Status:        http.StatusInternalServerError,
			CustomMessage: "can not get course",
			Err:           err,
		})
		return
	}

	switch c.Query("type") {
	case "serve":
		sendImageContent(c, file)
	case "download":
		fallthrough
	default:
		if !tumLiveContext.User.IsAdminOfCourse(course) {
			if !course.DownloadsEnabled || !(course.Visibility == "hidden" || course.Visibility == "public") ||
				!tumLiveContext.User.IsEligibleToWatchCourse(course) || !tumLiveContext.User.IsAdminOfCourse(course) {
				_ = c.Error(tools.RequestError{
					Status:        http.StatusForbidden,
					CustomMessage: "user not allowed to get file",
				})
				return
			}
		}
		log.Info(fmt.Sprintf("Download request, user: %d, file: %d[%s]", tumLiveContext.User.ID, file.ID, file.Path))
		sendDownloadFile(c, file)
	}
}

func sendImageContent(c *gin.Context, file model.File) {
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
		_ = c.Error(tools.RequestError{
			Status:        http.StatusNotFound,
			CustomMessage: "can not open file",
			Err:           err,
		})
		return
	}
	defer f.Close()
	stat, err := f.Stat()
	if err != nil {
		_ = c.Error(tools.RequestError{
			Status:        http.StatusInternalServerError,
			CustomMessage: "can not read stats from file",
			Err:           err,
		})
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
