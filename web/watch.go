package web

import (
	"TUM-Live/dao"
	"TUM-Live/model"
	"TUM-Live/tools"
	"context"
	"github.com/gin-gonic/gin"
	"log"
	"net/http"
)

func WatchPage(c *gin.Context) {
	var data WatchPageData
	if c.Param("version") != "" {
		data.Version = c.Param("version")
	}
	user, userErr := tools.GetUser(c)
	student, studentErr := tools.GetStudent(c)
	if userErr == nil {
		data.IndexData.IsUser = true
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
	err = templ.ExecuteTemplate(c.Writer, "watch.gohtml", data)
	if err != nil {
		log.Printf("couldn't render template: %v\n", err)
	}
}

type WatchPageData struct {
	IndexData IndexData
	Stream    model.Stream
	Course    model.Course
	Version   string
}
