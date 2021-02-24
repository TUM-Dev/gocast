package web

import (
	"TUM-Live-Backend/dao"
	"context"
	"github.com/julienschmidt/httprouter"
	"log"
	"net/http"
	"time"
)

func LoginPage(writer http.ResponseWriter, request *http.Request, params httprouter.Params) {
	err := templ.ExecuteTemplate(writer, "login.gohtml", nil)
	if err != nil {
		log.Printf("couldn't render template: %v\n", err)
	}
}

func LogoutPage(writer http.ResponseWriter, request *http.Request, params httprouter.Params) {
	sid, err := getSID(request)
	if err == nil { // logged in
		err = dao.DeleteSession(context.Background(), sid)
		if err != nil {
			log.Printf("couldn't delete session: %v\n", err)
		}
	}
	c := http.Cookie{Name: "SID", Value: "", Expires: time.Unix(0, 0), Path: "/"} //cookie expired
	http.SetCookie(writer, &c)
	http.Redirect(writer, request, "/", http.StatusFound)
}
