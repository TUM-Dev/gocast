package web

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
	log "github.com/sirupsen/logrus"
	"gorm.io/gorm"
	"net/http"
	"regexp"
)

// AdminPage serves all administration pages. todo: refactor into multiple methods
func (r mainRoutes) AdminPage(c *gin.Context) {
	foundContext, exists := c.Get("TUMLiveContext")
	if !exists {
		sentry.CaptureException(errors.New("context should exist but doesn't"))
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
	tumLiveContext := foundContext.(tools.TUMLiveContext)
	if tumLiveContext.User == nil {
		c.Redirect(http.StatusFound, "/login")
		return
	}
	var users []model.User
	_ = r.UsersDao.GetAllAdminsAndLecturers(&users)
	courses, err := r.CoursesDao.GetAdministeredCoursesByUserId(context.Background(), tumLiveContext.User.ID, "", 0)
	if err != nil {
		log.WithError(err).Error("couldn't query courses for user.")
		courses = []model.Course{}
	}
	workers, err := r.WorkerDao.GetAllWorkers()
	if err != nil {
		sentry.CaptureException(err)
	}
	lectureHalls := r.LectureHallsDao.GetAllLectureHalls()
	indexData := NewIndexData()
	indexData.TUMLiveContext = tumLiveContext
	page := "schedule"
	if c.Request.URL.Path == "/admin/users" {
		page = "users"
	}
	if c.Request.URL.Path == "/admin/lectureHalls" {
		page = "lectureHalls"
	}
	if c.Request.URL.Path == "/admin/lectureHalls/new" {
		page = "createLectureHalls"
	}
	if c.Request.URL.Path == "/admin/workers" {
		page = "workers"
	}
	if c.Request.URL.Path == "/admin/create-course" {
		page = "createCourse"
	}
	if c.Request.URL.Path == "/admin/course-import" {
		page = "courseImport"
	}
	if c.Request.URL.Path == "/admin/audits" {
		page = "audits"
	}
	if c.Request.URL.Path == "/admin/maintenance" {
		page = "maintenance"
	}
	var notifications []model.Notification
	if c.Request.URL.Path == "/admin/notifications" {
		page = "notifications"
		found, err := r.NotificationsDao.GetAllNotifications()
		if err != nil {
			log.WithError(err).Error("couldn't query notifications")
		} else {
			notifications = found
		}
	}
	var tokens []dao.AllTokensDto
	if c.Request.URL.Path == "/admin/token" {
		page = "token"
		tokens, err = r.TokenDao.GetAllTokens()
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			log.WithError(err).Error("couldn't query tokens")
			c.AbortWithStatus(http.StatusInternalServerError)
		}
	}
	var infopages []model.InfoPage
	if c.Request.URL.Path == "/admin/infopages" {
		page = "info-pages"
		infopages, err = r.InfoPageDao.GetAll()
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			log.WithError(err).Error("couldn't query texts")
			c.AbortWithStatus(http.StatusInternalServerError)
		}
	}
	if c.Request.URL.Path == "/admin/server-stats" {
		page = "serverStats"
		streams, err := r.StreamsDao.GetAllStreams()
		if err != nil {
			log.WithError(err).Error("Can't get all streams")
			sentry.CaptureException(err)
			streams = []model.Stream{}
		}
		indexData.TUMLiveContext.Course = &model.Course{
			Model:   gorm.Model{ID: 0},
			Streams: streams,
		}
	}
	var serverNotifications []model.ServerNotification
	if c.Request.URL.Path == "/admin/server-notifications" {
		page = "serverNotifications"
		if res, err := r.ServerNotificationDao.GetAllServerNotifications(); err == nil {
			serverNotifications = res
		} else {
			log.WithError(err).Warn("could not get all server notifications")
		}
	}
	semesters := r.CoursesDao.GetAvailableSemesters(c)
	y, t := tum.GetCurrentSemester()
	err = templateExecutor.ExecuteTemplate(c.Writer, "admin.gohtml",
		AdminPageData{Users: users,
			Courses:             courses,
			IndexData:           indexData,
			LectureHalls:        lectureHalls,
			Page:                page,
			Workers:             WorkersData{Workers: workers, Token: tools.Cfg.WorkerToken},
			Semesters:           semesters,
			CurY:                y,
			CurT:                t,
			Tokens:              tokens,
			InfoPages:           infopages,
			ServerNotifications: serverNotifications,
			Notifications:       notifications,
		})
	if err != nil {
		log.Printf("%v", err)
	}
}

type WorkersData struct {
	Workers []model.Worker
	Token   string
}

func (r mainRoutes) LectureCutPage(c *gin.Context) {
	foundContext, exists := c.Get("TUMLiveContext")
	if !exists {
		sentry.CaptureException(errors.New("context should exist but doesn't"))
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
	tumLiveContext := foundContext.(tools.TUMLiveContext)
	if err := templateExecutor.ExecuteTemplate(c.Writer, "lecture-cut.gohtml", tumLiveContext); err != nil {
		log.Fatalln(err)
	}
}

func (r mainRoutes) LectureUnitsPage(c *gin.Context) {
	foundContext, exists := c.Get("TUMLiveContext")
	if !exists {
		sentry.CaptureException(errors.New("context should exist but doesn't"))
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
	tumLiveContext := foundContext.(tools.TUMLiveContext)
	indexData := NewIndexData()
	indexData.TUMLiveContext = tumLiveContext
	if err := templateExecutor.ExecuteTemplate(c.Writer, "lecture-units.gohtml", LectureUnitsPageData{
		IndexData: indexData,
		Lecture:   *tumLiveContext.Stream,
		Units:     tumLiveContext.Stream.Units,
	}); err != nil {
		sentry.CaptureException(err)
	}
}

func (r mainRoutes) CourseStatsPage(c *gin.Context) {
	foundContext, exists := c.Get("TUMLiveContext")
	if !exists {
		sentry.CaptureException(errors.New("context should exist but doesn't"))
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
	tumLiveContext := foundContext.(tools.TUMLiveContext)
	indexData := NewIndexData()
	indexData.TUMLiveContext = tumLiveContext
	courses, err := r.CoursesDao.GetAdministeredCoursesByUserId(context.Background(), tumLiveContext.User.ID, "", 0)
	if err != nil {
		log.Printf("couldn't query courses for user. %v\n", err)
		courses = []model.Course{}
	}
	semesters := r.CoursesDao.GetAvailableSemesters(c)
	err = templateExecutor.ExecuteTemplate(c.Writer, "admin.gohtml", AdminPageData{
		IndexData: indexData,
		Courses:   courses,
		Page:      "stats",
		Semesters: semesters,
		CurY:      tumLiveContext.Course.Year,
		CurT:      tumLiveContext.Course.TeachingTerm,
	})
	if err != nil {
		log.Printf("%v\n", err)
	}
}

func (r mainRoutes) EditCoursePage(c *gin.Context) {
	foundContext, exists := c.Get("TUMLiveContext")
	if !exists {
		sentry.CaptureException(errors.New("context should exist but doesn't"))
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
	tumLiveContext := foundContext.(tools.TUMLiveContext)
	lectureHalls := r.LectureHallsDao.GetAllLectureHalls()
	err := r.CoursesDao.GetInvitedUsersForCourse(tumLiveContext.Course)
	if err != nil {
		log.Printf("%v", err)
	}
	indexData := NewIndexData()
	indexData.TUMLiveContext = tumLiveContext
	courses, err := r.CoursesDao.GetAdministeredCoursesByUserId(context.Background(), tumLiveContext.User.ID, "", 0)
	if err != nil {
		log.Printf("couldn't query courses for user. %v\n", err)
		courses = []model.Course{}
	}
	semesters := r.CoursesDao.GetAvailableSemesters(c)
	for i := range tumLiveContext.Course.Streams {
		err := tools.SetSignedPlaylists(&tumLiveContext.Course.Streams[i], tumLiveContext.User, true)
		if err != nil {
			log.WithError(err).Error("could not set signed playlist for admin page")
		}
	}
	err = templateExecutor.ExecuteTemplate(c.Writer, "admin.gohtml", AdminPageData{
		IndexData:      indexData,
		Courses:        courses,
		Page:           "course",
		Semesters:      semesters,
		CurY:           tumLiveContext.Course.Year,
		CurT:           tumLiveContext.Course.TeachingTerm,
		EditCourseData: EditCourseData{IndexData: indexData, IngestBase: tools.Cfg.IngestBase, LectureHalls: lectureHalls},
	})
	if err != nil {
		log.Printf("%v\n", err)
	}
}

func (r mainRoutes) RecordCoursePage(c *gin.Context) {
	foundContext, exists := c.Get("TUMLiveContext")
	if !exists {
		sentry.CaptureException(errors.New("context should exist but doesn't"))
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
	tumLiveContext := foundContext.(tools.TUMLiveContext)
	lectureHalls := r.LectureHallsDao.GetAllLectureHalls()
	err := r.CoursesDao.GetInvitedUsersForCourse(tumLiveContext.Course)
	if err != nil {
		log.Printf("%v", err)
	}
	indexData := NewIndexData()
	indexData.TUMLiveContext = tumLiveContext
	courses, err := r.CoursesDao.GetAdministeredCoursesByUserId(context.Background(), tumLiveContext.User.ID, "", 0)
	if err != nil {
		log.Printf("couldn't query courses for user. %v\n", err)
		courses = []model.Course{}
	}
	semesters := r.CoursesDao.GetAvailableSemesters(c)
	for i := range tumLiveContext.Course.Streams {
		err := tools.SetSignedPlaylists(&tumLiveContext.Course.Streams[i], tumLiveContext.User, true)
		if err != nil {
			log.WithError(err).Error("could not set signed playlist for admin page")
		}
	}
	err = templateExecutor.ExecuteTemplate(c.Writer, "admin.gohtml", AdminPageData{
		IndexData:      indexData,
		Courses:        courses,
		Page:           "record",
		Semesters:      semesters,
		CurY:           tumLiveContext.Course.Year,
		CurT:           tumLiveContext.Course.TeachingTerm,
		EditCourseData: EditCourseData{IndexData: indexData, IngestBase: tools.Cfg.IngestBase, LectureHalls: lectureHalls},
	})
	if err != nil {
		log.Printf("%v\n", err)
	}
}

func (r mainRoutes) UpdateCourse(c *gin.Context) {
	foundContext, exists := c.Get("TUMLiveContext")
	if !exists {
		sentry.CaptureException(errors.New("context should exist but doesn't"))
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
	tumLiveContext := foundContext.(tools.TUMLiveContext)
	if c.PostForm("submit") == "Reload Students From TUMOnline" {
		tum.FindStudentsForCourses([]model.Course{*tumLiveContext.Course}, r.UsersDao)
		c.Redirect(http.StatusFound, fmt.Sprintf("/admin/course/%v", tumLiveContext.Course.ID))
		return
	} else if c.PostForm("submit") == "Reload Lectures From TUMOnline" {
		tum.GetEventsForCourses([]model.Course{*tumLiveContext.Course}, r.DaoWrapper)
		c.Redirect(http.StatusFound, fmt.Sprintf("/admin/course/%v", tumLiveContext.Course.ID))
		return
	}
	access := c.PostForm("access")
	if match, err := regexp.MatchString("(public|loggedin|enrolled|hidden)", access); err != nil || !match {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"msg": "bad course id"})
		return
	}
	enVOD := c.PostForm("enVOD") == "on"
	enDL := c.PostForm("enDL") == "on"
	enChat := c.PostForm("enChat") == "on"
	enChatAnon := c.PostForm("enChatAnon") == "on"
	enChatMod := c.PostForm("enChatMod") == "on"
	livePrivate := c.PostForm("livePrivate") == "on"
	vodPrivate := c.PostForm("vodPrivate") == "on"
	tumLiveContext.Course.Visibility = access
	tumLiveContext.Course.VODEnabled = enVOD
	tumLiveContext.Course.DownloadsEnabled = enDL
	tumLiveContext.Course.ChatEnabled = enChat
	tumLiveContext.Course.AnonymousChatEnabled = enChatAnon
	tumLiveContext.Course.ModeratedChatEnabled = enChatMod
	tumLiveContext.Course.LivePrivate = livePrivate
	tumLiveContext.Course.VodPrivate = vodPrivate
	r.CoursesDao.UpdateCourseMetadata(context.Background(), *tumLiveContext.Course)
	c.Redirect(http.StatusFound, fmt.Sprintf("/admin/course/%v", tumLiveContext.Course.ID))
}

type AdminPageData struct {
	IndexData           IndexData
	Users               []model.User
	Courses             []model.Course
	LectureHalls        []model.LectureHall
	Page                string
	Workers             WorkersData
	Semesters           []dao.Semester
	CurY                int
	CurT                string
	EditCourseData      EditCourseData
	ServerNotifications []model.ServerNotification
	Tokens              []dao.AllTokensDto
	InfoPages           []model.InfoPage
	Notifications       []model.Notification
}

func (apd AdminPageData) UsersAsJson() string {
	type relevantUserInfo struct {
		ID    uint   `json:"id"`
		Name  string `json:"name"`
		Role  uint   `json:"role"`
		Email string `json:"email"`
	}

	users := make([]relevantUserInfo, len(apd.Users))
	for i, user := range apd.Users {
		users[i] = relevantUserInfo{
			ID:    user.ID,
			Name:  user.GetPreferredName(),
			Role:  user.Role,
			Email: user.Email.String,
		}
	}
	jsonStr, _ := json.Marshal(users)
	return string(jsonStr)
}

type EditCourseData struct {
	IndexData    IndexData
	IngestBase   string
	LectureHalls []model.LectureHall
}

type LectureUnitsPageData struct {
	IndexData IndexData
	Lecture   model.Stream
	Units     []model.StreamUnit
}
