package tools

import (
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/golang-jwt/jwt/v5"

	"github.com/TUM-Dev/gocast/dao"
	"github.com/TUM-Dev/gocast/model"
)

// errorPageRenderer answers a browser request with the error page. The web package
// installs it, because only it holds the SPA shell; see web.serveErrorPage.
var errorPageRenderer func(w http.ResponseWriter, r *http.Request, status int, message string)

// SetErrorPageRenderer sets how RenderErrorPage answers a request for a page.
func SetErrorPageRenderer(f func(w http.ResponseWriter, r *http.Request, status int, message string)) {
	errorPageRenderer = f
}

// JWTClaims are the claims contained in a session
type JWTClaims struct {
	*jwt.RegisteredClaims
	UserID        uint
	SamlSubjectID *string // identifier of the SAML session (if any)
}

// InitContext puts the signed-in user, if any, on the request as its TUMLiveContext.
// Every route behind it can rely on the context existing; a missing or broken session
// is an empty context, not an error.
func InitContext(daoWrapper dao.DaoWrapper) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// no context initialisation required for static assets.
			if strings.HasPrefix(r.RequestURI, "/public") ||
				strings.HasPrefix(r.RequestURI, "/favicon") {
				next.ServeHTTP(w, r)
				return
			}
			next.ServeHTTP(w, WithTUMLiveContext(r, sessionContext(w, r, daoWrapper)))
		})
	}
}

// sessionContext reads the session cookie. A cookie that does not verify is cleared,
// so the browser stops sending it.
func sessionContext(w http.ResponseWriter, r *http.Request, daoWrapper dao.DaoWrapper) TUMLiveContext {
	cookie, err := Cookie(r, "jwt")
	if err != nil {
		return TUMLiveContext{}
	}

	token, err := jwt.ParseWithClaims(cookie, &JWTClaims{}, func(token *jwt.Token) (interface{}, error) {
		key := Cfg.GetJWTKey().Public()
		return key, nil
	})
	if err != nil {
		logger.Info("JWT parsing error: ", "err", err)
		SetCookie(w, "jwt", "", -1, false)
		return TUMLiveContext{}
	}
	if !token.Valid {
		logger.Info("JWT token is not valid")
		SetCookie(w, "jwt", "", -1, false)
		return TUMLiveContext{}
	}

	user, err := daoWrapper.UsersDao.GetUserByID(r.Context(), token.Claims.(*JWTClaims).UserID)
	if err != nil {
		return TUMLiveContext{}
	}
	return TUMLiveContext{User: &user, SamlSubjectID: token.Claims.(*JWTClaims).SamlSubjectID}
}

// redirectToLogin sends an anonymous visitor to the login page, with the page to come
// back to.
func redirectToLogin(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, "/login?return="+url.QueryEscape(r.RequestURI), http.StatusFound)
}

// LoggedIn is a middleware that checks if the user is logged in and redirects to the login page if not
func LoggedIn(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tumLiveContext, ok := requireTUMLiveContext(w, r)
		if !ok {
			return
		}
		if tumLiveContext.User == nil {
			// Pass the page along so the login flow can return to it, as the course and
			// admin middlewares below do.
			redirectToLogin(w, r)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// RenderErrorPage answers with the given status and message. A page request gets the
// SPA shell, which renders the error; under /api/ the caller is a script, which gets
// the message as plain text.
func RenderErrorPage(w http.ResponseWriter, r *http.Request, status int, message string) {
	if errorPageRenderer == nil || strings.HasPrefix(r.URL.Path, "/api/") {
		WriteText(w, status, message)
	} else {
		errorPageRenderer(w, r, status, message)
	}
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

// InitCourse loads the course named by the route's courseID, or year, teachingTerm and
// slug, and refuses a caller who may not see it.
func InitCourse(wrapper dao.DaoWrapper) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tumLiveContext, ok := requireTUMLiveContext(w, r)
			if !ok {
				return
			}
			// Get course based on context:
			var course model.Course
			switch {
			case r.PathValue("courseID") != "":
				cIDInt, err := strconv.ParseInt(r.PathValue("courseID"), 10, 32)
				if err != nil {
					w.WriteHeader(http.StatusBadRequest)
					return
				}
				foundCourse, err := wrapper.CoursesDao.GetCourseById(r.Context(), uint(cIDInt))
				if err != nil {
					RenderErrorPage(w, r, http.StatusNotFound, CourseNotFoundErrMsg)
					return
				}
				course = foundCourse
			case r.PathValue("year") != "" && r.PathValue("teachingTerm") != "" && r.PathValue("slug") != "":
				yInt, err := strconv.Atoi(r.PathValue("year"))
				if err != nil {
					w.WriteHeader(http.StatusBadRequest)
					return
				}
				foundCourse, err := wrapper.CoursesDao.GetCourseBySlugYearAndTerm(r.Context(), r.PathValue("slug"), r.PathValue("teachingTerm"), yInt)
				if err != nil {
					RenderErrorPage(w, r, http.StatusNotFound, CourseNotFoundErrMsg)
					return
				}
				course = foundCourse
			default:
				RenderErrorPage(w, r, http.StatusNotFound, CourseNotFoundErrMsg)
				return
			}
			// check if course is accessible by user:
			switch {
			case course.IsPublic(), course.IsHidden(), tumLiveContext.User != nil && tumLiveContext.User.IsEligibleToWatchCourse(course):
				tumLiveContext.Course = &course
				next.ServeHTTP(w, WithTUMLiveContext(r, tumLiveContext))
			case tumLiveContext.User == nil:
				redirectToLogin(w, r)
			default:
				RenderErrorPage(w, r, http.StatusForbidden, ForbiddenCourseAccess)
			}
		})
	}
}

// InitStream loads the stream named by the route's streamID, and refuses a caller who
// may not watch it.
func InitStream(wrapper dao.DaoWrapper) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tumLiveContext, ok := requireTUMLiveContext(w, r)
			if !ok {
				return
			}
			// Get stream based on context:
			if r.PathValue("streamID") == "" {
				RenderErrorPage(w, r, http.StatusNotFound, StreamNotFoundErrMsg)
				return
			}
			stream, err := wrapper.StreamsDao.GetStreamByID(r.Context(), r.PathValue("streamID"))
			if err != nil {
				RenderErrorPage(w, r, http.StatusNotFound, StreamNotFoundErrMsg)
				return
			}
			var course model.Course
			if tumLiveContext.Course != nil {
				// InitCourse ran first and AdminOfCourse/visibility checks vouch only for
				// that course, so a course's routes must not reach another course's stream.
				// Answer as for a missing stream rather than 403, so the reply doesn't reveal
				// that the ID exists elsewhere.
				if stream.CourseID != tumLiveContext.Course.ID {
					RenderErrorPage(w, r, http.StatusNotFound, StreamNotFoundErrMsg)
					return
				}
				course = *tumLiveContext.Course
			} else {
				foundCourse, err := wrapper.CoursesDao.GetCourseById(r.Context(), stream.CourseID)
				if err != nil {
					w.WriteHeader(http.StatusBadRequest)
					return
				}
				course = foundCourse
			}

			if stream.Private && (tumLiveContext.User == nil || !tumLiveContext.User.IsAdminOfCourse(course)) {
				// The page alone would go out as a 200: a refused stream must say so to
				// whatever frames or fetches it (the Artemis embed does).
				RenderErrorPage(w, r, http.StatusForbidden, ForbiddenStreamAccess)
				return
			}
			if !course.IsPublic() && !course.IsHidden() {
				if tumLiveContext.User == nil {
					redirectToLogin(w, r)
					return
				} else if !tumLiveContext.User.IsEligibleToWatchCourse(course) {
					RenderErrorPage(w, r, http.StatusForbidden, ForbiddenStreamAccess)
					return
				}
			}
			tumLiveContext.Course = &course
			tumLiveContext.Stream = &stream
			next.ServeHTTP(w, WithTUMLiveContext(r, tumLiveContext))
		})
	}
}

// AdminOfCourse checks if the user is an admin of the course or admin.
// If not, answers Forbidden.
func AdminOfCourse(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tumLiveContext, ok := requireTUMLiveContext(w, r)
		if !ok {
			return
		}
		if tumLiveContext.User == nil {
			redirectToLogin(w, r)
			return
		}
		if tumLiveContext.User.IsAdminOfCourse(*tumLiveContext.Course) {
			next.ServeHTTP(w, r)
			return
		}
		// The SPA renders the refusal from the shell; a bare status left a blank page.
		RenderErrorPage(w, r, http.StatusForbidden, ForbiddenGenericErrMsg)
	})
}

// RequirePermission answers Forbidden unless the caller holds the permission. A
// route group states the capability it needs rather than the role that has it, so
// moving a capability between roles is a change to model.rolePermissions alone.
func RequirePermission(p model.Permission) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tumLiveContext, ok := requireTUMLiveContext(w, r)
			if !ok {
				return
			}
			if !tumLiveContext.User.Can(p) {
				RenderErrorPage(w, r, http.StatusForbidden, ForbiddenGenericErrMsg)
				return
			}
			next.ServeHTTP(w, r)
		})
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
