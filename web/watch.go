package web

import (
	"TUM-Live/dao"
	"TUM-Live/model"
	"TUM-Live/tools"
	"context"
	"github.com/getsentry/sentry-go"
	"github.com/gin-gonic/gin"
	"html/template"
	"log"
	"net/http"
)

func WatchPage(c *gin.Context) {
	span := sentry.StartSpan(c, "GET /w", sentry.TransactionName("GET /w"))
	defer span.Finish()
	var data WatchPageData
	if c.Param("version") != "" {
		data.Version = c.Param("version")
	}
	user, userErr := tools.GetUser(c)
	student, studentErr := tools.GetStudent(c)
	data.IndexData = NewIndexData()
	if userErr == nil {
		data.IndexData.IsUser = true
		data.IndexData.IsAdmin = user.Role == model.AdminType || user.Role == model.LecturerType
	}
	if studentErr == nil {
		data.IndexData.IsStudent = true
	}
	vodID := c.Param("id")
	vod, err := dao.GetStreamByID(context.Background(), vodID)
	if err != nil {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}
	data.Stream = vod
	course, err := dao.GetCourseById(context.Background(), vod.CourseID)
	if err != nil {
		log.Printf("couldn't find course for stream: %v\n", err)
		c.AbortWithStatus(http.StatusNotFound)
		return
	}
	if course.Visibility == "loggedin" && userErr != nil && studentErr != nil {
		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"Error": "you are not allowed to watch this lecture."})
		return
	}
	if course.Visibility == "enrolled" && !dao.IsUserAllowedToWatchPrivateCourse(course.ID, user, userErr, student, studentErr) {
		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"Error": "you are not allowed to watch this lecture."})
		return
	}
	data.Course = course
	data.Description = template.HTML(data.Stream.GetDescriptionHTML())
	if c.Param("version") == "video-only" {
		err = templ.ExecuteTemplate(c.Writer, "video_only.gohtml", data)
	}else {
		err = templ.ExecuteTemplate(c.Writer, "watch.gohtml", data)
	}
	if err != nil {
		log.Printf("couldn't render template: %v\n", err)
	}
}

type WatchPageData struct {
	IndexData   IndexData
	Stream      model.Stream
	Description template.HTML
	Course      model.Course
	Version     string
}
