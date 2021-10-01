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
	campusonline "github.com/RBG-TUM/CAMPUSOnline"
	"github.com/getsentry/sentry-go"
	"github.com/gin-gonic/gin"
	uuid "github.com/satori/go.uuid"
	log "github.com/sirupsen/logrus"
	"io/ioutil"
	"net/http"
	"strconv"
	"strings"
	"text/template"
	"time"
	"unicode"
)

func configGinLectureHallApiRouter(router *gin.Engine) {
	admins := router.Group("/api")
	admins.Use(tools.Admin)
	admins.POST("/createLectureHall", createLectureHall)
	admins.POST("/takeSnapshot/:lectureHallID/:presetID", takeSnapshot)
	admins.POST("/updateLecturesLectureHall", updateLecturesLectureHall)
	admins.GET("/course-schedule", getSchedule)
	admins.POST("/course-schedule/:year/:term", postSchedule)
	admins.GET("/refreshLectureHallPresets/:lectureHallID", refreshLectureHallPresets)

	adminsOfCourse := router.Group("/api/course/:courseID/")
	adminsOfCourse.Use(tools.InitCourse)
	adminsOfCourse.Use(tools.InitStream)
	adminsOfCourse.Use(tools.AdminOfCourse)
	adminsOfCourse.POST("/switchPreset/:lectureHallID/:presetID/:streamID", switchPreset)

	router.GET("/api/hall/all.ics", lectureHallIcal)
}

func refreshLectureHallPresets(c *gin.Context) {
	lhIDStr := c.Param("lectureHallID")
	lhID, err := strconv.Atoi(lhIDStr)
	if err != nil {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}
	lh, err := dao.GetLectureHallByID(uint(lhID))
	if err != nil {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}
	tools.FetchLHPresets(lh)
}

func postSchedule(c *gin.Context) {
	foundContext, exists := c.Get("TUMLiveContext")
	if !exists {
		sentry.CaptureException(errors.New("context should exist but doesn't"))
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
	tumLiveContext := foundContext.(tools.TUMLiveContext)
	var req []campusonline.Course
	err := c.BindJSON(&req)
	yearStr := c.Param("year")
	year, err := strconv.Atoi(yearStr)
	term := c.Param("term")
	if err != nil || !(term == "W" || term == "S") {
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}
	if err != nil {
		return
	}
	for _, courseReq := range req {
		token := strings.ReplaceAll(uuid.NewV4().String(), "-", "")[:15]

		course := model.Course{
			UserID:              tumLiveContext.User.ID,
			Name:                courseReq.Title,
			Slug:                courseReq.Slug,
			Year:                year,
			TeachingTerm:        term,
			TUMOnlineIdentifier: fmt.Sprintf("%d", courseReq.CourseID),
			LiveEnabled:         true,
			VODEnabled:          true,
			DownloadsEnabled:    false,
			ChatEnabled:         false,
			Visibility:          "loggedin",
			Streams:             nil,
			Users:               nil,
			Token:               token,
		}

		var streams []model.Stream
		for _, event := range courseReq.Events {
			lectureHall, err := dao.GetLectureHallByPartialName(event.RoomName)
			if err != nil {
				log.WithError(err).Error("No room found for request")
				continue
			}
			streams = append(streams, model.Stream{
				Start:         event.Start,
				End:           event.End,
				LectureHallID: lectureHall.ID,
			})
		}
		course.Streams = streams
		log.Println(token)
	}
}

func getSchedule(c *gin.Context) {
	err := c.Request.ParseForm()
	if err != nil {
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}
	rng := strings.Split(c.Request.Form.Get("range"), " to ")
	if len(rng) != 2 {
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}
	from, err := time.Parse("2006-01-02", rng[0])
	if err != nil {
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}
	to, err := time.Parse("2006-01-02", rng[1])
	if err != nil {
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}
	campus, err := campusonline.New(tools.Cfg.CampusToken, "")
	if err != nil {
		log.WithError(err).Error("Can't create campus client")
		return
	}
	room, err := campus.GetXCalOrgIN(from, to)
	if err != nil {
		c.AbortWithStatus(http.StatusInternalServerError)
		log.WithError(err).Error("Can't get room schedule")
		return
	}
	ical := &room
	ical.Filter()
	ical.Sort()
	courses := ical.GroupByCourse()
	courses, err = campus.LoadCourseContacts(courses)
	if err != nil {
		_ = c.AbortWithError(http.StatusInternalServerError, err)
	}
	for _, crs := range courses {
		courseSlug := ""
		print(crs.Title)
		for _, l := range strings.Split(crs.Title, " ") {
			runes := []rune(l)
			if len(runes) != 0 && (unicode.IsNumber(runes[0]) || unicode.IsLetter(runes[0])) {
				courseSlug += string(runes[0])
			}
		}
		println(": ", courseSlug)
	}
	c.JSON(http.StatusOK, courses)
}

//go:embed template
var staticFS embed.FS

func lectureHallIcal(c *gin.Context) {
	templ, err := template.ParseFS(staticFS, "template/*.gotemplate")
	if err != nil {
		return
	}
	foundContext, exists := c.Get("TUMLiveContext")
	if !exists {
		sentry.CaptureException(errors.New("context should exist but doesn't"))
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
	tumLiveContext := foundContext.(tools.TUMLiveContext)
	lectureHalls := dao.GetAllLectureHalls()
	var streams []model.Stream
	courses, err := dao.GetAllCourses(true)
	if err != nil {
		return
	}
	for _, course := range courses {
		if tumLiveContext.User == nil || tumLiveContext.User.Role == model.AdminType || course.UserID == tumLiveContext.User.ID {
			streams = append(streams, course.Streams...)
		}
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
