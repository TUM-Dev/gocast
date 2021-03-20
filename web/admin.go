package web

import (
	"TUM-Live/dao"
	"TUM-Live/model"
	"TUM-Live/tools"
	"context"
	"github.com/gin-gonic/gin"
	"log"
	"net/http"
	"strconv"
)

func AdminPage(c *gin.Context) {
	user, err := tools.GetUser(c)
	if err != nil {
		c.Redirect(http.StatusFound, "/login")
		return
	}
	var users []model.User
	_ = dao.GetAllUsers(context.Background(), &users)
	courses, err := dao.GetCoursesByUserId(context.Background(), user.ID)
	if err != nil {
		log.Printf("couldn't query courses for user. %v\n", err)
		courses = []model.Course{}
	}
	_ = templ.ExecuteTemplate(c.Writer, "admin.gohtml", AdminPageData{User: user, Users: users, Courses: courses, IndexData: IndexData{IsStudent: false, IsUser: true}})
}

func EditCoursePage(c *gin.Context) {
	user, err := tools.GetUser(c)
	if err != nil {
		c.Redirect(http.StatusFound, "/login")
		return
	}
	u64, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}
	course, err := dao.GetCourseById(context.Background(), uint(u64))
	if err != nil {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}
	// user has to be course owner or admin
	if user.Role != 1 && course.UserID != user.ID {
		c.AbortWithStatus(http.StatusForbidden)
		return
	}
	err = templ.ExecuteTemplate(c.Writer, "edit-course.gohtml", EditCourseData{IndexData: IndexData{IsUser: true}, Course: course})
	if err != nil {
		log.Printf("%v\n", err)
	}
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

type EditCourseData struct {
	IndexData IndexData
	Course    model.Course
}
