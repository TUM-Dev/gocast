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
	"github.com/meilisearch/meilisearch-go"
	log "github.com/sirupsen/logrus"
	"gorm.io/gorm"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
)

const (
	WorkerHTTPPort = "8060"
	CutOffLength   = 256
)

func configGinCourseRouter(router *gin.Engine, daoWrapper dao.DaoWrapper) {
	routes := coursesRoutes{daoWrapper}

	router.POST("/api/course/activate/:token", routes.activateCourseByToken)
	router.GET("/api/lecture-halls-by-id", routes.lectureHallsByID)

	api := router.Group("/api")
	{
		courseById := api.Group("/courses/:id")
		{
			courseById.GET("", routes.getCourse)
		}
		lecturers := api.Group("")
		{
			lecturers.Use(tools.AtLeastLecturer)
			lecturers.POST("/courseInfo", routes.courseInfo)
			lecturers.POST("/createCourse", routes.createCourse)
			lecturers.GET("/searchCourse", routes.searchCourse)
		}

		api.DELETE("/course/by-token/:courseID", routes.deleteCourseByToken)
	}
}

type coursesRoutes struct {
	dao.DaoWrapper
}

func (r coursesRoutes) activateCourseByToken(c *gin.Context) {
	tlctx := c.MustGet("TUMLiveContext").(tools.TUMLiveContext)
	t := c.Param("token")
	if t == "" {
		_ = c.Error(tools.RequestError{
			Status:        http.StatusBadRequest,
			CustomMessage: "token is missing",
		})
		return
	}
	course, err := r.CoursesDao.GetCourseByToken(t)
	if err != nil {
		_ = c.Error(tools.RequestError{
			Status:        http.StatusBadRequest,
			CustomMessage: "course not found. is the token correct?",
			Err:           err,
		})
		return
	}
	course.DeletedAt = gorm.DeletedAt{Valid: false}
	course.VODEnabled = true
	course.Visibility = "loggedin"
	err = r.CoursesDao.UnDeleteCourse(c, course)
	if err != nil {
		_ = c.Error(tools.RequestError{
			Status:        http.StatusInternalServerError,
			CustomMessage: "can not update course settings",
			Err:           err,
		})
		return
	}
	err = r.AuditDao.Create(&model.Audit{User: tlctx.User, Type: model.AuditCourseCreate, Message: fmt.Sprintf("opted in by token, %s:'%s'", course.Name, course.Slug)})
	if err != nil {
		log.WithError(err).Error("create opt in audit failed")
	}
}

func (r coursesRoutes) lectureHallsByID(c *gin.Context) {
	foundContext, exists := c.Get("TUMLiveContext")
	if !exists {
		sentry.CaptureException(errors.New("context should exist but doesn't"))
		_ = c.Error(tools.RequestError{
			Status:        http.StatusInternalServerError,
			CustomMessage: "context should exist but doesn't",
		})
		return
	}
	tumLiveContext := foundContext.(tools.TUMLiveContext)
	err := c.Request.ParseForm()
	if err != nil {
		_ = c.Error(tools.RequestError{
			Status:        http.StatusBadRequest,
			CustomMessage: "can not parse request form",
		})
		return
	}
	token := c.Request.Form.Get("id")
	id, err := strconv.Atoi(token)
	if err != nil {
		_ = c.Error(tools.RequestError{
			Status:        http.StatusBadRequest,
			CustomMessage: "invalid id",
		})
		return
	}
	course, err := r.CoursesDao.GetCourseById(c, uint(id))
	if err != nil {
		_ = c.Error(tools.RequestError{
			Status:        http.StatusNotFound,
			CustomMessage: "can not find course",
			Err:           err,
		})
		return
	}
	if !tumLiveContext.User.IsAdminOfCourse(course) {
		_ = c.Error(tools.RequestError{
			Status:        http.StatusForbidden,
			CustomMessage: "not a admin",
			Err:           err,
		})
		return
	}

	r.lectureHalls(c, course)
}

type coursesByIdURI struct {
	ID uint `uri:"id" binding:"required"`
}

func (r coursesRoutes) getCourse(c *gin.Context) {
	tumLiveContext := c.MustGet("TUMLiveContext").(tools.TUMLiveContext)

	var uri coursesByIdURI
	if err := c.ShouldBindUri(&uri); err != nil {
		_ = c.Error(tools.RequestError{
			Err:           err,
			Status:        http.StatusBadRequest,
			CustomMessage: "invalid URI",
		})
		return
	}

	// watchedStateData is used by the client to track the which VoDs are watched.
	type watchedStateData struct {
		ID        uint   `json:"streamID"`
		Month     string `json:"month"`
		Watched   bool   `json:"watched"`
		Recording bool   `json:"recording"`
	}

	type Response struct {
		Course       model.Course
		WatchedState []watchedStateData `json:",omitempty"`
	}

	course, err := r.CoursesDao.GetCourseById(c, uri.ID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			_ = c.Error(tools.RequestError{
				Status:        http.StatusBadRequest,
				CustomMessage: "can't find course",
			})
		} else {
			sentry.CaptureException(err)
			_ = c.Error(tools.RequestError{
				Err:           err,
				Status:        http.StatusInternalServerError,
				CustomMessage: "can't retrieve course",
			})
		}
		return
	}
	var response Response
	if tumLiveContext.User == nil {
		// Not-Logged-In Users do not receive the watch state
		response = Response{Course: course}
	} else {
		streamsWithWatchState, err := r.StreamsDao.GetStreamsWithWatchState(course.ID, (*tumLiveContext.User).ID)
		if err != nil {
			sentry.CaptureException(err)
			_ = c.Error(tools.RequestError{
				Err:           err,
				Status:        http.StatusInternalServerError,
				CustomMessage: "loading streamsWithWatchState and progresses for a given course and user failed",
			})
		}

		course.Streams = streamsWithWatchState // Update the course streams to contain the watch state.

		var clientWatchState = make([]watchedStateData, 0)
		for _, s := range streamsWithWatchState {
			clientWatchState = append(clientWatchState, watchedStateData{
				ID:        s.Model.ID,
				Month:     s.Start.Month().String(),
				Watched:   s.Watched,
				Recording: s.Recording,
			})
		}

		response = Response{Course: course, WatchedState: clientWatchState}
	}

	c.JSON(http.StatusOK, response)
}

func (r coursesRoutes) courseInfo(c *gin.Context) {
	jsonData, err := io.ReadAll(c.Request.Body)
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

func (r coursesRoutes) createCourse(c *gin.Context) {
	tumLiveContext := c.MustGet("TUMLiveContext").(tools.TUMLiveContext)

	var req createCourseRequest
	if err := c.BindJSON(&req); err != nil {
		_ = c.Error(tools.RequestError{
			Status:        http.StatusBadRequest,
			CustomMessage: "invalid body",
			Err:           err,
		})
		return
	}

	match, err := regexp.MatchString("(enrolled|public|loggedin|hidden)", req.Access)
	if err != nil || !match {
		_ = c.Error(tools.RequestError{
			Status:        http.StatusBadRequest,
			CustomMessage: "invalid access",
			Err:           err,
		})
		return
	}

	//verify teaching term input, should either be Sommersemester 2020 or Wintersemester 2020/21
	match, err = regexp.MatchString("(Sommersemester [0-9]{4}|Wintersemester [0-9]{4}/[0-9]{2})$", req.TeachingTerm)
	if err != nil || !match {
		_ = c.Error(tools.RequestError{
			Status:        http.StatusBadRequest,
			CustomMessage: "invalid semester format",
			Err:           err,
		})
		return
	}
	reYear := regexp.MustCompile("[0-9]{4}")
	year, err := strconv.Atoi(reYear.FindStringSubmatch(req.TeachingTerm)[0])
	if err != nil {
		_ = c.Error(tools.RequestError{
			Status:        http.StatusBadRequest,
			CustomMessage: "invalid semester format",
			Err:           err,
		})
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
		_ = c.Error(tools.RequestError{
			Status:        http.StatusConflict,
			CustomMessage: "course with slug already exists",
		})
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
		_ = c.Error(tools.RequestError{
			Status:        http.StatusInternalServerError,
			CustomMessage: "Couldn't save course. Please reach out to us.",
			Err:           err,
		})
		return
	}
	courseWithID, err := r.CoursesDao.GetCourseBySlugYearAndTerm(context.Background(), req.Slug, semester, year)
	if err != nil {
		_ = c.Error(tools.RequestError{
			Status:        http.StatusInternalServerError,
			CustomMessage: "Could not get course for slug and term. Please reach out to us.",
			Err:           err,
		})
		return
	}
	// refresh enrollments and lectures
	courses := make([]model.Course, 1)
	courses[0] = courseWithID
	go tum.GetEventsForCourses(courses, r.DaoWrapper)
	go tum.FindStudentsForCourses(courses, r.DaoWrapper)
	go tum.FetchCourses(r.DaoWrapper)

	// send id to client for further requests
	c.JSON(http.StatusCreated, gin.H{"id": courseWithID.ID})
}

func (r coursesRoutes) searchCourse(c *gin.Context) {
	client, err := tools.Cfg.GetMeiliClient()
	if err != nil {
		_ = c.Error(tools.RequestError{
			Status:        http.StatusInternalServerError,
			CustomMessage: "Can't create client for search backend",
			Err:           err,
		})
		return
	}
	var request struct {
		Q string `form:"q"`
	}
	err = c.BindQuery(&request)
	if err != nil {
		_ = c.Error(tools.RequestError{Status: http.StatusBadRequest, CustomMessage: "Bad request", Err: err})
		return
	}
	index := client.Index("PREFETCHED_COURSES")
	search, err := index.Search(request.Q, &meilisearch.SearchRequest{
		Limit: 10,
	})
	if err != nil {
		_ = c.Error(tools.RequestError{
			Status:        http.StatusInternalServerError,
			CustomMessage: "Can't perform search",
			Err:           err,
		})
		return
	}
	c.JSON(http.StatusOK, search.Hits)
}

func (r coursesRoutes) deleteCourseByToken(c *gin.Context) {
	tumLiveContext := c.MustGet("TUMLiveContext").(tools.TUMLiveContext)

	err := c.Request.ParseForm()
	if err != nil {
		_ = c.AbortWithError(http.StatusBadRequest, tools.RequestError{Status: http.StatusBadRequest, CustomMessage: "invalid form", Err: err})
		return
	}
	token := c.Request.Form.Get("token")
	if token == "" {
		_ = c.AbortWithError(http.StatusBadRequest, tools.RequestError{Status: http.StatusBadRequest, CustomMessage: "token is missing"})
		return
	}
	course, err := r.CoursesDao.GetCourseByToken(token)
	if err != nil {
		_ = c.AbortWithError(http.StatusNotFound, tools.RequestError{Status: http.StatusNotFound, CustomMessage: "course not found", Err: err})
		return
	}

	if err := r.AuditDao.Create(&model.Audit{
		User:    tumLiveContext.User,
		Message: fmt.Sprintf("'%s' (%d, %s)[%d]. Token: %s", course.Name, course.Year, course.TeachingTerm, course.ID, token),
		Type:    model.AuditCourseDelete,
	}); err != nil {
		log.Error("Create Audit:", err)
	}

	r.CoursesDao.DeleteCourse(course)
	dao.Cache.Clear()
}

type lhResp struct {
	LectureHallName  string               `json:"lecture_hall_name"`
	LectureHallID    uint                 `json:"lecture_hall_id"`
	Presets          []model.CameraPreset `json:"presets"`
	SourceMode       model.SourceMode     `json:"source_mode"`
	SelectedPresetID int                  `json:"selected_preset_id"`
}

func (r coursesRoutes) lectureHalls(c *gin.Context, course model.Course) {
	var lectureHallData []lhResp
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
			// Find if sourceMode is specified for this lecture hall
			lectureHallData = append(lectureHallData, lhResp{
				LectureHallName: lh.Name,
				LectureHallID:   lh.ID,
				Presets:         lh.CameraPresets,
				SourceMode:      course.GetSourceModeForLectureHall(lh.ID),
			})
		}
	}

	for i, response := range lectureHallData {
		if len(response.Presets) != 0 {
			for _, coursePref := range course.GetCameraPresetPreference() {
				if response.LectureHallID == coursePref.LectureHallID {
					lectureHallData[i].SelectedPresetID = coursePref.PresetID
					break
				}
			}
		}
	}

	c.JSON(http.StatusOK, lectureHallData)
}
