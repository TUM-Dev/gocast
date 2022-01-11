package tools

import (
	"TUM-Live/dao"
	"TUM-Live/model"
	"errors"
	"github.com/getsentry/sentry-go"
	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

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
			c.AbortWithStatus(http.StatusNotFound)
		} else {
			course = foundCourse
		}
	} else if c.Param("year") != "" && c.Param("teachingTerm") != "" && c.Param("slug") != "" {
		foundCourse, err := dao.GetCourseBySlugYearAndTerm(c, c.Param("slug"), c.Param("teachingTerm"), c.Param("year"))
		if err != nil {
			c.AbortWithStatus(http.StatusNotFound)
		} else {
			course = foundCourse
		}
	} else {
		c.AbortWithStatus(http.StatusNotFound)
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
		c.AbortWithStatus(http.StatusForbidden)
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
			c.AbortWithStatus(http.StatusNotFound)
		} else {
			stream = foundStream
		}
	} else {
		c.AbortWithStatus(http.StatusNotFound)
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
			c.AbortWithStatus(http.StatusForbidden)
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
	if tumLiveContext.User.Role != model.AdminType && tumLiveContext.User.Model.ID != tumLiveContext.Course.UserID {
		c.AbortWithStatus(http.StatusForbidden)
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
		c.AbortWithStatus(http.StatusForbidden)
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
		c.AbortWithStatus(http.StatusForbidden)
	}
}

type TUMLiveContext struct {
	User   *model.User
	Course *model.Course
	Stream *model.Stream
}
