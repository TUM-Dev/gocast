package web

import (
	"TUM-Live/dao"
	"TUM-Live/model"
	"TUM-Live/tools"
	"TUM-Live/tools/tum"
	"context"
	"fmt"
	"github.com/getsentry/sentry-go"
	"github.com/gin-gonic/gin"
	"log"
	"net/http"
	"regexp"
	"strconv"
)

func AdminPage(c *gin.Context) {
	user, err := tools.GetUser(c)
	if err != nil {
		c.Redirect(http.StatusFound, "/login")
		return
	}
	var users []model.User
	_ = dao.GetAllAdminsAndLecturers(&users)
	courses, err := dao.GetCoursesByUserId(context.Background(), user.ID)
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
	indexData.IsStudent = false
	indexData.IsUser = true
	indexData.IsAdmin = user.Role == model.AdminType || user.Role == model.LecturerType
	page := "courses"
	if _, ok := c.Request.URL.Query()["users"]; ok {
		page = "users"
	}
	if _, ok := c.Request.URL.Query()["lectureHalls"]; ok {
		page = "lectureHalls"
	}
	if _, ok := c.Request.URL.Query()["workers"]; ok {
		page = "workers"
	}
	if _, ok := c.Request.URL.Query()["schedule"]; ok {
		page = "schedule"
	}
	_ = templ.ExecuteTemplate(c.Writer, "admin.gohtml", AdminPageData{User: user, Users: users, Courses: courses, IndexData: indexData, LectureHalls: lectureHalls, Page: page, Workers: workers})
}

func LectureCutPage(c *gin.Context) {
	if u, uErr := tools.GetUser(c); uErr == nil {
		stream, sErr := dao.GetStreamByID(context.Background(), c.Param("streamID"))
		if sErr != nil {
			c.AbortWithStatusJSON(http.StatusNotFound, gin.H{"msg": "Not found."})
			return
		}
		if u.Role == model.AdminType || u.IsAdminOfCourse(stream.CourseID) {
			indexData := NewIndexData()
			indexData.IsAdmin = true
			indexData.IsUser = true
			if err := templ.ExecuteTemplate(c.Writer, "lecture-cut.gohtml", LectureUnitsPageData{
				IndexData: indexData,
				Lecture:   stream,
				Units:     stream.Units,
			}); err != nil {
				log.Fatalln(err)
			}
			return
		}
	}

	c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"msg": "you are not allowed to access this resource."})
}

func LectureUnitsPage(c *gin.Context) {
	if u, uErr := tools.GetUser(c); uErr == nil {
		stream, sErr := dao.GetStreamByID(context.Background(), c.Param("streamID"))
		if sErr != nil {
			c.AbortWithStatusJSON(http.StatusNotFound, gin.H{"msg": "Not found."})
			return
		}
		if u.Role == model.AdminType || u.IsAdminOfCourse(stream.CourseID) {
			indexData := NewIndexData()
			indexData.IsAdmin = true
			indexData.IsUser = true
			if err := templ.ExecuteTemplate(c.Writer, "lecture-units.gohtml", LectureUnitsPageData{
				IndexData: indexData,
				Lecture:   stream,
				Units:     stream.Units,
			}); err != nil {
				log.Fatalln(err)
			}
			return
		}
	}

	c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"msg": "you are not allowed to access this resource."})
}

func EditCoursePage(c *gin.Context) {
	user, err := tools.GetUser(c)
	if err != nil {
		c.Redirect(http.StatusFound, "/login")
		return
	}
	u64, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}
	course, err := dao.GetCourseById(context.Background(), uint(u64))
	if err != nil {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}
	// user has to be course owner or admin
	if user.Role != 1 && course.UserID != user.ID {
		c.AbortWithStatus(http.StatusForbidden)
		return
	}
	lectureHalls := dao.GetAllLectureHalls()
	indexData := NewIndexData()
	indexData.IsUser = true
	indexData.IsAdmin = user.Role == model.AdminType || user.Role == model.LecturerType
	err = templ.ExecuteTemplate(c.Writer, "edit-course.gohtml", EditCourseData{IndexData: indexData, IngestBase: tools.Cfg.IngestBase, Course: course, User: user, LectureHalls: lectureHalls})
	if err != nil {
		log.Printf("%v\n", err)
	}
}

func UpdateCourse(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"msg": "bad course id"})
		return
	}
	course, err := dao.GetCourseById(context.Background(), uint(id))
	if err != nil {
		c.AbortWithStatusJSON(http.StatusNotFound, gin.H{"msg": "course not found"})
		return
	}
	u, uErr := tools.GetUser(c)
	if uErr != nil || (u.Role > 1 && !u.IsAdminOfCourse(uint(id))) {
		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"msg": "unauthorized to edit this course."})
		return
	}
	if c.PostForm("submit") == "Reload Students From TUMOnline" {
		tum.FindStudentsForCourses([]model.Course{course})
		c.Redirect(http.StatusFound, fmt.Sprintf("/admin/course/%v", id))
		return
	} else if c.PostForm("submit") == "Reload Lectures From TUMOnline" {
		tum.GetEventsForCourses([]model.Course{course})
		c.Redirect(http.StatusFound, fmt.Sprintf("/admin/course/%v", id))
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
	course.Visibility = access
	course.VODEnabled = enVOD
	course.DownloadsEnabled = enDL
	course.ChatEnabled = enChat
	dao.UpdateCourseMetadata(context.Background(), course)
	c.Redirect(http.StatusFound, fmt.Sprintf("/admin/course/%v", id))
}

func CreateCoursePage(c *gin.Context) {
	user, err := tools.GetUser(c)
	if err != nil {
		c.Redirect(http.StatusFound, "/login")
		return
	}
	// check if user is admin or lecturer
	if user.Role > 2 {
		c.Redirect(http.StatusFound, "/login")
		return
	}
	indexData := NewIndexData()
	indexData.IsStudent = false
	indexData.IsUser = true
	indexData.IsAdmin = user.Role == model.AdminType || user.Role == model.LecturerType
	err = templ.ExecuteTemplate(c.Writer, "create-course.gohtml", CreateCourseData{User: user, IndexData: indexData})
	if err != nil {
		log.Printf("%v", err)
	}
}

type AdminPageData struct {
	IndexData    IndexData
	User         model.User
	Users        []model.User
	Courses      []model.Course
	LectureHalls []model.LectureHall
	Page         string
	Workers      []model.Worker
}

type CreateCourseData struct {
	IndexData IndexData
	User      model.User
}

type EditCourseData struct {
	IndexData    IndexData
	IngestBase   string
	Course       model.Course
	User         model.User
	LectureHalls []model.LectureHall
}

type LectureUnitsPageData struct {
	IndexData IndexData
	Lecture   model.Stream
	Units     []model.StreamUnit
}
