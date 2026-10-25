package apiv2

import (
	"bytes"
	"errors"
	"html/template"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/TUM-Dev/gocast/model"
	"github.com/TUM-Dev/gocast/tools"
)

// courseCalendarRoute is a request for GET /api/v2/courses/{year}/{term}/{slug}/events.ics,
// a course's lectures as an iCalendar feed.
type courseCalendarRoute struct {
	year       string
	term, slug string
}

// courseCalendarRouteOf matches a request against the calendar endpoint.
func courseCalendarRouteOf(r *http.Request) (courseCalendarRoute, bool) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		return courseCalendarRoute{}, false
	}
	rest, ok := strings.CutPrefix(r.URL.Path, "/api/v2/courses/")
	if !ok {
		return courseCalendarRoute{}, false
	}
	parts := strings.Split(rest, "/")
	if len(parts) != 4 || parts[3] != "events.ics" || parts[1] == "" || parts[2] == "" {
		return courseCalendarRoute{}, false
	}
	return courseCalendarRoute{year: parts[0], term: parts[1], slug: parts[2]}, true
}

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
const LegacyCalendarPath = "/api/download_ics/:year/:term/:slug/events.ics"

// RegisterLegacyCalendar serves the feed at its original address, LegacyCalendarPath.
// People subscribed their calendars to it and apps poll it for years, so it must keep
// working. It answers in place rather than redirecting to the v2 address, because some
// calendar clients do not follow a redirect and would silently end the subscription.
func (a *API) RegisterLegacyCalendar(r gin.IRoutes) {
	r.GET(LegacyCalendarPath, func(c *gin.Context) {
		a.handleCourseCalendar(c, c.Param("year"), c.Param("term"), c.Param("slug"))
	})
}

// handleCourseCalendar serves a course's lectures as an iCalendar feed.
//
// There is deliberately no access check. A calendar client cannot sign in, and v1 never
// checked either: the feed of any course is public to whoever knows its slug. That is
// kept as it was, because adding a check would silently empty every existing
// subscription; tightening it is a decision about those subscribers, not a port.
func (a *API) handleCourseCalendar(c *gin.Context, yearStr, term, slug string) {
	year, err := strconv.Atoi(yearStr)
	if err != nil {
		c.String(http.StatusBadRequest, "invalid year")
		return
	}

	course, err := a.dao.CoursesDao.GetCourseBySlugYearAndTerm(c.Request.Context(), slug, term, year)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.String(http.StatusNotFound, "no such course")
			return
		}
		a.log.Error("could not get the course for its calendar", "err", err)
		c.String(http.StatusInternalServerError, "can not get course")
		return
	}

	entries := make([]calendarEntry, 0, len(course.Streams))
	now := time.Now()
	for _, s := range course.Streams {
		if course.IsLectureExpired(s, now) {
			continue
		}
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
		c.String(http.StatusInternalServerError, "can not generate .ics")
		return
	}

	c.Header("Content-Transfer-Encoding", "binary")
	c.Header("Content-Disposition", "attachment; filename="+course.Slug+course.TeachingTerm+strconv.Itoa(course.Year)+".ics")
	c.Data(http.StatusOK, "text/calendar", body.Bytes())
}
