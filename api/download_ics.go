package api

import (
	"github.com/gin-gonic/gin"
	"github.com/TUM-Dev/gocast/dao"
	"github.com/TUM-Dev/gocast/model"
	"github.com/TUM-Dev/gocast/tools"
	log "github.com/sirupsen/logrus"
	"html/template"
	"net/http"
	"strconv"
	"strings"
)

func configGinDownloadICSRouter(router *gin.Engine, daoWrapper dao.DaoWrapper) {
	templates, err := template.ParseFS(staticFS, "template/*.gotemplate")
	if err != nil {
		log.WithError(err).Fatal("could not parse templates")
		return
	}
	routes := downloadICSRoutes{daoWrapper, templates}
	router.GET("/api/download_ics/:year/:term/:slug/events.ics", routes.downloadICS)
}

type downloadICSRoutes struct {
	dao.DaoWrapper
	templates *template.Template
}

func (r downloadICSRoutes) downloadICS(c *gin.Context) {
	slug, term := c.Param("slug"), c.Param("term")
	year, err := strconv.Atoi(c.Param("year"))
	if err != nil {
		_ = c.Error(tools.RequestError{
			Status:        http.StatusBadRequest,
			CustomMessage: "invalid year",
			Err:           err,
		})
		return
	}

	course, err := r.CoursesDao.GetCourseBySlugYearAndTerm(c, slug, term, year)
	if err != nil {
		_ = c.Error(tools.RequestError{
			Status:        http.StatusBadRequest,
			CustomMessage: "can not get course",
			Err:           err,
		})
		return
	}

	var acc []CalendarEntry
	for _, s := range course.Streams {
		acc = append(acc, streamToCalendarEntry(s, course))
	}

	c.Header("content-type", "text/calendar")
	c.Header("Content-Transfer-Encoding", "binary")
	c.Header("Content-Disposition", "attachment; filename="+course.Slug+course.TeachingTerm+strconv.Itoa(course.Year)+".ics")
	err = r.templates.ExecuteTemplate(c.Writer, "ics.gotemplate", acc)
	if err != nil {
		_ = c.Error(tools.RequestError{
			Status:        http.StatusInternalServerError,
			CustomMessage: "can not get generate .ics",
			Err:           err,
		})
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
		ID:          strconv.Itoa(int(s.ID)),
		Url:         strings.Join([]string{tools.Cfg.WebUrl, strconv.Itoa(c.Year), c.TeachingTerm, c.Slug}, "/"),
		Location:    location,
		Summary:     c.Name,
		Description: s.Name,
	}
}
