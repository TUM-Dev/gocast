package web

import (
	"TUM-Live-Backend/dao"
	"TUM-Live-Backend/model"
	"context"
	"github.com/julienschmidt/httprouter"
	"net/http"
)

func MainPage(writer http.ResponseWriter, request *http.Request, ps httprouter.Params) {
	res, err := dao.AreUsersEmpty(context.Background()) // fresh installation?
	if err != nil {
		_ = templ.ExecuteTemplate(writer, "error.gohtml", nil)
	} else if res {
		_ = templ.ExecuteTemplate(writer, "onboarding.gohtml", nil)
	} else {
		user := model.User{}
		err = getUser(request, &user)
		if err != nil {
			_ = templ.ExecuteTemplate(writer, "index.gohtml", nil)
		} else {
			_ = templ.ExecuteTemplate(writer, "index.gohtml", user)
		}
	}

}
