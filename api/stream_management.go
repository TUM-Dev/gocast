package api

import (
	"TUM-Live-Backend/dao"
	"context"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/julienschmidt/httprouter"
	"net/http"
)

func configGinStreamAuthRouter(router gin.IRoutes) {
	router.POST("/stream-management/on_publish", ConverHttprouterToGin(AuthenticateStream))
	router.POST("/stream-management/on_publish_done", ConverHttprouterToGin(EndStream))
	router.POST("/stream-management/on_record_done", ConverHttprouterToGin(OnRecordingFinished))
}

/**
* This function is called when a user attempts to push a stream to the server.
* @w: response writer. Status code determines wether streaming is approved: 200 if yes, 402 otherwise.
* @r: request. Form if valid: POST /on_publish/app/kurs-key example: {/on_publish/eidi-3zt45z452h4754nj2q74}
 */
func AuthenticateStream(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	fmt.Printf("%v\n", r.Header.Get("key"))
	res, err := dao.GetStreamByKey(context.Background(), "key1")
	if err != nil {
		w.WriteHeader(403) //reject when no results in database
		fmt.Printf("stream rejected. cause: %v\n", err)
		return
	}
	fmt.Printf("stream approved: id=%d\n", res.ID)
	w.WriteHeader(200)
}

func EndStream(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	println(FormatRequest(r))
}

// TODO: Convert recording to mp4 and put into correct directory. Delete flv file.
func OnRecordingFinished(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	println(FormatRequest(r))
}
