package web

import (
	"TUM-Live/dao"
	"TUM-Live/model"
	"TUM-Live/tools"
	"context"
	"github.com/gin-gonic/gin"
	"net/http"
)

func CoursePage(c *gin.Context) {
	slug := c.Param("slug")
	teachingTerm := c.Param("teachingTerm")
	course, err := dao.GetCourseBySlugAndTerm(context.Background(), slug, teachingTerm)
	if err != nil {
		c.Status(http.StatusNotFound)
		_ = templ.ExecuteTemplate(c.Writer, "error.gohtml", nil)
		return
	}
	indexData := IndexData{}
	u, uErr := tools.GetUser(c)
	s, sErr := tools.GetStudent(c)
	if uErr == nil {
		indexData.IsUser = true
	}
	if sErr == nil {
		indexData.IsStudent = true
	}
	if course.Visibility == "loggedin" {
		if !indexData.IsStudent && !indexData.IsUser {
			c.Status(http.StatusForbidden)
			_ = templ.ExecuteTemplate(c.Writer, "error.gohtml", nil)
			return
		}
	} else if course.Visibility == "enrolled" {
		if uErr == nil { // logged in with internal account, check if authorized
			if u.Role != 1 && course.UserID != u.ID {
				//logged in but not admin or owner
				c.Status(http.StatusForbidden)
				_ = templ.ExecuteTemplate(c.Writer, "error.gohtml", nil)
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
				_ = templ.ExecuteTemplate(c.Writer, "error.gohtml", nil)
				return
			}
		} else { // not logged in
			c.Status(http.StatusForbidden)
			_ = templ.ExecuteTemplate(c.Writer, "error.gohtml", nil)
			return
		}
	}
	// in any other case assume either validated before or public course
	_ = templ.ExecuteTemplate(c.Writer, "course.gohtml", CoursePageData{IndexData: indexData, Course: course})
}

type CoursePageData struct {
	IndexData IndexData
	Course    model.Course
}
