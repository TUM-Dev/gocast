package api

import (
	"TUM-Live/dao"
	"TUM-Live/tools"
	"context"
	"errors"
	"github.com/getsentry/sentry-go"
	"github.com/gin-gonic/gin"
	"log"
	"net/http"
)

func configGinDownloadRouter(router *gin.Engine) {
	router.GET("/api/download/:id/:slug/:name", downloadVod)
}

func downloadVod(c *gin.Context) {
	foundContext, exists := c.Get("TUMLiveContext")
	if !exists {
		sentry.CaptureException(errors.New("context should exist but doesn't"))
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
	tumLiveContext := foundContext.(tools.TUMLiveContext)
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
	//student, serr := tools.GetStudent(c)
	if !course.DownloadsEnabled {
		// only allow for owner or admin
		if tumLiveContext.User == nil {
			log.Printf("Deny download, cause: download disabled but not logged in")
			c.AbortWithStatus(http.StatusNotFound)
			return
		} else {
			if tumLiveContext.User.Role > 1 && tumLiveContext.User.ID != course.UserID {
				log.Printf("Deny download, cause: download disabled and user not admin or owner.")
				c.AbortWithStatus(http.StatusNotFound)
				return
			}
		}
	} else if course.Visibility != "public" {
		if tumLiveContext.User != nil {
			if tumLiveContext.User.Role > 1 && tumLiveContext.User.ID != course.UserID {
				log.Printf("Deny download, cause: user but not admin or owner")
				// logged in as user but not owner of course or admin
				c.AbortWithStatus(http.StatusNotFound)
				return
			}
		} /*else if serr == nil { todo
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
		}*/
	}
	// public -> download
	log.Printf("Download stream: %v", stream.FilePath)
	c.File(stream.FilePath)
}
