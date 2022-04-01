package web

import (
	"TUM-Live/dao"
	"TUM-Live/model"
	"TUM-Live/tools"
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"

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

	progs, err := dao.LoadProgressesForCourseAndUser((*tumLiveContext.Course).ID, (*tumLiveContext.User).ID)
	if err != nil {
		sentry.CaptureException(err)
		log.Printf("%v", err)
	}

	progressResponses, progressStreams := prepareCourseProgressData(tumLiveContext, progs)

	encoded, err := json.Marshal(progressResponses)
	if err != nil {
		sentry.CaptureException(err)
		log.Printf("%v", err)
	}
	// in any other case assume either validated before or public course
	err = templ.ExecuteTemplate(c.Writer, "course-overview.gohtml",
		CoursePageData{IndexData: indexData, Course: *tumLiveContext.Course, StreamsWithProgress: progressStreams, ProgressResponses: string(encoded)})
	if err != nil {
		sentrygin.GetHubFromContext(c).CaptureException(err)
	}
}

func prepareCourseProgressData(tumLiveContext tools.TUMLiveContext, streamProgresses []model.StreamProgress) ([]ProgressInfo, []ProgressStream) {
	var progressStreams []ProgressStream
	var progressResponses []ProgressInfo

	courseStreams := (*tumLiveContext.Course).Streams
	// Combine streams with existing progresses.
	for _, stream := range courseStreams {
		// We only want to track the progress for recordings.
		if !stream.Recording {
			continue
		}
		var prog model.StreamProgress
		var info ProgressInfo // Populated with minimum information to track stream progress.

		for _, p := range streamProgresses {
			if p.StreamID == stream.ID {
				prog = p
				info.Progress = p.Progress
				info.Watched = p.Watched
				break
			}
		}
		// Add match stream and progress.
		progressStreams = append(progressStreams, ProgressStream{Stream: stream, Progress: prog})

		info.Month = stream.Start.Month().String()
		info.ID = stream.ID
		progressResponses = append(progressResponses, info)
	}

	return progressResponses, progressStreams
}

// ProgressStream is a stream with its progress information. Used to generate a list of VoDs.
type ProgressStream struct {
	Progress model.StreamProgress
	Stream   model.Stream
}

// ProgressInfo is used by the client to track the which VoDs are watched.
type ProgressInfo struct {
	ID       uint    `json:"streamID"`
	Month    string  `json:"month"`
	Watched  bool    `json:"watched"`
	Progress float64 `json:"progress"`
}

// CoursePageData is the data for the course page.
type CoursePageData struct {
	IndexData           IndexData
	User                model.User
	Course              model.Course
	HighlightPage       bool
	StreamsWithProgress []ProgressStream
	ProgressResponses   string // JSON encoded
}
