package apiv2

import (
	"net/http"
	"strconv"
	"strings"
	"text/template"

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
func (a *API) handleLectureHallSchedule(w http.ResponseWriter, r *http.Request) {
	templ, err := template.ParseFS(courseImportTemplateFS, "template/ical.gotemplate")
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	tumLiveContext, exists := tools.GetTUMLiveContext(r)
	if !exists {
		a.log.Error("context should exist but doesn't")
		writeLegacyError(w, http.StatusInternalServerError, "context should exist but doesn't", nil)
		return
	}
	if err := r.ParseForm(); err != nil {
		writeLegacyError(w, http.StatusBadRequest, "Bad Request", err)
		return
	}
	lectureHallsStr := strings.Split(r.Form.Get("lecturehalls"), ",")
	lectureHalls := make([]uint, 0, len(lectureHallsStr))
	for _, l := range lectureHallsStr {
		if l == "" {
			continue
		}
		id, err := strconv.Atoi(l)
		if err != nil {
			writeLegacyError(w, http.StatusBadRequest, "Lecture Hall ID must be a number.", err)
			return
		}
		lectureHalls = append(lectureHalls, uint(id))
	}
	all := !r.Form.Has("lecturehalls") // if none requested, deliver all

	// pass 0 to db query to get all lectures if user is not logged in or admin
	queryUid := uint(0)
	if tumLiveContext.User != nil && !tumLiveContext.User.Can(model.PermViewAllCourses) {
		queryUid = tumLiveContext.User.ID
	}
	icalData, err := a.dao.LectureHallsDao.GetStreamsForLectureHallIcal(queryUid, lectureHalls, all)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/calendar")
	if err := templ.ExecuteTemplate(w, "ical.gotemplate", icalData); err != nil {
		a.log.Error("Error executing template ical.gotemplate", "err", err)
	}
}
