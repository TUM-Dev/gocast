package api

import (
	"TUM-Live/dao"
	"TUM-Live/tools"
	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
	"net/http"
)

func configProgressRouter(router *gin.Engine) {
	router.POST("/api/progress", postProgress)
	router.POST("/api/progressRequest", requestProgress)
}

type progressRequest struct {
	VideoID  uint    `json:"video_id"`
	Progress float64 `json:"progress"`
}
type progressReply struct {
	VideoID  uint    `json:"video_id"`
}

type Response struct {
	Progress float64 `json:"progress"`
}

func requestProgress(c *gin.Context) {
	var reply progressReply

	err := c.BindJSON(&reply)

	if err != nil {
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}

	foundContext, exists := c.Get("TUMLiveContext")

	if !exists {
		return
	}

	tumLiveContext := foundContext.(tools.TUMLiveContext)

	if tumLiveContext.User == nil {
		c.AbortWithStatus(http.StatusForbidden)
		return
	}

	var progress float64

	log.Info("VideoID")
	log.Info(reply.VideoID)

	progress = dao.LoadProgress(tumLiveContext.User.ID, reply.VideoID)

	log.Info("Progress:")
	log.Info(progress)

	c.JSON(http.StatusOK, gin.H{"progress": progress})
}

func postProgress(c *gin.Context) {
	var req progressRequest

	err := c.BindJSON(&req)

	if err != nil {
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}

	foundContext, exists := c.Get("TUMLiveContext")

	if !exists {
		return
	}

	tumLiveContext := foundContext.(tools.TUMLiveContext)

	if tumLiveContext.User == nil {
		c.AbortWithStatus(http.StatusForbidden)
		return
	}

	dao.SaveProgress(req.Progress, tumLiveContext.User.ID, req.VideoID)
	log.Info(req.Progress)
}
