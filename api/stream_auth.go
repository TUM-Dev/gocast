package api

import (
	"github.com/gin-gonic/gin"
	"github.com/julienschmidt/httprouter"
	"net/http"
)

func configGinStreamAuthRouter(router gin.IRoutes) {
	router.POST("/on_publish", ConverHttprouterToGin(AuthenticateStream))
	router.POST("/on_publish_done", ConverHttprouterToGin(EndStream))
}

/**
* This function is called when a user attempts to push a stream to the server.
* @w: response writer. Status code determines wether streaming is approved: 200 if yes, 402 otherwise.
* @r: request. Form if valid: POST /on_publish/app/kurs-key example: {/on_publish/eidi-3zt45z452h4754nj2q74}
 */
func AuthenticateStream(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	w.WriteHeader(200)
}


func EndStream(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {

}
