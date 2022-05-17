package api

import (
	"github.com/gin-gonic/gin"
	"github.com/joschahenningsen/TUM-Live/dao"
	log "github.com/sirupsen/logrus"
	"net/http"
	"strconv"
	"time"
)

func configGinSearchRouter(router *gin.Engine, daoWrapper dao.DaoWrapper) {
	routes := searchRoutes{daoWrapper}
	router.GET("/api/search/streams", routes.searchStreams)
}

type searchRoutes struct {
	dao.DaoWrapper
}

func (r searchRoutes) searchStreams(c *gin.Context) {
	q, courseIdS := c.Query("q"), c.Query("courseId")
	if q == "" {
		c.AbortWithStatusJSON(http.StatusBadRequest, "query parameter 'q' missing")
		return
	}

	courseId, err := strconv.Atoi(courseIdS)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, "can not parse courseId")
		return
	}

	startTime := time.Now()
	searchResults, err := r.SearchDao.Search(q, uint(courseId))
	endTime := time.Now()
	if err != nil {
		log.WithError(err).Error("could not perform search")
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}

	response := []gin.H{}
	for _, sr := range searchResults {
		sr.Name = sr.GetName()
		response = append(response, gin.H{
			"ID":           sr.ID,
			"name":         sr.Name,
			"friendlyTime": sr.FriendlyTime(),
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"duration": endTime.Sub(startTime).Milliseconds(),
		"results":  response,
	})
}
