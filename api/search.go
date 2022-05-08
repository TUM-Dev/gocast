package api

import (
	"github.com/RBG-TUM/commons"
	"github.com/gin-gonic/gin"
	"github.com/joschahenningsen/TUM-Live/dao"
	"github.com/joschahenningsen/TUM-Live/model"
	log "github.com/sirupsen/logrus"
	"net/http"
)

func configGinSearchRouter(router *gin.Engine, dao dao.VideoSectionDao) {
	api := router.Group("/api")
	api.GET("/search", search)
}

func search(c *gin.Context) {
	q := c.Query("q")
	if q == "" {
		c.AbortWithStatusJSON(http.StatusBadRequest, "empty query (?q= missing)")
	}

	streamIDs := make([]uint, 0)

	sections, err := dao.NewVideoSectionDao().Search(q)
	if err != nil {
		log.WithError(err).Error("could not perform fulltext search")
		c.AbortWithStatus(http.StatusInternalServerError)
	}

	// Take streamIDs only
	uniqueStreams := commons.Unique(sections, func(vs model.VideoSection) uint {
		return vs.StreamID
	})
	for _, s := range uniqueStreams {
		streamIDs = append(streamIDs, s.StreamID)
	}

	c.JSON(http.StatusOK, streamIDs)
}
