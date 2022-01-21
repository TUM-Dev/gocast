package api

import (
	"TUM-Live/dao"
	"TUM-Live/model"
	"TUM-Live/tools"
	"bytes"
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
	admins.PUT("/lectureHall/:id", updateLectureHall)
	admins.DELETE("/lectureHall/:id", deleteLectureHall)
	admins.POST("/createLectureHall", createLectureHall)
	admins.POST("/takeSnapshot/:lectureHallID/:presetID", takeSnapshot)
	admins.GET("/course-schedule", getSchedule)
	admins.POST("/course-schedule/:year/:term", postSchedule)
	admins.GET("/refreshLectureHallPresets/:lectureHallID", refreshLectureHallPresets)
	admins.POST("/setLectureHall", setLectureHall)

	adminsOfCourse := router.Group("/api/course/:courseID/")
	adminsOfCourse.Use(tools.InitCourse)
	adminsOfCourse.Use(tools.InitStream)
	adminsOfCourse.Use(tools.AdminOfCourse)
	adminsOfCourse.POST("/switchPreset/:lectureHallID/:presetID/:streamID", switchPreset)

	router.GET("/api/hall/all.ics", lectureHallIcal)
}

type updateLectureHallReq struct {
	CamIp     string `json:"camIp"`
	CombIp    string `json:"combIp"`
	PresIP    string `json:"presIp"`
	CameraIp  string `json:"cameraIp"`
	PwrCtrlIp string `json:"pwrCtrlIp"`
}

func updateLectureHall(c *gin.Context) {
	var req updateLectureHallReq
	err := c.BindJSON(&req)
	if err != nil {
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}
	id := c.Param("id")
	idUint, err := strconv.ParseUint(id, 10, 32)
	if err != nil {
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}
	lectureHall, err := dao.GetLectureHallByID(uint(idUint))
	if err != nil {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}
	lectureHall.CamIP = req.CamIp
	lectureHall.CombIP = req.CombIp
	lectureHall.PresIP = req.PresIP
	lectureHall.CameraIP = req.CameraIp
	lectureHall.PwrCtrlIp = req.PwrCtrlIp
	err = dao.SaveLectureHall(lectureHall)
	if err != nil {
		log.WithError(err).Error("Error while updating lecture hall")
		c.AbortWithStatus(http.StatusInternalServerError)
	}
}

func deleteLectureHall(c *gin.Context) {
	lhIDStr := c.Param("id")
	lhID, err := strconv.Atoi(lhIDStr)
	if err != nil {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}

	err = dao.DeleteLectureHall(uint(lhID))
	if err != nil {
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
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
	resp := ""
	foundContext, exists := c.Get("TUMLiveContext")
	if !exists {
		sentry.CaptureException(errors.New("context should exist but doesn't"))
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
	tumLiveContext := foundContext.(tools.TUMLiveContext)
	var req []campusonline.Course
	err := c.BindJSON(&req)
	if err != nil {
		sentry.CaptureException(errors.New("could not bind JSON request"))
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
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
		if !courseReq.Import {
			continue
		}
		token := strings.ReplaceAll(uuid.NewV4().String(), "-", "")[:15]
		course := model.Course{
			UserID:              tumLiveContext.User.ID,
			Name:                courseReq.Title,
			Slug:                courseReq.Slug,
			Year:                year,
			TeachingTerm:        term,
			TUMOnlineIdentifier: fmt.Sprintf("%d", courseReq.CourseID),
			LiveEnabled:         false,
			VODEnabled:          false,
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
				RoomName:      event.RoomName,
				LectureHallID: lectureHall.ID,
			})
		}
		course.Streams = streams
		//todo let user pick keep -> opt in/out
		err := dao.CreateCourse(c, course, false)
		if err != nil {
			resp += err.Error()
		} else {
			name := ""
			mail := ""
			for _, contact := range courseReq.Contacts {
				if contact.MainContact {
					name = contact.FirstName + " " + contact.LastName
					mail = contact.Email
					break
				}
			}
			err := notifyCourseCreated(MailTmpl{
				Name:   name,
				Course: course,
			}, mail, fmt.Sprintf("Vorlesungsstreams %s | Lecture streaming %s", course.Name, course.Name))
			if err != nil {
				log.WithFields(log.Fields{"course": course.Name, "email": mail}).WithError(err).Error("cant send email")
			}
			time.Sleep(time.Millisecond * 100) // 1/10th second delay, being nice to our mailrelay
		}
	}
	if resp != "" {
		c.AbortWithStatusJSON(http.StatusInternalServerError, resp)
	}
}

type MailTmpl struct {
	Name   string
	Course model.Course
}

func notifyCourseCreated(d MailTmpl, mailAddr string, subject string) error {
	templ, err := template.ParseFS(staticFS, "template/*.gotemplate")
	if err != nil {
		return err
	}
	log.Println(mailAddr)
	var body bytes.Buffer
	_ = templ.ExecuteTemplate(&body, "mail-course-registered.gotemplate", d)
	return tools.SendMail(tools.Cfg.Mail.Server, tools.Cfg.Mail.Sender, subject, body.String(), []string{mailAddr})
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
	//todo figure out right token
	campus, err := campusonline.New(tools.Cfg.Campus.Tokens[0], "")
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
		for _, l := range strings.Split(crs.Title, " ") {
			runes := []rune(l)
			if len(runes) != 0 && (unicode.IsNumber(runes[0]) || unicode.IsLetter(runes[0])) {
				courseSlug += string(runes[0])
			}
		}
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
	// pass 0 to db query to get all lectures if user is not logged in or admin
	queryUid := uint(0)
	if tumLiveContext.User != nil && tumLiveContext.User.Role != model.AdminType {
		queryUid = tumLiveContext.User.ID
	}
	icalData, err := dao.GetStreamsForLectureHallIcal(queryUid)
	if err != nil {
		return
	}
	c.Header("content-type", "text/calendar")
	err = templ.ExecuteTemplate(c.Writer, "ical.gotemplate", icalData)
	if err != nil {
		log.Printf("%v", err)
	}
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

func setLectureHall(c *gin.Context) {
	var req setLectureHallRequest
	err := c.BindJSON(&req)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"msg": "Bad request"})
		return
	}

	streams, err := dao.GetStreamsByIds(req.StreamIDs)
	if err != nil || len(streams) != len(req.StreamIDs) {
		log.WithError(err).Error("Can't get all streams to update lecture hall")
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}

	if req.LectureHallID == 0 {
		err = dao.UnsetLectureHall(req.StreamIDs)
		if err != nil {
			log.WithError(err).Error("Can't update lecture hall for streams")
			c.AbortWithStatus(http.StatusInternalServerError)
		}
		return
	}

	_, err = dao.GetLectureHallByID(req.LectureHallID)
	if err != nil {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}
	err = dao.SetLectureHall(req.StreamIDs, req.LectureHallID)
	if err != nil {
		log.WithError(err).Error("can't update lecture hall")
		c.AbortWithStatus(http.StatusInternalServerError)
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
		Name:      req.Name,
		CombIP:    req.CombIP,
		PresIP:    req.PresIP,
		CamIP:     req.CamIP,
		CameraIP:  req.CameraIP,
		PwrCtrlIp: req.PwrCtrlIP,
	})
}

type createLectureHallRequest struct {
	Name      string `json:"name"`
	CombIP    string `json:"combIP"`
	PresIP    string `json:"presIP"`
	CamIP     string `json:"camIP"`
	CameraIP  string `json:"cameraIP"`
	PwrCtrlIP string `json:"pwrCtrlIp"`
}

type setLectureHallRequest struct {
	StreamIDs     []uint `json:"streamIDs"`
	LectureHallID uint   `json:"lectureHall"`
}
