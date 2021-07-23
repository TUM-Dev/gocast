package api

import (
	"TUM-Live/dao"
	"TUM-Live/model"
	"TUM-Live/tools"
	"TUM-Live/tools/tum"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/getsentry/sentry-go"
	"github.com/gin-gonic/gin"
	uuid "github.com/satori/go.uuid"
	log "github.com/sirupsen/logrus"
	"gorm.io/gorm"
	"io"
	"io/ioutil"
	"mime/multipart"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"
)

func configGinCourseRouter(router *gin.Engine) {
	atLeastLecturerGroup := router.Group("/")
	atLeastLecturerGroup.Use(tools.AtLeastLecturer)
	atLeastLecturerGroup.POST("/api/courseInfo", courseInfo)
	atLeastLecturerGroup.POST("/api/createCourse", createCourse)

	adminOfCourseGroup := router.Group("/api/course/:courseID")
	adminOfCourseGroup.Use(tools.InitCourse)
	adminOfCourseGroup.Use(tools.AdminOfCourse)
	adminOfCourseGroup.POST("/createLecture", createLecture)
	adminOfCourseGroup.POST("/deleteLecture/:streamID", deleteLecture)
	adminOfCourseGroup.POST("/renameLecture/:streamID", renameLecture)
	adminOfCourseGroup.POST("/updateDescription/:streamID", updateDescription)
	adminOfCourseGroup.POST("/addUnit", addUnit)
	adminOfCourseGroup.POST("/submitCut", submitCut)
	adminOfCourseGroup.POST("/deleteUnit/:unitID", deleteUnit)
	adminOfCourseGroup.GET("/stats", getStats)
}

func submitCut(c *gin.Context) {
	body, err := ioutil.ReadAll(c.Request.Body)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"msg": "bad request"})
		return
	}
	var req submitCutRequest
	if err = json.Unmarshal(body, &req); err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"msg": "bad request"})
		return
	}
	stream, err := dao.GetStreamByID(context.Background(), strconv.Itoa(int(req.LectureID)))
	if err != nil {
		c.AbortWithStatusJSON(http.StatusNotFound, gin.H{"msg": "stream not found"})
		return
	}
	stream.StartOffset = req.From
	stream.EndOffset = req.To
	if err = dao.SaveStream(&stream); err != nil {
		panic(err)
	}
}

type submitCutRequest struct {
	LectureID uint `json:"lectureID"`
	From      uint `json:"from"`
	To        uint `json:"to"`
}

func deleteUnit(c *gin.Context) {
	unit, err := dao.GetUnitByID(c.Param("unitID"))
	if err != nil {
		c.AbortWithStatusJSON(http.StatusNotFound, gin.H{"msg": "not found"})
		return
	}
	dao.DeleteUnit(unit.Model.ID)
}

func addUnit(c *gin.Context) {
	body, err := ioutil.ReadAll(c.Request.Body)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"msg": "bad request"})
		return
	}
	var req addUnitRequest
	if err = json.Unmarshal(body, &req); err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"msg": "bad request"})
		return
	}
	stream, err := dao.GetStreamByID(context.Background(), strconv.Itoa(int(req.LectureID)))
	if err != nil {
		c.AbortWithStatusJSON(http.StatusNotFound, gin.H{"msg": "stream not found"})
		return
	}
	stream.Units = append(stream.Units, model.StreamUnit{
		UnitName:        req.Title,
		UnitDescription: req.Description,
		UnitStart:       req.From,
		UnitEnd:         req.To,
		StreamID:        stream.Model.ID,
	})
	if err = dao.UpdateStreamFullAssoc(&stream); err != nil {
		panic(err)
	}
}

type addUnitRequest struct {
	LectureID   uint   `json:"lectureID"`
	From        uint   `json:"from"`
	To          uint   `json:"to"`
	Title       string `json:"title"`
	Description string `json:"description"`
}

func updateDescription(c *gin.Context) {
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
	stream, err := dao.GetStreamByID(context.Background(), c.Param("streamID"))
	if err != nil {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}
	stream.Description = req.Name
	if err := dao.UpdateStream(stream); err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, "couldn't update lecture Description")
		return
	}
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
	stream, err := dao.GetStreamByID(context.Background(), c.Param("streamID"))
	if err != nil {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}
	stream.Name = req.Name
	if err := dao.UpdateStream(stream); err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, "couldn't update lecture name")
		return
	}
}

type renameLectureRequest struct {
	Name string
}

func deleteLecture(c *gin.Context) {
	foundContext, exists := c.Get("TUMLiveContext")
	if !exists {
		sentry.CaptureException(errors.New("context should exist but doesn't"))
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
	tumLiveContext := foundContext.(tools.TUMLiveContext)
	stream, err := dao.GetStreamByID(context.Background(), c.Param("streamID"))
	if err != nil || stream.CourseID != tumLiveContext.Course.ID {
		c.AbortWithStatus(http.StatusForbidden)
		return
	}
	stream.Model.DeletedAt = gorm.DeletedAt{Time: time.Now()} // todo ?!
	dao.DeleteStream(strconv.Itoa(int(stream.ID)))
}

func createLecture(c *gin.Context) {
	foundContext, exists := c.Get("TUMLiveContext")
	if !exists {
		sentry.CaptureException(errors.New("context should exist but doesn't"))
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
	tumLiveContext := foundContext.(tools.TUMLiveContext)
	var req createLectureRequest
	if err := c.ShouldBind(&req); err != nil {
		log.WithError(err).Error("invalid form")
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}
	// name for folder for premiere file if needed
	premiereFolder := fmt.Sprintf("%s/%d/%s/%s",
		tools.Cfg.MassStorage,
		tumLiveContext.Course.Year,
		tumLiveContext.Course.TeachingTerm,
		tumLiveContext.Course.Slug)
	premiereFileName := fmt.Sprintf("%s_%s.mp4",
		tumLiveContext.Course.Slug,
		req.Start.Format("2006-01-02_15-04"))
	if req.Premiere {
		err := os.MkdirAll(premiereFolder, os.ModePerm)
		if err != nil {
			c.AbortWithStatus(http.StatusInternalServerError)
			log.WithError(err).Error("Can't create folder for premiere")
			return
		}
		// Copy file to shared storage
		file, err := os.Create(fmt.Sprintf("%s/%s", premiereFolder, premiereFileName))
		if err != nil {
			log.WithError(err).Error("Can't create file for premiere")
			c.AbortWithStatus(http.StatusInternalServerError)
			return
		}
		reqFile, _ := req.File.Open()
		_, err = io.Copy(file, reqFile)
		if err != nil {
			log.WithError(err).Error("Can't write file for premiere")
			c.AbortWithStatus(http.StatusInternalServerError)
		}
		_ = file.Close()
	}
	streamKey := uuid.NewV4().String()
	streamKey = strings.ReplaceAll(streamKey, "-", "")
	lecture := model.Stream{
		Name:        req.Title,
		CourseID:    tumLiveContext.Course.ID,
		Start:       req.Start,
		End:         req.End,
		StreamKey:   streamKey,
		PlaylistUrl: "",
		LiveNow:     false,
		Premiere:    req.Premiere,
		FilePath:    fmt.Sprintf("%s/%s", premiereFolder, premiereFileName),
	}
	// remove file path if not premiere
	if !req.Premiere {
		lecture.FilePath = ""
	}
	tumLiveContext.Course.Streams = append(tumLiveContext.Course.Streams, lecture)
	dao.UpdateCourse(context.Background(), *tumLiveContext.Course)
}

type createLectureRequest struct {
	Title    string                `form:"title"`
	Start    time.Time             `form:"start"`
	End      time.Time             `form:"end"`
	Premiere bool                  `form:"premiere"`
	File     *multipart.FileHeader `form:"file"`
}

func createCourse(c *gin.Context) {
	foundContext, exists := c.Get("TUMLiveContext")
	if !exists {
		sentry.CaptureException(errors.New("context should exist but doesn't"))
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
	tumLiveContext := foundContext.(tools.TUMLiveContext)
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
		UserID:              tumLiveContext.User.ID,
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
	}
	err = dao.CreateCourse(context.Background(), course)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, "Couldn't save course. Please reach out to us.")
		return
	}
	courseWithID, err := dao.GetCourseBySlugYearAndTerm(context.Background(), req.Slug, semester, fmt.Sprintf("%v", year))
	// refresh enrollments and lectures
	courses := make([]model.Course, 1)
	courses[0] = courseWithID
	go tum.GetEventsForCourses(courses)
	go tum.FindStudentsForCourses(courses)
	go tum.FetchCourses()
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

func courseInfo(c *gin.Context) {
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
