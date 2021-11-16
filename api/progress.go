package api

import (
	"TUM-Live/dao"
	"TUM-Live/tools"
	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
	"net/http"
)

func configProgressRouter(router *gin.Engine) {
	router.POST("/api/progressReport", saveProgress)
}

// ProgressRequest corresponds the request that is sent by the video player when it reports its progress for VODs.
type ProgressRequest struct {
	StreamID uint    `json:"streamID"`
	Progress float64 `json:"progress"` // A fraction that represents currentTime / totalTime for a given video.
	// Note: To be able to save the progress, we also need the userID, but it`s already contained in the Gin context.
}

func saveProgress(c *gin.Context) {
	var request ProgressRequest

	err := c.BindJSON(&request)

	if err != nil {
		log.WithError(err).Warn("Could not bind JSON from progressReport.")
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

	err = dao.SaveProgress(request.Progress, tumLiveContext.User.ID, request.StreamID)

	if err != nil {
		log.WithError(err).Warn("Could not save progress in the database.")
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
}
