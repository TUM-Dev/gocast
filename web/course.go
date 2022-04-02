package web

import (
	"TUM-Live/dao"
	"TUM-Live/model"
	"TUM-Live/tools"
	"encoding/json"
	"fmt"
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

func editCourseByTokenPage(c *gin.Context) {
	err := c.Request.ParseForm()
	if err != nil {
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}

	indexData := NewIndexDataWithContext(c)
	d := editCourseByTokenPageData{
		Token:     c.Request.Form.Get("token"),
		IndexData: indexData,
	}

	err = templ.ExecuteTemplate(c.Writer, "edit-course-by-token.gohtml", d)
	if err != nil {
		log.Println(err)
	}
}

func HighlightPage(c *gin.Context) {
	course, err := dao.GetCourseByShortLink(c.Param("shortLink"))
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
	s, err := dao.GetCurrentOrNextLectureForCourse(c, course.ID)
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
	if err = templ.ExecuteTemplate(c.Writer, "watch.gohtml", d2); err != nil {
		log.Printf("%v", err)
		return
	}
}

func CoursePage(c *gin.Context) {
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

	if tumLiveContext.User == nil {
		var streamWithProgesses []dao.ProgressStream
		for _, s := range tumLiveContext.Course.Streams {
			streamWithProgesses = append(streamWithProgesses, dao.ProgressStream{Stream: s})
		}
		// in any other case assume either validated before or public course
		err := templ.ExecuteTemplate(c.Writer, "course-overview.gohtml",
			CoursePageData{IndexData: indexData, Course: *tumLiveContext.Course, StreamsWithProgress: streamWithProgesses})
		if err != nil {
			sentrygin.GetHubFromContext(c).CaptureException(err)
		}
		return
	}

	progressStreams, err := dao.GetStreamsWithProgress((*tumLiveContext.Course).ID, (*tumLiveContext.User).ID)
	if err != nil {
		sentry.CaptureException(err)
		log.WithError(err).Error("loading progresses for course and user failed")
	}

	// StreamInfo is used by the client to track the which VoDs are watched.
	type streamInfo struct {
		ID      uint   `json:"streamID"`
		Month   string `json:"month"`
		Watched bool   `json:"watched"`
	}

	var streamInfos []streamInfo
	for _, s := range progressStreams {
		streamInfos = append(streamInfos, streamInfo{
			ID:      s.Stream.ID,
			Month:   s.Stream.Start.Month().String(),
			Watched: s.Progress.Watched,
		})
	}

	encoded, err := json.Marshal(streamInfos)
	if err != nil {
		sentry.CaptureException(err)
		log.WithError(err).Error("Marshalling progress data failed")
	}
	// in any other case assume either validated before or public course
	err = templ.ExecuteTemplate(c.Writer, "course-overview.gohtml",
		CoursePageData{IndexData: indexData, Course: *tumLiveContext.Course, StreamsWithProgress: progressStreams, ProgressResponses: string(encoded)})
	if err != nil {
		sentrygin.GetHubFromContext(c).CaptureException(err)
	}
}

// CoursePageData is the data for the course page.
type CoursePageData struct {
	IndexData           IndexData
	User                model.User
	Course              model.Course
	HighlightPage       bool
	StreamsWithProgress []dao.ProgressStream
	ProgressResponses   string // JSON encoded
}
