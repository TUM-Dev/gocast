package web

import (
	"TUM-Live/dao"
	"TUM-Live/model"
	"TUM-Live/tools"
	"context"
	"github.com/gin-gonic/gin"
)

func MainPage(c *gin.Context) {
	res, err := dao.AreUsersEmpty(context.Background()) // fresh installation?
	if err != nil {
		_ = templ.ExecuteTemplate(c.Writer, "error.gohtml", nil)
	} else if res {
		_ = templ.ExecuteTemplate(c.Writer, "onboarding.gohtml", nil)
	} else {
		var indexData IndexData
		user, userErr := tools.GetUser(c)
		student, studentErr := tools.GetStudent(c)
		if userErr == nil {
			indexData.IsUser = true
			courses, err := dao.GetCoursesByUserId(context.Background(), user.ID)
			if err == nil {
				indexData.Courses = courses
			}
		} else if studentErr == nil {
			indexData.IsStudent = true
			indexData.Courses = student.Courses
		}
		// Todo get live streams for user
		var streams []model.Stream
		err = dao.GetCurrentLive(context.Background(), &streams)
		indexData.LiveStreams = streams
		public, err := dao.GetPublicCourses()
		if err != nil {
			indexData.PublicCourses = []model.Course{}
		} else {
			indexData.PublicCourses = public
		}
		_ = templ.ExecuteTemplate(c.Writer, "index.gohtml", indexData)
	}
}

func AboutPage(c *gin.Context) {
	var indexData IndexData
	_, userErr := tools.GetUser(c)
	_, studentErr := tools.GetStudent(c)
	if userErr == nil {
		indexData.IsUser = true
	}
	if studentErr == nil {
		indexData.IsStudent = true
	}
	_ = templ.ExecuteTemplate(c.Writer, "about.gohtml", indexData)
}

type IndexData struct {
	IsUser        bool
	IsStudent     bool
	LiveStreams   []model.Stream
	Courses       []model.Course
	PublicCourses []model.Course
}
