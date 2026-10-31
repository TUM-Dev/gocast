package apiv2

import (
	"bytes"
	"errors"
	"html/template"
	"net/http"
	"strconv"
	"strings"

	"gorm.io/gorm"

	"github.com/TUM-Dev/gocast/model"
	"github.com/TUM-Dev/gocast/tools"
)

// courseCalendarPath is GET /api/v2/courses/{year}/{term}/{slug}/events.ics, a course's
// lectures as an iCalendar feed.
const courseCalendarPath = "GET /api/v2/courses/{year}/{term}/{slug}/events.ics"

// calendarEntry is one VEVENT of ics.gotemplate.
type calendarEntry struct {
	CreatedAt   string
	Start       string
	End         string
	ID          string
	Url         string
	Location    string
	Summary     string
	Description string
}

func streamToCalendarEntry(s model.Stream, c model.Course) calendarEntry {
	layout := "20060102T150405"
	location := ""
	if len(s.RoomName) > 0 {
		location += s.RoomName + " "
	}
	if len(s.RoomCode) > 0 {
		location += s.RoomCode
	}
	return calendarEntry{
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

// LegacyCalendarPath is where the calendar feed was first served, by v1.
const LegacyCalendarPath = "GET /api/download_ics/{year}/{term}/{slug}/events.ics"

// RegisterLegacyCalendar serves the feed at its original address, LegacyCalendarPath.
// People subscribed their calendars to it and apps poll it for years, so it must keep
// working. It answers in place rather than redirecting to the v2 address, because some
// calendar clients do not follow a redirect and would silently end the subscription.
func (a *API) RegisterLegacyCalendar(r Registrar) {
	r.Handle(LegacyCalendarPath, http.HandlerFunc(a.handleCourseCalendar))
}

// handleCourseCalendar serves a course's lectures as an iCalendar feed.
//
// There is deliberately no access check. A calendar client cannot sign in, and v1 never
// checked either: the feed of any course is public to whoever knows its slug. That is
// kept as it was, because adding a check would silently empty every existing
// subscription; tightening it is a decision about those subscribers, not a port.
func (a *API) handleCourseCalendar(w http.ResponseWriter, r *http.Request) {
	term, slug := r.PathValue("term"), r.PathValue("slug")
	year, err := strconv.Atoi(r.PathValue("year"))
	if err != nil {
		tools.WriteText(w, http.StatusBadRequest, "invalid year")
		return
	}

	course, err := a.dao.CoursesDao.GetCourseBySlugYearAndTerm(r.Context(), slug, term, year)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			tools.WriteText(w, http.StatusNotFound, "no such course")
			return
		}
		a.log.Error("could not get the course for its calendar", "err", err)
		tools.WriteText(w, http.StatusInternalServerError, "can not get course")
		return
	}

	entries := make([]calendarEntry, 0, len(course.Streams))
	for _, s := range course.Streams {
		entries = append(entries, streamToCalendarEntry(s, course))
	}

	// Rendered before any header is written, so a failure can still be a 500 rather
	// than a truncated calendar.
	tmpl, err := template.ParseFS(courseImportTemplateFS, "template/ics.gotemplate")
	var body bytes.Buffer
	if err == nil {
		err = tmpl.ExecuteTemplate(&body, "ics.gotemplate", entries)
	}
	if err != nil {
		a.log.Error("could not generate the calendar", "err", err)
		tools.WriteText(w, http.StatusInternalServerError, "can not generate .ics")
		return
	}

	w.Header().Set("Content-Transfer-Encoding", "binary")
	w.Header().Set("Content-Disposition", "attachment; filename="+course.Slug+course.TeachingTerm+strconv.Itoa(course.Year)+".ics")
	tools.WriteData(w, http.StatusOK, "text/calendar", body.Bytes())
}
