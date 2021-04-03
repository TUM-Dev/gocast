package api

import (
	"TUM-Live/dao"
	"TUM-Live/model"
	"TUM-Live/tools"
	"TUM-Live/tools/tum"
	"context"
	"encoding/json"
	"github.com/gin-gonic/gin"
	uuid "github.com/satori/go.uuid"
	"io/ioutil"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"
)

func configGinCourseRouter(router gin.IRoutes) {
	router.POST("/api/courseInfo", courseInfo)
	router.POST("/api/createCourse", createCourse)
	router.POST("/api/createLecture", createLecture)
	router.POST("/api/renameLecture", renameLecture)
}

func renameLecture(c *gin.Context) {
	var req renameLectureRequest
	jsonData, err := ioutil.ReadAll(c.Request.Body)
	if err != nil {
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}
	if err := json.Unmarshal(jsonData, &req); err != nil {
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}
	u, err := tools.GetUser(c)
	if err != nil {
		c.AbortWithStatus(http.StatusForbidden)
		return
	}
	stream, err := dao.GetStreamByID(context.Background(), strconv.Itoa(int(req.Id)))
	if err != nil {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}
	course, _ := dao.GetCourseById(context.Background(), stream.CourseID)
	if u.Role != 1 && course.UserID != u.ID {
		c.AbortWithStatus(http.StatusForbidden)
		return
	}
	stream.Name = req.Name
	if err := dao.UpdateStream(stream); err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, "couldn't update lecture name")
		return
	}
}

func createLecture(c *gin.Context) {
	user, err := tools.GetUser(c)
	if err != nil {
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}
	if user.Role > 2 { // not lecturer or admin
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}
	var req createLectureRequest
	jsonData, err := ioutil.ReadAll(c.Request.Body)
	if err != nil {
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}
	err = json.Unmarshal(jsonData, &req)
	if err != nil {
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}
	u64, err := strconv.Atoi(req.Id)
	courseID := uint(u64)
	course, err := dao.GetCourseById(context.Background(), courseID)
	if err != nil {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}
	if user.Role != 1 && course.UserID != user.ID {
		c.AbortWithStatus(http.StatusForbidden)
		return
	}
	streamKey := uuid.NewV4().String()
	streamKey = strings.ReplaceAll(streamKey, "-", "")
	lecture := model.Stream{
		Name:        req.Name,
		CourseID:    uint(u64),
		Start:       req.Start,
		End:         req.End,
		StreamKey:   streamKey,
		PlaylistUrl: "",
		LiveNow:     false,
	}
	course.Streams = append(course.Streams, lecture)
	dao.UpdateCourse(context.Background(), course)
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

	//verify teaching term input, should either be Sommersemester 2020 or Wintersemester 2020/21
	match, err = regexp.MatchString("(Sommersemester [0-9]{4}|Wintersemester [0-9]{4}/[0-9]{2})$", req.TeachingTerm)
	if err != nil || !match {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"message": "Semester is not in the correct format"})
		return
	}
	reYear := regexp.MustCompile("[0-9]{4}")
	year, err := strconv.Atoi(reYear.FindStringSubmatch(req.TeachingTerm)[0])
	if err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"message": "Semester is not in the correct format"})
		return
	}
	var semester string
	if strings.Contains(req.TeachingTerm, "Wintersemester") {
		semester = "W"
	} else {
		semester = "S"
	}

	course := model.Course{
		UserID:              user.ID,
		Name:                req.Name,
		Slug:                req.Slug,
		Year:                year,
		TeachingTerm:        semester,
		TUMOnlineIdentifier: req.CourseID,
		VODEnabled:          req.EnVOD,
		DownloadsEnabled:    req.EnDL,
		ChatEnabled:         req.EnChat,
		Visibility:          req.Access,
		Streams:             []model.Stream{},
		Students:            []model.Student{},
	}
	err = dao.CreateCourse(context.Background(), course)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, "Couldn't save course. Please reach out to us.")
		return
	}
	// refresh enrollments
	go tum.FetchCourses()
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

type createLectureRequest struct {
	Id    string
	Name  string
	Start time.Time
	End   time.Time
}

type renameLectureRequest struct {
	Id   uint
	Name string
}
