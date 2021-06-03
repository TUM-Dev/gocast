package web

import (
	"TUM-Live/dao"
	"TUM-Live/model"
	"TUM-Live/tools"
	"TUM-Live/tools/tum"
	"context"
	"errors"
	"fmt"
	"github.com/getsentry/sentry-go"
	"github.com/gin-gonic/gin"
	"log"
	"net/http"
	"regexp"
)

func AdminPage(c *gin.Context) {
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
	_ = dao.GetAllAdminsAndLecturers(&users)
	courses, err := dao.GetCoursesByUserId(context.Background(), tumLiveContext.User.ID)
	if err != nil {
		log.Printf("couldn't query courses for user. %v\n", err)
		courses = []model.Course{}
	}
	workers, err := dao.GetAllWorkers()
	if err != nil {
		sentry.CaptureException(err)
	}
	lectureHalls := dao.GetAllLectureHalls()
	indexData := NewIndexData()
	indexData.TUMLiveContext = tumLiveContext
	page := "schedule"
	if c.Request.URL.Path == "/admin/users"{
		page = "users"
	}
	if c.Request.URL.Path == "/admin/lectureHalls"{
		page = "lectureHalls"
	}
	if c.Request.URL.Path == "/admin/workers"{
		page = "workers"
	}
	semesters := dao.GetAvailableSemesters(c)
	y, t := tum.GetCurrentSemester()
	err = templ.ExecuteTemplate(c.Writer, "admin.gohtml",
		AdminPageData{Users: users,
			Courses:      courses,
			IndexData:    indexData,
			LectureHalls: lectureHalls,
			Page:         page,
			Workers:      workers,
			Semesters:    semesters,
			CurY:         y,
			CurT:         t})
	if err != nil {
		log.Printf("%v", err)
	}
}

func LectureCutPage(c *gin.Context) {
	foundContext, exists := c.Get("TUMLiveContext")
	if !exists {
		sentry.CaptureException(errors.New("context should exist but doesn't"))
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
	tumLiveContext := foundContext.(tools.TUMLiveContext)
	if err := templ.ExecuteTemplate(c.Writer, "lecture-cut.gohtml", tumLiveContext); err != nil {
		log.Fatalln(err)
	}
}

func LectureUnitsPage(c *gin.Context) {
	foundContext, exists := c.Get("TUMLiveContext")
	if !exists {
		sentry.CaptureException(errors.New("context should exist but doesn't"))
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
	tumLiveContext := foundContext.(tools.TUMLiveContext)
	indexData := NewIndexData()
	indexData.TUMLiveContext = tumLiveContext
	if err := templ.ExecuteTemplate(c.Writer, "lecture-units.gohtml", LectureUnitsPageData{
		IndexData: indexData,
		Lecture:   *tumLiveContext.Stream,
		Units:     tumLiveContext.Stream.Units,
	}); err != nil {
		sentry.CaptureException(err)
	}
}

func EditCoursePage(c *gin.Context) {
	foundContext, exists := c.Get("TUMLiveContext")
	if !exists {
		sentry.CaptureException(errors.New("context should exist but doesn't"))
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
	tumLiveContext := foundContext.(tools.TUMLiveContext)
	lectureHalls := dao.GetAllLectureHalls()
	err := dao.GetInvitedUsersForCourse(tumLiveContext.Course)
	if err != nil {
		log.Printf("%v", err)
	}
	indexData := NewIndexData()
	indexData.TUMLiveContext = tumLiveContext
	courses, err := dao.GetCoursesByUserId(context.Background(), tumLiveContext.User.ID)
	if err != nil {
		log.Printf("couldn't query courses for user. %v\n", err)
		courses = []model.Course{}
	}
	semesters := dao.GetAvailableSemesters(c)
	err = templ.ExecuteTemplate(c.Writer, "admin.gohtml", AdminPageData{
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

func UpdateCourse(c *gin.Context) {
	foundContext, exists := c.Get("TUMLiveContext")
	if !exists {
		sentry.CaptureException(errors.New("context should exist but doesn't"))
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
	tumLiveContext := foundContext.(tools.TUMLiveContext)
	if c.PostForm("submit") == "Reload Students From TUMOnline" {
		tum.FindStudentsForCourses([]model.Course{*tumLiveContext.Course})
		c.Redirect(http.StatusFound, fmt.Sprintf("/admin/course/%v", tumLiveContext.Course.ID))
		return
	} else if c.PostForm("submit") == "Reload Lectures From TUMOnline" {
		tum.GetEventsForCourses([]model.Course{*tumLiveContext.Course})
		c.Redirect(http.StatusFound, fmt.Sprintf("/admin/course/%v", tumLiveContext.Course.ID))
		return
	}
	access := c.PostForm("access")
	if match, err := regexp.MatchString("(public|loggedin|enrolled)", access); err != nil || !match {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"msg": "bad course id"})
		return
	}
	enVOD := c.PostForm("enVOD") == "on"
	enDL := c.PostForm("enDL") == "on"
	enChat := c.PostForm("enChat") == "on"
	tumLiveContext.Course.Visibility = access
	tumLiveContext.Course.VODEnabled = enVOD
	tumLiveContext.Course.DownloadsEnabled = enDL
	tumLiveContext.Course.ChatEnabled = enChat
	dao.UpdateCourseMetadata(context.Background(), *tumLiveContext.Course)
	c.Redirect(http.StatusFound, fmt.Sprintf("/admin/course/%v", tumLiveContext.Course.ID))
}

func CreateCoursePage(c *gin.Context) {
	foundContext, exists := c.Get("TUMLiveContext")
	if !exists {
		sentry.CaptureException(errors.New("context should exist but doesn't"))
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
	tumLiveContext := foundContext.(tools.TUMLiveContext)
	indexData := NewIndexData()
	indexData.TUMLiveContext = tumLiveContext
	err := templ.ExecuteTemplate(c.Writer, "create-course.gohtml", CreateCourseData{IndexData: indexData})
	if err != nil {
		log.Printf("%v", err)
	}
}

type AdminPageData struct {
	IndexData      IndexData
	Users          []model.User
	Courses        []model.Course
	LectureHalls   []model.LectureHall
	Page           string
	Workers        []model.Worker
	Semesters      []dao.Semester
	CurY           int
	CurT           string
	EditCourseData EditCourseData
}

type CreateCourseData struct {
	IndexData IndexData
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
