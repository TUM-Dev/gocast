package web

import (
	"TUM-Live/dao"
	"TUM-Live/tools"
	"context"
	"github.com/gin-gonic/gin"
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
	sid, err := tools.GetSID(request)
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

func CreatePasswordPage(c *gin.Context) {
	key := c.Param("key")
	err := templ.ExecuteTemplate(c.Writer, "passwordreset.gohtml", key)
	if err != nil {
		log.Printf("couldn't render template: %v\n", err)
	}
}

