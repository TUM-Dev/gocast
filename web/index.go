package web

import (
	"TUM-Live/dao"
	"TUM-Live/model"
	"TUM-Live/tools"
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
		err = tools.GetUser(writer, request, &user)
		var streams []model.Stream
		err := dao.GetCurrentLive(context.Background(), &streams)
		if err != nil {
			_ = templ.ExecuteTemplate(writer, "index.gohtml", IndexData{User: user, LiveStreams: streams})
		} else {
			_ = templ.ExecuteTemplate(writer, "index.gohtml", IndexData{User: user, LiveStreams: streams})
		}
	}
}

type IndexData struct {
	User        model.User
	LiveStreams []model.Stream
}
