package api

import (
	"TUM-Live/dao"
	"TUM-Live/tools"
	"context"
	"github.com/gin-gonic/gin"
	"log"
	"net/http"
)

func configGinDownloadRouter(router gin.IRoutes) {
	router.GET("/api/download/:id/:slug/:name", downloadVod)
}

func downloadVod(c *gin.Context) {
	stream, err := dao.GetStreamByID(context.Background(), c.Param("id"))
	if err != nil || !stream.Recording {
		log.Printf("Deny download, cause: error or not recording: %v", err)
		c.AbortWithStatus(http.StatusNotFound)
		return
	}
	course, err := dao.GetCourseById(context.Background(), stream.CourseID)
	if err != nil {
		log.Printf("Deny download, cause: error or download disabled: %v", err)
		c.AbortWithStatus(http.StatusNotFound)
		return
	}
	if course.ID != stream.CourseID {
		log.Printf("Deny download, cause: courseid and stream-courseid don't match")
		c.AbortWithStatus(http.StatusNotFound)
		return
	}
	user, uerr := tools.GetUser(c)
	student, serr := tools.GetStudent(c)
	if !course.DownloadsEnabled {
		// only allow for owner or admin
		if uerr != nil {
			log.Printf("Deny download, cause: download disabled but not logged in")
			c.AbortWithStatus(http.StatusNotFound)
			return
		} else {
			if user.Role > 1 && !user.IsAdminOfCourse(course.ID) {
				log.Printf("Deny download, cause: download disabled and user not admin or owner.")
				c.AbortWithStatus(http.StatusNotFound)
				return
			}
		}
	} else if course.Visibility != "public" {
		if uerr == nil {
			if user.Role > 1 && !user.IsAdminOfCourse(course.ID) {
				log.Printf("Deny download, cause: user but not admin or owner")
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
				log.Printf("Deny download, cause: student but not their course.")
				// student but not allowed enrolled to course
				c.AbortWithStatus(http.StatusNotFound)
				return
			}
		} else {
			log.Printf("Deny download, private course but not logged in")
			// not logged in to a course that is private.
			c.AbortWithStatus(http.StatusNotFound)
			return
		}
	}
	// public -> download
	log.Printf("Download stream: %v", stream.FilePath)
	c.File(stream.FilePath)
}
