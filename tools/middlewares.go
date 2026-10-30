package tools

import (
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"

	"github.com/TUM-Dev/gocast/dao"
	"github.com/TUM-Dev/gocast/model"
)

// errorPageRenderer answers a browser request with the error page. The web package
// installs it, because only it holds the SPA shell; see web.serveErrorPage.
var errorPageRenderer func(c *gin.Context, status int, message string)

// SetErrorPageRenderer sets how RenderErrorPage answers a request for a page.
func SetErrorPageRenderer(f func(c *gin.Context, status int, message string)) {
	errorPageRenderer = f
}

// JWTClaims are the claims contained in a session
type JWTClaims struct {
	*jwt.RegisteredClaims
	UserID        uint
	SamlSubjectID *string // identifier of the SAML session (if any)
}

func InitContext(daoWrapper dao.DaoWrapper) gin.HandlerFunc {
	return func(c *gin.Context) {
		// no context initialisation required for static assets.
		if strings.HasPrefix(c.Request.RequestURI, "/public") ||
			strings.HasPrefix(c.Request.RequestURI, "/favicon") {
			return
		}

		// get the session
		cookie, err := c.Cookie("jwt")
		if err != nil {
			c.Set("TUMLiveContext", TUMLiveContext{})
			return
		}

		token, err := jwt.ParseWithClaims(cookie, &JWTClaims{}, func(token *jwt.Token) (interface{}, error) {
			key := Cfg.GetJWTKey().Public()
			return key, nil
		})
		if err != nil {
			logger.Info("JWT parsing error: ", "err", err)
			c.Set("TUMLiveContext", TUMLiveContext{})
			c.SetCookie("jwt", "", -1, "/", "", false, true)
			return
		}
		if !token.Valid {
			logger.Info("JWT token is not valid")
			c.Set("TUMLiveContext", TUMLiveContext{})
			c.SetCookie("jwt", "", -1, "/", "", false, true)
			return
		}

		user, err := daoWrapper.UsersDao.GetUserByID(c, token.Claims.(*JWTClaims).UserID)
		if err != nil {
			c.Set("TUMLiveContext", TUMLiveContext{})
			return
		}
		c.Set("TUMLiveContext", TUMLiveContext{User: &user, SamlSubjectID: token.Claims.(*JWTClaims).SamlSubjectID})
	}
}

// LoggedIn is a middleware that checks if the user is logged in and redirects to the login page if not
func LoggedIn(c *gin.Context) {
	if c.MustGet("TUMLiveContext").(TUMLiveContext).User == nil {
		// Pass the page along so the login flow can return to it, as the course and
		// admin middlewares below do.
		c.Redirect(http.StatusFound, "/login?return="+url.QueryEscape(c.Request.RequestURI))
		c.Abort()
	}
}

// RenderErrorPage answers with the given status and message, and aborts the context.
// A page request gets the SPA shell, which renders the error; under /api/ the caller
// is a script, which gets the message as plain text.
func RenderErrorPage(c *gin.Context, status int, message string) {
	if errorPageRenderer == nil || strings.HasPrefix(c.Request.URL.Path, "/api/") {
		c.String(status, message)
	} else {
		errorPageRenderer(c, status, message)
	}
	c.Abort()
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

func InitCourse(wrapper dao.DaoWrapper) gin.HandlerFunc {
	return func(c *gin.Context) {
		foundContext, exists := c.Get("TUMLiveContext")
		if !exists {
			logger.Error("context should exist but doesn't")
			c.AbortWithStatus(http.StatusInternalServerError)
			return
		}
		tumLiveContext := foundContext.(TUMLiveContext)
		// Get course based on context:
		var course model.Course
		switch {
		case c.Param("courseID") != "":
			cIDInt, err := strconv.ParseInt(c.Param("courseID"), 10, 32)
			if err != nil {
				c.AbortWithStatus(http.StatusBadRequest)
				return
			}
			foundCourse, err := wrapper.CoursesDao.GetCourseById(c, uint(cIDInt))
			if err != nil {
				c.Status(http.StatusNotFound)
				RenderErrorPage(c, http.StatusNotFound, CourseNotFoundErrMsg)
			} else {
				course = foundCourse
			}
		case c.Param("year") != "" && c.Param("teachingTerm") != "" && c.Param("slug") != "":
			y := c.Param("year")
			yInt, err := strconv.Atoi(y)
			if err != nil {
				c.AbortWithStatus(http.StatusBadRequest)
				return
			}
			foundCourse, err := wrapper.CoursesDao.GetCourseBySlugYearAndTerm(c, c.Param("slug"), c.Param("teachingTerm"), yInt)
			if err != nil {
				c.Status(http.StatusNotFound)
				RenderErrorPage(c, http.StatusNotFound, CourseNotFoundErrMsg)
			} else {
				course = foundCourse
			}
		default:
			c.Status(http.StatusNotFound)
			RenderErrorPage(c, http.StatusNotFound, CourseNotFoundErrMsg)
		}
		if c.IsAborted() {
			return
		}
		// check if course is accessible by user:
		switch {
		case course.IsPublic(), course.IsHidden(), tumLiveContext.User != nil && tumLiveContext.User.IsEligibleToWatchCourse(course):
			tumLiveContext.Course = &course
			c.Set("TUMLiveContext", tumLiveContext)
		case tumLiveContext.User == nil:
			c.Redirect(http.StatusFound, "/login?return="+url.QueryEscape(c.Request.RequestURI))
			c.Abort()
			return
		default:
			c.Status(http.StatusForbidden)
			RenderErrorPage(c, http.StatusForbidden, ForbiddenCourseAccess)
		}
	}
}

func InitStream(wrapper dao.DaoWrapper) gin.HandlerFunc {
	return func(c *gin.Context) {
		foundContext, exists := c.Get("TUMLiveContext")
		if !exists {
			logger.Error("context should exist but doesn't")
			c.AbortWithStatus(http.StatusInternalServerError)
			return
		}
		tumLiveContext := foundContext.(TUMLiveContext)
		// Get stream based on context:
		var stream model.Stream
		if c.Param("streamID") != "" {
			foundStream, err := wrapper.StreamsDao.GetStreamByID(c, c.Param("streamID"))
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
		var course model.Course
		if tumLiveContext.Course != nil {
			// InitCourse ran first and AdminOfCourse/visibility checks vouch only for
			// that course, so a course's routes must not reach another course's stream.
			// Answer as for a missing stream rather than 403, so the reply doesn't reveal
			// that the ID exists elsewhere.
			if stream.CourseID != tumLiveContext.Course.ID {
				c.Status(http.StatusNotFound)
				RenderErrorPage(c, http.StatusNotFound, StreamNotFoundErrMsg)
				return
			}
			course = *tumLiveContext.Course
		} else {
			foundCourse, err := wrapper.CoursesDao.GetCourseById(c, stream.CourseID)
			if err != nil {
				c.AbortWithStatus(http.StatusBadRequest)
				return
			}
			course = foundCourse
		}

		if stream.Private && (tumLiveContext.User == nil || !tumLiveContext.User.IsAdminOfCourse(course)) {
			// The page alone would go out as a 200: a refused stream must say so to
			// whatever frames or fetches it (the Artemis embed does).
			c.Status(http.StatusForbidden)
			RenderErrorPage(c, http.StatusForbidden, ForbiddenStreamAccess)
			c.Abort()
			return
		}
		if !course.IsPublic() && !course.IsHidden() {
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
}

func OwnerOfCourse(c *gin.Context) {
	foundContext, exists := c.Get("TUMLiveContext")
	if !exists {
		logger.Error("context should exist but doesn't")
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
	tumLiveContext := foundContext.(TUMLiveContext)
	if !tumLiveContext.User.Can(model.PermAdministerAllCourses) && (tumLiveContext.User == nil || tumLiveContext.User.Model.ID != tumLiveContext.Course.UserID) {
		c.Status(http.StatusForbidden)
		RenderErrorPage(c, http.StatusForbidden, ForbiddenGenericErrMsg)
	}
}

// AdminOfCourse checks if the user is an admin of the course or admin.
// If not, aborts with status Forbidden.
func AdminOfCourse(c *gin.Context) {
	foundContext, exists := c.Get("TUMLiveContext")
	if !exists {
		logger.Error("context should exist but doesn't")
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
	tumLiveContext := foundContext.(TUMLiveContext)
	if tumLiveContext.User == nil {
		c.Redirect(http.StatusFound, "/login?return="+url.QueryEscape(c.Request.RequestURI))
		c.Abort()
		return
	}
	if tumLiveContext.User.IsAdminOfCourse(*tumLiveContext.Course) {
		return
	}
	// The SPA renders the refusal from the shell; a bare status left a blank page.
	RenderErrorPage(c, http.StatusForbidden, ForbiddenGenericErrMsg)
}

// RequirePermission aborts with Forbidden unless the caller holds the permission. A
// route group states the capability it needs rather than the role that has it, so
// moving a capability between roles is a change to model.rolePermissions alone.
func RequirePermission(p model.Permission) gin.HandlerFunc {
	return func(c *gin.Context) {
		foundContext, exists := c.Get("TUMLiveContext")
		if !exists {
			logger.Error("context should exist but doesn't")
			c.AbortWithStatus(http.StatusInternalServerError)
			return
		}
		tumLiveContext := foundContext.(TUMLiveContext)
		if !tumLiveContext.User.Can(p) {
			c.Status(http.StatusForbidden)
			RenderErrorPage(c, http.StatusForbidden, ForbiddenGenericErrMsg)
		}
	}
}

type TUMLiveContext struct {
	User          *model.User
	Course        *model.Course
	Stream        *model.Stream
	SamlSubjectID *string
}

func (c *TUMLiveContext) UserIsAdmin() bool {
	if c.User == nil {
		return false
	}
	return c.User.IsAdminOfCourse(*c.Course)
}
