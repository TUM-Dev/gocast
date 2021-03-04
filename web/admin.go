package web

import (
	"TUM-Live/dao"
	"TUM-Live/model"
	"TUM-Live/tools"
	"context"
	"github.com/julienschmidt/httprouter"
	"net/http"
)

func AdminPage(writer http.ResponseWriter, request *http.Request, params httprouter.Params) {
	user := tools.RequirePermission(writer, *request, 2) // user has to be admin or lecturer
	if user == nil {
		return
	}
	var users []model.User
	_ = dao.GetAllUsers(context.Background(), &users)
	courses, _ := dao.GetCoursesByUserId(context.Background(), user.Model.ID)
	_ = templ.ExecuteTemplate(writer, "admin.gohtml", AdminPageData{User: *user, Users: users, Courses: courses})
}

type AdminPageData struct {
	User    model.User
	Users   []model.User
	Courses []model.Course
}
