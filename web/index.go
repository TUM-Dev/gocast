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

func LoadCurrentNotifications() []model.ServerNotification {
	if notifications, err := dao.GetCurrentServerNotifications(); err == nil {
		return notifications
	} else if err != gorm.ErrRecordNotFound {
		log.WithError(err).Warn("could not get server notifications")
	}
	return nil
}

func GetYearAndTerm(c *gin.Context) (int, string) {
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

	return year, term
}

func LoadLivestreams(tumLiveContext tools.TUMLiveContext, c *gin.Context) []CourseStream {
	streams, err := dao.GetCurrentLive(context.Background())
	if err != nil {
		c.AbortWithStatusJSON(http.StatusNotFound, gin.H{"message": "Could not load current livestream from database."})
	}

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

	return livestreams
}

// TODO: Too many parameters
func LoadCoursesForRole(tumLiveContext tools.TUMLiveContext, year int, term string, spanMain *sentry.Span, c *gin.Context) []model.Course {
	var courses []model.Course

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

	return courses
}

func CourseListToIdList(courses []model.Course) []uint {
	var idList []uint
	for _, c := range courses {
		idList = append(idList, c.ID)
	}
	return idList
}

func MainPage(c *gin.Context) {
	tName := sentry.TransactionName("GET /")
	spanMain := sentry.StartSpan(c.Request.Context(), "MainPageHandler", tName)
	defer spanMain.Finish()

	IsFreshInstallation(c)

	indexData, tumLiveContext := NewIndexDataWithContext(c)
	indexData.ServerNotifications = LoadCurrentNotifications()

	year, term := GetYearAndTerm(c)

	indexData.Semesters = dao.GetAvailableSemesters(spanMain.Context())
	indexData.CurrentYear = year
	indexData.CurrentTerm = term

	indexData.Courses = LoadCoursesForRole(tumLiveContext, year, term, spanMain, c)
	indexData.LiveStreams = LoadLivestreams(tumLiveContext, c)

	var public []model.Course
	var err error
	myCourses := CourseListToIdList(indexData.Courses)
	if len(myCourses) > 0 {
		public, err = dao.GetPublicCoursesWithoutOwn(year, term, myCourses)
	} else {
		public, err = dao.GetPublicCourses(year, term)
	}

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

	if err != nil {
		indexData.PublicCourses = []model.Course{}
	} else {
		var publicFiltered = public

		if tumLiveContext.User != nil {
			loggedIn, _ := dao.GetCoursesForLoggedInUsers(year, term)
			for _, c := range loggedIn {
				if !tools.CourseListContains(indexData.Courses, c.ID) {
					publicFiltered = append(publicFiltered, c)
				}
			}
		}
		indexData.PublicCourses = publicFiltered
	}
	sort.Slice(indexData.PublicCourses, func(i, j int) bool {
		return indexData.PublicCourses[i].CompareTo(indexData.PublicCourses[j])
	})

	sort.Slice(indexData.Courses, func(i, j int) bool {
		return indexData.Courses[i].CompareTo(indexData.Courses[j])
	})
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

func NewIndexDataWithContext(c *gin.Context) (IndexData, tools.TUMLiveContext) {
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

	return indexData, tumLiveContext
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
