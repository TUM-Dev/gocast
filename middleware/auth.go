package middleware

import (
	"TUM-Live/dao"
	"TUM-Live/model"
	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	"net/http"
)

// InitContext
//
// Initializes gin context on every request. Sets values for session specific stuff (user, student, isAdmin etc.)
func InitContext() gin.HandlerFunc {
	return func(c *gin.Context) {
		// init user attributes
		s := sessions.Default(c)
		var tumLiveContext TUMLiveContext
		uid := s.Get("UserID")
		if uid != nil {
			user, err := dao.GetUserByID(c, uid.(uint))
			if err == nil {
				tumLiveContext.User = &user
				tumLiveContext.IsAdmin = user.Role == model.AdminType || user.Role == model.LecturerType
			}
		} else {
			studentID := s.Get("StudentID")
			if studentID != nil {
				student, err := dao.GetStudent(c, studentID.(string))
				if err != nil {
					tumLiveContext.Student = &student
					tumLiveContext.IsStudent = true
				}
			}
		}
		name := s.Get("Name")
		if name != nil {
			tumLiveContext.Name = name.(string)
		}
		if streamID := c.Param("streamID"); streamID != "" {
			if stream, err := dao.GetStreamByID(c, streamID); err == nil {
				tumLiveContext.Stream = &stream
			}
		}
		if courseID := c.Param("courseID"); courseID != "" {
			if course, err := dao.GetCourseByIdStr(courseID); err == nil {
				tumLiveContext.Course = &course
			}
		}

		c.Set("TUMLiveContext", tumLiveContext)
	}
}

// RequireAtLeastLecturer
//
// Middleware that aborts with 403 Forbidden if the user is not an administrator or Lecturer
func RequireAtLeastLecturer() gin.HandlerFunc {
	return func(context *gin.Context) {
		found, exists := context.Get("TUMLiveContext")
		tumLiveContext := found.(TUMLiveContext)
		if exists {
			if !tumLiveContext.IsAdmin {
				context.AbortWithStatus(http.StatusForbidden)
			}
		} else {
			context.AbortWithStatus(http.StatusForbidden)
		}
	}
}

//RequireAtLeastViewer
//
//Middleware that aborts the context with 403 Forbidden if User is not allowed to see a course
func RequireAtLeastViewer() gin.HandlerFunc {
	return func(context *gin.Context) {
		var tumLiveContext TUMLiveContext
		if found, exists := context.Get("TUMLiveContext"); exists {
			tumLiveContext = found.(TUMLiveContext)
		} else {
			context.AbortWithStatus(http.StatusInternalServerError)
			return
		}
		slug := context.Param("slug")
		teachingTerm := context.Param("teachingTerm")
		year := context.Param("year")
		if course, err := dao.GetCourseBySlugYearAndTerm(context, slug, teachingTerm, year); err == nil {
			tumLiveContext.Course = &course
			context.Set("TUMLiveContext", tumLiveContext)
			if course.Visibility == "loggedin" && !(tumLiveContext.IsAdmin || tumLiveContext.IsStudent) {
				// logged in course but not logged in
				context.AbortWithStatus(http.StatusForbidden)
			} else if course.Visibility == "enrolled" {
				if tumLiveContext.User != nil {
					if !tumLiveContext.User.IsEligibleToWatchCourse(course) {
						// user but not admin, owner or invited
						context.AbortWithStatus(http.StatusForbidden)
					}
				} else if tumLiveContext.Student != nil {
					for _, sCourse := range tumLiveContext.Student.Courses {
						if sCourse.ID == course.ID {
							return
						}
					}
					// student but not enrolled
					context.AbortWithStatus(http.StatusForbidden)
				} else {
					// enrolled but neither student or user
					context.AbortWithStatus(http.StatusForbidden)
				}
			}
		} else {
			// course does not exist
			context.AbortWithStatus(http.StatusNotFound)
		}

	}
}

type TUMLiveContext struct {
	Name      string
	User      *model.User
	IsAdmin   bool
	Student   *model.Student
	IsStudent bool
	Course    *model.Course
	Stream    *model.Stream
}
