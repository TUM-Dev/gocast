package web

import (
	"TUM-Live/dao"
	"TUM-Live/model"
	"TUM-Live/tools"
	"TUM-Live/tools/tum"
	"context"
	"github.com/getsentry/sentry-go"
	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
	"gorm.io/gorm"
	"net/http"
	"sort"
	"strconv"
)

var VersionTag string

func MainPage(c *gin.Context) {
	tName := sentry.TransactionName("GET /")
	spanMain := sentry.StartSpan(c.Request.Context(), "MainPageHandler", tName)
	defer spanMain.Finish()

	IsFreshInstallation(c)

	indexData := NewIndexDataWithContext(c)
	indexData.LoadCurrentNotifications()
	indexData.SetYearAndTerm(c)
	indexData.LoadSemesters(spanMain)
	indexData.LoadCoursesForRole(c, spanMain)
	indexData.LoadLivestreams(c)
	indexData.LoadPublicCourses()

	// TODO:
	/*
		Possibilites
		myCourses.len = 0
			- loggedIn
			- not loggedIn
		myCourses.len > 0
			- loggedIn
			- not loggedIn
	*/
	_ = templ.ExecuteTemplate(c.Writer, "index.gohtml", indexData)
}

func AboutPage(c *gin.Context) {
	var indexData IndexData
	var tumLiveContext tools.TUMLiveContext
	tumLiveContextQueried, found := c.Get("TUMLiveContext")
	if found {
		tumLiveContext = tumLiveContextQueried.(tools.TUMLiveContext)
		indexData.TUMLiveContext = tumLiveContext
	} else {
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
	indexData.VersionTag = VersionTag

	_ = templ.ExecuteTemplate(c.Writer, "about.gohtml", indexData)
}

type IndexData struct {
	VersionTag          string
	TUMLiveContext      tools.TUMLiveContext
	IsUser              bool
	IsAdmin             bool
	IsStudent           bool
	LiveStreams         []CourseStream
	Courses             []model.Course
	PublicCourses       []model.Course
	Semesters           []dao.Semester
	CurrentYear         int
	CurrentTerm         string
	UserName            string
	ServerNotifications []model.ServerNotification
}

func NewIndexData() IndexData {
	return IndexData{
		VersionTag: VersionTag,
	}
}

func NewIndexDataWithContext(c *gin.Context) IndexData {
	indexData := IndexData{
		VersionTag: VersionTag,
	}

	var tumLiveContext tools.TUMLiveContext
	tumLiveContextQueried, found := c.Get("TUMLiveContext")
	if found {
		tumLiveContext = tumLiveContextQueried.(tools.TUMLiveContext)
		indexData.TUMLiveContext = tumLiveContext
	} else {
		log.Warn("could not get TUMLiveContext")
		c.AbortWithStatus(http.StatusInternalServerError)
	}

	return indexData
}

func IsFreshInstallation(c *gin.Context) {
	res, err := dao.AreUsersEmpty(context.Background()) // fresh installation?
	if err != nil {
		_ = templ.ExecuteTemplate(c.Writer, "error.gohtml", nil)
		return
	} else if res {
		_ = templ.ExecuteTemplate(c.Writer, "onboarding.gohtml", nil)
		return
	}
}

func (d *IndexData) LoadCurrentNotifications() {
	if notifications, err := dao.GetCurrentServerNotifications(); err == nil {
		d.ServerNotifications = notifications
	} else if err != gorm.ErrRecordNotFound {
		log.WithError(err).Warn("could not get server notifications")
	}
}

func (d *IndexData) SetYearAndTerm(c *gin.Context) {
	var year int
	var term string
	var err error
	if c.Param("year") == "" {
		year, term = tum.GetCurrentSemester()
	} else {
		term = c.Param("term")
		year, err = strconv.Atoi(c.Param("year"))
		if err != nil || (term != "W" && term != "S") {
			c.AbortWithStatusJSON(http.StatusNotFound, gin.H{"message": "Bad semester format in url."})
		}
	}

	d.CurrentYear = year
	d.CurrentTerm = term
}

func (d *IndexData) LoadSemesters(spanMain *sentry.Span) {
	d.Semesters = dao.GetAvailableSemesters(spanMain.Context())
}

func (d *IndexData) LoadLivestreams(c *gin.Context) {
	streams, err := dao.GetCurrentLive(context.Background())
	if err != nil {
		c.AbortWithStatusJSON(http.StatusNotFound, gin.H{"message": "Could not load current livestream from database."})
	}

	tumLiveContext := d.TUMLiveContext

	var livestreams []CourseStream

	for _, stream := range streams {
		courseForLiveStream, _ := dao.GetCourseById(context.Background(), stream.CourseID)
		// Todo: refactor into dao
		if courseForLiveStream.Visibility == "hidden" {
			continue
		}
		if courseForLiveStream.Visibility == "loggedin" && tumLiveContext.User == nil {
			continue
		}
		if courseForLiveStream.Visibility == "enrolled" {
			if !isUserAllowedToWatchPrivateCourse(courseForLiveStream, tumLiveContext.User) {
				continue
			}
		}
		livestreams = append(livestreams, CourseStream{
			Course: courseForLiveStream,
			Stream: stream,
		})
	}

	d.LiveStreams = livestreams
}

func (d *IndexData) LoadCoursesForRole(c *gin.Context, spanMain *sentry.Span) {
	var courses []model.Course
	tumLiveContext := d.TUMLiveContext
	year := d.CurrentYear
	term := d.CurrentTerm

	if tumLiveContext.User != nil {
		switch tumLiveContext.User.Role {
		case model.AdminType:
			courses = dao.GetAllCoursesForSemester(year, term, spanMain.Context())
		case model.LecturerType:
			{
				courses = tumLiveContext.User.CoursesForSemester(year, term, spanMain.Context())
				coursesForLecturer, err := dao.GetCourseForLecturerIdByYearAndTerm(c, year, term, tumLiveContext.User.ID)
				if err == nil {
					courses = append(courses, coursesForLecturer...)
				}
			}
		default:
			courses = tumLiveContext.User.CoursesForSemester(year, term, spanMain.Context())
		}
	}

	sort.Slice(courses, func(i, j int) bool {
		return courses[i].CompareTo(courses[j])
	})

	d.Courses = courses
}

func (d *IndexData) LoadPublicCourses() {
	year := d.CurrentYear
	term := d.CurrentTerm
	tumLiveContext := d.TUMLiveContext
	courses := d.Courses

	var public []model.Course
	var err error

	if len(d.Courses) > 0 {
		public, err = dao.GetPublicCoursesWithoutOwn(year, term, CourseListToIdList(d.Courses))
	} else {
		public, err = dao.GetPublicCourses(year, term)
	}
	if err != nil {
		d.PublicCourses = []model.Course{}
	} else {
		var publicFiltered = public

		if tumLiveContext.User != nil {
			loggedIn, _ := dao.GetCoursesForLoggedInUsers(year, term)
			for _, c := range loggedIn {
				if !tools.CourseListContains(courses, c.ID) {
					publicFiltered = append(publicFiltered, c)
				}
			}
		}

		sort.Slice(publicFiltered, func(i, j int) bool {
			return publicFiltered[i].CompareTo(publicFiltered[j])
		})

		d.PublicCourses = publicFiltered
	}
}

func CourseListToIdList(courses []model.Course) []uint {
	var idList []uint
	for _, c := range courses {
		idList = append(idList, c.ID)
	}
	return idList
}

type CourseStream struct {
	Course model.Course
	Stream model.Stream
}

func isUserAllowedToWatchPrivateCourse(course model.Course, user *model.User) bool {
	if user != nil {
		for _, c := range user.Courses {
			if c.ID == course.ID {
				return true
			}
		}
		return user.Role == model.AdminType || user.ID == course.UserID
	}
	return false
}
