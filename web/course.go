package web

import (
	"TUM-Live/dao"
	"TUM-Live/model"
	"TUM-Live/tools"
	"context"
	"fmt"
	"github.com/getsentry/sentry-go"
	sentrygin "github.com/getsentry/sentry-go/gin"
	"github.com/gin-gonic/gin"
	"net/http"
)

func CoursePage(c *gin.Context) {
	slug := c.Param("slug")
	teachingTerm := c.Param("teachingTerm")
	year := c.Param("year")
	span := sentry.StartSpan(c, fmt.Sprintf("GET /course/%v/%v/%v", year, teachingTerm, slug), sentry.TransactionName(fmt.Sprintf("GET /course/%v/%v/%v", year, teachingTerm, slug)))
	defer span.Finish()
	indexData := NewIndexData()
	u, uErr := tools.GetUser(c)
	s, sErr := tools.GetStudent(c)
	if uErr == nil {
		indexData.IsUser = true
		indexData.IsAdmin = u.Role == model.AdminType || u.Role == model.LecturerType
	}
	if sErr == nil {
		indexData.IsStudent = true
	}
	course, err := dao.GetCourseBySlugYearAndTerm(context.Background(), slug, teachingTerm, year)
	if err != nil {
		c.Status(http.StatusNotFound)
		_ = templ.ExecuteTemplate(c.Writer, "error.gohtml", ErrorPageData{IndexData: indexData, Status: http.StatusNotFound, Message: "Course not found."})
		return
	}
	if course.Visibility == "loggedin" {
		if !indexData.IsStudent && !indexData.IsUser {
			c.Status(http.StatusForbidden)
			_ = templ.ExecuteTemplate(c.Writer, "error.gohtml", ErrorPageData{IndexData: indexData, Status: http.StatusForbidden, Message: "Please log in to access this course."})
			return
		}
	} else if course.Visibility == "enrolled" {
		if uErr == nil { // logged in with internal account, check if authorized
			if !u.IsEligibleToWatchCourse(course) {
				c.Status(http.StatusForbidden)
				_ = templ.ExecuteTemplate(c.Writer, "error.gohtml", ErrorPageData{IndexData: indexData, Status: http.StatusForbidden, Message: "You are not allowed to see this course. Please log in or contact your instructor."})
				return
			}
		} else if sErr == nil { // logged in as student check if authorized
			isEnrolled := false
			for i := range s.Courses {
				if s.Courses[i].ID == course.ID {
					isEnrolled = true
					break
				}
			}
			if !isEnrolled {
				c.Status(http.StatusForbidden)
				_ = templ.ExecuteTemplate(c.Writer, "error.gohtml", ErrorPageData{IndexData: indexData, Status: http.StatusForbidden, Message: "You are not allowed to see this course. Please log in or contact your instructor."})
				return
			}
		} else { // not logged in
			c.Status(http.StatusForbidden)
			_ = templ.ExecuteTemplate(c.Writer, "error.gohtml", ErrorPageData{IndexData: indexData, Status: http.StatusForbidden, Message: "You are not allowed to see this course. Please log in or contact your instructor."})
			return
		}
	}
	// in any other case assume either validated before or public course
	err = templ.ExecuteTemplate(c.Writer, "course.gohtml", CoursePageData{IndexData: indexData, Course: course})
	if err != nil {
		sentrygin.GetHubFromContext(c).CaptureException(err)
	}
}

type CoursePageData struct {
	IndexData IndexData
	Course    model.Course
}
