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
}

type progressRequest struct {
	VideoID  uint    `json:"video_id"`
	Progress float64 `json:"progress"`
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