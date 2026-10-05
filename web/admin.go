package web

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/TUM-Dev/gocast/model"
	"github.com/TUM-Dev/gocast/tools"
)

// AdminPage serves all administration pages. todo: refactor into multiple methods
func (r mainRoutes) LectureCutPage(c *gin.Context) {
	foundContext, exists := c.Get("TUMLiveContext")
	if !exists {
		logger.Error("context should exist but doesn't")
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
	tumLiveContext := foundContext.(tools.TUMLiveContext)
	if err := templateExecutor.ExecuteTemplate(c.Writer, "lecture-cut.gohtml", tumLiveContext); err != nil {
		logger.Error("Error executing template lecture-cut.gohtml", "err", err)
	}
}

func (r mainRoutes) LectureUnitsPage(c *gin.Context) {
	foundContext, exists := c.Get("TUMLiveContext")
	if !exists {
		logger.Error("context should exist but doesn't")
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
	tumLiveContext := foundContext.(tools.TUMLiveContext)
	indexData := NewIndexData()
	indexData.TUMLiveContext = tumLiveContext
	if err := templateExecutor.ExecuteTemplate(c.Writer, "lecture-units.gohtml", LectureUnitsPageData{
		IndexData: indexData,
		Lecture:   *tumLiveContext.Stream,
		Units:     tumLiveContext.Stream.Units,
	}); err != nil {
		logger.Error("can not execute template", "err", err)
	}
}

func (r mainRoutes) LectureLiveManagementPage(c *gin.Context) {
	foundContext, exists := c.Get("TUMLiveContext")
	if !exists {
		logger.Error("context should exist but doesn't")
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
	tumLiveContext := foundContext.(tools.TUMLiveContext)
	indexData := NewIndexData()
	indexData.TUMLiveContext = tumLiveContext
	stream := tumLiveContext.Stream

	if stream == nil {
		tools.RenderErrorPage(c, http.StatusNotFound, "Lecture not found")
		return
	}

	if !stream.LiveNow {
		tools.RenderErrorPage(c, http.StatusNotFound, "Lecture is not live")
		return
	}

	if c.Query("restart") == "1" {
		c.Redirect(http.StatusFound, strings.Split(c.Request.RequestURI, "?")[0])
		return
	}

	if err := templateExecutor.ExecuteTemplate(c.Writer, "lecture-live-management.gohtml", LiveLectureManagementData{
		IndexData: indexData,
		Lecture:   *tumLiveContext.Stream,
		ChatData: ChatData{
			IsAdminOfCourse: tumLiveContext.UserIsAdmin(),
			IndexData:       indexData,
		},
	}); err != nil {
		logger.Error("can not execute template", "err", err)
	}
}

type LectureUnitsPageData struct {
	IndexData IndexData
	Lecture   model.Stream
	Units     []model.StreamUnit
}

type LiveLectureManagementData struct {
	IndexData IndexData
	Lecture   model.Stream
	ChatData  ChatData
}
