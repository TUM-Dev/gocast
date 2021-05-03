package web

import (
	"TUM-Live/middleware"
	"TUM-Live/model"
	sentrygin "github.com/getsentry/sentry-go/gin"
	"github.com/gin-gonic/gin"
)

func CoursePage(c *gin.Context) {
	var tumLiveContext middleware.TUMLiveContext
	if found, exists := c.Get("TUMLiveContext"); exists {
		tumLiveContext = found.(middleware.TUMLiveContext)
	}
	indexData := NewIndexData()
	indexData.IsUser = tumLiveContext.User != nil || tumLiveContext.Student != nil
	indexData.IsAdmin = tumLiveContext.IsAdmin
	indexData.IsStudent = tumLiveContext.IsStudent
	err := templ.ExecuteTemplate(c.Writer, "course.gohtml", CoursePageData{IndexData: indexData, Course: *tumLiveContext.Course})
	if err != nil {
		sentrygin.GetHubFromContext(c).CaptureException(err)
	}
}

type CoursePageData struct {
	IndexData IndexData
	Course    model.Course
}
