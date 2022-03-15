package tools

import (
	"TUM-Live/dao"
	"TUM-Live/model"
	"errors"
	"github.com/getsentry/sentry-go"
	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
	"html/template"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

var templ *template.Template

// SetTemplates sets the templates for the middlewares to execute error pages
func SetTemplates(t *template.Template) {
	templ = t
}

func InitContext(c *gin.Context) {
	// no context initialisation required for static assets.
	if strings.HasPrefix(c.Request.RequestURI, "/static") ||
		strings.HasPrefix(c.Request.RequestURI, "/public") ||
		strings.HasPrefix(c.Request.RequestURI, "/favicon") {
		return
	}

	session := sessions.Default(c)
	userID := session.Get("UserID")
	if userID != nil {
		user, err := dao.GetUserByID(c, userID.(uint))
		if err != nil {
			session.Clear()
			_ = session.Save()
			c.Set("TUMLiveContext", TUMLiveContext{})
			return
		} else {
			c.Set("TUMLiveContext", TUMLiveContext{User: &user})
			return
		}
	}
	c.Set("TUMLiveContext", TUMLiveContext{})
}

func RenderErrorPage(c *gin.Context, status int, message string) {
	err := templ.ExecuteTemplate(c.Writer, "error.gohtml", ErrorPageData{
		Status:  status,
		Message: message,
	})
	if err != nil {
		log.Error(err)
	}
	c.Abort()
}

type ErrorPageData struct {
	Status  int
	Message string
}

const (
	PageNotFoundErrMsg     = "This page does not exist."
	CourseNotFoundErrMsg   = "We couldn't find the course you were looking for."
	StreamNotFoundErrMsg   = "We couldn't find the stream you were looking for."
	ForbiddenGenericErrMsg = "You don't have permissions to access this resource. " +
		"Please reach out if this seems wrong :)"
	ForbiddenStreamAccess = "You don't have permissions to access this stream. " +
		"Please make sure to use the correct login."
	ForbiddenCourseAccess = "You don't have permissions to access this course. " +
		"Please make sure to use the correct login."
)

func InitCourse(c *gin.Context) {
	foundContext, exists := c.Get("TUMLiveContext")
	if !exists {
		sentry.CaptureException(errors.New("context should exist but doesn't"))
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
	tumLiveContext := foundContext.(TUMLiveContext)
	// Get course based on context:
	var course model.Course
	if c.Param("courseID") != "" {
		cIDInt, err := strconv.ParseInt(c.Param("courseID"), 10, 32)
		if err != nil {
			c.AbortWithStatus(http.StatusBadRequest)
			return
		}
		foundCourse, err := dao.GetCourseById(c, uint(cIDInt))
		if err != nil {
			c.Status(http.StatusNotFound)
			RenderErrorPage(c, http.StatusNotFound, CourseNotFoundErrMsg)
		} else {
			course = foundCourse
		}
	} else if c.Param("year") != "" && c.Param("teachingTerm") != "" && c.Param("slug") != "" {
		y := c.Param("year")
		yInt, err := strconv.Atoi(y)
		if err != nil {
			c.AbortWithStatus(http.StatusBadRequest)
			return
		}
		foundCourse, err := dao.GetCourseBySlugYearAndTerm(c, c.Param("slug"), c.Param("teachingTerm"), yInt)
		if err != nil {
			c.Status(http.StatusNotFound)
			RenderErrorPage(c, http.StatusNotFound, CourseNotFoundErrMsg)
		} else {
			course = foundCourse
		}
	} else {
		c.Status(http.StatusNotFound)
		RenderErrorPage(c, http.StatusNotFound, CourseNotFoundErrMsg)
	}
	if c.IsAborted() {
		return
	}
	// check if course is accessible by user:
	if course.Visibility == "public" || course.Visibility == "hidden" || (tumLiveContext.User != nil && tumLiveContext.User.IsEligibleToWatchCourse(course)) {
		tumLiveContext.Course = &course
		c.Set("TUMLiveContext", tumLiveContext)
	} else if tumLiveContext.User == nil {
		c.Redirect(http.StatusFound, "/login?return="+url.QueryEscape(c.Request.RequestURI))
		c.Abort()
		return
	} else {
		c.Status(http.StatusForbidden)
		RenderErrorPage(c, http.StatusForbidden, ForbiddenCourseAccess)
	}
}

func InitStream(c *gin.Context) {
	foundContext, exists := c.Get("TUMLiveContext")
	if !exists {
		sentry.CaptureException(errors.New("context should exist but doesn't"))
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
	tumLiveContext := foundContext.(TUMLiveContext)
	// Get stream based on context:
	var stream model.Stream
	if c.Param("streamID") != "" {
		foundStream, err := dao.GetStreamByID(c, c.Param("streamID"))
		if err != nil {
			c.Status(http.StatusNotFound)
			RenderErrorPage(c, http.StatusNotFound, StreamNotFoundErrMsg)
		} else {
			stream = foundStream
		}
	} else {
		c.Status(http.StatusNotFound)
		RenderErrorPage(c, http.StatusNotFound, StreamNotFoundErrMsg)
	}
	if c.IsAborted() {
		return
	}
	course, err := dao.GetCourseById(c, stream.CourseID)
	if err != nil {
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}
	if course.Visibility != "public" && course.Visibility != "hidden" {
		if tumLiveContext.User == nil {
			c.Redirect(http.StatusFound, "/login?return="+url.QueryEscape(c.Request.RequestURI))
			c.Abort()
			return
		} else if tumLiveContext.User == nil || !tumLiveContext.User.IsEligibleToWatchCourse(course) {
			c.Status(http.StatusForbidden)
			RenderErrorPage(c, http.StatusForbidden, ForbiddenStreamAccess)
			return
		}
	}
	tumLiveContext.Course = &course
	tumLiveContext.Stream = &stream
	c.Set("TUMLiveContext", tumLiveContext)
}

func AdminOfCourse(c *gin.Context) {
	foundContext, exists := c.Get("TUMLiveContext")
	if !exists {
		sentry.CaptureException(errors.New("context should exist but doesn't"))
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
	tumLiveContext := foundContext.(TUMLiveContext)
	if tumLiveContext.User == nil || (tumLiveContext.User.Role != model.AdminType && tumLiveContext.User.Model.ID != tumLiveContext.Course.UserID) {
		c.Status(http.StatusForbidden)
		RenderErrorPage(c, http.StatusForbidden, ForbiddenGenericErrMsg)
	}
}

func AtLeastLecturer(c *gin.Context) {
	foundContext, exists := c.Get("TUMLiveContext")
	if !exists {
		sentry.CaptureException(errors.New("context should exist but doesn't"))
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
	tumLiveContext := foundContext.(TUMLiveContext)
	if tumLiveContext.User == nil || (tumLiveContext.User.Role != model.AdminType && tumLiveContext.User.Role != model.LecturerType) {
		c.Status(http.StatusForbidden)
		RenderErrorPage(c, http.StatusForbidden, ForbiddenGenericErrMsg)
	}
}

func Admin(c *gin.Context) {
	foundContext, exists := c.Get("TUMLiveContext")
	if !exists {
		sentry.CaptureException(errors.New("context should exist but doesn't"))
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
	tumLiveContext := foundContext.(TUMLiveContext)
	if tumLiveContext.User == nil || tumLiveContext.User.Role != model.AdminType {
		c.Status(http.StatusForbidden)
		RenderErrorPage(c, http.StatusForbidden, ForbiddenGenericErrMsg)
	}
}

func AdminToken(c *gin.Context) {
	queryParams := c.Request.URL.Query()
	token := queryParams.Get("token")
	t, err := dao.GetToken(token)
	if err != nil {
		c.Status(http.StatusForbidden)
		RenderErrorPage(c, http.StatusForbidden, ForbiddenGenericErrMsg)
		return
	}
	if t.Scope != model.TokenScopeAdmin {
		c.Status(http.StatusForbidden)
		RenderErrorPage(c, http.StatusForbidden, ForbiddenGenericErrMsg)
		return
	}
	err = dao.TokenUsed(t)
	if err != nil {
		log.WithError(err).Warn("error marking token as used")
		return
	}
}

type TUMLiveContext struct {
	User   *model.User
	Course *model.Course
	Stream *model.Stream
}

func (c *TUMLiveContext) UserIsAdmin() bool {
	return c.User != nil && (c.User.Role == model.AdminType || c.User.ID == c.Course.UserID)
}
