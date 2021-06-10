package web

import (
	"TUM-Live/dao"
	"TUM-Live/model"
	"TUM-Live/tools"
	sentrygin "github.com/getsentry/sentry-go/gin"
	"github.com/gin-gonic/gin"
	"html/template"
	"log"
	"net/http"
)

func HighlightPage(c *gin.Context) {
	course, err := dao.GetCourseByShortLink(c.Param("shortLink"))
	if err != nil {
		c.AbortWithStatus(http.StatusNotFound)
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
	indexData.TUMLiveContext.Stream = course.GetCurrentOrNextLecture()
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
	// in any other case assume either validated before or public course
	err := templ.ExecuteTemplate(c.Writer, "course.gohtml", CoursePageData{IndexData: indexData, Course: *tumLiveContext.Course})
	if err != nil {
		sentrygin.GetHubFromContext(c).CaptureException(err)
	}
}

type CoursePageData struct {
	IndexData     IndexData
	Course        model.Course
	HighlightPage bool
}
