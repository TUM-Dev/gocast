package web

import (
	"TUM-Live-Backend/model"
	"github.com/julienschmidt/httprouter"
	"net/http"
)

func AdminPage(writer http.ResponseWriter, request *http.Request, params httprouter.Params) {
	user := model.User{}
	err := getUser(request, &user)
	if err != nil {
		if err==errorNotLoggedIn {
			http.Redirect(writer, request, "/login", http.StatusFound)
		}else {
			writer.WriteHeader(http.StatusInternalServerError)
			return
		}
	}
	_ = templ.ExecuteTemplate(writer, "admin.gohtml", user)
}
