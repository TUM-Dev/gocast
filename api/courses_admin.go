package api

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/joschahenningsen/TUM-Live/dao"
	"github.com/joschahenningsen/TUM-Live/model"
	"github.com/joschahenningsen/TUM-Live/tools"
	uuid "github.com/satori/go.uuid"
	log "github.com/sirupsen/logrus"
	"gorm.io/gorm"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

func configGinCourseAdminRouter(router *gin.Engine, daoWrapper dao.DaoWrapper) {
	routes := coursesAdminRoutes{daoWrapper}

	api := router.Group("/api")
	{
		courses := api.Group("/course/:courseID")
		{
			courses.Use(tools.InitCourse(daoWrapper))
			courses.Use(tools.AdminOfCourse)
			courses.DELETE("/", routes.deleteCourse)
			courses.POST("/uploadVOD", routes.uploadVOD)
			courses.POST("/copy", routes.copyCourse)
			courses.POST("/createLecture", routes.createLecture)
			courses.POST("/presets", routes.updateSourceSettings)
			courses.POST("/deleteLectures", routes.deleteLectures)
			courses.POST("/renameLecture/:streamID", routes.renameLecture)
			courses.POST("/updateLectureSeries/:streamID", routes.updateLectureSeries)
			courses.PUT("/updateDescription/:streamID", routes.updateDescription)
			courses.DELETE("/deleteLectureSeries/:streamID", routes.deleteLectureSeries)
			courses.POST("/submitCut", routes.submitCut)

			courses.POST("/addUnit", routes.addUnit)
			courses.POST("/deleteUnit/:unitID", routes.deleteUnit)

			stream := courses.Group("/stream/:streamID")
			{
				stream.Use(tools.InitStream(daoWrapper))
				stream.GET("/transcodingProgress", routes.getTranscodingProgress)
			}

			stats := courses.Group("/stats")
			{
				stats.GET("", routes.getStats)
				stats.GET("/export", routes.exportStats)
			}

			admins := courses.Group("admins")
			{
				admins.GET("", routes.getAdmins)
				admins.PUT("/:userID", routes.addAdminToCourse)
				admins.DELETE("/:userID", routes.removeAdminFromCourse)
			}
		}
	}
}

type coursesAdminRoutes struct {
	dao.DaoWrapper
}

func (r coursesAdminRoutes) submitCut(c *gin.Context) {
	var req submitCutRequest
	if err := c.BindJSON(&req); err != nil {
		_ = c.Error(tools.RequestError{
			Status:        http.StatusBadRequest,
			CustomMessage: "invalid body",
			Err:           err,
		})
		return
	}
	stream, err := r.StreamsDao.GetStreamByID(context.Background(), strconv.Itoa(int(req.LectureID)))
	if err != nil {
		_ = c.Error(tools.RequestError{
			Status:        http.StatusNotFound,
			CustomMessage: "stream not found",
			Err:           err,
		})
		return
	}
	stream.StartOffset = req.From
	stream.EndOffset = req.To
	if err = r.StreamsDao.SaveStream(&stream); err != nil {
		_ = c.Error(tools.RequestError{
			Status:        http.StatusInternalServerError,
			CustomMessage: "can not save stream",
			Err:           err,
		})
		return
	}
}

type submitCutRequest struct {
	LectureID uint `json:"lectureID"`
	From      uint `json:"from"`
	To        uint `json:"to"`
}

func (r coursesAdminRoutes) deleteUnit(c *gin.Context) {
	unit, err := r.StreamsDao.GetUnitByID(c.Param("unitID"))
	if err != nil {
		_ = c.Error(tools.RequestError{
			Status:        http.StatusNotFound,
			CustomMessage: "can not find unit",
			Err:           err,
		})
		return
	}
	r.StreamsDao.DeleteUnit(unit.Model.ID)
}

func (r coursesAdminRoutes) addUnit(c *gin.Context) {
	var req addUnitRequest
	if err := c.BindJSON(&req); err != nil {
		_ = c.Error(tools.RequestError{
			Status:        http.StatusBadRequest,
			CustomMessage: "invalid body",
			Err:           err,
		})
		return
	}

	stream, err := r.StreamsDao.GetStreamByID(context.Background(), strconv.Itoa(int(req.LectureID)))
	if err != nil {
		_ = c.Error(tools.RequestError{
			Status:        http.StatusNotFound,
			CustomMessage: "stream not found",
			Err:           err,
		})
		return
	}
	stream.Units = append(stream.Units, model.StreamUnit{
		UnitName:        req.Title,
		UnitDescription: req.Description,
		UnitStart:       req.From,
		UnitEnd:         req.To,
		StreamID:        stream.Model.ID,
	})
	if err = r.StreamsDao.UpdateStreamFullAssoc(&stream); err != nil {
		_ = c.Error(tools.RequestError{
			Status:        http.StatusInternalServerError,
			CustomMessage: "can not update stream full assoc",
			Err:           err,
		})
		return
	}
}

type addUnitRequest struct {
	LectureID   uint   `json:"lectureID"`
	From        uint   `json:"from"`
	To          uint   `json:"to"`
	Title       string `json:"title"`
	Description string `json:"description"`
}

func (r coursesAdminRoutes) updateDescription(c *gin.Context) {
	sIDInt, err := strconv.Atoi(c.Param("streamID"))
	if err != nil {
		_ = c.Error(tools.RequestError{
			Status:        http.StatusBadRequest,
			CustomMessage: "invalid streamID",
			Err:           err,
		})
		return
	}
	sID := uint(sIDInt)
	var req renameLectureRequest
	if err = c.Bind(&req); err != nil {
		_ = c.Error(tools.RequestError{
			Status:        http.StatusBadRequest,
			CustomMessage: "invalid body",
			Err:           err,
		})
		return
	}
	stream, err := r.StreamsDao.GetStreamByID(context.Background(), c.Param("streamID"))
	if err != nil {
		_ = c.Error(tools.RequestError{
			Status:        http.StatusNotFound,
			CustomMessage: "can not find stream",
			Err:           err,
		})
		return
	}
	stream.Description = req.Name
	if err = r.StreamsDao.UpdateStream(stream); err != nil {
		_ = c.Error(tools.RequestError{
			Status:        http.StatusInternalServerError,
			CustomMessage: "couldn't update lecture Description",
			Err:           err,
		})
		return
	}
	wsMsg := gin.H{
		"description": gin.H{
			"full": stream.GetDescriptionHTML(),
		},
	}
	if msg, err := json.Marshal(wsMsg); err == nil {
		broadcastStream(sID, msg)
	} else {
		log.WithError(err).Error("couldn't marshal stream rename ws msg")
	}
}

func (r coursesAdminRoutes) renameLecture(c *gin.Context) {
	sIDInt, err := strconv.Atoi(c.Param("streamID"))
	if err != nil {
		_ = c.Error(tools.RequestError{
			Status:        http.StatusBadRequest,
			CustomMessage: "invalid streamID",
			Err:           err,
		})
		return
	}
	sID := uint(sIDInt)
	var req renameLectureRequest
	if err = c.Bind(&req); err != nil {
		_ = c.Error(tools.RequestError{
			Status:        http.StatusBadRequest,
			CustomMessage: "invalid body",
			Err:           err,
		})
		return
	}
	stream, err := r.StreamsDao.GetStreamByID(context.Background(), c.Param("streamID"))
	if err != nil {
		_ = c.Error(tools.RequestError{
			Status:        http.StatusNotFound,
			CustomMessage: "can not find stream",
			Err:           err,
		})
		return
	}
	stream.Name = req.Name
	if err = r.StreamsDao.UpdateStream(stream); err != nil {
		_ = c.Error(tools.RequestError{
			Status:        http.StatusInternalServerError,
			CustomMessage: "couldn't update lecture name",
			Err:           err,
		})
		return
	}
	wsMsg := gin.H{
		"title": req.Name,
	}
	if msg, err := json.Marshal(wsMsg); err == nil {
		broadcastStream(sID, msg)
	} else {
		log.WithError(err).Error("couldn't marshal stream rename ws msg")
	}
}

func (r coursesAdminRoutes) updateLectureSeries(c *gin.Context) {
	stream, err := r.StreamsDao.GetStreamByID(context.Background(), c.Param("streamID"))
	if err != nil {
		_ = c.Error(tools.RequestError{
			Status:        http.StatusNotFound,
			CustomMessage: "can not find stream",
			Err:           err,
		})
		return
	}

	if err = r.StreamsDao.UpdateLectureSeries(stream); err != nil {
		log.WithError(err).Error("couldn't update lecture series")
		_ = c.Error(tools.RequestError{
			Status:        http.StatusInternalServerError,
			CustomMessage: "couldn't update lecture series",
			Err:           err,
		})
		return
	}
	// Series changes could be theoretically broadcasted here through the websocket to live listeners.
}

type renameLectureRequest struct {
	Name string
}

func (r coursesAdminRoutes) deleteLectureSeries(c *gin.Context) {
	ctx := c.MustGet("TUMLiveContext").(tools.TUMLiveContext)
	stream, err := r.StreamsDao.GetStreamByID(context.Background(), c.Param("streamID"))
	if err != nil {
		_ = c.Error(tools.RequestError{
			Status:        http.StatusNotFound,
			CustomMessage: "can not find stream",
			Err:           err,
		})
		return
	}

	if stream.SeriesIdentifier == "" {
		_ = c.Error(tools.RequestError{
			Status:        http.StatusBadRequest,
			CustomMessage: "the stream is not in a lecture series",
		})
		return
	}
	if err := r.AuditDao.Create(&model.Audit{
		User:    ctx.User,
		Message: fmt.Sprintf("'%s': %s (%d and series)", ctx.Course.Name, stream.Start.Format("2006 02 Jan, 15:04"), stream.ID),
		Type:    model.AuditStreamDelete,
	}); err != nil {
		log.Error("Create Audit:", err)
	}
	if err := r.StreamsDao.DeleteLectureSeries(stream.SeriesIdentifier); err != nil {
		log.WithError(err).Error("couldn't delete lecture series")
		c.AbortWithStatusJSON(http.StatusInternalServerError, "couldn't delete lecture series")
		return
	}
}

func (r coursesAdminRoutes) deleteLectures(c *gin.Context) {
	tumLiveContext := c.MustGet("TUMLiveContext").(tools.TUMLiveContext)

	var req deleteLecturesRequest
	if err := c.Bind(&req); err != nil {
		_ = c.Error(tools.RequestError{
			Status:        http.StatusBadRequest,
			CustomMessage: "invalid body",
			Err:           err,
		})
		return
	}

	var streams []model.Stream
	for _, streamID := range req.StreamIDs {
		stream, err := r.StreamsDao.GetStreamByID(context.Background(), streamID)
		if err != nil || stream.CourseID != tumLiveContext.Course.ID {
			_ = c.Error(tools.RequestError{
				Status:        http.StatusForbidden,
				CustomMessage: "not allowed to delete stream",
				Err:           err,
			})
			return
		}
		streams = append(streams, stream)
	}

	for _, stream := range streams {
		if err := r.AuditDao.Create(&model.Audit{
			User:    tumLiveContext.User,
			Message: fmt.Sprintf("'%s': %s (%d)", tumLiveContext.Course.Name, stream.Start.Format("2006 02 Jan, 15:04"), stream.ID),
			Type:    model.AuditStreamDelete,
		}); err != nil {
			log.Error("Create Audit:", err)
		}
		r.StreamsDao.DeleteStream(strconv.Itoa(int(stream.ID)))
	}
}

type createLectureRequest struct {
	Title         string      `json:"title"`
	LectureHallId string      `json:"lectureHallId"`
	Start         time.Time   `json:"start"`
	Duration      int         `json:"duration"`
	ChatEnabled   bool        `json:"isChatEnabled"`
	Premiere      bool        `json:"premiere"`
	Vodup         bool        `json:"vodup"`
	DateSeries    []time.Time `json:"dateSeries"`
}

func (r coursesAdminRoutes) createLecture(c *gin.Context) {
	tumLiveContext := c.MustGet("TUMLiveContext").(tools.TUMLiveContext)

	var req createLectureRequest
	if err := c.ShouldBind(&req); err != nil {
		log.WithError(err).Error("invalid form")
		_ = c.Error(tools.RequestError{
			Status:        http.StatusBadRequest,
			CustomMessage: "invalid form",
			Err:           err,
		})
		return
	}

	// Forbid setting lectureHall for vod or premiere
	if (req.Premiere || req.Vodup) && req.LectureHallId != "0" {
		log.Error("cannot set lectureHallId on 'Premiere' or 'Vodup' Lecture.")
		_ = c.Error(tools.RequestError{
			Status:        http.StatusBadRequest,
			CustomMessage: "cannot set lectureHallId on 'Premiere' or 'Vodup' Lecture.",
		})
		return
	}

	// try parse lectureHallId
	lectureHallId, err := strconv.ParseInt(req.LectureHallId, 10, 32)
	if err != nil {
		log.WithError(err).Error("invalid LectureHallID format")
		_ = c.Error(tools.RequestError{
			Status:        http.StatusBadRequest,
			CustomMessage: "invalid LectureHallId format",
			Err:           err,
		})
		return
	}

	// name for folder for premiere file if needed
	premiereFolder := fmt.Sprintf("%s/%d/%s/%s",
		tools.Cfg.Paths.Mass,
		tumLiveContext.Course.Year,
		tumLiveContext.Course.TeachingTerm,
		tumLiveContext.Course.Slug)
	premiereFileName := fmt.Sprintf("%s_%s.mp4",
		tumLiveContext.Course.Slug,
		req.Start.Format("2006-01-02_15-04"))
	if req.Premiere || req.Vodup {
		err = os.MkdirAll(premiereFolder, os.ModePerm)
		if err != nil {
			log.WithError(err).Error("can not create folder for premiere")
			_ = c.Error(tools.RequestError{
				Status:        http.StatusInternalServerError,
				CustomMessage: "can not create folder for premiere",
				Err:           err,
			})
			return
		}
	}
	playlist := ""
	if req.Vodup {
		err = tools.UploadLRZ(fmt.Sprintf("%s/%s", premiereFolder, premiereFileName))
		if err != nil {
			log.WithError(err).Error("can not upload file for premiere")
			_ = c.Error(tools.RequestError{
				Status:        http.StatusInternalServerError,
				CustomMessage: "can not upload file for premiere",
				Err:           err,
			})
			return
		}
		playlist = fmt.Sprintf("https://stream.lrz.de/vod/_definst_/mp4:tum/RBG/%s/playlist.m3u8", strings.ReplaceAll(premiereFileName, "-", "_"))
	}

	// Add start date as first event
	seriesIdentifier := uuid.NewV4().String()
	req.DateSeries = append(req.DateSeries, req.Start)

	for _, date := range req.DateSeries {
		endTime := date.Add(time.Minute * time.Duration(req.Duration))

		streamKey := uuid.NewV4().String()
		streamKey = strings.ReplaceAll(streamKey, "-", "")

		lecture := model.Stream{
			Name:          req.Title,
			CourseID:      tumLiveContext.Course.ID,
			LectureHallID: uint(lectureHallId),
			Start:         date,
			End:           endTime,
			ChatEnabled:   req.ChatEnabled,
			StreamKey:     streamKey,
			PlaylistUrl:   playlist,
			LiveNow:       false,
			Recording:     req.Vodup,
			Premiere:      req.Premiere,
		}

		// add Series Identifier
		if len(req.DateSeries) > 1 {
			lecture.SeriesIdentifier = seriesIdentifier
		}

		// add file if premiere
		if req.Premiere || req.Vodup {
			lecture.Files = []model.File{{Path: fmt.Sprintf("%s/%s", premiereFolder, premiereFileName)}}
		}

		if err := r.AuditDao.Create(&model.Audit{
			User:    tumLiveContext.User,
			Message: fmt.Sprintf("Stream for '%s' Created. Time: %s", tumLiveContext.Course.Name, lecture.Start.Format("2006 02 Jan, 15:04")),
			Type:    model.AuditStreamCreate,
		}); err != nil {
			log.Error("Create Audit:", err)
		}

		tumLiveContext.Course.Streams = append(tumLiveContext.Course.Streams, lecture)
	}

	err = r.CoursesDao.UpdateCourse(context.Background(), *tumLiveContext.Course)
	if err != nil {
		log.WithError(err).Warn("can not update course")
		_ = c.Error(tools.RequestError{
			Status:        http.StatusInternalServerError,
			CustomMessage: "can not update course",
			Err:           err,
		})
		return
	}
}

func (r coursesAdminRoutes) deleteCourse(c *gin.Context) {
	tumLiveContext := c.MustGet("TUMLiveContext").(tools.TUMLiveContext)

	log.WithFields(log.Fields{
		"user":   tumLiveContext.User.ID,
		"course": tumLiveContext.Course.ID,
	}).Info("Delete Course Called")

	if err := r.AuditDao.Create(&model.Audit{
		User:    tumLiveContext.User,
		Message: fmt.Sprintf("'%s' (%d, %s)[%d]", tumLiveContext.Course.Name, tumLiveContext.Course.Year, tumLiveContext.Course.TeachingTerm, tumLiveContext.Course.ID),
		Type:    model.AuditCourseDelete,
	}); err != nil {
		log.Error("Create Audit:", err)
	}

	r.CoursesDao.DeleteCourse(*tumLiveContext.Course)
	dao.Cache.Clear()
}

func (r coursesAdminRoutes) getTranscodingProgress(c *gin.Context) {
	ctx := c.MustGet("TUMLiveContext").(tools.TUMLiveContext)
	version := c.DefaultQuery("v", string(model.COMB))
	p, err := r.StreamsDao.GetTranscodingProgressByVersion(model.StreamVersion(version), ctx.Stream.ID)
	if err == gorm.ErrRecordNotFound {
		c.JSON(http.StatusOK, 100)
		return
	}
	if err != nil {
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusOK, p.Progress)
}

type copyCourseRequest struct {
	Semester string
	Year     string
	YearW    string
}

func (r coursesAdminRoutes) copyCourse(c *gin.Context) {
	var request copyCourseRequest
	err := c.BindJSON(&request)
	if err != nil {
		_ = c.Error(tools.RequestError{Status: http.StatusBadRequest, CustomMessage: "Bad request", Err: err})
		return
	}
	tlctx := c.MustGet("TUMLiveContext").(tools.TUMLiveContext)
	course := tlctx.Course
	streams := course.Streams

	course.Model = gorm.Model{}
	course.Streams = nil
	yearInt, err := strconv.Atoi(request.Year)
	if err != nil {
		_ = c.Error(tools.RequestError{Status: http.StatusBadRequest, CustomMessage: "Semester must be a number", Err: err})
		return
	}
	course.Year = yearInt
	switch request.Semester {
	case "Sommersemester":
		course.TeachingTerm = "S"
	case "Wintersemester":
		course.TeachingTerm = "W"
	default:
		_ = c.Error(tools.RequestError{Status: http.StatusBadRequest, CustomMessage: "Teaching must be a 'Sommmersemester' or 'Wintersemester'", Err: err})
		return
	}

	err = r.CoursesDao.CreateCourse(c, course, true)
	if err != nil {
		log.WithError(err).Error("Can't create course")
		_ = c.Error(tools.RequestError{Status: http.StatusInternalServerError, CustomMessage: "Can't create course", Err: err})
		return
	}
	numErrors := 0
	for _, stream := range streams {
		stream.CourseID = course.ID
		stream.Model = gorm.Model{}
		err := r.StreamsDao.CreateStream(&stream)
		if err != nil {
			log.WithError(err).Error("Can't create stream")
			numErrors++
		}
	}
	c.JSON(http.StatusOK, gin.H{"numErrs": numErrors, "newCourse": course.ID})
}

type getCourseRequest struct {
	CourseID string `json:"courseID"`
}

type deleteLecturesRequest struct {
	StreamIDs []string `json:"streamIDs"`
}

type uploadVodReq struct {
	Start time.Time `form:"start" binding:"required"`
	Title string    `form:"title"`
}

func (r coursesAdminRoutes) uploadVOD(c *gin.Context) {
	log.Info("uploadVOD")
	var req uploadVodReq
	err := c.BindQuery(&req)
	if err != nil {
		_ = c.Error(tools.RequestError{
			Status:        http.StatusBadRequest,
			CustomMessage: "can not bind query",
			Err:           err,
		})
		return
	}
	tlctx := c.MustGet("TUMLiveContext").(tools.TUMLiveContext)
	stream := model.Stream{
		Name:     req.Title,
		Start:    req.Start,
		End:      req.Start.Add(time.Hour),
		CourseID: tlctx.Course.ID,
	}
	err = r.StreamsDao.CreateStream(&stream)
	if err != nil {
		_ = c.Error(tools.RequestError{
			Status:        http.StatusInternalServerError,
			CustomMessage: "can not save stream",
			Err:           err,
		})
		return
	}
	key := uuid.NewV4().String()
	err = r.UploadKeyDao.CreateUploadKey(key, stream.ID)
	if err != nil {
		_ = c.Error(tools.RequestError{
			Status:        http.StatusInternalServerError,
			CustomMessage: "can not create upload key",
			Err:           err,
		})
		return
	}
	workers := r.WorkerDao.GetAliveWorkers()
	if len(workers) == 0 {
		_ = c.Error(tools.RequestError{
			Status:        http.StatusInternalServerError,
			CustomMessage: "no workers available",
			Err:           err,
		})
		return
	}
	w := workers[getWorkerWithLeastWorkload(workers)]
	u, err := url.Parse("http://" + w.Host + ":" + WorkerHTTPPort + "/upload?" + c.Request.URL.Query().Encode() + "&key=" + key)
	if err != nil {
		_ = c.Error(tools.RequestError{
			Status:        http.StatusInternalServerError,
			CustomMessage: fmt.Sprintf("parse proxy url: %v", err),
			Err:           err,
		})
		return
	}
	p := httputil.NewSingleHostReverseProxy(u)
	p.Director = func(req *http.Request) {
		req.URL.Scheme = u.Scheme
		req.URL.Host = u.Host
		req.Host = u.Host
		req.URL.Path = u.Path
		req.URL.RawQuery = u.RawQuery
	}
	p.ServeHTTP(c.Writer, c.Request)
}

// updateSourceSettings updates the CameraPresets of a course
func (r coursesAdminRoutes) updateSourceSettings(c *gin.Context) {
	tumLiveContext := c.MustGet("TUMLiveContext").(tools.TUMLiveContext)
	course := tumLiveContext.Course
	if course == nil {
		_ = c.Error(tools.RequestError{
			Status:        http.StatusNotFound,
			CustomMessage: "course not found",
		})
		return
	}

	var req []lhResp
	err := c.BindJSON(&req)
	if err != nil {
		_ = c.Error(tools.RequestError{
			Status:        http.StatusBadRequest,
			CustomMessage: "invalid body",
			Err:           err,
		})
		return
	}

	var presetSettings []model.CameraPresetPreference
	for _, hall := range req {
		if len(hall.Presets) != 0 && hall.SelectedPresetID != 0 {
			presetSettings = append(presetSettings, model.CameraPresetPreference{
				LectureHallID: hall.LectureHallID,
				PresetID:      hall.SelectedPresetID,
			})
		}
	}

	if err := r.AuditDao.Create(&model.Audit{
		User:    tumLiveContext.User,
		Message: fmt.Sprintf("%s:'%s'", tumLiveContext.Course.Name, tumLiveContext.Course.Slug),
		Type:    model.AuditCourseEdit,
	}); err != nil {
		log.Error("Create Audit:", err)
	}

	course.SetCameraPresetPreference(presetSettings)

	var sourceModeSettings []model.SourcePreference
	for _, hall := range req {
		sourceModeSettings = append(sourceModeSettings, model.SourcePreference{
			LectureHallID: hall.LectureHallID,
			SourceMode:    hall.SourceMode,
		})
	}

	course.SetSourcePreference(sourceModeSettings)

	if err = r.CoursesDao.UpdateCourse(c, *course); err != nil {
		log.WithError(err).Error("failed to update course")
		_ = c.Error(tools.RequestError{
			Status:        http.StatusInternalServerError,
			CustomMessage: "failed to update course",
			Err:           err,
		})
		return
	}
}

func (r coursesAdminRoutes) removeAdminFromCourse(c *gin.Context) {
	tumLiveContext := c.MustGet("TUMLiveContext").(tools.TUMLiveContext)
	userID, err := strconv.ParseUint(c.Param("userID"), 10, 32)
	if err != nil {
		_ = c.Error(tools.RequestError{
			Status:        http.StatusBadRequest,
			CustomMessage: "invalid userID",
			Err:           err,
		})
		return
	}

	admins, err := r.CoursesDao.GetCourseAdmins(tumLiveContext.Course.ID)
	if err != nil {
		log.WithError(err).Error("could not get course admins")
		_ = c.Error(tools.RequestError{
			Status:        http.StatusInternalServerError,
			CustomMessage: "could not get course admins",
			Err:           err,
		})
		return
	}
	if len(admins) == 1 {
		_ = c.Error(tools.RequestError{
			Status:        http.StatusBadRequest,
			CustomMessage: "can not delete last admin",
		})
		return
	}
	var user *model.User
	for _, u := range admins {
		if u.ID == uint(userID) {
			user = &u
			break
		}
	}
	if user == nil {
		_ = c.Error(tools.RequestError{
			Status:        http.StatusNotFound,
			CustomMessage: "can not find user",
		})
		return
	}

	if err := r.AuditDao.Create(&model.Audit{
		User:    tumLiveContext.User,
		Message: fmt.Sprintf("%s:'%s' remove: %s (%d)", tumLiveContext.Course.Name, tumLiveContext.Course.Slug, user.GetPreferredName(), user.ID), // e.g. "eidi:'Einführung in die Informatik' (2020, S)"
		Type:    model.AuditCourseEdit,
	}); err != nil {
		log.Error("Create Audit:", err)
	}

	err = r.CoursesDao.RemoveAdminFromCourse(user.ID, tumLiveContext.Course.ID)
	if err != nil {
		log.WithError(err).Error("could not remove admin from course")
		_ = c.Error(tools.RequestError{
			Status:        http.StatusInternalServerError,
			CustomMessage: "could not remove admin from course",
			Err:           err,
		})
		return
	}
	c.JSON(http.StatusOK, userForLecturerDto{
		ID:    user.ID,
		Name:  user.Name,
		Login: user.GetLoginString(),
	})
}

func (r coursesAdminRoutes) addAdminToCourse(c *gin.Context) {
	tumLiveContext := c.MustGet("TUMLiveContext").(tools.TUMLiveContext)
	id := c.Param("userID")
	idUint, err := strconv.ParseUint(id, 10, 32)
	if err != nil {
		_ = c.Error(tools.RequestError{
			Status:        http.StatusBadRequest,
			CustomMessage: "invalid userID",
			Err:           err,
		})
		return
	}
	user, err := r.UsersDao.GetUserByID(c, uint(idUint))
	if err != nil {
		_ = c.Error(tools.RequestError{
			Status:        http.StatusNotFound,
			CustomMessage: "can not find user",
			Err:           err,
		})
		return
	}

	if err := r.AuditDao.Create(&model.Audit{
		User:    tumLiveContext.User,
		Message: fmt.Sprintf("%s:'%s' add: %s (%d)", tumLiveContext.Course.Name, tumLiveContext.Course.Slug, user.GetPreferredName(), user.ID), // e.g. "eidi:'Einführung in die Informatik' (2020, S)"
		Type:    model.AuditCourseEdit,
	}); err != nil {
		log.Error("Create Audit:", err)
	}

	err = r.CoursesDao.AddAdminToCourse(user.ID, tumLiveContext.Course.ID)
	if err != nil {
		log.WithError(err).Error("could not add admin to course")
		_ = c.Error(tools.RequestError{
			Status:        http.StatusInternalServerError,
			CustomMessage: "could not add admin to course",
			Err:           err,
		})
		return
	}
	if user.Role == model.GenericType || user.Role == model.StudentType {
		user.Role = model.LecturerType
		err = r.UsersDao.UpdateUser(user)
		if err != nil {
			log.WithError(err).Error("could not update user")
			_ = c.Error(tools.RequestError{
				Status:        http.StatusInternalServerError,
				CustomMessage: "could not update user",
				Err:           err,
			})
			return
		}
	}
	c.JSON(http.StatusOK, userForLecturerDto{
		ID:    user.ID,
		Name:  user.Name,
		Login: user.GetLoginString(),
	})
}

func (r coursesAdminRoutes) getAdmins(c *gin.Context) {
	tumLiveContext := c.MustGet("TUMLiveContext").(tools.TUMLiveContext)
	admins, err := r.CoursesDao.GetCourseAdmins(tumLiveContext.Course.ID)
	if err != nil {
		log.WithError(err).Error("error getting course admins")
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
	res := make([]userForLecturerDto, len(admins))
	for i, admin := range admins {
		res[i] = userForLecturerDto{
			ID:    admin.ID,
			Name:  admin.Name,
			Login: admin.GetLoginString(),
		}
	}
	c.JSON(http.StatusOK, res)
}
