package web

import (
	"errors"
	"fmt"
	"html/template"
	"net/http"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/TUM-Dev/gocast/tools"
)

// shortLinkOrInfoPage serves an info page created after the three built-in ones,
// which cannot be given a route of their own at boot: it checks the info_pages table
// before falling back to a course short link, so a page an administrator creates is
// reachable immediately, without a deploy.
func (r mainRoutes) shortLinkOrInfoPage(c *gin.Context) {
	slug := c.Param("shortLink")
	if _, err := r.InfoPageDao.GetBySlug(slug); err == nil {
		if !spaAvailable() {
			// No build to serve the page from; a course short link is at least
			// something rather than a broken response.
			r.HighlightPage(c)
			return
		}
		serveSPAShell(c)
		return
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		logger.Error("can't look up info page for short link", "err", err, "slug", slug)
	}

	r.HighlightPage(c)
}

func (r mainRoutes) HighlightPage(c *gin.Context) {
	course, err := r.CoursesDao.GetCourseByShortLink(c.Param("shortLink"))
	if err != nil {
		tools.RenderErrorPage(c, http.StatusNotFound, tools.PageNotFoundErrMsg)
		return
	}
	indexData := NewIndexData()
	var tumLiveContext tools.TUMLiveContext
	tumLiveContextQueried, found := c.Get("TUMLiveContext")
	if found {
		tumLiveContext = tumLiveContextQueried.(tools.TUMLiveContext)
		indexData.TUMLiveContext = tumLiveContext
	} else {
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
	indexData.TUMLiveContext.Course = &course
	s, err := r.CoursesDao.GetCurrentOrNextLectureForCourse(c, course.ID)
	switch {
	case err == nil:
		indexData.TUMLiveContext.Stream = &s
	case errors.Is(err, gorm.ErrRecordNotFound):
		c.Redirect(http.StatusFound, fmt.Sprintf("/course/%d/%s/%s", course.Year, course.TeachingTerm, course.Slug))
		return
	default:
		logger.Error("Error getting current or next lecture for course", "err", err)
	}
	description := ""
	if indexData.TUMLiveContext.Stream != nil {
		description = indexData.TUMLiveContext.Stream.GetDescriptionHTML()
	}
	d2 := WatchPageData{
		IndexData:       indexData,
		Description:     template.HTML(description),
		Version:         "",
		IsHighlightPage: true,
	}
	if err = templateExecutor.ExecuteTemplate(c.Writer, "watch.gohtml", d2); err != nil {
		logger.Error("Error executing template watch.gohtml", "err", err)
		return
	}
}
