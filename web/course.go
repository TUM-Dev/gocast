package web

import (
	"TUM-Live/model"
	"TUM-Live/tools"
	sentrygin "github.com/getsentry/sentry-go/gin"
	"github.com/gin-gonic/gin"
	"net/http"
)

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
	IndexData IndexData
	Course    model.Course
}
