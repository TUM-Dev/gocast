package api

import (
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/joschahenningsen/TUM-Live/dao"
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
		HandleError(c, Error{
			Status:  http.StatusBadRequest,
			Message: "query parameter 'q' missing",
		})
		return
	}

	courseId, err := strconv.Atoi(courseIdS)
	if err != nil {
		HandleError(c, Error{
			Status:  http.StatusBadRequest,
			Message: "query parameter 'q' missing",
		})
		return
	}

	startTime := time.Now()
	searchResults, err := r.SearchDao.Search(q, uint(courseId))
	endTime := time.Now()
	if err != nil {
		HandleError(c, Error{
			Status:  http.StatusInternalServerError,
			Error:   err,
			Message: fmt.Sprintf("could not perform search: %s", err.Error()),
		})
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
