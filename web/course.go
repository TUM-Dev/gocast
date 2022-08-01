package web

import (
	"encoding/json"
	"fmt"
	"github.com/joschahenningsen/TUM-Live/model"
	"github.com/joschahenningsen/TUM-Live/tools"
	"html/template"
	"net/http"

	log "github.com/sirupsen/logrus"

	"github.com/getsentry/sentry-go"
	sentrygin "github.com/getsentry/sentry-go/gin"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type editCourseByTokenPageData struct {
	Token     string
	Course    model.Course
	IndexData IndexData
}

func (r mainRoutes) editCourseByTokenPage(c *gin.Context) {
	err := c.Request.ParseForm()
	if err != nil {
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}

	indexData := NewIndexDataWithContext(c)
	course, err := r.CoursesDao.GetCourseByToken(c, c.Request.Form.Get("token"))
	if err != nil {
		c.AbortWithStatusJSON(http.StatusNotFound, gin.H{"error": "Course not found"})
		return
	}
	d := editCourseByTokenPageData{
		Token:     c.Request.Form.Get("token"),
		Course:    course,
		IndexData: indexData,
	}

	err = templateExecutor.ExecuteTemplate(c, c.Writer, "edit-course-by-token.gohtml", d)
	if err != nil {
		log.Println(err)
	}
}

func (r mainRoutes) HighlightPage(c *gin.Context) {
	course, err := r.CoursesDao.GetCourseByShortLink(c, c.Param("shortLink"))
	if err != nil {
		tools.RenderErrorPage(c, http.StatusNotFound, tools.PageNotFoundErrMsg)
		return
	}
	indexData := NewIndexData()
	var tumLiveContext tools.TUMLiveContext
	tumLiveContextQueried, found := c.Get("TUMLiveContext")
	if found {
		tumLiveContext = tumLiveContextQueried.(tools.TUMLiveContext)
		indexData.TUMLiveContext = tumLiveContext
	} else {
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
	indexData.TUMLiveContext.Course = &course
	s, err := r.CoursesDao.GetCurrentOrNextLectureForCourse(c, course.ID)
	if err == nil {
		indexData.TUMLiveContext.Stream = &s
	} else if err == gorm.ErrRecordNotFound {
		c.Redirect(http.StatusFound, fmt.Sprintf("/course/%d/%s/%s", course.Year, course.TeachingTerm, course.Slug))
		return
	} else {
		sentry.CaptureException(err)
		log.Printf("%v", err)
	}
	description := ""
	if indexData.TUMLiveContext.Stream != nil {
		description = indexData.TUMLiveContext.Stream.GetDescriptionHTML()
	}
	_ = CoursePageData{IndexData: indexData, HighlightPage: true}
	d2 := WatchPageData{
		IndexData:       indexData,
		Description:     template.HTML(description),
		Version:         "",
		IsHighlightPage: true,
	}
	if err = templateExecutor.ExecuteTemplate(c, c.Writer, "watch.gohtml", d2); err != nil {
		log.Printf("%v", err)
		return
	}
}

func (r mainRoutes) CoursePage(c *gin.Context) {
	indexData := NewIndexData()
	var tumLiveContext tools.TUMLiveContext
	tumLiveContextQueried, found := c.Get("TUMLiveContext")
	if found {
		tumLiveContext = tumLiveContextQueried.(tools.TUMLiveContext)
		indexData.TUMLiveContext = tumLiveContext
	} else {
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}

	// When a user is not logged-in, we don't need the progress data for watch page since
	// it is only saved for logged-in users.
	if tumLiveContext.User == nil {
		err := templateExecutor.ExecuteTemplate(c, c.Writer, "course-overview.gohtml", CoursePageData{IndexData: indexData, Course: *tumLiveContext.Course})
		if err != nil {
			sentrygin.GetHubFromContext(c).CaptureException(err)
		}
		return
	}

	streamsWithWatchState, err := r.StreamsDao.GetStreamsWithWatchState(c, (*tumLiveContext.Course).ID, (*tumLiveContext.User).ID)
	if err != nil {
		sentry.CaptureException(err)
		log.WithError(err).Error("loading streamsWithWatchState and progresses for a given course and user failed")
	}

	tumLiveContext.Course.Streams = streamsWithWatchState // Update the course streams to contain the watch state.

	// watchedStateData is used by the client to track the which VoDs are watched.
	type watchedStateData struct {
		ID      uint   `json:"streamID"`
		Month   string `json:"month"`
		Watched bool   `json:"watched"`
	}

	var clientWatchState = make([]watchedStateData, 0)
	for _, s := range streamsWithWatchState {
		if !s.Recording {
			continue
		}
		clientWatchState = append(clientWatchState, watchedStateData{
			ID:      s.Model.ID,
			Month:   s.Start.Month().String(),
			Watched: s.Watched,
		})
	}
	// Create JSON encoded info about which streamsWithWatchState are watched. Used by the client to track the watched status.
	encoded, err := json.Marshal(clientWatchState)
	if err != nil {
		sentry.CaptureException(err)
		log.WithError(err).Error("marshalling watched infos for client failed")
	}
	err = templateExecutor.ExecuteTemplate(c, c.Writer, "course-overview.gohtml", CoursePageData{IndexData: indexData, Course: *tumLiveContext.Course, WatchedData: string(encoded)})
	if err != nil {
		sentrygin.GetHubFromContext(c).CaptureException(err)
	}
}

// CoursePageData is the data for the course page.
type CoursePageData struct {
	IndexData     IndexData
	Course        model.Course
	HighlightPage bool
	WatchedData   string
}
