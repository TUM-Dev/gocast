package web

import (
	"TUM-Live/dao"
	"TUM-Live/model"
	"TUM-Live/tools"
	"context"
	"github.com/gin-gonic/gin"
	"net/http"
)

func AdminPage(c *gin.Context) {
	user, err := tools.GetUser(c)
	if err != nil {
		c.Redirect(http.StatusFound, "/login")
		return
	}
	var users []model.User
	_ = dao.GetAllUsers(context.Background(), &users)
	_ = templ.ExecuteTemplate(c.Writer, "admin.gohtml", AdminPageData{User: user, Users: users, Courses: user.Courses, IndexData: IndexData{IsStudent: false, IsUser: true}})
}

func CreateCoursePage(c *gin.Context) {
	user, err := tools.GetUser(c)
	if err != nil {
		c.Redirect(http.StatusFound, "/login")
		return
	}
	// check if user is admin or lecturer
	if user.Role > 2 {
		c.Redirect(http.StatusFound, "/login")
		return
	}
	_ = templ.ExecuteTemplate(c.Writer, "create-course.gohtml", CreateCourseData{User: user, IndexData: IndexData{IsStudent: false, IsUser: true}})
}

type AdminPageData struct {
	IndexData IndexData
	User      model.User
	Users     []model.User
	Courses   []model.Course
}

type CreateCourseData struct {
	IndexData IndexData
	User      model.User
}
