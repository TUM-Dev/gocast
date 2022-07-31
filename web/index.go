package web

import (
	"context"
	"errors"
	"github.com/RBG-TUM/commons"
	"github.com/getsentry/sentry-go"
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
	"time"
)

var VersionTag string

func (r mainRoutes) MainPage(c *gin.Context) {
	tName := sentry.TransactionName("GET /")
	spanMain := sentry.StartSpan(c.Request.Context(), "MainPageHandler", tName)
	defer spanMain.Finish()

	IsFreshInstallation(c, r.UsersDao)

	indexData := NewIndexDataWithContext(c)
	indexData.LoadCurrentNotifications(r.ServerNotificationDao)
	indexData.SetYearAndTerm(c)
	indexData.LoadSemesters(spanMain, r.CoursesDao)
	indexData.LoadCoursesForRole(c, spanMain, r.CoursesDao)
	indexData.LoadLivestreams(c, r.DaoWrapper)
	indexData.LoadPublicCourses(r.CoursesDao)
	indexData.LoadPinnedCourses()
	indexData.LoadSuggestedStreams(c, r.DaoWrapper, 16)
	indexData.PropagatePinnedCourses()

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
	SuggestedStreams    []CourseStream
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
func (d *IndexData) LoadSemesters(spanMain *sentry.Span, coursesDao dao.CoursesDao) {
	d.Semesters = coursesDao.GetAvailableSemesters(spanMain.Context())
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

// LoadSuggestedStreams loads suggested streams into the IndexData. Livestreams, courses and pinned courses need to be loaded already.
// TODO: properly query the database, this is a hack
// TODO: respect pinning
func (d *IndexData) LoadSuggestedStreams(c *gin.Context, daoWrapper dao.DaoWrapper, limit int) {
	if limit == 0 {
		limit = 16
	}
	var suggestedStreams []CourseStream
	var relevantCourses []model.Course

	relevantCourses = append(relevantCourses, d.Courses...)
	relevantCourses = append(relevantCourses, d.PinnedCourses...)
	relevantCourses = commons.Unique(relevantCourses, func(c model.Course) uint { return c.ID })

	for _, stream := range d.LiveStreams {
		// logged-in users only see streams from their courses and their pinned courses suggested
		var keep = true
		if len(relevantCourses) > 0 {
			keep = false
			for _, course := range relevantCourses {
				if stream.Course.ID == course.ID {
					keep = true
					break
				}
			}
		}
		if !keep {
			continue
		}
		suggestedStreams = append(suggestedStreams, stream)
	}

	// TODO: logged out users should maybe see recent VoDs from all courses instead of none
	if d.TUMLiveContext.User != nil {
		var suggestedVoDs []CourseStream
		var now = time.Now()
		for _, course := range relevantCourses {
			streams, err := daoWrapper.GetStreamsWithWatchState(course.ID, d.TUMLiveContext.User.ID)
			if err != nil {
				log.WithError(err).Error("could not get live streams??")
				continue
			}
			for _, stream := range streams {
				// filter out watched streams and upcoming streams
				if stream.End.After(now) || stream.Watched || stream.Progress > 0.5 {
					continue
				}
				var lectureHall *model.LectureHall
				if d.TUMLiveContext.User.Role == model.AdminType && stream.LectureHallID != 0 {
					lh, err := daoWrapper.LectureHallsDao.GetLectureHallByID(stream.LectureHallID)
					if err != nil {
						log.WithError(err).Error(err)
					} else {
						lectureHall = &lh
					}
				}
				suggestedVoDs = append(suggestedVoDs, CourseStream{
					Course:      course,
					Stream:      stream,
					LectureHall: lectureHall,
				})
			}
		}
		sort.Slice(suggestedVoDs, func(i, j int) bool { return suggestedVoDs[i].Stream.End.After(suggestedVoDs[j].Stream.End) })
		suggestedStreams = append(suggestedStreams, suggestedVoDs...)
	}

	suggestedStreams = commons.Unique(suggestedStreams, func(c CourseStream) uint { return c.Stream.ID })
	d.SuggestedStreams = suggestedStreams
	if len(suggestedStreams) > limit {
		d.SuggestedStreams = suggestedStreams[0:limit]
	}
}

//LiveStreams         []CourseStream
//SuggestedStreams    []CourseStream
//Courses             []model.Course
//PinnedCourses       []model.Course
//PublicCourses

// PropagatePinnedCourses
// TODO replace with proper implementation
func (d *IndexData) PropagatePinnedCourses() {
	if d.TUMLiveContext.User != nil {
		var targetCourses []*model.Course
		for _, streamCourses := range []*[]CourseStream{&d.LiveStreams, &d.SuggestedStreams} {
			for i := range *streamCourses {
				targetCourses = append(targetCourses, &((*streamCourses)[i].Course))
			}
		}
		for _, courses := range []*[]model.Course{&d.Courses, &d.PinnedCourses} {
			for i := range *courses {
				targetCourses = append(targetCourses, &((*courses)[i]))
			}
		}
		for _, pinnedCourse := range d.TUMLiveContext.User.PinnedCourses {
			for _, targetCourse := range targetCourses {
				if pinnedCourse.ID == targetCourse.ID {
					targetCourse.Pinned = true
				}
			}
		}
	}
}

// LoadCoursesForRole Load all courses of user. Distinguishes between admin, lecturer, and normal users.
func (d *IndexData) LoadCoursesForRole(c *gin.Context, spanMain *sentry.Span, coursesDao dao.CoursesDao) {
	var courses []model.Course

	if d.TUMLiveContext.User != nil {
		switch d.TUMLiveContext.User.Role {
		case model.AdminType:
			courses = coursesDao.GetAllCoursesForSemester(d.CurrentYear, d.CurrentTerm, spanMain.Context())
		case model.LecturerType:
			{
				courses = d.TUMLiveContext.User.CoursesForSemester(d.CurrentYear, d.CurrentTerm, spanMain.Context())
				coursesForLecturer, err :=
					coursesDao.GetCourseForLecturerIdByYearAndTerm(c, d.CurrentYear, d.CurrentTerm, d.TUMLiveContext.User.ID)
				if err == nil {
					courses = append(courses, coursesForLecturer...)
				}
			}
		default:
			courses = d.TUMLiveContext.User.CoursesForSemester(d.CurrentYear, d.CurrentTerm, spanMain.Context())
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
