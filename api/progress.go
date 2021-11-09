package api

import (
	"TUM-Live/dao"
	"TUM-Live/tools"
	"github.com/gin-gonic/gin"
	"net/http"
)

func configProgressRouter(router *gin.Engine) {
	router.POST("/api/progressReport", saveProgress)
	router.POST("/api/progressRequest", fetchProgress)
}

type ProgressRequest struct {
	StreamID uint    `json:"streamID"`
	Progress float64 `json:"progress"`
}

type ProgressReply struct {
	StreamID uint `json:"streamID"`
}

type Response struct {
	Progress float64 `json:"progress"`
}

func saveProgress(c *gin.Context) {
	var request ProgressRequest

	err := c.BindJSON(&request)

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

	dao.SaveProgress(request.Progress, tumLiveContext.User.ID, request.StreamID)
}

func fetchProgress(c *gin.Context) {
	var reply ProgressReply

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

	progress = dao.LoadProgress(tumLiveContext.User.ID, reply.StreamID)

	c.JSON(http.StatusOK, gin.H{"progress": progress})
}
