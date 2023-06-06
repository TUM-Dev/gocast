package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/RBG-TUM/commons"
	"github.com/getsentry/sentry-go"
	"github.com/gin-gonic/gin"
	"github.com/joschahenningsen/TUM-Live/dao"
	"github.com/joschahenningsen/TUM-Live/model"
	"github.com/joschahenningsen/TUM-Live/tools"
	"github.com/joschahenningsen/TUM-Live/tools/tum"
	"github.com/meilisearch/meilisearch-go"
	uuid "github.com/satori/go.uuid"
	log "github.com/sirupsen/logrus"
	"gorm.io/gorm"
	"io"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

func configGinCourseRouter(router *gin.Engine, daoWrapper dao.DaoWrapper) {
	routes := coursesRoutes{daoWrapper}

	router.POST("/api/course/activate/:token", routes.activateCourseByToken)
	router.GET("/api/lecture-halls-by-id", routes.lectureHallsByID)

	api := router.Group("/api")
	{
		api.GET("/courses/live", routes.getLive)
		api.GET("/courses/public", routes.getPublic)
		api.GET("/courses/users", routes.getUsers)
		api.GET("/courses/users/pinned", routes.getPinned)

		courseBySlug := api.Group("/courses/:slug")
		{
			courseBySlug.GET("/", routes.getCourseBySlug)
		}

		lecturers := api.Group("")
		{
			lecturers.Use(tools.AtLeastLecturer)
			lecturers.POST("/courseInfo", routes.courseInfo)
			lecturers.POST("/createCourse", routes.createCourse)
			lecturers.GET("/searchCourse", routes.searchCourse)
		}

		api.DELETE("/course/by-token/:courseID", routes.deleteCourseByToken)

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

type coursesRoutes struct {
	dao.DaoWrapper
}

const (
	WorkerHTTPPort = "8060"
	CutOffLength   = 256
)

type uploadVodReq struct {
	Start time.Time `form:"start" binding:"required"`
	Title string    `form:"title"`
}

func (r coursesRoutes) getLive(c *gin.Context) {
	tumLiveContext := c.MustGet("TUMLiveContext").(tools.TUMLiveContext)

	streams, err := r.GetCurrentLive(context.Background())
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		log.WithError(err).Error("could not get current live streams")
		c.AbortWithStatusJSON(http.StatusNotFound, gin.H{"message": "Could not load current livestream from database."})
	}

	type CourseStream struct {
		Course      model.CourseDTO
		Stream      model.StreamDTO
		LectureHall *model.LectureHallDTO
		Viewers     uint
	}

	livestreams := make([]CourseStream, 0)

	for _, stream := range streams {
		courseForLiveStream, _ := r.GetCourseById(context.Background(), stream.CourseID)

		// only show streams for logged-in users if they are logged in
		if courseForLiveStream.Visibility == "loggedin" && tumLiveContext.User == nil {
			continue
		}
		// only show "enrolled" streams to users which are enrolled or admins
		if courseForLiveStream.Visibility == "enrolled" {
			if !isUserAllowedToWatchPrivateCourse(courseForLiveStream, tumLiveContext.User) {
				continue
			}
		}
		// Only show hidden streams to admins
		if courseForLiveStream.Visibility == "hidden" && (tumLiveContext.User == nil || tumLiveContext.User.Role != model.AdminType) {
			continue
		}
		var lectureHall *model.LectureHall
		if stream.LectureHallID != 0 {
			lh, err := r.LectureHallsDao.GetLectureHallByID(stream.LectureHallID)
			if err != nil {
				log.WithError(err).Error(err)
			} else {
				lectureHall = &lh
			}
		}

		viewers := uint(0)
		for sID, sessions := range sessionsMap {
			if sID == stream.ID {
				viewers = uint(len(sessions))
			}
		}

		livestreams = append(livestreams, CourseStream{
			Course:      courseForLiveStream.ToDTO(),
			Stream:      stream.ToDTO(),
			LectureHall: lectureHall.ToDTO(),
			Viewers:     viewers,
		})
	}

	c.JSON(http.StatusOK, livestreams)
}

func (r coursesRoutes) getPublic(c *gin.Context) {
	tumLiveContext := c.MustGet("TUMLiveContext").(tools.TUMLiveContext)

	year, term := tum.GetCurrentSemester()
	year, err := strconv.Atoi(c.DefaultQuery("year", strconv.Itoa(year)))
	if err != nil {
		_ = c.Error(tools.RequestError{
			Status:        http.StatusBadRequest,
			CustomMessage: "invalid year",
			Err:           err,
		})
		return
	}
	term = c.DefaultQuery("term", term)

	var courses, public []model.Course
	if tumLiveContext.User != nil {
		public, err = r.GetPublicAndLoggedInCourses(year, term)
	} else {
		public, err = r.GetPublicCourses(year, term)
	}
	if err != nil {
		courses = []model.Course{}
	} else {
		sortCourses(public)
		courses = commons.Unique(public, func(c model.Course) uint { return c.ID })
	}

	resp := make([]model.CourseDTO, len(courses))
	for i, course := range courses {
		resp[i] = course.ToDTO()
	}

	c.JSON(http.StatusOK, resp)
}

func (r coursesRoutes) getUsers(c *gin.Context) {
	tumLiveContext := c.MustGet("TUMLiveContext").(tools.TUMLiveContext)

	year, term := tum.GetCurrentSemester()
	year, err := strconv.Atoi(c.DefaultQuery("year", strconv.Itoa(year)))
	if err != nil {
		_ = c.Error(tools.RequestError{
			Status:        http.StatusBadRequest,
			CustomMessage: "invalid year",
			Err:           err,
		})
	}
	term = c.DefaultQuery("term", term)

	var courses []model.Course
	if tumLiveContext.User != nil {
		switch tumLiveContext.User.Role {
		case model.AdminType:
			courses = routes.GetAllCoursesForSemester(year, term, c)
		case model.LecturerType:
			courses = tumLiveContext.User.CoursesForSemester(year, term, context.Background())
			coursesForLecturer, err := r.GetAdministeredCoursesByUserId(c, tumLiveContext.User.ID, term, year)
			if err == nil {
				courses = append(courses, coursesForLecturer...)
			}
		default:
			courses = tumLiveContext.User.CoursesForSemester(year, term, context.Background())
		}
	}

	sortCourses(courses)
	courses = commons.Unique(courses, func(c model.Course) uint { return c.ID })
	resp := make([]model.CourseDTO, 0, len(courses))
	for _, course := range courses {
		if !course.IsHidden() {
			resp = append(resp, course.ToDTO())
		}
	}

	c.JSON(http.StatusOK, resp)
}

func (r coursesRoutes) getPinned(c *gin.Context) {
	tumLiveContext := c.MustGet("TUMLiveContext").(tools.TUMLiveContext)

	year, term := tum.GetCurrentSemester()
	year, err := strconv.Atoi(c.DefaultQuery("year", strconv.Itoa(year)))
	if err != nil {
		_ = c.Error(tools.RequestError{
			Status:        http.StatusBadRequest,
			CustomMessage: "invalid year",
			Err:           err,
		})
	}
	term = c.DefaultQuery("term", term)

	var pinnedCourses []model.Course
	if tumLiveContext.User != nil {
		pinnedCourses = tumLiveContext.User.PinnedCourses
	} else {
		pinnedCourses = []model.Course{}
	}

	pinnedCourses = commons.Unique(pinnedCourses, func(c model.Course) uint { return c.ID })
	resp := make([]model.CourseDTO, 0, len(pinnedCourses))
	for _, course := range pinnedCourses {
		if !course.IsHidden() && course.Year == year && course.TeachingTerm == term {
			resp = append(resp, course.ToDTO())
		}
	}

	c.JSON(http.StatusOK, resp)
}

func sortCourses(courses []model.Course) {
	sort.Slice(courses, func(i, j int) bool {
		return courses[i].CompareTo(courses[j])
	})
}

func (r coursesRoutes) getCourseBySlug(c *gin.Context) {
	type URI struct {
		Slug string `uri:"slug" binding:"required"`
	}

	type Query struct {
		Year int    `form:"year"`
		Term string `form:"term"`
	}

	var uri URI
	if err := c.ShouldBindUri(&uri); err != nil {
		_ = c.Error(tools.RequestError{
			Err:           err,
			Status:        http.StatusBadRequest,
			CustomMessage: "invalid URI",
		})
		return
	}

	var query Query
	if err := c.ShouldBindQuery(&query); err != nil {
		_ = c.Error(tools.RequestError{
			Err:           err,
			Status:        http.StatusBadRequest,
			CustomMessage: "invalid query",
		})
		return
	}

	if query.Year == 0 || query.Term == "" {
		query.Year, query.Term = tum.GetCurrentSemester()
	}

	type Response struct {
		Course  model.CourseDTO
		Streams []model.StreamDTO
	}

	course, err := r.CoursesDao.GetCourseBySlugYearAndTerm(c, uri.Slug, query.Term, query.Year)
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

	streams := course.Streams
	streamsDTO := make([]model.StreamDTO, len(streams))
	for i, s := range streams {
		streamsDTO[i] = s.ToDTO()
	}

	courseDTO := course.ToDTO()
	courseDTO.Streams = streamsDTO

	c.JSON(http.StatusOK, courseDTO)
}

func isUserAllowedToWatchPrivateCourse(course model.Course, user *model.User) bool {
	if user != nil {
		for _, c := range user.Courses {
			if c.ID == course.ID {
				return true
			}
		}
		return user.IsEligibleToWatchCourse(course)
	}
	return false
}

func (r coursesRoutes) uploadVOD(c *gin.Context) {
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
func (r coursesRoutes) updateSourceSettings(c *gin.Context) {
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

func (r coursesRoutes) removeAdminFromCourse(c *gin.Context) {
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

func (r coursesRoutes) addAdminToCourse(c *gin.Context) {
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

func (r coursesRoutes) getAdmins(c *gin.Context) {
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

type lhResp struct {
	LectureHallName  string               `json:"lecture_hall_name"`
	LectureHallID    uint                 `json:"lecture_hall_id"`
	Presets          []model.CameraPreset `json:"presets"`
	SourceMode       model.SourceMode     `json:"source_mode"`
	SelectedPresetID int                  `json:"selected_preset_id"`
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

func (r coursesRoutes) submitCut(c *gin.Context) {
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

func (r coursesRoutes) deleteUnit(c *gin.Context) {
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

func (r coursesRoutes) addUnit(c *gin.Context) {
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

func (r coursesRoutes) updateDescription(c *gin.Context) {
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

func (r coursesRoutes) renameLecture(c *gin.Context) {
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

func (r coursesRoutes) updateLectureSeries(c *gin.Context) {
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

func (r coursesRoutes) deleteLectureSeries(c *gin.Context) {
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

func (r coursesRoutes) deleteLectures(c *gin.Context) {
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

func (r coursesRoutes) createLecture(c *gin.Context) {
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
		playlist = fmt.Sprintf(tools.Cfg.VodURLTemplate, strings.ReplaceAll(premiereFileName, "-", "_"))
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

func (r coursesRoutes) getTranscodingProgress(c *gin.Context) {
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

func (r coursesRoutes) copyCourse(c *gin.Context) {
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

type getCourseRequest struct {
	CourseID string `json:"courseID"`
}

type deleteLecturesRequest struct {
	StreamIDs []string `json:"streamIDs"`
}
