package api

import (
	"TUM-Live/dao"
	"TUM-Live/tools"
	"context"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/julienschmidt/httprouter"
	"log"
	"net/http"
	"strings"
)

func configGinStreamAuthRouter(router gin.IRoutes) {
	router.POST("/stream-management/on_publish", ConvertHttprouterToGin(StartStream))
	router.POST("/stream-management/on_publish_done", ConvertHttprouterToGin(EndStream))
	router.POST("/stream-management/on_record_done", ConvertHttprouterToGin(OnRecordingFinished))
	router.POST("/api/createStream", ConvertHttprouterToGin(CreateStream))
}

func CreateStream(writer http.ResponseWriter, request *http.Request, params httprouter.Params) {

}

/**
* This function is called when a user attempts to push a stream to the server.
* @w: response writer. Status code determines wether streaming is approved: 200 if yes, 402 otherwise.
* @r: request. Form if valid: POST /on_publish/app/kurs-key example: {/on_publish/eidi-3zt45z452h4754nj2q74}
 */
func StartStream(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	_ = r.ParseForm()
	key := r.FormValue("name")
	parts := strings.Split(key, "-")
	if len(parts) != 2 {
		w.WriteHeader(403) //reject when no results in database
		fmt.Printf("stream rejected. cause: key not in correct form.\n")
		return
	}
	res, err := dao.GetStreamByKey(context.Background(), parts[1])
	if err != nil {
		w.WriteHeader(403) //reject when no results in database
		fmt.Printf("stream rejected. cause: %v\n", err)
		return
	}
	fmt.Printf("stream approved: id=%d\n", res.ID)
	err = dao.SetStreamLive(context.Background(), parts[1], tools.Cfg.LrzServerHls+key+"/playlist.m3u8")
	if err != nil {
		log.Printf("Couldn't create live stream: %v\n", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.WriteHeader(200)
}

func EndStream(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	_ = r.ParseForm()
	key := r.FormValue("name")
	parts := strings.Split(key, "-")
	if len(parts) != 2 {
		log.Printf("stream publish ended with invalid key. That's odd.")
		return
	}
	_ = dao.SetStreamNotLive(context.Background(), parts[1])
}

// TODO: Convert recording to mp4 and put into correct directory. Delete flv file.
func OnRecordingFinished(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	println(FormatRequest(r))
}
