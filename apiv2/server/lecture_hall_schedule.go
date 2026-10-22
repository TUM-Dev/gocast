package apiv2

import (
	"net/http"
	"strconv"
	"strings"
	"text/template"

	"github.com/gin-gonic/gin"

	"github.com/TUM-Dev/gocast/model"
	"github.com/TUM-Dev/gocast/tools"
)

// handleLectureHallSchedule serves the lecture hall iCal feed that calendar clients are
// subscribed to by URL. ?lecturehalls=1,2 narrows it to those halls; without the
// parameter it lists them all. Anyone may fetch it, but a signed-in user without
// PermViewAllCourses gets only the streams of their own courses, as in v1.
//
// text/template, not html/template: the output is a calendar, and HTML escaping would
// corrupt names containing an ampersand or a quote.
func (a *API) handleLectureHallSchedule(c *gin.Context) {
	templ, err := template.ParseFS(courseImportTemplateFS, "template/ical.gotemplate")
	if err != nil {
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
	foundContext, exists := c.Get("TUMLiveContext")
	if !exists {
		a.log.Error("context should exist but doesn't")
		abortLegacyError(c, http.StatusInternalServerError, "context should exist but doesn't", nil)
		return
	}
	if err := c.Request.ParseForm(); err != nil {
		abortLegacyError(c, http.StatusBadRequest, "Bad Request", err)
		return
	}
	lectureHallsStr := strings.Split(c.Request.Form.Get("lecturehalls"), ",")
	lectureHalls := make([]uint, 0, len(lectureHallsStr))
	for _, l := range lectureHallsStr {
		if l == "" {
			continue
		}
		id, err := strconv.Atoi(l)
		if err != nil {
			abortLegacyError(c, http.StatusBadRequest, "Lecture Hall ID must be a number.", err)
			return
		}
		lectureHalls = append(lectureHalls, uint(id))
	}
	all := !c.Request.Form.Has("lecturehalls") // if none requested, deliver all

	tumLiveContext := foundContext.(tools.TUMLiveContext)
	// pass 0 to db query to get all lectures if user is not logged in or admin
	queryUid := uint(0)
	if tumLiveContext.User != nil && !tumLiveContext.User.Can(model.PermViewAllCourses) {
		queryUid = tumLiveContext.User.ID
	}
	icalData, err := a.dao.LectureHallsDao.GetStreamsForLectureHallIcal(queryUid, lectureHalls, all)
	if err != nil {
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
	c.Header("content-type", "text/calendar")
	if err := templ.ExecuteTemplate(c.Writer, "ical.gotemplate", icalData); err != nil {
		a.log.Error("Error executing template ical.gotemplate", "err", err)
	}
}
