package api

import (
	"errors"
	"github.com/getsentry/sentry-go"
	"github.com/gin-gonic/gin"
	"github.com/joschahenningsen/TUM-Live/dao"
	"github.com/joschahenningsen/TUM-Live/model"
	"github.com/joschahenningsen/TUM-Live/tools"
	"html/template"
	"net/http"
	"strconv"
)

func configGinDownloadICSRouter(router *gin.Engine) {
	router.GET("/api/download_ics/:year/:term/:slug", downloadICS)
}

func downloadICS(c *gin.Context) {
	templates, err := template.ParseFS(staticFS, "template/*.gotemplate")
	if err != nil {
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}

	foundContext, exists := c.Get("TUMLiveContext")
	if !exists {
		sentry.CaptureException(errors.New("context should exist but doesn't"))
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
	tumLiveContext := foundContext.(tools.TUMLiveContext)
	if tumLiveContext.User == nil {
		c.AbortWithStatus(http.StatusForbidden)
		return
	}

	slug, term := c.Param("slug"), c.Param("term")
	year, err := strconv.Atoi(c.Param("year"))
	if err != nil {
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}

	course, err := dao.GetCourseBySlugYearAndTerm(c, slug, term, year)
	if err != nil {
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}

	if len(course.Streams) == 0 {
		_, err := c.Writer.WriteString("No lectures found!")
		if err != nil {
			c.AbortWithStatus(http.StatusInternalServerError)
			return
		}
		c.Writer.Flush()
	}

	var acc []CalendarEntry
	for _, s := range course.Streams {
		acc = append(acc, streamToCalendarEntry(s, course))
	}

	c.Header("content-type", "text/calendar")
	c.Header("Content-Transfer-Encoding", "binary")
	c.Header("Content-Disposition", "attachment; filename="+course.Slug+course.TeachingTerm+strconv.Itoa(course.Year)+".ics")
	err = templates.ExecuteTemplate(c.Writer, "ics.gotemplate", acc)
	if err != nil {
		c.AbortWithStatus(http.StatusInternalServerError)
	}
}

type CalendarEntry struct {
	CreatedAt   string
	Start       string
	End         string
	ID          string
	Url         string
	Location    string
	Summary     string
	Description string
}

func streamToCalendarEntry(s model.Stream, c model.Course) CalendarEntry {
	layout := "20060102T150405"
	var location = ""
	if len(s.RoomName) > 0 {
		location += s.RoomName + " "
	}
	if len(s.RoomCode) > 0 {
		location += s.RoomCode
	}
	return CalendarEntry{
		CreatedAt:   s.CreatedAt.Format(layout),
		Start:       s.Start.Format(layout),
		End:         s.End.Format(layout),
		ID:          s.StreamKey,
		Url:         "https://live.rbg.tum.de/course/" + strconv.Itoa(c.Year) + "/" + c.TeachingTerm + "/" + c.Slug,
		Location:    location,
		Summary:     c.Name,
		Description: s.Name,
	}
}
