package api

import (
	"embed"
	"net/http"
	"strconv"
	"strings"
	"text/template"

	"github.com/gin-gonic/gin"

	"github.com/TUM-Dev/gocast/dao"
	"github.com/TUM-Dev/gocast/model"
	"github.com/TUM-Dev/gocast/tools"
)

func configGinLectureHallApiRouter(router *gin.Engine, daoWrapper dao.DaoWrapper) {
	routes := lectureHallRoutes{DaoWrapper: daoWrapper}

	admins := router.Group("/api")
	admins.Use(tools.RequirePermission(model.PermAdministerServer))
	// CRUD on lecture halls (create/update/delete) and camera preset management
	// (refresh/default/snapshot) moved to v2 -- see apiv2/server/lecture_hall_admin.go
	// -- and switching a live stream's preset followed, to apiv2/server/stream_camera.go,
	// taking the camera service with it.
	admins.POST("/setLectureHall", routes.setLectureHall)

	router.GET("/api/schedule.ics", routes.lectureHallIcal)
}

type lectureHallRoutes struct {
	dao.DaoWrapper
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
