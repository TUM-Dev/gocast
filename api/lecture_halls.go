package api

import (
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/getsentry/sentry-go"
	"github.com/gin-gonic/gin"
	"github.com/joschahenningsen/TUM-Live/dao"
	"github.com/joschahenningsen/TUM-Live/model"
	"github.com/joschahenningsen/TUM-Live/tools"
	log "github.com/sirupsen/logrus"
	"io/ioutil"
	"net/http"
	"strconv"
	"text/template"
	"time"
)

func configGinLectureHallApiRouter(router *gin.Engine) {
	admins := router.Group("/api")
	admins.Use(tools.Admin)
	admins.PUT("/lectureHall/:id", updateLectureHall)
	admins.POST("/lectureHall/:id/defaultPreset", updateLectureHallsDefaultPreset)
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

func updateLectureHallsDefaultPreset(c *gin.Context) {
	var req struct {
		PresetID uint `json:"presetID"`
	}
	err := c.BindJSON(&req)
	if err != nil {
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}
	preset, err := dao.FindPreset(c.Param("id"), fmt.Sprintf("%d", req.PresetID))
	if err != nil {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}
	preset.IsDefault = true
	err = dao.UnsetDefaults(c.Param("id"))
	if err != nil {
		log.WithError(err).Error("Error unsetting default presets")
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
	err = dao.SavePreset(preset)
	if err != nil {
		log.WithError(err).Error("Error saving preset as default")
		c.AbortWithStatus(http.StatusInternalServerError)
		return
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
