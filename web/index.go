package web

import (
	"TUM-Live/dao"
	"TUM-Live/model"
	"TUM-Live/tools"
	"TUM-Live/tools/tum"
	"context"
	"github.com/gin-gonic/gin"
	"net/http"
	"strconv"
)

func MainPage(c *gin.Context) {
	res, err := dao.AreUsersEmpty(context.Background()) // fresh installation?
	if err != nil {
		_ = templ.ExecuteTemplate(c.Writer, "error.gohtml", nil)
		return
	} else if res {
		_ = templ.ExecuteTemplate(c.Writer, "onboarding.gohtml", nil)
		return
	}
	var indexData IndexData
	user, userErr := tools.GetUser(c)
	student, studentErr := tools.GetStudent(c)

	var year int
	var term string
	if c.Param("year") == "" {
		year, term = tum.GetCurrentSemester()
	} else {
		term = c.Param("term")
		year, err = strconv.Atoi(c.Param("year"))
		if err != nil || (term != "W" && term != "S") {
			c.AbortWithStatusJSON(http.StatusNotFound, gin.H{"message": "Bad semester format in url."})
			return
		}
	}
	indexData.Semesters = dao.GetAvailableSemesters()
	indexData.CurrentYear = year
	indexData.CurrentTerm = term
	if userErr == nil {
		indexData.IsUser = true
		indexData.Courses = user.CoursesForSemester(year, term)
	} else if studentErr == nil {
		indexData.IsStudent = true
		indexData.Courses = student.CoursesForSemester(year, term)
	}
	// Todo get live streams for user
	streams, err := dao.GetCurrentLive(context.Background())
	indexData.LiveStreams = streams
	public, err := dao.GetPublicCourses(year, term)
	if err != nil {
		indexData.PublicCourses = []model.Course{}
	} else {
		// filter out courses that already are in "my courses"
		var publicFiltered []model.Course
		for _, c := range public {
			if !tools.CourseListContains(indexData.Courses, c.ID) {
				publicFiltered = append(publicFiltered, c)
			}
		}
		indexData.PublicCourses = publicFiltered
	}
	_ = templ.ExecuteTemplate(c.Writer, "index.gohtml", indexData)
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
	Semesters     []dao.Semester
	CurrentYear   int
	CurrentTerm   string
}
