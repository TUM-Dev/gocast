package web

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strconv"

	"github.com/TUM-Dev/gocast/dao"
	"github.com/TUM-Dev/gocast/model"
	"github.com/TUM-Dev/gocast/tools"
	"github.com/TUM-Dev/gocast/tools/tum"
	"github.com/getsentry/sentry-go"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
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
	schools, err := r.SchoolsDao.GetAdministeredSchoolsByUser(context.Background(), tumLiveContext.User)
	if err != nil {
		logger.Error("couldn't query schools for user.", "err", err)
		schools = []model.School{}
	}
	workers := []model.Worker{}
	runners := []model.Runner{}
	ingestServers := []model.IngestServer{}
	for _, school := range schools {
		workers = append(workers, school.Workers...)
		runners = append(runners, school.Runners...)
		ingestServers = append(ingestServers, school.IngestServers...)
	}

	courses, err := r.CoursesDao.GetAdministeredCoursesByUserId(context.Background(), tumLiveContext.User.ID, "", 0)
	if err != nil {
		logger.Error("couldn't query courses for user.", "err", err)
		courses = []model.Course{}
	}

	lectureHalls := r.LectureHallsDao.GetAllLectureHalls()
	indexData := NewIndexData()
	indexData.TUMLiveContext = tumLiveContext
	page := "schedule"
	if c.Request.URL.Path == "/admin/users" {
		page = "users"
	}
	if c.Request.URL.Path == "/admin/schools" {
		page = "schools"
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
	if c.Request.URL.Path == "/admin/runners" {
		page = "runners"
	}
	if c.Request.URL.Path == "/admin/resources" {
		page = "resources"
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
			logger.Error("couldn't query notifications", "err", err)
		} else {
			notifications = found
		}
	}
	var tokens []dao.AllTokensDto
	if c.Request.URL.Path == "/admin/token" {
		page = "token"
		tokens, err = r.TokenDao.GetAllTokens(tumLiveContext.User)
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			logger.Error("couldn't query tokens", "err", err)
			c.AbortWithStatus(http.StatusInternalServerError)
		}
	}
	var infopages []model.InfoPage
	if c.Request.URL.Path == "/admin/infopages" {
		page = "info-pages"
		infopages, err = r.InfoPageDao.GetAll()
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			logger.Error("couldn't query texts", "err", err)
			c.AbortWithStatus(http.StatusInternalServerError)
		}
	}
	if c.Request.URL.Path == "/admin/server-stats" {
		page = "serverStats"
		streams, err := r.StreamsDao.GetAllStreams()
		if err != nil {
			logger.Error("Can't get all streams", "err", err)
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
			logger.Warn("could not get all server notifications", "err", err)
		}
	}
	semesters := r.CoursesDao.GetAvailableSemesters(c)
	y, t := tum.GetCurrentSemester()

	query, _ := strconv.ParseUint(c.Request.URL.Query().Get("id"), 10, 32)

	err = templateExecutor.ExecuteTemplate(c.Writer, "admin.gohtml",
		AdminPageData{
			Users:               users,
			Schools:             schools,
			Courses:             courses,
			IndexData:           indexData,
			LectureHalls:        lectureHalls,
			Page:                page,
			Workers:             WorkersData{Workers: workers, Token: tools.Cfg.WorkerToken},
			Semesters:           semesters,
			CurY:                y,
			CurT:                t,
			Tokens:              TokensData{Tokens: tokens, IngestBase: tools.Cfg.IngestBase, User: tumLiveContext.User},
			InfoPages:           infopages,
			ServerNotifications: serverNotifications,
			Notifications:       notifications,
			Runners:             RunnersData{Runners: runners},
			Resources: Resources{
				Workers:       workers,
				Runners:       runners,
				VODServices:   runners,
				IngestServers: ingestServers,
				Schools:       schools,
				Query:         uint(query),
			},
		})
	if err != nil {
		logger.Error("Error executing template admin.gohtml", "err", err)
	}
}

type WorkersData struct {
	Workers []model.Worker
	Token   string
}

type TokensData struct {
	Tokens     []dao.AllTokensDto
	IngestBase string
	User       *model.User
}

type Resources struct {
	Workers       []model.Worker
	Runners       []model.Runner
	VODServices   []model.Runner
	IngestServers []model.IngestServer
	Schools       []model.School
	Query         uint
}

type RunnersData struct {
	Runners []model.Runner
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
		logger.Error("Error executing template lecture-cut.gohtml", "err", err)
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
		logger.Error("couldn't query courses for user.", "err", err)
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
		logger.Error("Error getting available semesters", "err", err)
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
		logger.Error("Error getting invited users for course", "err", err)
	}
	indexData := NewIndexData()
	indexData.TUMLiveContext = tumLiveContext
	courses, err := r.CoursesDao.GetAdministeredCoursesByUserId(context.Background(), tumLiveContext.User.ID, "", 0)
	if err != nil {
		logger.Error("couldn't query courses for user.", "err", err)
		courses = []model.Course{}
	}
	semesters := r.CoursesDao.GetAvailableSemesters(c)
	for i := range tumLiveContext.Course.Streams {
		err := tools.SetSignedPlaylists(&tumLiveContext.Course.Streams[i], tumLiveContext.User, true)
		if err != nil {
			logger.Error("could not set signed playlist for admin page", "err", err)
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
		logger.Error("Error executing template admin.gohtml", "err", err)
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
	Schools             []model.School
	Courses             []model.Course
	LectureHalls        []model.LectureHall
	Page                string
	Workers             WorkersData
	Semesters           []dao.Semester
	CurY                int
	CurT                string
	EditCourseData      EditCourseData
	ServerNotifications []model.ServerNotification
	Tokens              TokensData
	InfoPages           []model.InfoPage
	Notifications       []model.Notification
	Runners             RunnersData
	Resources           Resources
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

func (apd AdminPageData) SchoolsAsJson() string {
	type relevantSchoolInfo struct {
		ID       uint         `json:"id"`
		Name     string       `json:"name"`
		OrgId    string       `json:"orgId"`
		OrgType  string       `json:"orgType"`
		OrgSlug  string       `json:"orgSlug"`
		Admins   []model.User `json:"admins"`
		ParentID uint         `json:"parent_id"`
	}

	schools := make([]relevantSchoolInfo, len(apd.Schools))
	for i, school := range apd.Schools {
		schools[i] = relevantSchoolInfo{
			ID:       school.ID,
			Name:     school.Name,
			OrgId:    school.OrgId,
			OrgType:  school.OrgType,
			OrgSlug:  school.OrgSlug,
			Admins:   school.Admins,
			ParentID: school.ParentID,
		}
	}

	jsonStr, _ := json.Marshal(schools)
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
