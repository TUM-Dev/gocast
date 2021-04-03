package api

import (
	"TUM-Live/dao"
	"TUM-Live/tools"
	"context"
	"github.com/gin-gonic/gin"
	"net/http"
)

func configGinDownloadRouter(router gin.IRoutes) {
	router.GET("/api/download/:id/:slug/:name", downloadVod)
}

func downloadVod(c *gin.Context) {
	stream, err := dao.GetStreamByID(context.Background(), c.Param("id"))
	if err != nil || !stream.Recording {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}
	course, err := dao.GetCourseById(context.Background(), stream.CourseID)
	if err != nil || !course.DownloadsEnabled {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}
	if course.Visibility != "public" {
		user, uerr := tools.GetUser(c)
		student, serr := tools.GetStudent(c)
		if uerr == nil {
			if user.Role > 1 && !user.IsAdminOfCourse(course.ID) {
				// logged in as user but not owner of course or admin
				c.AbortWithStatus(http.StatusNotFound)
				return
			}
		} else if serr == nil {
			canDownload := false
			for _, studentCourse := range student.Courses {
				if studentCourse.ID == course.ID {
					canDownload = true
					break
				}
			}
			if !canDownload {
				// student but not allowed enrolled to course
				c.AbortWithStatus(http.StatusNotFound)
				return
			}
		} else {
			// not logged in to a course that is private.
			c.AbortWithStatus(http.StatusNotFound)
			return
		}
	}
	// public -> download
	c.File(stream.FilePath)
}
