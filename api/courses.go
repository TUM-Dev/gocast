package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/getsentry/sentry-go"
	"github.com/gin-gonic/gin"
	"github.com/joschahenningsen/TUM-Live/dao"
	"github.com/joschahenningsen/TUM-Live/model"
	"github.com/joschahenningsen/TUM-Live/tools"
	"github.com/joschahenningsen/TUM-Live/tools/tum"
	uuid "github.com/satori/go.uuid"
	log "github.com/sirupsen/logrus"
	"gorm.io/gorm"
	"io/ioutil"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"
)

func configGinCourseRouter(router *gin.Engine, daoWrapper dao.DaoWrapper) {
	routes := coursesRoutes{daoWrapper}

	router.POST("/api/course/activate/:token", routes.activateCourseByToken)
	router.GET("/api/lecture-halls-by-id", routes.lectureHallsByID)
	atLeastLecturerGroup := router.Group("/")
	atLeastLecturerGroup.Use(tools.AtLeastLecturer)
	atLeastLecturerGroup.POST("/api/courseInfo", routes.courseInfo)
	atLeastLecturerGroup.POST("/api/createCourse", routes.createCourse)

	adminOfCourseGroup := router.Group("/api/course/:courseID")
	adminOfCourseGroup.Use(tools.InitCourse(daoWrapper))
	adminOfCourseGroup.Use(tools.AdminOfCourse)
	adminOfCourseGroup.DELETE("/", routes.deleteCourse)
	adminOfCourseGroup.POST("/uploadVOD", routes.uploadVOD)
	adminOfCourseGroup.POST("/createLecture", routes.createLecture)
	adminOfCourseGroup.POST("/presets", routes.updatePresets)
	adminOfCourseGroup.POST("/deleteLectures", routes.deleteLectures)
	adminOfCourseGroup.POST("/renameLecture/:streamID", routes.renameLecture)
	adminOfCourseGroup.POST("/updateLectureSeries/:streamID", routes.updateLectureSeries)
	adminOfCourseGroup.PUT("/updateDescription/:streamID", routes.updateDescription)
	adminOfCourseGroup.DELETE("/deleteLectureSeries/:streamID", routes.deleteLectureSeries)
	adminOfCourseGroup.POST("/addUnit", routes.addUnit)
	adminOfCourseGroup.POST("/submitCut", routes.submitCut)
	adminOfCourseGroup.POST("/deleteUnit/:unitID", routes.deleteUnit)
	adminOfCourseGroup.GET("/stats", routes.getStats)
	adminOfCourseGroup.GET("/stats/export", routes.exportStats)
	adminOfCourseGroup.GET("/admins", routes.getAdmins)
	adminOfCourseGroup.PUT("/admins/:userID", routes.addAdminToCourse)
	adminOfCourseGroup.DELETE("/admins/:userID", routes.removeAdminFromCourse)
}

type coursesRoutes struct {
	dao.DaoWrapper
}

const (
	WorkerHTTPPort = "8060"
	CutOffLength   = 100
)

type uploadVodReq struct {
	Start time.Time `form:"start" binding:"required"`
	Title string    `form:"title"`
}

func (r coursesRoutes) uploadVOD(c *gin.Context) {
	log.Info("uploadVOD")
	var req uploadVodReq
	err := c.BindQuery(&req)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "bad request: " + err.Error()})
		return
	}
	tlctx := c.MustGet("TUMLiveContext").(tools.TUMLiveContext)
	stream := model.Stream{
		Name:         req.Title,
		Start:        req.Start,
		End:          req.Start.Add(time.Hour),
		CourseID:     tlctx.Course.ID,
		StreamStatus: model.StatusConverting,
	}
	err = r.StreamsDao.CreateStream(&stream)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "could not save stream: " + err.Error()})
		return
	}
	key := uuid.NewV4().String()
	err = r.UploadKeyDao.CreateUploadKey(key, stream.ID)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "can't create upload key: " + err.Error()})
		return
	}
	workers := r.WorkerDao.GetAliveWorkers()
	if len(workers) == 0 {
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "No workers available"})
		return
	}
	w := workers[getWorkerWithLeastWorkload(workers)]
	u, err := url.Parse("http://" + w.Host + ":" + WorkerHTTPPort + "/upload?" + c.Request.URL.Query().Encode() + "&key=" + key)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Errorf("parse proxy url: %v", err)})
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

// updatePresets updates the CameraPresets of a course
func (r coursesRoutes) updatePresets(c *gin.Context) {
	foundContext, exists := c.Get("TUMLiveContext")
	if !exists {
		sentry.CaptureException(errors.New("context should exist but doesn't"))
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
	tumLiveContext := foundContext.(tools.TUMLiveContext)
	course := tumLiveContext.Course
	if course == nil {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}

	var req []lhResp
	err := c.BindJSON(&req)
	if err != nil {
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}
	var presetSettings []model.CameraPresetPreference
	for _, hall := range req {
		if len(hall.Presets) != 0 && hall.SelectedIndex != 0 {
			presetSettings = append(presetSettings, model.CameraPresetPreference{
				LectureHallID: hall.Presets[hall.SelectedIndex-1].LectureHallId, // index count starts at 1
				PresetID:      hall.SelectedIndex,
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
	if err := r.CoursesDao.UpdateCourse(c, *course); err != nil {
		log.WithError(err).Error("failed to update course")
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}

}

func (r coursesRoutes) activateCourseByToken(c *gin.Context) {
	t := c.Param("token")
	if t == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "token is missing"})
		return
	}
	course, err := r.CoursesDao.GetCourseByToken(t)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "Course not found. Is the token correct?"})
		return
	}
	course.DeletedAt = gorm.DeletedAt{Valid: false}
	course.VODEnabled = true
	course.Visibility = "loggedin"
	err = r.CoursesDao.UnDeleteCourse(c, course)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, "Could not update course settings")
		return
	}
}

func (r coursesRoutes) removeAdminFromCourse(c *gin.Context) {
	foundContext, exists := c.Get("TUMLiveContext")
	if !exists {
		sentry.CaptureException(errors.New("context should exist but doesn't"))
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
	tumLiveContext := foundContext.(tools.TUMLiveContext)

	userID, err := strconv.ParseUint(c.Param("userID"), 10, 32)
	if err != nil {
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}

	admins, err := r.CoursesDao.GetCourseAdmins(tumLiveContext.Course.ID)
	if err != nil {
		log.WithError(err).Error("could not get course admins")
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
	if len(admins) == 1 {
		c.AbortWithStatus(http.StatusBadRequest)
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
		c.AbortWithStatus(http.StatusNotFound)
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
		c.AbortWithStatus(http.StatusInternalServerError)
	}
	c.JSON(http.StatusOK, userForLecturerDto{
		ID:    user.ID,
		Name:  user.Name,
		Login: user.GetLoginString(),
	})
}

func (r coursesRoutes) addAdminToCourse(c *gin.Context) {
	foundContext, exists := c.Get("TUMLiveContext")
	if !exists {
		sentry.CaptureException(errors.New("context should exist but doesn't"))
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
	tumLiveContext := foundContext.(tools.TUMLiveContext)
	id := c.Param("userID")
	idUint, err := strconv.ParseUint(id, 10, 32)
	if err != nil {
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}
	user, err := r.UsersDao.GetUserByID(c, uint(idUint))
	if err != nil {
		c.AbortWithStatus(http.StatusNotFound)
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
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
	if user.Role == model.GenericType || user.Role == model.StudentType {
		user.Role = model.LecturerType
		err := r.UsersDao.UpdateUser(user)
		if err != nil {
			log.WithError(err).Error("could not update user")
			c.AbortWithStatus(http.StatusInternalServerError)
		}
	}
	c.JSON(http.StatusOK, userForLecturerDto{
		ID:    user.ID,
		Name:  user.Name,
		Login: user.GetLoginString(),
	})
}

func (r coursesRoutes) getAdmins(c *gin.Context) {
	foundContext, exists := c.Get("TUMLiveContext")
	if !exists {
		sentry.CaptureException(errors.New("context should exist but doesn't"))
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
	tumLiveContext := foundContext.(tools.TUMLiveContext)
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

type lhResp struct {
	LectureHallName string               `json:"lecture_hall_name"`
	Presets         []model.CameraPreset `json:"presets"`
	SelectedIndex   int                  `json:"selected_index"`
}

func (r coursesRoutes) lectureHallsByID(c *gin.Context) {
	foundContext, exists := c.Get("TUMLiveContext")
	if !exists {
		sentry.CaptureException(errors.New("context should exist but doesn't"))
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
	tumLiveContext := foundContext.(tools.TUMLiveContext)
	err := c.Request.ParseForm()
	if err != nil {
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}
	token := c.Request.Form.Get("id")
	id, err := strconv.Atoi(token)
	if err != nil {
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}
	course, err := r.CoursesDao.GetCourseById(c, uint(id))
	if err != nil {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}
	if !tumLiveContext.User.IsAdminOfCourse(course) {
		c.AbortWithStatus(http.StatusForbidden)
		return
	}

	r.lectureHalls(c, course)
}

func (r coursesRoutes) lectureHalls(c *gin.Context, course model.Course) {
	var res []lhResp
	lectureHallIDs := map[uint]bool{}
	for _, s := range course.Streams {
		if s.LectureHallID != 0 {
			lectureHallIDs[s.LectureHallID] = true
		}
	}
	for u := range lectureHallIDs {
		lh, err := r.LectureHallsDao.GetLectureHallByID(u)
		if err != nil {
			log.WithError(err).Error("Can't fetch lecture hall for stream")
		} else {
			res = append(res, lhResp{
				LectureHallName: lh.Name,
				Presets:         lh.CameraPresets,
			})
		}
	}
	for _, preference := range course.GetCameraPresetPreference() {
		for i, re := range res {
			if len(re.Presets) != 0 && re.Presets[0].LectureHallId == preference.LectureHallID {
				res[i].SelectedIndex = preference.PresetID
				break
			}
		}
	}

	c.JSON(http.StatusOK, res)
}

func (r coursesRoutes) submitCut(c *gin.Context) {
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
	stream, err := r.StreamsDao.GetStreamByID(context.Background(), strconv.Itoa(int(req.LectureID)))
	if err != nil {
		c.AbortWithStatusJSON(http.StatusNotFound, gin.H{"msg": "stream not found"})
		return
	}
	stream.StartOffset = req.From
	stream.EndOffset = req.To
	if err = r.StreamsDao.SaveStream(&stream); err != nil {
		panic(err)
	}
}

type submitCutRequest struct {
	LectureID uint `json:"lectureID"`
	From      uint `json:"from"`
	To        uint `json:"to"`
}

func (r coursesRoutes) deleteUnit(c *gin.Context) {
	unit, err := r.StreamsDao.GetUnitByID(c.Param("unitID"))
	if err != nil {
		c.AbortWithStatusJSON(http.StatusNotFound, gin.H{"msg": "not found"})
		return
	}
	r.StreamsDao.DeleteUnit(unit.Model.ID)
}

func (r coursesRoutes) addUnit(c *gin.Context) {
	var req addUnitRequest
	if err := c.BindJSON(&req); err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"msg": "bad request"})
		return
	}

	stream, err := r.StreamsDao.GetStreamByID(context.Background(), strconv.Itoa(int(req.LectureID)))
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
	if err = r.StreamsDao.UpdateStreamFullAssoc(&stream); err != nil {
		c.AbortWithStatus(http.StatusInternalServerError)
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

func (r coursesRoutes) updateDescription(c *gin.Context) {
	sIDInt, err := strconv.Atoi(c.Param("streamID"))
	if err != nil {
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}
	sID := uint(sIDInt)
	var req renameLectureRequest
	if err := c.Bind(&req); err != nil {
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}
	stream, err := r.StreamsDao.GetStreamByID(context.Background(), c.Param("streamID"))
	if err != nil {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}
	stream.Description = req.Name
	if err := r.StreamsDao.UpdateStream(stream); err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, "couldn't update lecture Description")
		return
	}
	wsMsg := gin.H{
		"description": gin.H{
			"full":      stream.GetDescriptionHTML(),
			"truncated": tools.Truncate(stream.GetDescriptionHTML(), 150),
		},
	}
	if msg, err := json.Marshal(wsMsg); err == nil {
		broadcastStream(sID, msg)
	} else {
		log.WithError(err).Error("couldn't marshal stream rename ws msg")
	}
}

func (r coursesRoutes) renameLecture(c *gin.Context) {
	sIDInt, err := strconv.Atoi(c.Param("streamID"))
	if err != nil {
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}
	sID := uint(sIDInt)
	var req renameLectureRequest
	if err = c.Bind(&req); err != nil {
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}
	stream, err := r.StreamsDao.GetStreamByID(context.Background(), c.Param("streamID"))
	if err != nil {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}
	stream.Name = req.Name
	if err := r.StreamsDao.UpdateStream(stream); err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, "couldn't update lecture name")
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

func (r coursesRoutes) updateLectureSeries(c *gin.Context) {
	stream, err := r.StreamsDao.GetStreamByID(context.Background(), c.Param("streamID"))
	if err != nil {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}

	if err := r.StreamsDao.UpdateLectureSeries(stream); err != nil {
		log.WithError(err).Error("couldn't update lecture series")
		c.AbortWithStatusJSON(http.StatusInternalServerError, "couldn't update lecture series")
		return
	}
	// Series changes could be theoretically broadcasted here through the websocket to live listeners.
}

type renameLectureRequest struct {
	Name string
}

func (r coursesRoutes) deleteLectureSeries(c *gin.Context) {
	ctx := c.MustGet("TUMLiveContext").(tools.TUMLiveContext)
	stream, err := r.StreamsDao.GetStreamByID(context.Background(), c.Param("streamID"))
	if err != nil {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}

	if stream.SeriesIdentifier == "" {
		c.AbortWithStatusJSON(http.StatusBadRequest, "the stream is not in a lecture series")
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

func (r coursesRoutes) deleteLectures(c *gin.Context) {
	tumLiveContext := c.MustGet("TUMLiveContext").(tools.TUMLiveContext)

	var req deleteLecturesRequest
	if err := c.Bind(&req); err != nil {
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}

	var streams []model.Stream
	for _, streamID := range req.StreamIDs {
		stream, err := r.StreamsDao.GetStreamByID(context.Background(), streamID)
		if err != nil || stream.CourseID != tumLiveContext.Course.ID {
			c.AbortWithStatus(http.StatusForbidden)
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

func (r coursesRoutes) createLecture(c *gin.Context) {
	tumLiveContext := c.MustGet("TUMLiveContext").(tools.TUMLiveContext)

	var req createLectureRequest
	if err := c.ShouldBind(&req); err != nil {
		log.WithError(err).Error("invalid form")
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}

	// Forbid setting lectureHall for vod or premiere
	if (req.Premiere || req.Vodup) && req.LectureHallId != "0" {
		log.Error("Cannot set lectureHallId on 'Premiere' or 'Vodup' Lecture.")
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}

	// try parse lectureHallId
	lectureHallId, err := strconv.ParseInt(req.LectureHallId, 10, 32)
	if err != nil {
		log.WithError(err).Error("invalid LectureHallId format")
		c.AbortWithStatus(http.StatusBadRequest)
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
		err := os.MkdirAll(premiereFolder, os.ModePerm)
		if err != nil {
			c.AbortWithStatus(http.StatusInternalServerError)
			log.WithError(err).Error("Can't create folder for premiere")
			return
		}
	}
	playlist := ""
	if req.Vodup {
		err := tools.UploadLRZ(fmt.Sprintf("%s/%s", premiereFolder, premiereFileName))
		if err != nil {
			log.WithError(err).Error("Can't upload file for premiere")
			c.AbortWithStatus(http.StatusInternalServerError)
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
		log.WithError(err).Warn("Can't update course")
		c.AbortWithStatus(http.StatusInternalServerError)
	}
}

type createLectureRequest struct {
	Title         string      `json:"title"`
	LectureHallId string      `json:"lectureHallId"`
	Start         time.Time   `json:"start"`
	Duration      int         `json:"duration"`
	Premiere      bool        `json:"premiere"`
	Vodup         bool        `json:"vodup"`
	DateSeries    []time.Time `json:"dateSeries"`
}

func (r coursesRoutes) createCourse(c *gin.Context) {
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
	match, err := regexp.MatchString("(enrolled|public|loggedin|hidden)", req.Access)
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
	_, err = r.CoursesDao.GetCourseBySlugYearAndTerm(c, req.Slug, semester, year)
	if err == nil {
		c.AbortWithStatusJSON(http.StatusConflict, gin.H{"message": "Course with slug already exists"})
		return
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
	if tumLiveContext.User.Role != model.AdminType {
		course.Admins = []model.User{*tumLiveContext.User}
	}

	if err := r.AuditDao.Create(&model.Audit{
		User:    tumLiveContext.User,
		Message: fmt.Sprintf("%s:'%s' (%d, %s)", course.Slug, course.Name, course.Year, course.TeachingTerm), // e.g. "eidi:'Einführung in die Informatik' (2020, S)"
		Type:    model.AuditCourseCreate,
	}); err != nil {
		log.Error("Create Audit:", err)
	}

	err = r.CoursesDao.CreateCourse(context.Background(), &course, true)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, "Couldn't save course. Please reach out to us.")
		return
	}
	courseWithID, err := r.CoursesDao.GetCourseBySlugYearAndTerm(context.Background(), req.Slug, semester, year)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, "Could not get course for slug and term. Please reach out to us.")
	}
	// refresh enrollments and lectures
	courses := make([]model.Course, 1)
	courses[0] = courseWithID
	go tum.GetEventsForCourses(courses, r.DaoWrapper)
	go tum.FindStudentsForCourses(courses, r.DaoWrapper)
	go tum.FetchCourses(r.DaoWrapper)
}

func (r coursesRoutes) deleteCourse(c *gin.Context) {
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

type createCourseRequest struct {
	Access       string //enrolled, public, hidden or loggedin
	CourseID     string
	EnChat       bool
	EnDL         bool
	EnVOD        bool
	Name         string
	Slug         string
	TeachingTerm string
}

func (r coursesRoutes) courseInfo(c *gin.Context) {
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
	var courseInfo tum.CourseInfo
	for _, token := range tools.Cfg.Campus.Tokens {
		courseInfo, err = tum.GetCourseInformation(req.CourseID, token)
		if err == nil {
			break
		}
	}
	if err != nil { // course not found
		log.WithError(err).Warn("Error getting course information")
		c.AbortWithStatus(http.StatusNotFound)
		return
	}
	c.JSON(200, courseInfo)
}

type getCourseRequest struct {
	CourseID string `json:"courseID"`
}

type deleteLecturesRequest struct {
	StreamIDs []string `json:"streamIDs"`
}
