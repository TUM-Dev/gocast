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

type AdminPageData struct {
	IndexData IndexData
	User      model.User
	Users     []model.User
	Courses   []model.Course
}
