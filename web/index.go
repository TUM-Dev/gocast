package web

import (
	"context"
	"errors"
	"github.com/RBG-TUM/commons"
	"github.com/gin-gonic/gin"
	"github.com/joschahenningsen/TUM-Live/dao"
	"github.com/joschahenningsen/TUM-Live/model"
	"github.com/joschahenningsen/TUM-Live/tools"
	"github.com/joschahenningsen/TUM-Live/tools/tum"
	log "github.com/sirupsen/logrus"
	"gorm.io/gorm"
	"html/template"
	"net/http"
	"sort"
	"strconv"
)

var VersionTag string

func (r mainRoutes) MainPage(c *gin.Context) {
	IsFreshInstallation(c, r.UsersDao)

	indexData := NewIndexDataWithContext(c)
	indexData.LoadCurrentNotifications(r.ServerNotificationDao)
	indexData.SetYearAndTerm(c)
	indexData.LoadSemesters(c, r.CoursesDao)
	indexData.LoadCoursesForRole(c, r.CoursesDao)
	indexData.LoadLivestreams(c, r.DaoWrapper)
	indexData.LoadPublicCourses(r.CoursesDao)
	indexData.LoadPinnedCourses()

	if err := templateExecutor.ExecuteTemplate(c.Writer, "index.gohtml", indexData); err != nil {
		log.WithError(err).Errorf("Could not execute template: 'index.gohtml'")
	}
}

func (r mainRoutes) AboutPage(c *gin.Context) {
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

	_ = templateExecutor.ExecuteTemplate(c.Writer, "about.gohtml", indexData)
}

func (r mainRoutes) InfoPage(id uint) gin.HandlerFunc {
	return func(c *gin.Context) {
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

		text, err := r.InfoPageDao.GetById(id)
		if err != nil {
			log.WithError(err).Error("Could not get text with id")
			c.AbortWithStatus(http.StatusInternalServerError)
			return
		}
		_ = templateExecutor.ExecuteTemplate(c.Writer, "info-page.gohtml", struct {
			IndexData
			Text template.HTML
		}{indexData, text.Render()})
	}
}

type IndexData struct {
	VersionTag          string
	TUMLiveContext      tools.TUMLiveContext
	IsUser              bool
	IsAdmin             bool
	IsStudent           bool
	LiveStreams         []CourseStream
	Courses             []model.Course
	PinnedCourses       []model.Course
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

// IsFreshInstallation Checks whether there are users in the database and executes the appropriate template for it
func IsFreshInstallation(c *gin.Context, usersDao dao.UsersDao) {
	res, err := usersDao.AreUsersEmpty(context.Background()) // fresh installation?
	if err != nil {
		_ = templateExecutor.ExecuteTemplate(c.Writer, "error.gohtml", nil)
		return
	} else if res {
		_ = templateExecutor.ExecuteTemplate(c.Writer, "onboarding.gohtml", NewIndexData())
		return
	}
}

// LoadCurrentNotifications Loads notifications from the database into the IndexData object
func (d *IndexData) LoadCurrentNotifications(serverNoticationDao dao.ServerNotificationDao) {
	if notifications, err := serverNoticationDao.GetCurrentServerNotifications(); err == nil {
		d.ServerNotifications = notifications
	} else if err != gorm.ErrRecordNotFound {
		log.WithError(err).Warn("could not get server notifications")
	}
}

// SetYearAndTerm Sets year and term on the IndexData object from the URL.
// Aborts with 404 if invalid
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

// LoadSemesters Load available Semesters from the database into the IndexData object
func (d *IndexData) LoadSemesters(ctx context.Context, coursesDao dao.CoursesDao) {
	d.Semesters = coursesDao.GetAvailableSemesters(ctx)
}

// LoadLivestreams Load non-hidden, currently live streams into the IndexData object.
// LoggedIn streams can only be seen by logged-in users.
// Enrolled streams can only be seen by users which are allowed to.
func (d *IndexData) LoadLivestreams(c *gin.Context, daoWrapper dao.DaoWrapper) {
	streams, err := daoWrapper.GetCurrentLive(context.Background())
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		log.WithError(err).Error("could not get current live streams")
		c.AbortWithStatusJSON(http.StatusNotFound, gin.H{"message": "Could not load current livestream from database."})
	}

	tumLiveContext := d.TUMLiveContext

	var livestreams []CourseStream

	for _, stream := range streams {
		courseForLiveStream, _ := daoWrapper.GetCourseById(context.Background(), stream.CourseID)

		// only show streams for logged in users if they are logged in
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
		if tumLiveContext.User != nil && tumLiveContext.User.Role == model.AdminType && stream.LectureHallID != 0 {
			lh, err := daoWrapper.LectureHallsDao.GetLectureHallByID(stream.LectureHallID)
			if err != nil {
				log.WithError(err).Error(err)
			} else {
				lectureHall = &lh
			}
		}
		livestreams = append(livestreams, CourseStream{
			Course:      courseForLiveStream,
			Stream:      stream,
			LectureHall: lectureHall,
		})
	}

	d.LiveStreams = livestreams
}

// LoadCoursesForRole Load all courses of user. Distinguishes between admin, lecturer, and normal users.
func (d *IndexData) LoadCoursesForRole(c *gin.Context, coursesDao dao.CoursesDao) {
	var courses []model.Course

	if d.TUMLiveContext.User != nil {
		switch d.TUMLiveContext.User.Role {
		case model.AdminType:
			courses = coursesDao.GetAllCoursesForSemester(d.CurrentYear, d.CurrentTerm, c)
		case model.LecturerType:
			{
				courses = d.TUMLiveContext.User.CoursesForSemester(d.CurrentYear, d.CurrentTerm, c)
				coursesForLecturer, err :=
					coursesDao.GetCourseForLecturerIdByYearAndTerm(c, d.CurrentYear, d.CurrentTerm, d.TUMLiveContext.User.ID)
				if err == nil {
					courses = append(courses, coursesForLecturer...)
				}
			}
		default:
			courses = d.TUMLiveContext.User.CoursesForSemester(d.CurrentYear, d.CurrentTerm, c)
		}
	}

	sortCourses(courses)

	d.Courses = commons.Unique(courses, func(c model.Course) uint { return c.ID })
}

func (d *IndexData) LoadPinnedCourses() {
	var pinnedCourses []model.Course

	if d.TUMLiveContext.User != nil {
		pinnedCourses = d.TUMLiveContext.User.PinnedCourses
		for i := range pinnedCourses {
			pinnedCourses[i].Pinned = true
		}
		sortCourses(pinnedCourses)
		d.PinnedCourses = commons.Unique(pinnedCourses, func(c model.Course) uint { return c.ID })
	} else {
		d.PinnedCourses = []model.Course{}
	}
}

// LoadPublicCourses Load public courses of user. Filter courses which are already in IndexData.Courses
func (d *IndexData) LoadPublicCourses(coursesDao dao.CoursesDao) {
	var public []model.Course
	var err error

	if d.TUMLiveContext.User != nil {
		public, err = coursesDao.GetPublicAndLoggedInCourses(d.CurrentYear, d.CurrentTerm)
	} else {
		public, err = coursesDao.GetPublicCourses(d.CurrentYear, d.CurrentTerm)
	}

	if err != nil {
		d.PublicCourses = []model.Course{}
	} else {
		sortCourses(public)
		d.PublicCourses = commons.Unique(public, func(c model.Course) uint { return c.ID })
	}
}

type CourseStream struct {
	Course      model.Course
	Stream      model.Stream
	LectureHall *model.LectureHall
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

func sortCourses(courses []model.Course) {
	sort.Slice(courses, func(i, j int) bool {
		return courses[i].CompareTo(courses[j])
	})
}
