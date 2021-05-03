package web

import (
	"TUM-Live/middleware"
	"TUM-Live/model"
	"github.com/gin-gonic/gin"
	"html/template"
	"strconv"
	"strings"
)

func WatchPage(c *gin.Context) {
	var tumLiveContext middleware.TUMLiveContext
	if found, exists := c.Get("TUMLiveContext"); exists {
		tumLiveContext = found.(middleware.TUMLiveContext)
	}
	var data WatchPageData
	data.IndexData = NewIndexData()
	data.IndexData.IsUser = tumLiveContext.User != nil
	data.IndexData.IsAdmin = tumLiveContext.IsAdmin
	data.Stream = *tumLiveContext.Stream
	if c.Param("version") != "" {
		data.Version = c.Param("version")
		if strings.HasPrefix(data.Version, "unit-") {
			if unitID, err := strconv.Atoi(strings.ReplaceAll(data.Version, "unit-", "")); err == nil && unitID < len(data.Stream.Units) {
				data.Unit = &data.Stream.Units[unitID]
			}
		}
	}
	data.Course = *tumLiveContext.Course
	if strings.HasPrefix(data.Version, "unit-") {
		data.Description = data.Unit.GetDescriptionHTML()
	} else {
		data.Description = template.HTML(data.Stream.GetDescriptionHTML())
	}
	if c.Query("video_only") == "1" {
		_ = templ.ExecuteTemplate(c.Writer, "video_only.gohtml", data)
	} else {
		_ = templ.ExecuteTemplate(c.Writer, "watch.gohtml", data)
	}
}

type WatchPageData struct {
	IndexData   IndexData
	Stream      model.Stream
	Unit        *model.StreamUnit
	Description template.HTML
	Course      model.Course
	Version     string
}
