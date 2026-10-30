package web

import (
	"errors"
	"fmt"
	"net/http"

	"gorm.io/gorm"
)

// shortLinkOrInfoPage serves an info page created after the three built-in ones,
// which cannot be given a route of their own at boot: it checks the info_pages table
// before falling back to a course short link, so a page an administrator creates is
// reachable immediately, without a deploy.
func (r mainRoutes) shortLinkOrInfoPage(w http.ResponseWriter, req *http.Request) {
	slug := req.PathValue("shortLink")
	if _, err := r.InfoPageDao.GetBySlug(slug); err == nil {
		serveSPAShell(w, req)
		return
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		logger.Error("can't look up info page for short link", "err", err, "slug", slug)
	}

	r.HighlightPage(w, req)
}

// HighlightPage sends a course short link to the watch page of the course's current
// or next lecture, or to the course page when there is none.
//
// It deliberately renders nothing about the lecture itself: the watch page enforces
// the course's visibility and the viewer's enrolment, while this handler never did, so
// rendering here leaked the lecture name and description of non-public courses.
//
// The redirect is a 302, not a 301: the target is "whatever is live or next", which
// changes over time, so a client or proxy must not cache it.
func (r mainRoutes) HighlightPage(w http.ResponseWriter, req *http.Request) {
	course, err := r.CoursesDao.GetCourseByShortLink(req.PathValue("shortLink"))
	if err != nil {
		// Neither an info page nor a short link: the same 404 as any other path.
		serveNotFound(w, req)
		return
	}
	s, err := r.CoursesDao.GetCurrentOrNextLectureForCourse(req.Context(), course.ID)
	switch {
	case err == nil:
		http.Redirect(w, req, fmt.Sprintf("/w/%s/%d", course.Slug, s.ID), http.StatusFound)
		return
	case !errors.Is(err, gorm.ErrRecordNotFound):
		logger.Error("Error getting current or next lecture for course", "err", err)
	}
	http.Redirect(w, req, fmt.Sprintf("/course/%d/%s/%s", course.Year, course.TeachingTerm, course.Slug), http.StatusFound)
}
