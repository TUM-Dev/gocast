package tools

import (
	"errors"
	"github.com/getsentry/sentry-go"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v4"
	"github.com/joschahenningsen/TUM-Live/dao"
	"github.com/joschahenningsen/TUM-Live/model"
	log "github.com/sirupsen/logrus"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

var templateExecutor TemplateExecutor

// SetTemplateExecutor sets the templates and template executor for the middlewares to execute error pages
func SetTemplateExecutor(e TemplateExecutor) {
	templateExecutor = e
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
		if strings.HasPrefix(c.Request.RequestURI, "/static") ||
			strings.HasPrefix(c.Request.RequestURI, "/public") ||
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
			log.Info("JWT parsing error: ", err)
			c.Set("TUMLiveContext", TUMLiveContext{})
			c.SetCookie("jwt", "", -1, "/", "", false, true)
			return
		}
		if !token.Valid {
			log.Info("JWT token is not valid")
			c.Set("TUMLiveContext", TUMLiveContext{})
			c.SetCookie("jwt", "", -1, "/", "", false, true)
			return
		}

		user, err := daoWrapper.UsersDao.GetUserByID(c, token.Claims.(*JWTClaims).UserID)
		if err != nil {
			c.Set("TUMLiveContext", TUMLiveContext{})
			return
		} else {
			c.Set("TUMLiveContext", TUMLiveContext{User: &user, SamlSubjectID: token.Claims.(*JWTClaims).SamlSubjectID})
			return
		}
	}
}

// LoggedIn is a middleware that checks if the user is logged in and redirects to the login page if not
func LoggedIn(c *gin.Context) {
	if c.MustGet("TUMLiveContext").(TUMLiveContext).User == nil {
		c.Redirect(http.StatusFound, "/login")
		c.Abort()
	}
}

// RenderErrorPage renders the error page with the given error code and message.
// the gin context is always aborted after this function is called.
func RenderErrorPage(c *gin.Context, status int, message string) {
	err := templateExecutor.ExecuteTemplate(c.Writer, "error.gohtml", ErrorPageData{
		Status:   status,
		Message:  message,
		Branding: BrandingCfg,
	})
	if err != nil {
		log.Error(err)
	}
	c.Abort()
}

//ErrorPageData is the required data for the error page
type ErrorPageData struct {
	Status   int
	Message  string
	Branding Branding
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
			foundCourse, err := wrapper.CoursesDao.GetCourseById(c, uint(cIDInt))
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
			foundCourse, err := wrapper.CoursesDao.GetCourseBySlugYearAndTerm(c, c.Param("slug"), c.Param("teachingTerm"), yInt)
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
}

func InitStream(wrapper dao.DaoWrapper) gin.HandlerFunc {
	return func(c *gin.Context) {
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
			RenderErrorPage(c, http.StatusForbidden, ForbiddenStreamAccess)
			c.Abort()
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
}

func OwnerOfCourse(c *gin.Context) {
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

// AdminOfCourse checks if the user is an admin of the course or admin.
// If not, aborts with status Forbidden.
func AdminOfCourse(c *gin.Context) {
	foundContext, exists := c.Get("TUMLiveContext")
	if !exists {
		sentry.CaptureException(errors.New("context should exist but doesn't"))
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
	c.AbortWithStatus(http.StatusForbidden) // user is not admin of course
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

func AdminToken(daoWrapper dao.DaoWrapper) gin.HandlerFunc {
	return func(c *gin.Context) {
		queryParams := c.Request.URL.Query()
		token := queryParams.Get("token")
		t, err := daoWrapper.TokenDao.GetToken(token)
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
		err = daoWrapper.TokenDao.TokenUsed(t)
		if err != nil {
			log.WithError(err).Warn("error marking token as used")
			return
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
