package api

import (
	"fmt"
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
		c.AbortWithStatusJSON(http.StatusBadRequest, fmt.Sprintf("'%s' is not a valid courseID", courseIdS))
		return
	}

	startTime := time.Now()
	searchResults, err := r.SearchDao.Search(q, uint(courseId))
	endTime := time.Now()
	if err != nil {
		log.WithError(err).Error("could not perform search")
		c.AbortWithStatusJSON(http.StatusInternalServerError, fmt.Sprintf("could not perform search: %s", err.Error()))
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
