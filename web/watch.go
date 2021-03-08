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
	_, userErr := tools.GetUser(c)
	_, studentErr := tools.GetStudent(c)
	if userErr == nil {
		data.IndexData.IsUser = true
	}
	if studentErr == nil {
		// todo: is student allowed to watch?
		data.IndexData.IsStudent = true
	}
	streamID := c.Param("id")
	stream, err := dao.GetStreamByID(context.Background(), streamID)
	if err != nil {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}
	data.Stream = stream

	course, err := dao.GetCourseById(context.Background(), stream.CourseID)
	if err!=nil {
		log.Printf("couldn't find course for stream: %v\n", err)
		c.AbortWithStatus(http.StatusNotFound)
	}
	data.Course = course
	println(stream.Name)
	err = templ.ExecuteTemplate(c.Writer, "watch.gohtml", data)
	if err != nil {
		log.Printf("couldn't render template: %v\n", err)
	}
}

type WatchPageData struct {
	IndexData IndexData
	Stream    model.Stream
	Course    model.Course
}
