package api

import (
	"TUM-Live/dao"
	"TUM-Live/model"
	"TUM-Live/tools"
	"context"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/getsentry/sentry-go"
	"github.com/gin-gonic/gin"
	"io/ioutil"
	"log"
	"net/http"
	"strconv"
	"text/template"
	"time"
)

func configGinLectureHallApiRouter(router *gin.Engine) {
	admins := router.Group("/api")
	admins.Use(tools.Admin)
	admins.POST("/createLectureHall", createLectureHall)
	admins.POST("/takeSnapshot/:lectureHallID/:presetID", takeSnapshot)
	admins.POST("/updateLecturesLectureHall", updateLecturesLectureHall)

	adminsOfCourse := router.Group("/api/course/:courseID/")
	adminsOfCourse.Use(tools.InitCourse)
	adminsOfCourse.Use(tools.InitStream)
	adminsOfCourse.Use(tools.AdminOfCourse)
	adminsOfCourse.POST("/switchPreset/:lectureHallID/:presetID/:streamID", switchPreset)

	router.GET("/api/hall/all.ics", lectureHallIcal)
}

//go:embed template
var staticFS embed.FS

func lectureHallIcal(c *gin.Context) {
	templ, err := template.ParseFS(staticFS, "template/*.gotemplate")
	if err != nil {
		return
	}
	lectureHalls := dao.GetAllLectureHalls()
	streams, err := dao.GetAllStreams()
	if err != nil {
		return
	}
	courses, err := dao.GetAllCourses()
	if err != nil {
		return
	}
	c.Header("content-type", "text/calendar")
	err = templ.ExecuteTemplate(c.Writer, "ical.gotemplate", ICALData{streams, lectureHalls, courses})
	if err != nil {
		log.Printf("%v", err)
	}
}

type ICALData struct {
	Streams      []model.Stream
	LectureHalls []model.LectureHall
	Courses      []model.Course
}

func switchPreset(c *gin.Context) {
	foundContext, exists := c.Get("TUMLiveContext")
	if !exists {
		sentry.CaptureException(errors.New("context should exist but doesn't"))
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
	tumLiveContext := foundContext.(tools.TUMLiveContext)
	if tumLiveContext.Stream == nil || !tumLiveContext.Stream.LiveNow {
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}
	preset, err := dao.FindPreset(c.Param("lectureHallID"), c.Param("presetID"))
	if err != nil {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}
	tools.UsePreset(preset)
	time.Sleep(time.Second * 10)
}

func takeSnapshot(c *gin.Context) {
	preset, err := dao.FindPreset(c.Param("lectureHallID"), c.Param("presetID"))
	if err != nil {
		c.AbortWithStatus(http.StatusNotFound)
		sentry.CaptureException(err)
	}
	tools.TakeSnapshot(preset)
	preset, err = dao.FindPreset(c.Param("lectureHallID"), c.Param("presetID"))
	if err != nil {
		c.AbortWithStatus(http.StatusNotFound)
		sentry.CaptureException(err)
	}
	c.JSONP(http.StatusOK, gin.H{"path": fmt.Sprintf("/public/%s", preset.Image)})
}

func updateLecturesLectureHall(c *gin.Context) {
	body, err := ioutil.ReadAll(c.Request.Body)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"msg": "Bad request"})
		return
	}
	var req updateLecturesLectureHallRequest

	if err = json.Unmarshal(body, &req); err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"msg": "Bad request"})
		return
	}
	lecture, err := dao.GetStreamByID(context.Background(), strconv.Itoa(int(req.LectureID)))
	if err != nil {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}
	lectureHall, err := dao.GetLectureHallByID(req.LectureHallID)
	if err != nil {
		dao.UnsetLectureHall(lecture.Model.ID)
		return
	} else {
		lectureHall.Streams = append(lectureHall.Streams, lecture)
		dao.SaveLectureHall(lectureHall)
	}
}

func createLectureHall(c *gin.Context) {
	body, err := ioutil.ReadAll(c.Request.Body)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"msg": "Bad request"})
		return
	}
	var req createLectureHallRequest
	if err = json.Unmarshal(body, &req); err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"msg": "Bad request"})
		return
	}
	dao.CreateLectureHall(model.LectureHall{
		Name:   req.Name,
		CombIP: req.CombIP,
		PresIP: req.PresIP,
		CamIP:  req.CamIP,
	})
}

type createLectureHallRequest struct {
	Name   string `json:"name"`
	CombIP string `json:"combIP"`
	PresIP string `json:"presIP"`
	CamIP  string `json:"camIP"`
}

type updateLecturesLectureHallRequest struct {
	LectureID     uint `json:"lecture"`
	LectureHallID uint `json:"lectureHall"`
}
