package api

import (
	"TUM-Live/dao"
	"TUM-Live/tools"
	"context"
	"fmt"
	"github.com/gin-gonic/gin"
	"log"
	"net/http"
	"strings"
)

func configGinStreamAuthRouter(router gin.IRoutes) {
	router.POST("/stream-management/on_publish", StartStream)
	router.POST("/stream-management/on_publish_done", EndStream)
	router.POST("/stream-management/on_record_done", OnRecordingFinished)
	router.POST("/api/createStream", CreateStream)
}

func CreateStream(c *gin.Context) {

}

/**
* This function is called when a user attempts to push a stream to the server.
* @w: response writer. Status code determines wether streaming is approved: 200 if yes, 402 otherwise.
* @r: request. Form if valid: POST /on_publish/app/kurs-key example: {/on_publish/eidi-3zt45z452h4754nj2q74}
 */
func StartStream(c *gin.Context) {
	_ = c.Request.ParseForm()
	key := c.Request.FormValue("name")
	parts := strings.Split(key, "-")
	if len(parts) != 2 {
		c.AbortWithStatus(http.StatusForbidden)
		fmt.Printf("stream rejected. cause: key not in correct form.\n")
		return
	}
	res, err := dao.GetStreamByKey(context.Background(), parts[1])
	if err != nil {
		c.AbortWithStatus(http.StatusForbidden)
		fmt.Printf("stream rejected. cause: %v\n", err)
		return
	}
	fmt.Printf("stream approved: id=%d\n", res.ID)
	err = dao.SetStreamLive(context.Background(), parts[1], tools.Cfg.LrzServerHls+key+"/playlist.m3u8")
	if err != nil {
		log.Printf("Couldn't create live stream: %v\n", err)
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
	c.Status(http.StatusOK)
}

func EndStream(c *gin.Context) {
	_ = c.Request.ParseForm()
	key := c.Request.FormValue("name")
	parts := strings.Split(key, "-")
	if len(parts) != 2 {
		log.Printf("stream publish ended with invalid key. That's odd.")
		return
	}
	_ = dao.SetStreamNotLive(context.Background(), parts[1])
}

// TODO: Convert recording to mp4 and put into correct directory. Delete flv file.
func OnRecordingFinished(c *gin.Context) {
	println(FormatRequest(c.Request))
}
