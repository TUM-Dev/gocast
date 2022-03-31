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

	var streamsWithProgress []StreamWithProgress
	var progressResponses []ProgressResponse

	for _, stream := range (*tumLiveContext.Course).Streams {
		var newProg model.StreamProgress
		var newResp ProgressResponse

		for _, prog := range progs {
			if prog.StreamID == stream.ID {
				newProg = prog
				newResp.Progress = prog.Progress
				newResp.Watched = prog.WatchStatus
				break
			}
		}
		newResp.Month = stream.Start.Month().String()
		newResp.ID = stream.ID
		progressResponses = append(progressResponses, newResp)
		streamsWithProgress = append(streamsWithProgress, StreamWithProgress{Stream: stream, Progress: newProg})
	}

	encoded, err := json.Marshal(progressResponses)
	if err != nil {
		sentry.CaptureException(err)
		log.Printf("%v", err)
	}
	// in any other case assume either validated before or public course
	err = templ.ExecuteTemplate(c.Writer, "course.gohtml", CoursePageData{IndexData: indexData, Course: *tumLiveContext.Course, StreamsWithProgress: streamsWithProgress, ProgressResponses: string(encoded)})
	if err != nil {
		sentrygin.GetHubFromContext(c).CaptureException(err)
	}
}

type StreamWithProgress struct {
	Progress model.StreamProgress
	Stream   model.Stream
}

// CoursePageData is the data for the course page.
type CoursePageData struct {
	IndexData           IndexData
	User                model.User
	Course              model.Course
	HighlightPage       bool
	StreamsWithProgress []StreamWithProgress
	ProgressResponses   string // JSON encoded
}

type ProgressResponse struct {
	ID       uint    `json:"streamID"`
	Month    string  `json:"month"`
	Watched  bool    `json:"watched"`
	Progress float64 `json:"progress"`
}
