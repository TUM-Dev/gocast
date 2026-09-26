package api

import (
	"embed"
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

func configGinLectureHallApiRouter(router *gin.Engine, daoWrapper dao.DaoWrapper, camService CamService) {
	routes := lectureHallRoutes{
		DaoWrapper:    daoWrapper,
		cameraService: camService,
	}

	admins := router.Group("/api")
	admins.Use(tools.RequirePermission(model.PermAdministerServer))
	// CRUD on lecture halls (create/update/delete) and camera preset management
	// (refresh/default/snapshot) moved to v2 -- see apiv2/server/lecture_hall_admin.go.
	// switchPreset below still points a camera at a preset, so the CamService stays
	// wired into this router. The preset image directory left with takeSnapshot,
	// which was the only thing here that wrote an image.
	admins.GET("/course-schedule", routes.getSchedule)
	admins.POST("/course-schedule/:year/:term", routes.postSchedule)
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

	cameraService CamService
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

type setLectureHallRequest struct {
	StreamIDs     []uint `json:"streamIDs"`
	LectureHallID uint   `json:"lectureHall"`
}
