package api

import (
	"TUM-Live/dao"
	"TUM-Live/model"
	"TUM-Live/tools"
	"TUM-Live/tools/tum"
	"context"
	"encoding/json"
	"github.com/gin-gonic/gin"
	"io/ioutil"
	"net/http"
	"regexp"
)

func configGinCourseRouter(router gin.IRoutes) {
	router.POST("/api/courseInfo", courseInfo)
	router.POST("/api/createCourse", createCourse)
}

func createCourse(c *gin.Context) {
	user, err := tools.GetUser(c)
	if err != nil {
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}
	if user.Role > 2 { // not lecturer or admin
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}
	jsonData, err := ioutil.ReadAll(c.Request.Body)
	if err != nil {
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}
	var req createCourseRequest
	err = json.Unmarshal(jsonData, &req)
	if err != nil {
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}
	match, err := regexp.MatchString("(enrolled|public|loggedin)", req.Access)
	if err != nil || !match {
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}
	course := model.Course{
		UserID:              user.ID,
		Name:                req.Name,
		Slug:                req.Slug,
		TUMOnlineIdentifier: req.CourseID,
		VODEnabled:          req.EnVOD,
		DownloadsEnabled:    req.EnDL,
		ChatEnabled:         req.EnChat,
		Streams:             []model.Stream{},
		Students:            []model.Student{},
	}
	err = dao.CreateCourse(context.Background(), course)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, "Couldn't save course. Please reach out to us.")
		return
	}
	// refresh enrollments
	go tum.FindStudentsForAllCourses()
}

func courseInfo(c *gin.Context) {
	user, userErr := tools.GetUser(c)
	if userErr != nil {
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}
	if user.Role > 2 {
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}
	jsonData, err := ioutil.ReadAll(c.Request.Body)
	if err != nil {
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}
	var req getCourseRequest
	err = json.Unmarshal(jsonData, &req)
	if err != nil {
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}
	courseInfo, err := tum.GetCourseInformation(req.CourseID)
	if err != nil { // course not found
		c.AbortWithStatus(http.StatusNotFound)
		return
	}
	c.JSON(200, courseInfo)
}

type getCourseRequest struct {
	CourseID string `json:"courseID"`
}

type createCourseRequest struct {
	Access       string //enrolled, public or loggedin
	CourseID     string
	EnChat       bool
	EnDL         bool
	EnVOD        bool
	Name         string
	Slug         string
	TeachingTerm string
}
