package web

import (
	"errors"
	"fmt"
	"html/template"
	"net/http"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/TUM-Dev/gocast/model"
	"github.com/TUM-Dev/gocast/tools"
)

type editCourseByTokenPageData struct {
	Token     string
	Course    model.Course
	IndexData IndexData
}

func (r mainRoutes) editCourseByTokenPage(c *gin.Context) {
	err := c.Request.ParseForm()
	if err != nil {
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}
	token := c.Request.Form.Get("token")
	if token == "" {
		_ = c.AbortWithError(http.StatusForbidden, tools.RequestError{
			Status:        http.StatusForbidden,
			CustomMessage: "please provide a token",
			Err:           fmt.Errorf("token is empty"),
		})
		return
	}

	indexData := NewIndexDataWithContext(c)
	course, err := r.CoursesDao.GetCourseByToken(token)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusNotFound, gin.H{"error": "Course not found"})
		return
	}
	d := editCourseByTokenPageData{
		Token:     c.Request.Form.Get("token"),
		Course:    course,
		IndexData: indexData,
	}

	err = templateExecutor.ExecuteTemplate(c.Writer, "edit-course-by-token.gohtml", d)
	if err != nil {
		logger.Error("Error executing template edit-course-by-token.gohtml", "err", err)
	}
}

type OptOutPageData struct {
	IndexData IndexData
	Course    *model.Course
}

func (r mainRoutes) optOutPage(c *gin.Context) {
	err := c.Request.ParseForm()
	if err != nil {
		_ = c.AbortWithError(http.StatusBadRequest, tools.RequestError{
			Status:        http.StatusBadRequest,
			CustomMessage: "Could not read your request.",
			Err:           err,
		})
	}
	token := c.Request.Form.Get("token")
	if token == "" {
		_ = c.AbortWithError(http.StatusForbidden, tools.RequestError{
			Status:        http.StatusForbidden,
			CustomMessage: "please provide a token",
			Err:           fmt.Errorf("token is empty"),
		})
		return
	}

	var d OptOutPageData
	d.IndexData = NewIndexData()
	course, err := r.CoursesDao.GetCourseByToken(token)
	if err != nil {
		d.Course = nil
	} else {
		d.Course = &course
	}
	err = templateExecutor.ExecuteTemplate(c.Writer, "opt-out.gohtml", d)
	if err != nil {
		logger.Error("can't render template", "err", err)
	}
}

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
