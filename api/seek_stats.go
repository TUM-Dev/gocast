package api

import (
	"github.com/gin-gonic/gin"
	"github.com/joschahenningsen/TUM-Live/dao"
	log "github.com/sirupsen/logrus"
	"net/http"
)

// progressRoutes contains a DaoWrapper object and all route functions dangle from it.
type seekStatsRoutes struct {
	dao.DaoWrapper
}

// configSeekStatsRouter sets up the router and initializes a progress buffer
// that is used to minimize writes to the database.
func configSeekStatsRouter(router *gin.Engine, daoWrapper dao.DaoWrapper) {
	routes := seekStatsRoutes{daoWrapper}

	router.POST("/api/seekReport/:streamID", routes.reportSeek)
	router.GET("/api/seekReport/:streamID", routes.getSeek)
}

type reportSeekRequest struct {
	Position float64 `json:"position"`
}

// reportSeek adds entry for a user performed seek, to generate a heatmap later on
func (r seekStatsRoutes) reportSeek(c *gin.Context) {
	var req reportSeekRequest
	if err := c.BindJSON(&req); err != nil {
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}

	if err := r.VideoSeekDao.Add(c.Param("streamID"), req.Position); err != nil {
		log.WithError(err).Error("Could not add seek hit")
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
}

const (
	minTotalHits     = 150
	minNonZeroChunks = 50
)

// getSeek get seeks for a video
func (r seekStatsRoutes) getSeek(c *gin.Context) {
	chunks, err := r.VideoSeekDao.Get(c.Param("streamID"))

	if err != nil {
		log.WithError(err).Error("Could not get seek hits")
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}

	var values []gin.H

	totalHits := 0
	nonZeroChunks := 0
	for _, chunk := range chunks {
		values = append(values, gin.H{
			"index": chunk.ChunkIndex,
			"value": chunk.Hits,
		})

		if chunk.Hits > 0 {
			totalHits += int(chunk.Hits)
			nonZeroChunks += 1
		}
	}

	if totalHits < minTotalHits || nonZeroChunks < minNonZeroChunks {
		c.JSON(http.StatusOK, gin.H{
			"values": []gin.H{},
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"values": values,
	})
}
