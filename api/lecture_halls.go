package api

import (
	"embed"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"text/template"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/TUM-Dev/gocast/dao"
	"github.com/TUM-Dev/gocast/model"
	"github.com/TUM-Dev/gocast/pkg/camera"
	"github.com/TUM-Dev/gocast/tools"
)

var camSwitchDelay = time.Second * 5

func configGinLectureHallApiRouter(router *gin.Engine, daoWrapper dao.DaoWrapper, camService CamService, presetImageDir string) {
	routes := lectureHallRoutes{
		DaoWrapper:     daoWrapper,
		cameraService:  camService,
		presetImageDir: presetImageDir,
	}

	admins := router.Group("/api")
	admins.Use(tools.RequirePermission(model.PermAdministerServer))
	admins.PUT("/lectureHall/:id", routes.updateLectureHall)
	admins.POST("/lectureHall/:id/defaultPreset", routes.updateLectureHallsDefaultPreset)
	admins.DELETE("/lectureHall/:id", routes.deleteLectureHall)
	admins.POST("/createLectureHall", routes.createLectureHall)
	admins.POST("/takeSnapshot/:lectureHallID/:presetID", routes.takeSnapshot)
	admins.GET("/course-schedule", routes.getSchedule)
	admins.POST("/course-schedule/:year/:term", routes.postSchedule)
	admins.GET("/refreshLectureHallPresets/:lectureHallID", routes.refreshLectureHallPresets)
	admins.POST("/setLectureHall", routes.setLectureHall)

	adminsOfCourse := router.Group("/api/course/:courseID/")
	adminsOfCourse.Use(tools.InitCourse(daoWrapper))
	adminsOfCourse.Use(tools.InitStream(daoWrapper))
	adminsOfCourse.Use(tools.AdminOfCourse)
	adminsOfCourse.POST("/switchPreset/:lectureHallID/:presetID/:streamID", routes.switchPreset)

	router.GET("/api/schedule.ics", routes.lectureHallIcal)
}

type CamService interface {
	For(address string, cameraType model.CameraType) (camera.Cam, error)
}

type lectureHallRoutes struct {
	dao.DaoWrapper

	presetImageDir string
	cameraService  CamService
}

type updateLectureHallReq struct {
	StreamProtocol int    `json:"streamProtocol"`
	CamIp          string `json:"camIp"`
	CombIp         string `json:"combIp"`
	PresIP         string `json:"presIp"`
	CameraIp       string `json:"cameraIp"`
	PwrCtrlIp      string `json:"pwrCtrlIp"`
}

func (r lectureHallRoutes) updateLectureHall(c *gin.Context) {
	var req updateLectureHallReq
	err := c.BindJSON(&req)
	if err != nil {
		_ = c.Error(tools.RequestError{
			Status:        http.StatusBadRequest,
			CustomMessage: "can not bind body",
			Err:           err,
		})
		return
	}
	id := c.Param("id")
	idUint, err := strconv.ParseUint(id, 10, 32)
	if err != nil {
		_ = c.Error(tools.RequestError{
			Status:        http.StatusBadRequest,
			CustomMessage: "invalid param 'id'",
			Err:           err,
		})
		return
	}
	lectureHall, err := r.LectureHallsDao.GetLectureHallByID(uint(idUint))
	if err != nil {
		_ = c.Error(tools.RequestError{
			Status:        http.StatusNotFound,
			CustomMessage: "can not find lecture hall",
			Err:           err,
		})
		return
	}
	lectureHall.StreamProtocol = model.StreamProtocol(req.StreamProtocol)
	logger.Debug("New stream protocol", "protocol", lectureHall.StreamProtocol)
	lectureHall.CamIP = req.CamIp
	lectureHall.CombIP = req.CombIp
	lectureHall.PresIP = req.PresIP
	lectureHall.CameraIP = req.CameraIp
	lectureHall.PwrCtrlIp = req.PwrCtrlIp
	err = r.LectureHallsDao.SaveLectureHall(lectureHall)
	if err != nil {
		logger.Error("error while updating lecture hall", "err", err)
		_ = c.Error(tools.RequestError{
			Status:        http.StatusInternalServerError,
			CustomMessage: "error while updating lecture hall",
			Err:           err,
		})
		return
	}
}

func (r lectureHallRoutes) updateLectureHallsDefaultPreset(c *gin.Context) {
	var req struct {
		PresetID uint `json:"presetID"`
	}
	err := c.BindJSON(&req)
	if err != nil {
		_ = c.Error(tools.RequestError{
			Status:        http.StatusBadRequest,
			CustomMessage: "can not bind body",
			Err:           err,
		})
		return
	}
	preset, err := r.LectureHallsDao.FindPreset(c.Param("id"), fmt.Sprintf("%d", req.PresetID))
	if err != nil {
		_ = c.Error(tools.RequestError{
			Status:        http.StatusNotFound,
			CustomMessage: "can not find preset",
			Err:           err,
		})
		return
	}
	preset.IsDefault = true
	err = r.LectureHallsDao.UnsetDefaults(c.Param("id"))
	if err != nil {
		logger.Error("error unsetting default presets", "err", err)
		_ = c.Error(tools.RequestError{
			Status:        http.StatusInternalServerError,
			CustomMessage: "error unsetting default presets",
			Err:           err,
		})
		return
	}
	err = r.LectureHallsDao.SavePreset(preset)
	if err != nil {
		logger.Error("error saving preset as default", "err", err)
		_ = c.Error(tools.RequestError{
			Status:        http.StatusInternalServerError,
			CustomMessage: "error saving preset as default",
			Err:           err,
		})
		return
	}
}

func (r lectureHallRoutes) deleteLectureHall(c *gin.Context) {
	lhIDStr := c.Param("id")
	lhID, err := strconv.Atoi(lhIDStr)
	if err != nil {
		_ = c.Error(tools.RequestError{
			Status:        http.StatusBadRequest,
			CustomMessage: "invalid param 'id'",
			Err:           err,
		})
		return
	}

	err = r.LectureHallsDao.DeleteLectureHall(uint(lhID))
	if err != nil {
		_ = c.Error(tools.RequestError{
			Status:        http.StatusInternalServerError,
			CustomMessage: "can not delete lecture hall",
			Err:           err,
		})
		return
	}
}

func (r lectureHallRoutes) refreshLectureHallPresets(c *gin.Context) {
	lhIDStr := c.Param("lectureHallID")
	lhID, err := strconv.Atoi(lhIDStr)
	if err != nil {
		_ = c.Error(tools.RequestError{
			Status:        http.StatusBadRequest,
			CustomMessage: "invalid param 'id'",
			Err:           err,
		})
		return
	}
	lh, err := r.LectureHallsDao.GetLectureHallByID(uint(lhID))
	if err != nil {
		_ = c.Error(tools.RequestError{
			Status:        http.StatusNotFound,
			CustomMessage: "can not find lecture hall",
			Err:           err,
		})
		return
	}
	err = r.fetchLHPresets(lh)
	if err != nil {
		_ = c.Error(tools.RequestError{
			Status:        http.StatusInternalServerError,
			CustomMessage: "can not fetch lecture hall presets",
			Err:           err,
		})
	}
}

//go:embed template
var staticFS embed.FS

func (r lectureHallRoutes) lectureHallIcal(c *gin.Context) {
	templ, err := template.ParseFS(staticFS, "template/*.gotemplate")
	if err != nil {
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
	foundContext, exists := c.Get("TUMLiveContext")
	if !exists {
		logger.Error("context should exist but doesn't")
		_ = c.Error(tools.RequestError{
			Status:        http.StatusInternalServerError,
			CustomMessage: "context should exist but doesn't",
		})
		return
	}
	err = c.Request.ParseForm()
	if err != nil {
		_ = c.Error(tools.RequestError{Status: http.StatusBadRequest, CustomMessage: "Bad Request", Err: err})
		return
	}
	lectureHallsStr := strings.Split(c.Request.Form.Get("lecturehalls"), ",")
	lectureHalls := make([]uint, 0, len(lectureHallsStr))
	for _, l := range lectureHallsStr {
		if l == "" {
			continue
		}
		a, err := strconv.Atoi(l)
		if err != nil {
			_ = c.Error(tools.RequestError{Status: http.StatusBadRequest, CustomMessage: "Lecture Hall ID must be a number.", Err: err})
			return
		}
		lectureHalls = append(lectureHalls, uint(a))
	}
	all := !c.Request.Form.Has("lecturehalls") // if none requested, deliver all

	tumLiveContext := foundContext.(tools.TUMLiveContext)
	// pass 0 to db query to get all lectures if user is not logged in or admin
	queryUid := uint(0)
	if tumLiveContext.User != nil && !tumLiveContext.User.Can(model.PermViewAllCourses) {
		queryUid = tumLiveContext.User.ID
	}
	icalData, err := r.LectureHallsDao.GetStreamsForLectureHallIcal(queryUid, lectureHalls, all)
	if err != nil {
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
	c.Header("content-type", "text/calendar")
	err = templ.ExecuteTemplate(c.Writer, "ical.gotemplate", icalData)
	if err != nil {
		logger.Error("Error executing template ical.gotemplate", "err", err)
	}
}

func (r lectureHallRoutes) switchPreset(c *gin.Context) {
	tumLiveContext := c.MustGet("TUMLiveContext").(tools.TUMLiveContext)

	if tumLiveContext.Stream == nil || !tumLiveContext.Stream.LiveNow {
		_ = c.Error(tools.RequestError{
			Status:        http.StatusBadRequest,
			CustomMessage: "invalid stream or stream not live",
		})
		return
	}
	preset, err := r.LectureHallsDao.FindPreset(c.Param("lectureHallID"), c.Param("presetID"))
	if err != nil {
		_ = c.Error(tools.RequestError{
			Status:        http.StatusNotFound,
			CustomMessage: "can not find preset",
			Err:           err,
		})
		return
	}
	lh, err := r.LectureHallsDao.GetLectureHallByID(preset.LectureHallID)
	if err != nil {
		_ = c.Error(tools.RequestError{
			Status:        http.StatusNotFound,
			CustomMessage: "can not find lecture hall",
		})
		return
	}

	ctrl, err := r.cameraService.For(lh.CameraIP, lh.CameraType)
	if err != nil {
		_ = c.Error(tools.RequestError{
			Status:        http.StatusInternalServerError,
			CustomMessage: "can not provide camera",
			Err:           err,
		})
		return
	}
	err = ctrl.SetPreset(preset.PresetID)
	if err != nil {
		_ = c.Error(tools.RequestError{
			Status:        http.StatusInternalServerError,
			CustomMessage: "can not set preset on camera",
			Err:           err,
		})
		return
	}
	select {
	case <-time.After(camSwitchDelay):
	case <-c.Request.Context().Done():
	}
}

func (r lectureHallRoutes) takeSnapshot(c *gin.Context) {
	preset, err := r.LectureHallsDao.FindPreset(c.Param("lectureHallID"), c.Param("presetID"))
	if err != nil {
		_ = c.Error(tools.RequestError{
			Status:        http.StatusNotFound,
			CustomMessage: "can not find preset",
			Err:           err,
		})
		return
	}
	lh, err := r.LectureHallsDao.GetLectureHallByID(preset.LectureHallID)
	if err != nil {
		_ = c.Error(tools.RequestError{
			Status:        http.StatusNotFound,
			CustomMessage: "can not find lecture hall",
			Err:           err,
		})
		return
	}
	ctrl, err := r.cameraService.For(lh.CameraIP, lh.CameraType)
	if err != nil {
		_ = c.Error(tools.RequestError{
			Status:        http.StatusInternalServerError,
			CustomMessage: "can not provide camera",
			Err:           err,
		})
		return
	}
	err = ctrl.SetPreset(preset.PresetID)
	if err != nil {
		_ = c.Error(tools.RequestError{
			Status:        http.StatusInternalServerError,
			CustomMessage: "can not set preset on camera",
			Err:           err,
		})
		return
	}
	select {
	case <-c.Request.Context().Done():
		return
	case <-time.After(camSwitchDelay):
	}
	snapshot, err := ctrl.TakeSnapshot(r.presetImageDir)
	if err != nil {
		_ = c.Error(tools.RequestError{
			Status:        http.StatusInternalServerError,
			CustomMessage: "can not take snapshot",
			Err:           err,
		})
		return
	}
	preset.Image = snapshot
	err = r.LectureHallsDao.SavePreset(preset)
	if err != nil {
		_ = c.Error(tools.RequestError{
			Status:        http.StatusInternalServerError,
			CustomMessage: "can not save snapshot",
			Err:           err,
		})
		return
	}
	c.JSONP(http.StatusOK, gin.H{"path": fmt.Sprintf("/public/%s", preset.Image)})
}

func (r lectureHallRoutes) setLectureHall(c *gin.Context) {
	var req setLectureHallRequest
	err := c.BindJSON(&req)
	if err != nil {
		_ = c.Error(tools.RequestError{
			Status:        http.StatusBadRequest,
			CustomMessage: "can not bind body",
			Err:           err,
		})
		return
	}

	streams, err := r.StreamsDao.GetStreamsByIds(req.StreamIDs)
	if err != nil || len(streams) != len(req.StreamIDs) {
		logger.Error("can not get all streams to update lecture hall", "err", err)
		_ = c.Error(tools.RequestError{
			Status:        http.StatusInternalServerError,
			CustomMessage: "can not get all streams to update lecture hall",
			Err:           err,
		})
		return
	}

	if req.LectureHallID == 0 {
		err = r.StreamsDao.UnsetLectureHall(req.StreamIDs)
		if err != nil {
			logger.Error("can not update lecture hall for streams", "err", err)
			_ = c.Error(tools.RequestError{
				Status:        http.StatusInternalServerError,
				CustomMessage: "can not update lecture hall for streams",
				Err:           err,
			})
		}
		return
	}

	_, err = r.LectureHallsDao.GetLectureHallByID(req.LectureHallID)
	if err != nil {
		_ = c.Error(tools.RequestError{
			Status:        http.StatusNotFound,
			CustomMessage: "can not get lecture hall",
			Err:           err,
		})
		return
	}
	err = r.StreamsDao.SetLectureHall(req.StreamIDs, req.LectureHallID)
	if err != nil {
		logger.Error("can not update lecture hall", "err", err)
		_ = c.Error(tools.RequestError{
			Status:        http.StatusInternalServerError,
			CustomMessage: "can not update lecture hall",
			Err:           err,
		})
		return
	}
}

func (r lectureHallRoutes) createLectureHall(c *gin.Context) {
	var req createLectureHallRequest
	err := c.BindJSON(&req)
	if err != nil {
		logger.Error("can not bind body", "err", err)
		_ = c.Error(tools.RequestError{
			Status:        http.StatusBadRequest,
			CustomMessage: "can not bind body",
			Err:           err,
		})
		return
	}
	r.LectureHallsDao.CreateLectureHall(model.LectureHall{
		Name:           req.Name,
		StreamProtocol: model.StreamProtocol(req.StreamProtocol),
		CombIP:         req.CombIP,
		PresIP:         req.PresIP,
		CamIP:          req.CamIP,
		CameraIP:       req.CameraIP,
		PwrCtrlIp:      req.PwrCtrlIP,
	})
}

func (r lectureHallRoutes) fetchLHPresets(lh model.LectureHall) error {
	cam, err := r.cameraService.For(lh.CameraIP, lh.CameraType)
	if err != nil {
		return err
	}
	presets, err := cam.GetPresets()
	if err != nil {
		return err
	}
	lh.CameraPresets = presets
	r.LectureHallsDao.SaveLectureHallFullAssoc(lh)
	return nil
}

type createLectureHallRequest struct {
	Name           string `json:"name"`
	StreamProtocol int    `json:"streamProtocol"` // 1 = rtmp, 2 = srt
	CombIP         string `json:"combIP"`
	PresIP         string `json:"presIP"`
	CamIP          string `json:"camIP"`
	CameraIP       string `json:"cameraIP"`
	PwrCtrlIP      string `json:"pwrCtrlIp"`
}

type setLectureHallRequest struct {
	StreamIDs     []uint `json:"streamIDs"`
	LectureHallID uint   `json:"lectureHall"`
}
